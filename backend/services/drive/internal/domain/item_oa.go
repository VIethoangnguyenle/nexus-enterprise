package domain

import (
	"context"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/provision"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// Which OA an item is authorized on, and which OAs are an item's own.
//
// A folder has an OA of its own. A file has none until it is shared: it is
// authorized on its folder's OA (or the drive root's at the top level). When a
// file is shared it gets an OA of its own beneath that one (ngac.FileOAName), so
// the share OA holds the file alone; with the folder's OA under the share OA,
// sharing one file would grant the whole folder. A drive root's OA is the
// workspace's Documents OA or a channel's drive OA, which no item owns.

// rootAction says why a drive root was refused.
type rootAction string

const (
	rootRename rootAction = "renamed"
	rootTrash  rootAction = "moved to the trash"
	rootDelete rootAction = "deleted"
	rootMove   rootAction = "moved"
	rootShare  rootAction = "shared"
)

// refuseRoot refuses an edit of a drive root. A root is the drive itself: its
// OA is the workspace's Documents OA (or a channel's drive OA), which every
// member writes into, so write on it must not be enough to rename, trash, move
// or delete the drive, nor share to hand it over (sharing a root would put the
// Documents OA, or a channel's drive OA, under the share). The one exception is a channel drive moved to the trash
// by the person who created the channel, which is how a group is disbanded.
func (s *Service) refuseRoot(ctx context.Context, item *store.DriveItem, action rootAction) error {
	if !item.IsRoot {
		return nil
	}
	if action == rootTrash && item.DriveContext == "channel" {
		creator, err := s.store.ChannelCreator(ctx, item.DriveContextID)
		if err != nil {
			return fmt.Errorf("look up channel: %w", err)
		}
		caller := grpcauth.CallerFrom(ctx).UserID
		if creator != "" && caller == creator {
			return nil
		}
		return denied("only the group's creator may move its drive to the trash")
	}
	return invalid("the drive root cannot be %s", string(action))
}

// fileOwnsOA reports whether a file is authorized on an OA of its own (it has
// been shared), rather than on its folder's. The OA is the file's only when the
// node it points at is named for this very file. It asks the policy writer,
// which is authoritative: the read replica can lag a moment behind a share
// that just gave the file its OA.
func (s *Service) fileOwnsOA(ctx context.Context, item *store.DriveItem) (bool, error) {
	if item.ItemType != "file" || item.NGACNodeID == "" {
		return false, nil
	}
	node, err := s.policyWrite.GetNode(ctx, &policypb.GetNodeRequest{NodeId: item.NGACNodeID})
	if err != nil {
		if policyclient.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("look up the file's node: %w", err)
	}
	return node.GetName() == ngac.FileOAName(ngac.FileID(item.ID)), nil
}

// ensureFileOA gives a file an OA of its own beneath the OA it is authorized on
// today, and points the file at it, unless it already has one. The file keeps
// every right it inherited (its OA is under its folder's) and gains a node that
// a share can hold without holding the folder.
//
// Shares made before files had OAs of their own hold the folder's OA (or the
// Documents OA, for a top-level file), which grants everything in it. Before the
// row is pointed at the new OA, each of the file's shares is narrowed to it (see
// narrowShare), so a file that has its own OA has no over-broad share left. A
// failure leaves the row as it was; the next share or the startup repair
// (RepairFileShares) finishes the work, and every step is safe to repeat.
//
// It runs under the item's lock, as moves do, and the row changes only if it
// still points at the OA it started from. A trashed file is handled too: its
// shares come back with it on restore.
func (s *Service) ensureFileOA(ctx context.Context, item *store.DriveItem) (*store.DriveItem, error) {
	if owns, err := s.fileOwnsOA(ctx, item); err != nil || owns {
		return item, err
	}
	unlock, err := s.store.LockItem(ctx, item.ID)
	if err != nil {
		return nil, unavailable("wait for a concurrent change: %v", err)
	}
	defer unlock()

	current, err := s.store.GetItem(ctx, item.ID)
	if err != nil {
		return nil, fmt.Errorf("load file: %w", err)
	}
	if current == nil {
		return nil, notFound("item not found")
	}
	if owns, err := s.fileOwnsOA(ctx, current); err != nil || owns {
		return current, err
	}

	containerOA := current.NGACNodeID
	prov := provision.NewCreator(s.policyWrite)
	node, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.FileOAName(ngac.FileID(current.ID)), NodeType: ngac.TypeOA,
		Properties: map[string]string{
			"type": "drive_file", "workspace_id": current.WorkspaceID, ngac.PropDisplayName: current.Name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create file node: %w", err)
	}
	// Without this edge the file's OA reaches no policy class and every check
	// on the file would deny. Assigning twice is harmless.
	if err := prov.Assign(ctx, node.Id, containerOA); err != nil {
		return nil, fmt.Errorf("assign file node under its folder: %w", prov.Fail(ctx, err))
	}
	// From here the node is kept even if a later step fails: it only inherits
	// what the file already has, and a retry finds and reuses it.
	prov.Done()

	shares, err := s.store.ListSharesByItem(ctx, current.ID)
	if err != nil {
		return nil, fmt.Errorf("list the file's shares: %w", err)
	}
	shareOAs := make(map[string]bool, len(shares))
	for _, sh := range shares {
		shareOAs[sh.NGACShareOA] = true
	}
	// A node left by an earlier attempt may still hang under a folder the file
	// has since left; that edge would hand the file to that folder's readers.
	if err := s.keepOnlyParents(ctx, node.Id, containerOA, shareOAs); err != nil {
		return nil, err
	}
	for _, sh := range shares {
		if err := s.narrowShare(ctx, sh.NGACShareOA, containerOA, node.Id); err != nil {
			return nil, fmt.Errorf("narrow share %s to the file: %w", sh.ID, err)
		}
	}

	swapped, err := s.store.SwapNGACNodeID(ctx, current.ID, containerOA, node.Id)
	if err != nil {
		return nil, fmt.Errorf("point file at its node: %w", err)
	}
	if !swapped {
		return nil, aborted("item was changed by another request; retry")
	}
	current.NGACNodeID = node.Id
	return current, nil
}

// narrowShare makes a file's share OA hold the file's own OA instead of the
// container it was authorized on. It never widens what the share grants: the
// file's OA is put under the share only when the share holds the container the
// file is in now (whose reach includes the file's OA), and before the container
// edge is removed, so the grantee keeps the file throughout and loses the rest.
// Anything else the share holds (a folder the file has since been moved out of)
// is only removed. A share already narrowed is left as it is.
func (s *Service) narrowShare(ctx context.Context, shareOA, containerOA, fileOA string) error {
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: shareOA})
	if err != nil {
		if policyclient.IsNotFound(err) {
			return nil // the share OA is gone, so it grants nothing
		}
		return fmt.Errorf("read the share's contents: %w", err)
	}
	holdsContainer := false
	var stale []string
	for _, c := range children.GetNodes() {
		switch c.GetId() {
		case fileOA:
		case containerOA:
			holdsContainer = true
		default:
			stale = append(stale, c.GetId())
		}
	}
	if holdsContainer {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: fileOA, ParentId: shareOA,
		}); err != nil {
			return fmt.Errorf("put the file under the share: %w", err)
		}
		stale = append(stale, containerOA)
	}
	for _, c := range stale {
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: c, ParentId: shareOA,
		}); err != nil {
			return fmt.Errorf("take %s out of the share: %w", c, err)
		}
	}
	return nil
}

// keepOnlyParents removes every parent edge of a file's OA except the one to
// the container it is in and those to its own shares.
func (s *Service) keepOnlyParents(ctx context.Context, fileOA, containerOA string, shareOAs map[string]bool) error {
	parents, err := s.policyWrite.GetParents(ctx, &policypb.GetParentsRequest{NodeId: fileOA})
	if err != nil {
		return fmt.Errorf("read the file node's parents: %w", err)
	}
	for _, p := range parents.GetNodes() {
		if p.GetId() == containerOA || shareOAs[p.GetId()] {
			continue
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: fileOA, ParentId: p.GetId(),
		}); err != nil {
			return fmt.Errorf("detach the file node from %s: %w", p.GetId(), err)
		}
	}
	return nil
}

// releaseOwnOA deletes the OA a removed item owned: a folder's, or a shared
// file's. An OA the item did not own (a file that inherits its folder's, a drive
// root's) is never deleted, and neither is one that a remaining item or a
// workspace still points at. The item's row is already gone, so a failure is
// logged for repair, not returned.
func (s *Service) releaseOwnOA(ctx context.Context, item *store.DriveItem) {
	if item.IsRoot || item.NGACNodeID == "" {
		return
	}
	if item.ItemType == "file" {
		owns, err := s.fileOwnsOA(ctx, item)
		if err != nil {
			slog.Error("deleted file's node not checked; may need cleanup", "item", item.ID, "node", item.NGACNodeID, "err", err)
			return
		}
		if !owns {
			return
		}
	}
	inUse, err := s.store.NodeInUse(ctx, item.NGACNodeID)
	if err != nil {
		slog.Error("deleted item's node not checked; may need cleanup", "item", item.ID, "node", item.NGACNodeID, "err", err)
		return
	}
	if inUse {
		slog.Warn("deleted item's node is still in use and was kept", "item", item.ID, "node", item.NGACNodeID)
		return
	}
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: item.NGACNodeID}); err != nil {
		slog.Error("deleted item's OA not removed; node needs cleanup", "item", item.ID, "node", item.NGACNodeID, "err", err)
	}
}
