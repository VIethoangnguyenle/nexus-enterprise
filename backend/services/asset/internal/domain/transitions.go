package domain

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/asset"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

func (s *AssetService) TransitionAsset(ctx context.Context, req *pb.TransitionRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, lookupErr(err, "asset")
	}
	if asset.Deleted {
		return nil, conflict("cannot transition deleted asset")
	}

	// Load lifecycle from type
	at, err := s.store.GetType(ctx, asset.TypeID)
	if err != nil {
		return nil, fmt.Errorf("get asset type: %w", err)
	}
	var ld LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, fmt.Errorf("parse lifecycle: %w", err)
	}

	// Find the requested transition
	tr, found := FindTransition(ld, asset.State, req.Action)
	if !found {
		return nil, conflict("no transition %q available from state %q", req.Action, asset.State)
	}

	// Check NGAC permission for the transition
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, tr.NgacPermission); err != nil {
		return nil, err
	}

	// Giving an asset away needs someone to give it to; a bare transition would
	// leave it "assigned" to nobody.
	if req.Action == ActionAssign {
		return nil, invalid("assigning needs a person; use the hand-over")
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
	s.producer.PublishLifecycle(ctx, events.LifecycleEvent{
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
		return nil, fmt.Errorf("read back asset: %w", err)
	}
	return assetToProto(updated), nil
}

func (s *AssetService) GetAvailableTransitions(ctx context.Context, req *pb.GetTransitionsRequest) (*pb.TransitionList, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, lookupErr(err, "asset")
	}
	// An asset's state is read like the asset: on its type's OA.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpRead); err != nil {
		return nil, err
	}

	at, err := s.store.GetType(ctx, asset.TypeID)
	if err != nil {
		return nil, fmt.Errorf("get asset type: %w", err)
	}
	var ld LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, fmt.Errorf("parse lifecycle: %w", err)
	}

	allTransitions := AvailableTransitions(ld, asset.State)
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
	batch, err := policyclient.New(s.policyRead).BatchCheckCaller(ctx, []string{asset.TypeOAID}, ops)
	if err != nil {
		return nil, fmt.Errorf("batch access check: %w", err)
	}
	granted := batch[asset.TypeOAID]
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

func (s *AssetService) GetAssetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.TransitionHistoryList, error) {
	// Check read access
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, lookupErr(err, "asset")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, asset.TypeOAID, ngac.OpRead); err != nil {
		return nil, err
	}

	records, err := s.store.GetAssetHistory(ctx, req.AssetId)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
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
func (s *AssetService) HandOverAsset(ctx context.Context, req *pb.HandOverRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil || asset.Deleted {
		return nil, notFound("asset not found")
	}
	who := grpcauth.CallerFrom(ctx)
	if err := s.checkAccess(ctx, who.NGACNodeID, asset.TypeOAID, ngac.OpManage); err != nil {
		return nil, err
	}
	if req.AssigneeId == "" {
		return nil, invalid("assignee_id is required")
	}
	if err := s.store.HandOver(ctx, req.AssetId, req.AssigneeId, who.UserID, req.Comment); err != nil {
		return nil, storeErr(err, "hand over")
	}
	s.producer.PublishAssignment(ctx, events.AssignmentEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		FromUserID:  derefString(asset.AssignedTo),
		ToUserID:    req.AssigneeId,
		Action:      ActionAssign,
		ActorID:     who.UserID,
		WorkspaceID: asset.WorkspaceID,
	})
	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, fmt.Errorf("read back asset: %w", err)
	}
	return assetToProto(updated), nil
}
