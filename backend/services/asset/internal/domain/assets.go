package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

// AssetService handles gRPC calls for asset CRUD and lifecycle management.
//
// It holds a read client only. An asset is not a node: the graph holds
// attributes, and every check on an asset is a check on the OA of its type
// (store.Asset.TypeOAID). There is nothing here for a write client to write.
type AssetService struct {
	store      *store.Store
	policyRead policypb.PolicyReadServiceClient
	producer   events.Publisher
}

// NewAssetService creates the asset gRPC handler.
func NewAssetService(s *store.Store, pr policypb.PolicyReadServiceClient, p events.Publisher) *AssetService {
	return &AssetService{store: s, policyRead: pr, producer: orDiscard(p)}
}

func (s *AssetService) CreateAsset(ctx context.Context, req *pb.CreateAssetRequest) (*pb.Asset, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.TypeId == "" || req.WorkspaceId == "" {
		return nil, invalid("name, type_id, and workspace_id are required")
	}
	if utf8.RuneCountInString(req.Name) > MaxNameRunes {
		return nil, invalid("name is too long")
	}

	// Fetch type for schema validation and lifecycle initial state
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, lookupErr(err, "asset type")
	}

	// Check write permission on type's OA
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, at.NgacOAID, ngac.OpWrite); err != nil {
		return nil, err
	}

	// The asset belongs to its type's workspace; the request may not file it
	// under another. Checked after authorization so a caller without write on
	// the type learns nothing about which workspace it is in.
	if at.WorkspaceID != req.WorkspaceId {
		return nil, invalid("asset type does not belong to this workspace")
	}

	// Validate custom fields against type schema
	fieldsJSON := json.RawMessage("{}")
	if req.CustomFields != nil {
		b, err := req.CustomFields.MarshalJSON()
		if err != nil {
			return nil, invalid("invalid custom_fields: %v", err)
		}
		fieldsJSON = b
	}
	if err := ValidateCustomFields(at.FieldsSchema, fieldsJSON); err != nil {
		return nil, invalid("field validation failed: %v", err)
	}
	if err := s.checkPeople(ctx, at.WorkspaceID, at.FieldsSchema, fieldsJSON); err != nil {
		return nil, err
	}

	// Get initial state from lifecycle
	var ld LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, fmt.Errorf("parse lifecycle: %w", err)
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
		return nil, fmt.Errorf("create asset: %w", err)
	}

	// Re-read for complete data (type_name, etc.)
	created, err := s.store.GetAsset(ctx, asset.ID)
	if err != nil {
		return nil, fmt.Errorf("read back asset: %w", err)
	}
	return assetToProto(created), nil
}

func (s *AssetService) GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, lookupErr(err, "asset")
	}
	if asset.Deleted {
		return nil, notFound("asset has been deleted")
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
func (s *AssetService) ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.AssetList, error) {
	types, err := s.store.ListTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, fmt.Errorf("list asset types: %w", err)
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
		return nil, fmt.Errorf("batch access check: %w", err)
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
		return nil, fmt.Errorf("list assets: %w", err)
	}

	result := &pb.AssetList{Total: total}
	for _, a := range assets {
		result.Assets = append(result.Assets, assetToProto(a))
	}
	return result, nil
}

func (s *AssetService) UpdateAsset(ctx context.Context, req *pb.UpdateAssetRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, lookupErr(err, "asset")
	}

	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpWrite); err != nil {
		return nil, err
	}

	req.Name = strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(req.Name) > MaxNameRunes {
		return nil, invalid("name is too long")
	}
	fieldsJSON := asset.CustomFields
	if req.CustomFields != nil {
		b, err := req.CustomFields.MarshalJSON()
		if err != nil {
			return nil, invalid("invalid custom_fields: %v", err)
		}
		fieldsJSON = b

		// Validate against type schema
		at, err := s.store.GetType(ctx, asset.TypeID)
		if err != nil {
			return nil, fmt.Errorf("get type for validation: %w", err)
		}
		if err := ValidateCustomFields(at.FieldsSchema, fieldsJSON); err != nil {
			return nil, invalid("field validation failed: %v", err)
		}
		if err := s.checkPeople(ctx, asset.WorkspaceID, at.FieldsSchema, fieldsJSON); err != nil {
			return nil, err
		}
	}

	if err := s.store.UpdateAsset(ctx, req.AssetId, req.Name, fieldsJSON); err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}

	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, fmt.Errorf("read back asset: %w", err)
	}
	return assetToProto(updated), nil
}

func (s *AssetService) DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.Empty, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, lookupErr(err, "asset")
	}

	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpManage); err != nil {
		return nil, err
	}

	if err := s.store.SoftDeleteAsset(ctx, req.AssetId); err != nil {
		return nil, fmt.Errorf("delete asset: %w", err)
	}

	return &pb.Empty{}, nil
}

// ============================================
// Lifecycle Operations
// ============================================

// MaxNameRunes bounds an asset's name.
const MaxNameRunes = 120

// checkPeople requires every person named in a person-kind field to be an
// active member of the workspace the asset type belongs to, so a field cannot
// point at another tenant's user.
func (s *AssetService) checkPeople(ctx context.Context, workspaceID string, schema, fields json.RawMessage) error {
	ok, err := s.store.AllActiveMembers(ctx, workspaceID, PersonValues(schema, fields))
	if err != nil {
		return errors.New("check people")
	}
	if !ok {
		return Refuse(ErrInvalidInput, ReasonNotAMember, "a person in the fields is not a member of this workspace")
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
func (s *AssetService) checkAccess(ctx context.Context, userNodeID, oaID, operation string) error {
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

// orDiscard turns a missing publisher into one that discards, so a server built
// without Kafka (or in a test) never has to check before announcing.
func orDiscard(p events.Publisher) events.Publisher {
	if p == nil {
		return (*events.Producer)(nil)
	}
	return p
}
