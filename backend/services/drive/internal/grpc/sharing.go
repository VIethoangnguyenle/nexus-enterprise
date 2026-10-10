package grpc

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// CreateShare creates an NGAC association to share a file or folder.
//
// req.Operations carries exactly one share permission ("read" or "write", see
// ngac.ShareOps), never operation names: the operations a share grants are
// decided here, so a caller cannot hand out rights such as manage or share.
func (s *DriveServer) CreateShare(ctx context.Context, req *pb.CreateShareRequest) (*pb.ShareInfo, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	// A trashed item cannot be shared: the grant would outlive the trash view
	// and surface again on restore.
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	// Sharing hands the item to someone else, which is what the share right is
	// for. Write is not enough: it lets a member edit, not widen who can.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpShare); err != nil {
		return nil, err
	}

	// Validate everything that can be refused before the first policy write.
	if len(req.Operations) != 1 {
		return nil, status.Errorf(codes.InvalidArgument, "share permission must be one of: read, write")
	}
	ops, ok := ngac.ShareOps(req.Operations[0])
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid share permission: %q", req.Operations[0])
	}
	targetUA, targetLabel, err := s.resolveShareTarget(ctx, req)
	if err != nil {
		return nil, err
	}

	// The share's ID is chosen first so the OA can be named by it: a name taken
	// from the item would be the same for two items called "report.pdf", in this
	// workspace or another, and the graph resolves nodes by exact name.
	shareID := uuid.New().String()

	// Create a Share OA wrapping the item. From here on a failure must not leave
	// the share OA, its assignments or a half-built association behind; deleting
	// the node cascades all of them.
	prov := provision.NewCreator(s.policyWrite)
	shareOA, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.ShareOAName(ngac.ShareID(shareID)), NodeType: ngac.TypeOA,
		Properties: map[string]string{ngac.PropDisplayName: item.Name, "workspace_id": item.WorkspaceID},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create share OA: %v", err)
	}
	fail := func(err error) (*pb.ShareInfo, error) {
		if rbErr := prov.Fail(ctx, err); rbErr != nil && rbErr != err {
			slog.Error("could not remove share OA after a failed share", "share_oa", shareOA.Id, "error", rbErr)
		}
		return nil, err
	}

	// Assign item under share OA
	if err := prov.Assign(ctx, item.NGACNodeID, shareOA.Id); err != nil {
		return fail(status.Errorf(codes.Internal, "assign item under share OA: %v", err))
	}

	// Assign share OA under PC_Global
	pcGlobal, _ := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePCGlobal, NodeType: ngac.TypePC,
	})
	if pcGlobal != nil {
		if err := prov.Assign(ctx, shareOA.Id, pcGlobal.Id); err != nil {
			return fail(status.Errorf(codes.Internal, "assign share OA under PC_Global: %v", err))
		}
	}

	// Create association. This is the share: if it fails we must not write the
	// DB row, or the UI shows a share that grants nothing.
	if err := prov.Associate(ctx, targetUA, shareOA.Id, ops); err != nil {
		return fail(status.Errorf(codes.Internal, "create share association: %v", err))
	}

	share := &store.DriveShare{
		ID:           shareID,
		DriveItemID:  req.ItemId,
		ShareType:    req.ShareType,
		TargetNGACID: nilStr(req.TargetNgacNodeId),
		TargetLabel:  &targetLabel,
		Operations:   ops,
		NGACShareOA:  shareOA.Id,
		CreatedBy:    grpcauth.CallerFrom(ctx).NGACNodeID,
	}
	if err := s.store.InsertShare(ctx, share); err != nil {
		return fail(status.Errorf(codes.Internal, "insert share: %v", err))
	}
	prov.Done()

	slog.Info("share created", "item", item.Name, "type", req.ShareType, "target", targetLabel)
	return &pb.ShareInfo{
		Id: share.ID, DriveItemId: req.ItemId, ShareType: req.ShareType,
		TargetNgacId: req.TargetNgacNodeId, TargetLabel: targetLabel,
		Operations: ops, CreatedAt: timestamppb.Now(),
	}, nil
}

// resolveShareTarget returns the UA a share's association will start from and
// the label to show for it. It writes nothing for roles, workspaces and public
// links; for a person it may create that person's personal UA (see
// personalUA), which is harmless to leave if the share then fails.
func (s *DriveServer) resolveShareTarget(ctx context.Context, req *pb.CreateShareRequest) (ua, label string, err error) {
	switch req.ShareType {
	case "public":
		pubUA, _ := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
			Name: ngac.NodePublicUsers, NodeType: ngac.TypeUA,
		})
		if pubUA == nil {
			return "", "", status.Errorf(codes.Internal, "PublicUsers UA not found")
		}
		return pubUA.Id, "Anyone with link", nil
	case "user", "role", "workspace":
		if req.TargetNgacNodeId == "" {
			return "", "", status.Errorf(codes.InvalidArgument, "share target is required")
		}
		node, gerr := s.policyRead.GetNode(ctx, &policypb.GetNodeRequest{NodeId: req.TargetNgacNodeId})
		if gerr != nil || node == nil || node.GetId() == "" {
			return "", "", status.Errorf(codes.InvalidArgument, "share target not found")
		}
		label = ngac.DisplayName(node.Name, node.Properties)
		if req.ShareType == "workspace" {
			label += " (workspace)"
		}
		switch node.NodeType {
		case ngac.TypeUA:
			return node.Id, label, nil
		case ngac.TypeU:
			// Only a person can be the target of a "user" share. For other
			// types a U node is the wrong kind of target.
			if req.ShareType != "user" {
				return "", "", status.Errorf(codes.InvalidArgument, "share target must be a group")
			}
			ua, err = s.personalUA(ctx, node)
			return ua, label, err
		}
		return "", "", status.Errorf(codes.InvalidArgument, "share target must be a person or a group")
	}
	return "", "", status.Errorf(codes.InvalidArgument, "invalid share_type: %s", req.ShareType)
}

// personalUA returns the user attribute that contains exactly this user,
// creating it on first use. An association can only start from a UA, never
// from a user node, so granting an item to one person goes through this
// attribute.
//
// A node is trusted as the person's UA only by what it is, never by its name:
// its properties must mark it as the personal UA of this user (see
// ngac.IsPersonalUAOf), and the user must be inside it. Role names are chosen
// by workspace administrators, so a role called "User_<id>" would otherwise
// receive every share made to that person. At most one such UA exists per user
// (unique index), which makes a lost creation race resolvable by looking again.
func (s *DriveServer) personalUA(ctx context.Context, user *policypb.NGACNode) (string, error) {
	if id, err := s.findPersonalUA(ctx, user); err != nil || id != "" {
		return id, err
	}

	prov := provision.NewCreator(s.policyWrite)
	ua, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.PersonalUAName(ngac.UserNodeID(user.Id)), NodeType: ngac.TypeUA,
		Properties: ngac.PersonalUAProperties(user.Id),
	})
	if err != nil {
		// Most likely a concurrent first share created it between our lookup
		// and our create; give it a moment to finish assigning, then look again.
		for i := 0; i < 3; i++ {
			time.Sleep(50 * time.Millisecond)
			if id, ferr := s.findPersonalUA(ctx, user); ferr == nil && id != "" {
				return id, nil
			}
		}
		return "", status.Errorf(codes.Internal, "create personal UA: %v", err)
	}
	if err := s.assignToPersonalUA(ctx, user.Id, ua.Id); err != nil {
		if rbErr := prov.Fail(ctx, err); rbErr != err {
			slog.Error("could not remove personal UA after a failed assignment", "ua", ua.Id, "error", rbErr)
		}
		return "", err
	}
	prov.Done()
	return ua.Id, nil
}

func (s *DriveServer) assignToPersonalUA(ctx context.Context, userID, uaID string) error {
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: userID, ParentId: uaID,
	}); err != nil {
		return status.Errorf(codes.Internal, "assign user to personal UA: %v", err)
	}
	return nil
}

// findPersonalUA looks for the user's personal UA: first among the attributes
// the user is already in, then by name for one that was created but not yet
// assigned (adopting it only if its properties say it is this user's). A node
// that merely has the right name is never returned. "" means none exists.
func (s *DriveServer) findPersonalUA(ctx context.Context, user *policypb.NGACNode) (string, error) {
	anc, err := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: user.Id})
	if err != nil {
		return "", status.Errorf(codes.Internal, "look up personal UA: %v", err)
	}
	for _, n := range anc.GetNodes() {
		if n.GetNodeType() == ngac.TypeUA && ngac.IsPersonalUAOf(n.GetProperties(), user.Id) {
			return n.Id, nil
		}
	}

	byName, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.PersonalUAName(ngac.UserNodeID(user.Id)), NodeType: ngac.TypeUA,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return "", status.Errorf(codes.Internal, "look up personal UA: %v", err)
	}
	if err == nil && byName.GetId() != "" && ngac.IsPersonalUAOf(byName.GetProperties(), user.Id) {
		if aerr := s.assignToPersonalUA(ctx, user.Id, byName.Id); aerr != nil {
			return "", aerr
		}
		return byName.Id, nil
	}
	return "", nil
}

// RevokeShare removes a share.
func (s *DriveServer) RevokeShare(ctx context.Context, req *pb.RevokeShareRequest) (*pb.Empty, error) {
	share, err := s.store.GetShare(ctx, req.ShareId)
	if err != nil || share == nil {
		return nil, status.Errorf(codes.NotFound, "share not found")
	}
	// Revoking changes who can reach the item. It is allowed for:
	//
	//   - the user who created this share. They must be able to withdraw what
	//     they granted even if their share right has since been taken away.
	//     Revoking only narrows access, so this cannot be used to widen anyone's
	//     rights. created_by holds the creator's NGAC node; an empty value
	//     never matches.
	//   - anyone holding share on the OA the item row points at. Without an
	//     item to authorize against nothing could grant that right, so a
	//     missing item denies.
	isCreator := grpcauth.CallerFrom(ctx).NGACNodeID != "" && share.CreatedBy == grpcauth.CallerFrom(ctx).NGACNodeID
	if !isCreator {
		item, err := s.store.GetItem(ctx, share.DriveItemID)
		if err != nil || item == nil {
			return nil, status.Errorf(codes.PermissionDenied, "access denied")
		}
		if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpShare); err != nil {
			return nil, err
		}
	}
	// Delete the NGAC share OA (cascades associations). This is what actually
	// revokes access — the DB row is only bookkeeping. If it fails we must not
	// delete the row and report success, or the share disappears from the UI
	// while the association keeps granting access to the recipient.
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: share.NGACShareOA}); err != nil {
		return nil, status.Errorf(codes.Internal, "revoke share OA: %v", err)
	}
	if err := s.store.DeleteShare(ctx, req.ShareId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete share record: %v", err)
	}
	return &pb.Empty{}, nil
}

// ListShares returns all shares for an item.
func (s *DriveServer) ListShares(ctx context.Context, req *pb.ListSharesRequest) (*pb.ShareList, error) {
	// Who an item is shared with is information about that item.
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

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
		NodeId: grpcauth.CallerFrom(ctx).NGACNodeID,
	})
	targetIDs := []string{grpcauth.CallerFrom(ctx).NGACNodeID}
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
//
// Safe to call again for the same channel: an existing drive is returned as is,
// and an OA left by an earlier attempt that did not finish is reused. A failure
// part way removes the OA this call created.
func (s *DriveServer) CreateDriveForChannel(ctx context.Context, req *pb.CreateDriveForChannelRequest) (*pb.DriveItem, error) {
	if req.ChannelId == "" || req.WorkspaceId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "channel_id and workspace_id are required")
	}
	// The channel must be one of this workspace's (or the workspace itself, which
	// the workspace service registers as the context of its own root drive). The
	// OA below is found by the channel's ID alone, so without this a request that
	// paired workspace B with workspace A's channel would adopt A's drive and
	// grant B's channel UA access to it.
	if req.ChannelId != req.WorkspaceId {
		chWS, err := s.store.GetChannelWorkspaceID(ctx, req.ChannelId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "look up channel: %v", err)
		}
		if chWS != req.WorkspaceId {
			return nil, status.Errorf(codes.PermissionDenied, "channel does not belong to this workspace")
		}
	}
	if existing, err := s.store.FindRootByContext(ctx, req.WorkspaceId, "channel", req.ChannelId); err != nil {
		return nil, status.Errorf(codes.Internal, "find channel drive: %v", err)
	} else if existing != nil {
		return itemToProto(existing), nil
	}

	// The OA is named by the channel's ID, never its name: two channels may
	// share a name, in one workspace or across tenants.
	driveName := ngac.ChannelDriveName(ngac.ChannelID(req.ChannelId))
	displayName := req.ChannelName
	if displayName == "" {
		displayName = "Drive"
	}

	prov := provision.NewCreator(s.policyWrite)
	driveOA, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: driveName, NodeType: ngac.TypeOA,
		Properties: map[string]string{
			"type": "channel_drive", "channel_id": req.ChannelId, "workspace_id": req.WorkspaceId,
			ngac.PropDisplayName: displayName,
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create channel drive OA: %v", err)
	}
	// An OA found by name must not be another workspace's drive root.
	if owner, oerr := s.store.RootWorkspaceByNode(ctx, driveOA.Id); oerr != nil {
		return nil, status.Errorf(codes.Internal, "check drive owner: %v", prov.Fail(ctx, oerr))
	} else if (owner != "" && owner != req.WorkspaceId) ||
		(driveOA.Properties["workspace_id"] != "" && driveOA.Properties["workspace_id"] != req.WorkspaceId) {
		_ = prov.Fail(ctx, errors.New("drive belongs to another workspace"))
		return nil, status.Errorf(codes.PermissionDenied, "channel drive belongs to another workspace")
	}

	// Assign under channel's Content OA (inherits channel permissions)
	if err := prov.Assign(ctx, driveOA.Id, req.ChannelNgacOaId); err != nil {
		return nil, status.Errorf(codes.Internal, "assign channel drive under content OA: %v", prov.Fail(ctx, err))
	}

	// Association: channel Members UA → drive OA [read, write, upload, share]
	if err := prov.Associate(ctx, req.ChannelNgacUaId, driveOA.Id, ngac.ChannelDriveOps()); err != nil {
		return nil, status.Errorf(codes.Internal, "grant channel members access to drive: %v", prov.Fail(ctx, err))
	}

	item := &store.DriveItem{
		ID: uuid.New().String(), WorkspaceID: req.WorkspaceId,
		DriveContext: "channel", DriveContextID: req.ChannelId,
		ItemType: "folder", Name: displayName,
		NGACNodeID: driveOA.Id, ScopeOAID: driveOA.Id,
		OwnerID: "system", Status: "active", IsRoot: true,
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		rbErr := prov.Fail(ctx, err)
		if errors.Is(err, store.ErrRootExists) {
			if winner, ferr := s.store.FindRootByContext(ctx, req.WorkspaceId, "channel", req.ChannelId); ferr == nil && winner != nil {
				return itemToProto(winner), nil
			}
		}
		return nil, status.Errorf(codes.Internal, "insert channel drive: %v", rbErr)
	}
	prov.Done()

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
	// Storage consumption describes the workspace, so reading it requires
	// reaching that workspace's drive rather than merely holding a valid token.
	root, err := s.ensureRoot(ctx, req.WorkspaceId, "workspace", "", grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, root.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

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
//
// Quota limits are workspace administration, so this takes manage on the
// workspace's Mgmt OA, for the caller on the context (see package grpcauth).
func (s *DriveServer) UpdateQuota(ctx context.Context, req *pb.UpdateQuotaRequest) (*pb.Quota, error) {
	userNodeID := grpcauth.CallerFrom(ctx).NGACNodeID
	if err := s.checkAccessOnNamedOA(ctx, userNodeID, ngac.MgmtOAName(ngac.WorkspaceID(req.WorkspaceId)), ngac.OpManage); err != nil {
		return nil, err
	}
	if err := s.store.UpdateQuotaLimits(ctx, req.WorkspaceId, req.MaxBytes, req.MaxFiles); err != nil {
		return nil, status.Errorf(codes.Internal, "update quota: %v", err)
	}
	return s.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: req.WorkspaceId})
}
