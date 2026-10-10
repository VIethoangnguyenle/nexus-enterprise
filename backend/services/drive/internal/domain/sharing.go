package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// CreateShare creates an NGAC association to share a file or folder.
//
// req.Operations carries exactly one share permission ("read" or "write", see
// ngac.ShareOps), never operation names: the operations a share grants are
// decided here, so a caller cannot hand out rights such as manage or share.
func (s *Service) CreateShare(ctx context.Context, req *pb.CreateShareRequest) (*pb.ShareInfo, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	// A trashed item cannot be shared: the grant would outlive the trash view
	// and surface again on restore.
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, notFound("item not found")
	}
	// Sharing hands the item to someone else, which is what the share right is
	// for. Write is not enough: it lets a member edit, not widen who can.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpShare); err != nil {
		return nil, err
	}

	// Validate everything that can be refused before the first policy write.
	if len(req.Operations) != 1 {
		return nil, invalid("share permission must be one of: read, write")
	}
	ops, ok := ngac.ShareOps(req.Operations[0])
	if !ok {
		return nil, invalid("invalid share permission: %q", req.Operations[0])
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
		return nil, fmt.Errorf("create share OA: %w", err)
	}
	fail := func(err error) (*pb.ShareInfo, error) {
		if rbErr := prov.Fail(ctx, err); rbErr != nil && rbErr != err {
			slog.Error("could not remove share OA after a failed share", "share_oa", shareOA.Id, "error", rbErr)
		}
		return nil, err
	}

	// Assign item under share OA
	if err := prov.Assign(ctx, item.NGACNodeID, shareOA.Id); err != nil {
		return fail(fmt.Errorf("assign item under share OA: %w", err))
	}

	// Assign share OA under PC_Global
	// PC_Global may legitimately be absent (a graph that has none yet); a failed
	// lookup is not that, and must not silently produce a share that reaches no
	// policy class.
	pcGlobal, perr := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePCGlobal, NodeType: ngac.TypePC,
	})
	if perr != nil && !policyclient.IsNotFound(perr) {
		return fail(fmt.Errorf("find PC_Global: %w", perr))
	}
	if perr == nil && pcGlobal != nil {
		if err := prov.Assign(ctx, shareOA.Id, pcGlobal.Id); err != nil {
			return fail(fmt.Errorf("assign share OA under PC_Global: %w", err))
		}
	}

	// Create association. This is the share: if it fails we must not write the
	// DB row, or the UI shows a share that grants nothing.
	if err := prov.Associate(ctx, targetUA, shareOA.Id, ops); err != nil {
		return fail(fmt.Errorf("create share association: %w", err))
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
		return fail(fmt.Errorf("insert share: %w", err))
	}
	prov.Done()

	slog.Info("share created", "item", item.Name, "type", req.ShareType, "target", targetLabel)
	s.announceShare(ctx, realtime.KindShareCreated, item)
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
func (s *Service) resolveShareTarget(ctx context.Context, req *pb.CreateShareRequest) (ua, label string, err error) {
	switch req.ShareType {
	case "public":
		pubUA, perr := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
			Name: ngac.NodePublicUsers, NodeType: ngac.TypeUA,
		})
		if perr != nil {
			return "", "", fmt.Errorf("look up PublicUsers UA: %w", perr)
		}
		if pubUA == nil {
			return "", "", errors.New("PublicUsers UA not found")
		}
		return pubUA.Id, "Anyone with link", nil
	case "user", "role", "workspace":
		if req.TargetNgacNodeId == "" {
			return "", "", invalid("share target is required")
		}
		node, gerr := s.policyRead.GetNode(ctx, &policypb.GetNodeRequest{NodeId: req.TargetNgacNodeId})
		if gerr != nil || node == nil || node.GetId() == "" {
			return "", "", invalid("share target not found")
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
				return "", "", invalid("share target must be a group")
			}
			ua, err = s.personalUA(ctx, node)
			return ua, label, err
		}
		return "", "", invalid("share target must be a person or a group")
	}
	return "", "", invalid("invalid share_type: %s", req.ShareType)
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
func (s *Service) personalUA(ctx context.Context, user *policypb.NGACNode) (string, error) {
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
		return "", fmt.Errorf("create personal UA: %w", err)
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

func (s *Service) assignToPersonalUA(ctx context.Context, userID, uaID string) error {
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: userID, ParentId: uaID,
	}); err != nil {
		return fmt.Errorf("assign user to personal UA: %w", err)
	}
	return nil
}

// findPersonalUA looks for the user's personal UA: first among the attributes
// the user is already in, then by name for one that was created but not yet
// assigned (adopting it only if its properties say it is this user's). A node
// that merely has the right name is never returned. "" means none exists.
func (s *Service) findPersonalUA(ctx context.Context, user *policypb.NGACNode) (string, error) {
	anc, err := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: user.Id})
	if err != nil {
		return "", fmt.Errorf("look up personal UA: %w", err)
	}
	for _, n := range anc.GetNodes() {
		if n.GetNodeType() == ngac.TypeUA && ngac.IsPersonalUAOf(n.GetProperties(), user.Id) {
			return n.Id, nil
		}
	}

	byName, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.PersonalUAName(ngac.UserNodeID(user.Id)), NodeType: ngac.TypeUA,
	})
	if err != nil && !policyclient.IsNotFound(err) {
		return "", fmt.Errorf("look up personal UA: %w", err)
	}
	if err == nil && byName.GetId() != "" && ngac.IsPersonalUAOf(byName.GetProperties(), user.Id) {
		if aerr := s.assignToPersonalUA(ctx, user.Id, byName.Id); aerr != nil {
			return "", aerr
		}
		return byName.Id, nil
	}
	return "", nil
}

// announceShare reports a change to who can reach item. The item itself did not
// move, so the event carries no folder.
func (s *Service) announceShare(ctx context.Context, kind string, item *store.DriveItem) {
	s.announce(ctx, kind, item, func(e *realtime.Event) { e.ParentID = "" })
}

// RevokeShare removes a share.
func (s *Service) RevokeShare(ctx context.Context, req *pb.RevokeShareRequest) (*pb.Empty, error) {
	share, err := s.store.GetShare(ctx, req.ShareId)
	if err != nil || share == nil {
		return nil, notFound("share not found")
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
	item, err := s.store.GetItem(ctx, share.DriveItemID)
	if !isCreator {
		if err != nil || item == nil {
			return nil, denied("access denied")
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
		return nil, fmt.Errorf("revoke share OA: %w", err)
	}
	if err := s.store.DeleteShare(ctx, req.ShareId); err != nil {
		return nil, fmt.Errorf("delete share record: %w", err)
	}
	s.announceShare(ctx, realtime.KindShareRevoked, item)
	return &pb.Empty{}, nil
}

// ListShares returns all shares for an item.
func (s *Service) ListShares(ctx context.Context, req *pb.ListSharesRequest) (*pb.ShareList, error) {
	// Who an item is shared with is information about that item.
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, notFound("item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	shares, err := s.store.ListSharesByItem(ctx, req.ItemId)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
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
func (s *Service) GetSharedWithMe(ctx context.Context, req *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error) {
	// Find all UAs the user belongs to
	ancestors, err := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{
		NodeId: grpcauth.CallerFrom(ctx).NGACNodeID,
	})
	if err != nil {
		// A partial answer would list only what was shared with the person
		// directly and silently hide everything shared with their groups.
		return nil, fmt.Errorf("look up the caller's groups: %w", err)
	}
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
		return nil, fmt.Errorf("list shared: %w", err)
	}

	seen := make(map[string]bool)
	var items []*pb.DriveItem
	for _, sh := range shares {
		if seen[sh.DriveItemID] {
			continue
		}
		seen[sh.DriveItemID] = true
		item, err := s.store.GetItem(ctx, sh.DriveItemID)
		if err != nil {
			return nil, fmt.Errorf("load shared item: %w", err)
		}
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
func (s *Service) CreateDriveForChannel(ctx context.Context, req *pb.CreateDriveForChannelRequest) (*pb.DriveItem, error) {
	if req.ChannelId == "" || req.WorkspaceId == "" {
		return nil, invalid("channel_id and workspace_id are required")
	}
	// The channel must be one of this workspace's (or the workspace itself, which
	// the workspace service registers as the context of its own root drive). The
	// OA below is found by the channel's ID alone, so without this a request that
	// paired workspace B with workspace A's channel would adopt A's drive and
	// grant B's channel UA access to it.
	if req.ChannelId != req.WorkspaceId {
		chWS, err := s.store.GetChannelWorkspaceID(ctx, req.ChannelId)
		if err != nil {
			return nil, fmt.Errorf("look up channel: %w", err)
		}
		if chWS != req.WorkspaceId {
			return nil, denied("channel does not belong to this workspace")
		}
	}
	if existing, err := s.store.FindRootByContext(ctx, req.WorkspaceId, "channel", req.ChannelId); err != nil {
		return nil, fmt.Errorf("find channel drive: %w", err)
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
		return nil, fmt.Errorf("create channel drive OA: %w", err)
	}
	// An OA found by name must not be another workspace's drive root.
	if owner, oerr := s.store.RootWorkspaceByNode(ctx, driveOA.Id); oerr != nil {
		return nil, fmt.Errorf("check drive owner: %w", prov.Fail(ctx, oerr))
	} else if (owner != "" && owner != req.WorkspaceId) ||
		(driveOA.Properties["workspace_id"] != "" && driveOA.Properties["workspace_id"] != req.WorkspaceId) {
		// Run logs any undo step that fails, so the leftover is not silent.
		_ = prov.Fail(ctx, errors.New("drive belongs to another workspace")) // refused either way
		return nil, denied("channel drive belongs to another workspace")
	}

	// Assign under channel's Content OA (inherits channel permissions)
	if err := prov.Assign(ctx, driveOA.Id, req.ChannelNgacOaId); err != nil {
		return nil, fmt.Errorf("assign channel drive under content OA: %w", prov.Fail(ctx, err))
	}

	// Association: channel Members UA → drive OA [read, write, upload, share]
	if err := prov.Associate(ctx, req.ChannelNgacUaId, driveOA.Id, ngac.ChannelDriveOps()); err != nil {
		return nil, fmt.Errorf("grant channel members access to drive: %w", prov.Fail(ctx, err))
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
		return nil, fmt.Errorf("insert channel drive: %w", rbErr)
	}
	prov.Done()

	slog.Info("channel drive created", "channel", req.ChannelId, "drive", item.ID)
	return itemToProto(item), nil
}

// GetChannelDrive returns the root folder of a channel's drive.
// Returns nil if channel drive doesn't exist yet (lazy creation handled by Gateway).
func (s *Service) GetChannelDrive(ctx context.Context, req *pb.GetChannelDriveRequest) (*pb.DriveItem, error) {
	wsID, err := s.store.GetChannelWorkspaceID(ctx, req.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("look up channel: %w", err)
	}
	if wsID == "" {
		return nil, notFound("channel not found")
	}

	root, err := s.store.FindRootByContext(ctx, wsID, "channel", req.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("find channel drive: %w", err)
	}
	if root == nil {
		// Not an error — channel just doesn't have a drive yet
		return &pb.DriveItem{}, nil
	}
	return itemToProto(root), nil
}

// GetQuota returns workspace storage quota.
func (s *Service) GetQuota(ctx context.Context, req *pb.GetQuotaRequest) (*pb.Quota, error) {
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
		return nil, fmt.Errorf("get quota: %w", err)
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
func (s *Service) UpdateQuota(ctx context.Context, req *pb.UpdateQuotaRequest) (*pb.Quota, error) {
	userNodeID := grpcauth.CallerFrom(ctx).NGACNodeID
	if err := s.checkAccessOnNamedOA(ctx, userNodeID, ngac.MgmtOAName(ngac.WorkspaceID(req.WorkspaceId)), ngac.OpManage); err != nil {
		return nil, err
	}
	if err := s.store.UpdateQuotaLimits(ctx, req.WorkspaceId, req.MaxBytes, req.MaxFiles); err != nil {
		return nil, fmt.Errorf("update quota: %w", err)
	}
	return s.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: req.WorkspaceId})
}
