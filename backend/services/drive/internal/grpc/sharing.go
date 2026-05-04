package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// CreateShare creates an NGAC association to share a file or folder.
func (s *DriveServer) CreateShare(ctx context.Context, req *pb.CreateShareRequest) (*pb.ShareInfo, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, req.UserNgacNodeId, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	// Create a Share OA wrapping the item
	shareOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.ShareOAName(item.Name, uuid.New().String()[:8]), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create share OA: %v", err)
	}

	// Assign item under share OA
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: item.NGACNodeID, ParentId: shareOA.Id,
	})

	// Assign share OA under PC_Global
	pcGlobal, _ := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePCGlobal, NodeType: ngac.TypePC,
	})
	if pcGlobal != nil {
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: shareOA.Id, ParentId: pcGlobal.Id,
		})
	}

	// Determine target UA
	var targetUA string
	var targetLabel string
	switch req.ShareType {
	case "public":
		pubUA, _ := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
			Name: ngac.NodePublicUsers, NodeType: ngac.TypeUA,
		})
		if pubUA == nil {
			return nil, status.Errorf(codes.Internal, "PublicUsers UA not found")
		}
		targetUA = pubUA.Id
		targetLabel = "Anyone with link"
	case "user", "role":
		targetUA = req.TargetNgacNodeId
		node, _ := s.policyRead.GetNode(ctx, &policypb.GetNodeRequest{NodeId: targetUA})
		if node != nil {
			targetLabel = node.Name
		}
	case "workspace":
		targetUA = req.TargetNgacNodeId
		node, _ := s.policyRead.GetNode(ctx, &policypb.GetNodeRequest{NodeId: targetUA})
		if node != nil {
			targetLabel = node.Name + " (workspace)"
		}
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid share_type: %s", req.ShareType)
	}

	// Create association
	s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: targetUA, OaId: shareOA.Id, Operations: req.Operations,
	})

	share := &store.DriveShare{
		ID:           uuid.New().String(),
		DriveItemID:  req.ItemId,
		ShareType:    req.ShareType,
		TargetNGACID: nilStr(req.TargetNgacNodeId),
		TargetLabel:  &targetLabel,
		Operations:   req.Operations,
		NGACShareOA:  shareOA.Id,
		CreatedBy:    req.UserNgacNodeId,
	}
	if err := s.store.InsertShare(ctx, share); err != nil {
		return nil, status.Errorf(codes.Internal, "insert share: %v", err)
	}

	slog.Info("share created", "item", item.Name, "type", req.ShareType, "target", targetLabel)
	return &pb.ShareInfo{
		Id: share.ID, DriveItemId: req.ItemId, ShareType: req.ShareType,
		TargetNgacId: req.TargetNgacNodeId, TargetLabel: targetLabel,
		Operations: req.Operations, CreatedAt: timestamppb.Now(),
	}, nil
}

// RevokeShare removes a share.
func (s *DriveServer) RevokeShare(ctx context.Context, req *pb.RevokeShareRequest) (*pb.Empty, error) {
	share, err := s.store.GetShare(ctx, req.ShareId)
	if err != nil || share == nil {
		return nil, status.Errorf(codes.NotFound, "share not found")
	}
	// Delete the NGAC share OA (cascades associations)
	s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: share.NGACShareOA})
	s.store.DeleteShare(ctx, req.ShareId)
	return &pb.Empty{}, nil
}

// ListShares returns all shares for an item.
func (s *DriveServer) ListShares(ctx context.Context, req *pb.ListSharesRequest) (*pb.ShareList, error) {
	shares, err := s.store.ListSharesByItem(ctx, req.ItemId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list shares: %v", err)
	}
	var result []*pb.ShareInfo
	for _, sh := range shares {
		info := &pb.ShareInfo{
			Id: sh.ID, DriveItemId: sh.DriveItemID, ShareType: sh.ShareType,
			Operations: sh.Operations, CreatedAt: timestamppb.New(sh.CreatedAt),
		}
		if sh.TargetNGACID != nil {
			info.TargetNgacId = *sh.TargetNGACID
		}
		if sh.TargetLabel != nil {
			info.TargetLabel = *sh.TargetLabel
		}
		result = append(result, info)
	}
	return &pb.ShareList{Shares: result}, nil
}

// GetSharedWithMe returns items shared with the current user.
func (s *DriveServer) GetSharedWithMe(ctx context.Context, req *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error) {
	// Find all UAs the user belongs to
	ancestors, _ := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{
		NodeId: req.UserNgacNodeId,
	})
	targetIDs := []string{req.UserNgacNodeId}
	if ancestors != nil {
		for _, n := range ancestors.Nodes {
			if n.NodeType == ngac.TypeUA {
				targetIDs = append(targetIDs, n.Id)
			}
		}
	}

	shares, err := s.store.ListSharesByTarget(ctx, targetIDs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list shared: %v", err)
	}

	seen := make(map[string]bool)
	var items []*pb.DriveItem
	for _, sh := range shares {
		if seen[sh.DriveItemID] {
			continue
		}
		seen[sh.DriveItemID] = true
		item, _ := s.store.GetItem(ctx, sh.DriveItemID)
		if item != nil {
			items = append(items, itemToProto(item))
		}
	}
	return &pb.DriveItemList{Items: items}, nil
}

// CreateDriveForChannel creates a channel/DM drive folder with NGAC OA.
func (s *DriveServer) CreateDriveForChannel(ctx context.Context, req *pb.CreateDriveForChannelRequest) (*pb.DriveItem, error) {
	driveName := fmt.Sprintf("Ch_%s_Drive", req.ChannelName)

	// Create NGAC OA for channel drive
	driveOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: driveName, NodeType: ngac.TypeOA,
		Properties: map[string]string{"type": "channel_drive", "channel_id": req.ChannelId},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create channel drive OA: %v", err)
	}

	// Assign under channel's Content OA (inherits channel permissions)
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: driveOA.Id, ParentId: req.ChannelNgacOaId,
	})

	// Association: channel Members UA → drive OA [read, write, upload]
	s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: req.ChannelNgacUaId, OaId: driveOA.Id,
		Operations: ngac.ChannelDriveOps(),
	})

	item := &store.DriveItem{
		ID: uuid.New().String(), WorkspaceID: req.WorkspaceId,
		DriveContext: "channel", DriveContextID: req.ChannelId,
		ItemType: "folder", Name: driveName,
		NGACNodeID: driveOA.Id, ScopeOAID: driveOA.Id,
		OwnerID: "system", Status: "active",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "insert channel drive: %v", err)
	}

	slog.Info("channel drive created", "channel", req.ChannelId, "drive", item.ID)
	return itemToProto(item), nil
}

// GetChannelDrive returns the root folder of a channel's drive.
// Returns nil if channel drive doesn't exist yet (lazy creation handled by Gateway).
func (s *DriveServer) GetChannelDrive(ctx context.Context, req *pb.GetChannelDriveRequest) (*pb.DriveItem, error) {
	wsID, _ := s.store.GetChannelWorkspaceID(ctx, req.ChannelId)
	if wsID == "" {
		return nil, status.Errorf(codes.NotFound, "channel not found")
	}

	root, err := s.store.FindRootByContext(ctx, wsID, "channel", req.ChannelId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find channel drive: %v", err)
	}
	if root == nil {
		// Not an error — channel just doesn't have a drive yet
		return &pb.DriveItem{}, nil
	}
	return itemToProto(root), nil
}

// GetQuota returns workspace storage quota.
func (s *DriveServer) GetQuota(ctx context.Context, req *pb.GetQuotaRequest) (*pb.Quota, error) {
	q, err := s.store.GetOrCreateQuota(ctx, req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get quota: %v", err)
	}
	return &pb.Quota{
		WorkspaceId: q.WorkspaceID, MaxBytes: q.MaxBytes, UsedBytes: q.UsedBytes,
		MaxFiles: q.MaxFiles, UsedFiles: q.UsedFiles,
	}, nil
}

// UpdateQuota sets workspace quota limits.
func (s *DriveServer) UpdateQuota(ctx context.Context, req *pb.UpdateQuotaRequest) (*pb.Quota, error) {
	if err := s.store.UpdateQuotaLimits(ctx, req.WorkspaceId, req.MaxBytes, req.MaxFiles); err != nil {
		return nil, status.Errorf(codes.Internal, "update quota: %v", err)
	}
	return s.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: req.WorkspaceId})
}
