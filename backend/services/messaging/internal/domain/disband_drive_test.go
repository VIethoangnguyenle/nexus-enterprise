package domain_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	drivepb "ngac-platform/proto/drive"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// fakeDrive is the drive service as disbanding sees it: the group's drive root
// (none when root is ""), and the items it was asked to trash, with the caller
// each request carried.
type fakeDrive struct {
	drivepb.DriveServiceClient

	mu       sync.Mutex
	root     string
	getErr   error
	trashErr error
	asked    []string // channel IDs GetChannelDrive was asked about
	trashed  []string
	callers  []grpcauth.Caller
}

func (d *fakeDrive) GetChannelDrive(_ context.Context, req *drivepb.GetChannelDriveRequest, _ ...grpc.CallOption) (*drivepb.DriveItem, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.asked = append(d.asked, req.ChannelId)
	if d.getErr != nil {
		return nil, d.getErr
	}
	if d.root == "" {
		return &drivepb.DriveItem{}, nil
	}
	return &drivepb.DriveItem{Id: d.root, DriveContext: "channel", DriveContextId: req.ChannelId, ItemType: "folder", Status: "active"}, nil
}

func (d *fakeDrive) TrashItem(ctx context.Context, req *drivepb.TrashItemRequest, _ ...grpc.CallOption) (*drivepb.Empty, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callers = append(d.callers, grpcauth.CallerFrom(ctx))
	if d.trashErr != nil {
		return nil, d.trashErr
	}
	d.trashed = append(d.trashed, req.ItemId)
	return &drivepb.Empty{}, nil
}

// withDrive rebuilds the world's service with the fake drive service wired in.
func (w *groupWorld) withDrive(d *fakeDrive) {
	w.f.svc = domain.NewService(store.NewStore(w.f.pool), w.f.read, w.f.write, stubAuth{}, d)
}

func (w *groupWorld) channelExists(t *testing.T) bool {
	t.Helper()
	var n int
	require.NoError(t, w.f.pool.QueryRow(context.Background(), `SELECT count(*) FROM channels WHERE id = $1`, w.chID).Scan(&n))
	return n == 1
}

// Disbanding moves the group's drive to the trash through the drive service,
// as the owner who disbands, before the group and its nodes are gone: its files
// are not left counted against the workspace and reachable by nobody.
func TestDisbandGroup_TrashesTheGroupsDrive(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	d := &fakeDrive{root: "drive-root-1"}
	w.withDrive(d)

	require.NoError(t, w.f.svc.DisbandGroup(w.ctx(w.owner()), w.chID, userNode))

	assert.Equal(t, []string{w.chID}, d.asked)
	assert.Equal(t, []string{"drive-root-1"}, d.trashed, "the drive root goes to the trash with everything in it")
	require.Len(t, d.callers, 1)
	assert.Equal(t, userNode, d.callers[0].NGACNodeID, "trashed as the owner, whose write the drive checks")
	assert.False(t, w.channelExists(t))
	assert.ElementsMatch(t, []string{w.managersUA, w.uaID, w.oaID}, w.f.write.deleted)
}

// A group that never had a drive disbands as before.
func TestDisbandGroup_WithoutADriveTrashesNothing(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	d := &fakeDrive{}
	w.withDrive(d)

	require.NoError(t, w.f.svc.DisbandGroup(w.ctx(w.owner()), w.chID, userNode))

	assert.Empty(t, d.trashed)
	assert.False(t, w.channelExists(t))
}

// Deny: when the drive cannot be trashed (or not even found), the group is not
// disbanded: the row, its nodes and its drive all stay, and a retry can finish.
func TestDisbandGroup_RefusedWhenTheDriveCannotBeTrashed(t *testing.T) {
	for name, d := range map[string]*fakeDrive{
		"trash fails":  {root: "drive-root-2", trashErr: errors.New("drive unavailable")},
		"lookup fails": {getErr: errors.New("drive unavailable")},
	} {
		t.Run(name, func(t *testing.T) {
			w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
			w.withDrive(d)

			err := w.f.svc.DisbandGroup(w.ctx(w.owner()), w.chID, userNode)

			require.ErrorIs(t, err, domain.ErrUnavailable)
			assert.True(t, w.channelExists(t), "the group stays")
			assert.Empty(t, w.f.write.deleted, "its nodes stay")
		})
	}
}

// Deny: a manager cannot disband, and the drive is not touched.
func TestDisbandGroup_ManagerDeniedLeavesTheDrive(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	d := &fakeDrive{root: "drive-root-3"}
	w.withDrive(d)
	m := w.manager(t)

	require.ErrorIs(t, w.f.svc.DisbandGroup(w.ctx(m), w.chID, m.node), domain.ErrAccessDenied)

	assert.Empty(t, d.asked)
	assert.Empty(t, d.trashed)
	assert.True(t, w.channelExists(t))
}
