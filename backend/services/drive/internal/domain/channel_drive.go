package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

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
