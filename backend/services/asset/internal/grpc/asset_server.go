package grpc

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

// AssetServer handles gRPC calls for asset CRUD and lifecycle management.
//
// It holds a read client only. An asset is not a node: the graph holds
// attributes, and every check on an asset is a check on the OA of its type
// (store.Asset.TypeOAID). There is nothing here for a write client to write.
type AssetServer struct {
	pb.UnimplementedAssetServiceServer
	store      *store.Store
	policyRead policypb.PolicyReadServiceClient
	producer   *events.Producer
}

// NewAssetServer creates the asset gRPC handler.
func NewAssetServer(s *store.Store, pr policypb.PolicyReadServiceClient, p *events.Producer) *AssetServer {
	return &AssetServer{store: s, policyRead: pr, producer: p}
}

func (s *AssetServer) CreateAsset(ctx context.Context, req *pb.CreateAssetRequest) (*pb.Asset, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.TypeId == "" || req.WorkspaceId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name, type_id, and workspace_id are required")
	}
	if utf8.RuneCountInString(req.Name) > MaxNameRunes {
		return nil, status.Errorf(codes.InvalidArgument, "name is too long")
	}

	// Fetch type for schema validation and lifecycle initial state
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset type not found: %v", err)
	}

	// Check write permission on type's OA
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, at.NgacOAID, ngac.OpWrite); err != nil {
		return nil, err
	}

	// The asset belongs to its type's workspace; the request may not file it
	// under another. Checked after authorization so a caller without write on
	// the type learns nothing about which workspace it is in.
	if at.WorkspaceID != req.WorkspaceId {
		return nil, status.Errorf(codes.InvalidArgument, "asset type does not belong to this workspace")
	}

	// Validate custom fields against type schema
	fieldsJSON := json.RawMessage("{}")
	if req.CustomFields != nil {
		b, err := req.CustomFields.MarshalJSON()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid custom_fields: %v", err)
		}
		fieldsJSON = b
	}
	if err := domain.ValidateCustomFields(at.FieldsSchema, fieldsJSON); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "field validation failed: %v", err)
	}
	if err := s.checkPeople(ctx, at.WorkspaceID, at.FieldsSchema, fieldsJSON); err != nil {
		return nil, err
	}

	// Get initial state from lifecycle
	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, status.Errorf(codes.Internal, "parse lifecycle: %v", err)
	}

	// No graph node is created for the asset. It is authorized through the OA of
	// its type, which the check above already used; two assets may share a name
	// and still share nothing but that type's grants.
	asset := &store.Asset{
		Name:         req.Name,
		TypeID:       req.TypeId,
		WorkspaceID:  req.WorkspaceId,
		State:        ld.InitialState,
		CustomFields: fieldsJSON,
		CreatedBy:    grpcauth.CallerFrom(ctx).UserID,
	}
	if err := s.store.CreateAsset(ctx, asset); err != nil {
		return nil, status.Errorf(codes.Internal, "create asset: %v", err)
	}

	// Re-read for complete data (type_name, etc.)
	created, err := s.store.GetAsset(ctx, asset.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(created), nil
}

func (s *AssetServer) GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if asset.Deleted {
		return nil, status.Errorf(codes.NotFound, "asset has been deleted")
	}

	// Check read permission on the OA of the asset's type
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpRead); err != nil {
		return nil, err
	}
	return assetToProto(asset), nil
}

// ListAssets returns the workspace's assets the caller may read.
//
// Read is decided per asset type, on the type OA: every asset of a type hangs
// under that OA, and a grant on the Assets or category OA reaches it too. The
// readable types are resolved in one batch call and pushed into the query, so
// pagination and the total count cover only what the caller can see — a
// post-query filter would still report how many hidden assets exist.
func (s *AssetServer) ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.AssetList, error) {
	types, err := s.store.ListTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list asset types: %v", err)
	}
	if req.TypeId != "" {
		var only []*store.AssetType
		for _, at := range types {
			if at.ID == req.TypeId {
				only = append(only, at)
			}
		}
		types = only
	}
	readable, err := permittedTypeIDs(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, types, ngac.OpRead)
	if err != nil {
		// Fail closed: an unreadable policy answer must not list everything.
		return nil, status.Errorf(codes.Internal, "batch access check: %v", err)
	}
	if len(readable) == 0 {
		return &pb.AssetList{}, nil
	}

	assets, total, err := s.store.ListAssets(ctx, store.ListAssetsFilter{
		WorkspaceID:    req.WorkspaceId,
		TypeID:         req.TypeId,
		State:          req.State,
		AssignedTo:     req.AssignedTo,
		Search:         req.Search,
		Limit:          req.Limit,
		Offset:         req.Offset,
		VisibleTypeIDs: readable,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list assets: %v", err)
	}

	result := &pb.AssetList{Total: total}
	for _, a := range assets {
		result.Assets = append(result.Assets, assetToProto(a))
	}
	return result, nil
}

func (s *AssetServer) UpdateAsset(ctx context.Context, req *pb.UpdateAssetRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpWrite); err != nil {
		return nil, err
	}

	req.Name = strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(req.Name) > MaxNameRunes {
		return nil, status.Errorf(codes.InvalidArgument, "name is too long")
	}
	fieldsJSON := asset.CustomFields
	if req.CustomFields != nil {
		b, err := req.CustomFields.MarshalJSON()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid custom_fields: %v", err)
		}
		fieldsJSON = b

		// Validate against type schema
		at, err := s.store.GetType(ctx, asset.TypeID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get type for validation: %v", err)
		}
		if err := domain.ValidateCustomFields(at.FieldsSchema, fieldsJSON); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "field validation failed: %v", err)
		}
		if err := s.checkPeople(ctx, asset.WorkspaceID, at.FieldsSchema, fieldsJSON); err != nil {
			return nil, err
		}
	}

	if err := s.store.UpdateAsset(ctx, req.AssetId, req.Name, fieldsJSON); err != nil {
		return nil, status.Errorf(codes.Internal, "update asset: %v", err)
	}

	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(updated), nil
}

func (s *AssetServer) DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.Empty, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpManage); err != nil {
		return nil, err
	}

	if err := s.store.SoftDeleteAsset(ctx, req.AssetId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete asset: %v", err)
	}

	return &pb.Empty{}, nil
}

// ============================================
// Lifecycle Operations
// ============================================

func (s *AssetServer) TransitionAsset(ctx context.Context, req *pb.TransitionRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if asset.Deleted {
		return nil, status.Errorf(codes.FailedPrecondition, "cannot transition deleted asset")
	}

	// Load lifecycle from type
	at, err := s.store.GetType(ctx, asset.TypeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get asset type: %v", err)
	}
	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, status.Errorf(codes.Internal, "parse lifecycle: %v", err)
	}

	// Find the requested transition
	tr, found := domain.FindTransition(ld, asset.State, req.Action)
	if !found {
		return nil, status.Errorf(codes.FailedPrecondition,
			"no transition %q available from state %q", req.Action, asset.State)
	}

	// Check NGAC permission for the transition
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, tr.NgacPermission); err != nil {
		return nil, err
	}

	// Giving an asset away needs someone to give it to; a bare transition would
	// leave it "assigned" to nobody.
	if req.Action == domain.ActionAssign {
		return nil, status.Errorf(codes.InvalidArgument, "assigning needs a person; use the hand-over")
	}

	// Change state and record who did it, atomically. The history row is the
	// audit trail, so if it cannot be written the transition fails as a whole
	// rather than taking effect unrecorded — and no event is published.
	if err := s.store.ApplyTransition(ctx, &store.TransitionRecord{
		AssetID:   req.AssetId,
		FromState: asset.State,
		ToState:   tr.ToState,
		Action:    req.Action,
		ActorID:   grpcauth.CallerFrom(ctx).UserID,
		Comment:   req.Comment,
	}, nil); err != nil {
		return nil, storeErr(err, "apply transition")
	}

	// Emit Kafka lifecycle event
	s.producer.PublishLifecycle(events.LifecycleEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		TypeName:    asset.TypeName,
		FromState:   asset.State,
		ToState:     tr.ToState,
		Action:      req.Action,
		ActorID:     grpcauth.CallerFrom(ctx).UserID,
		WorkspaceID: asset.WorkspaceID,
	})

	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(updated), nil
}

func (s *AssetServer) GetAvailableTransitions(ctx context.Context, req *pb.GetTransitionsRequest) (*pb.TransitionList, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	// An asset's state is read like the asset: on its type's OA.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpRead); err != nil {
		return nil, err
	}

	at, err := s.store.GetType(ctx, asset.TypeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get asset type: %v", err)
	}
	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, status.Errorf(codes.Internal, "parse lifecycle: %v", err)
	}

	allTransitions := domain.AvailableTransitions(ld, asset.State)
	result := &pb.TransitionList{CurrentState: asset.State}

	// Filter by NGAC permissions — only show transitions the user can execute
	// All transitions are checked against the same object, so the whole set of
	// permissions can be resolved in one call instead of one per transition.
	ops := []string{ngac.OpManage} // hand-over is asked about whatever the lifecycle offers
	seen := map[string]bool{ngac.OpManage: true}
	for _, t := range allTransitions {
		if !seen[t.NgacPermission] {
			seen[t.NgacPermission] = true
			ops = append(ops, t.NgacPermission)
		}
	}
	batch, err := s.policyRead.BatchCheckAccess(ctx, &policypb.BatchCheckAccessRequest{
		UserNodeId: grpcauth.CallerFrom(ctx).NGACNodeID,
		ObjectIds:  []string{asset.TypeOAID},
		Operations: ops,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "batch access check: %v", err)
	}
	granted := batch.GetResults()[asset.TypeOAID].GetPermissions()
	result.CanAssign = granted[ngac.OpManage] && (asset.State == "available" || asset.State == "assigned")

	for _, t := range allTransitions {
		if granted[t.NgacPermission] {
			result.Transitions = append(result.Transitions, &pb.AvailableTransition{
				Action:         t.Operation,
				ToState:        t.ToState,
				NgacPermission: t.NgacPermission,
			})
		}
	}
	return result, nil
}

func (s *AssetServer) GetAssetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.TransitionHistoryList, error) {
	// Check read access
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpRead); err != nil {
		return nil, err
	}

	records, err := s.store.GetAssetHistory(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get history: %v", err)
	}

	result := &pb.TransitionHistoryList{}
	for _, r := range records {
		result.Records = append(result.Records, &pb.TransitionRecord{
			Id:            r.ID,
			AssetId:       r.AssetID,
			FromState:     r.FromState,
			ToState:       r.ToState,
			Action:        r.Action,
			ActorId:       r.ActorID,
			ActorName:     r.ActorName,
			SubjectUserId: r.SubjectUserID,
			SubjectName:   r.SubjectName,
			Comment:       r.Comment,
			CreatedAt:     timestamppb.New(r.CreatedAt),
		})
	}
	return result, nil
}

// HandOverAsset gives an available asset, or moves an assigned one, to a person
// of the asset's workspace. It needs manage on the OA of the asset's type — the
// same right as any other assignment — and the person must belong to the
// workspace the asset is in.
func (s *AssetServer) HandOverAsset(ctx context.Context, req *pb.HandOverRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil || asset.Deleted {
		return nil, status.Errorf(codes.NotFound, "asset not found")
	}
	who := grpcauth.CallerFrom(ctx)
	if err := s.checkAccess(ctx, who.NGACNodeID, asset.TypeOAID, ngac.OpManage); err != nil {
		return nil, err
	}
	if req.AssigneeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "assignee_id is required")
	}
	if err := s.store.HandOver(ctx, req.AssetId, req.AssigneeId, who.UserID, req.Comment); err != nil {
		return nil, storeErr(err, "hand over")
	}
	s.producer.PublishAssignment(events.AssignmentEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		FromUserID:  derefString(asset.AssignedTo),
		ToUserID:    req.AssigneeId,
		Action:      domain.ActionAssign,
		ActorID:     who.UserID,
		WorkspaceID: asset.WorkspaceID,
	})
	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(updated), nil
}

// readableTypes returns the IDs of the workspace's asset types the caller may
// read, failing closed when the policy service cannot answer.
func (s *AssetServer) readableTypes(ctx context.Context, workspaceID string) ([]string, error) {
	types, err := s.store.ListTypes(ctx, workspaceID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list asset types: %v", err)
	}
	readable, err := permittedTypeIDs(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, types, ngac.OpRead)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "batch access check: %v", err)
	}
	return readable, nil
}

// GetSummary counts the workspace's assets the caller may read.
func (s *AssetServer) GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.AssetSummary, error) {
	readable, err := s.readableTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	sum, err := s.store.Summary(ctx, req.WorkspaceId, readable)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "summary: %v", err)
	}
	out := &pb.AssetSummary{Total: sum.Total, ByState: sum.ByState, Holders: sum.Holders, MaintenanceOverdue: sum.MaintenanceOverdue}
	for _, tc := range sum.ByType {
		out.ByType = append(out.ByType, &pb.TypeCount{TypeId: tc.TypeID, TypeName: tc.TypeName, Count: tc.Count})
	}
	return out, nil
}

// ListActivity returns the newest lifecycle steps on assets the caller may read.
func (s *AssetServer) ListActivity(ctx context.Context, req *pb.ListActivityRequest) (*pb.ActivityList, error) {
	readable, err := s.readableTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListActivity(ctx, req.WorkspaceId, readable, req.Limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "activity: %v", err)
	}
	// Decisions on requests of the same readable types, merged by time.
	decisions, err := s.store.ListRequestDecisions(ctx, req.WorkspaceId, readable, req.Limit)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "activity: %v", err)
	}
	entries = append(entries, decisions...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	limit := int(req.Limit)
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := &pb.ActivityList{}
	for _, e := range entries {
		out.Entries = append(out.Entries, &pb.ActivityEntry{
			Id: e.ID, AssetId: e.AssetID, AssetName: e.AssetName, TypeName: e.TypeName,
			FromState: e.FromState, ToState: e.ToState, Action: e.Action,
			ActorId: e.ActorID, ActorName: e.ActorName,
			SubjectUserId: e.SubjectUserID, SubjectName: e.SubjectName,
			Comment: e.Comment, CreatedAt: timestamppb.New(e.CreatedAt), RequestStatus: e.RequestStatus,
		})
	}
	return out, nil
}

// MaxNameRunes bounds an asset's name.
const MaxNameRunes = 120

// checkPeople requires every person named in a person-kind field to be an
// active member of the workspace the asset type belongs to, so a field cannot
// point at another tenant's user.
func (s *AssetServer) checkPeople(ctx context.Context, workspaceID string, schema, fields json.RawMessage) error {
	ok, err := s.store.AllActiveMembers(ctx, workspaceID, domain.PersonValues(schema, fields))
	if err != nil {
		return status.Errorf(codes.Internal, "check people")
	}
	if !ok {
		return refuse(codes.InvalidArgument, ReasonNotAMember, "a person in the fields is not a member of this workspace")
	}
	return nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ============================================
// Helpers
// ============================================

// checkAccess requires op for the caller on one OA. An empty caller or OA —
// an asset whose type has no OA — is a denial, not a lookup of "".
func (s *AssetServer) checkAccess(ctx context.Context, userNodeID, oaID, operation string) error {
	return authorize(ctx, s.policyRead, userNodeID, oaID, operation)
}

func assetToProto(a *store.Asset) *pb.Asset {
	result := &pb.Asset{
		Id:          a.ID,
		Name:        a.Name,
		TypeId:      a.TypeID,
		TypeName:    a.TypeName,
		WorkspaceId: a.WorkspaceID,
		State:       a.State,
		NgacNodeId:  a.TypeOAID, // the OA the asset is authorized on
		CreatedBy:   a.CreatedBy,
		Deleted:     a.Deleted,
		CreatedAt:   timestamppb.New(a.CreatedAt),
		UpdatedAt:   timestamppb.New(a.UpdatedAt),
	}

	if a.AssignedTo != nil {
		result.AssignedToUserId = *a.AssignedTo
		result.AssignedToUsername = a.AssignedToUsername
		result.AssignedToName = a.AssignedToName
	}

	// Convert custom_fields JSON to protobuf Struct
	if len(a.CustomFields) > 0 {
		var m map[string]any
		if err := json.Unmarshal(a.CustomFields, &m); err == nil {
			if s, err := structpb.NewStruct(m); err == nil {
				result.CustomFields = s
			}
		}
	}
	return result
}
