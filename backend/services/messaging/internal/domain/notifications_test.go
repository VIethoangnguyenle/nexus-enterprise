package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

type fakeNotifStore struct {
	rows      []*store.Notification
	members   map[string]store.Member // key: workspace + "|" + key
	invitees  []store.Invitee
	insertErr error
	listErr   error
	countErr  error
	gotLimit  int
	gotOffset int
	gotWS     string
	marked    [][3]string
	markedAll [][2]string
	about     [][4]string
	deleted   [][3]string
}

func (f *fakeNotifStore) InsertNotification(_ context.Context, n *store.Notification) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.rows = append(f.rows, n)
	return nil
}
func (f *fakeNotifStore) ListNotifications(_ context.Context, _, ws string, limit, offset int) ([]*store.Notification, error) {
	f.gotLimit, f.gotOffset, f.gotWS = limit, offset, ws
	return f.rows, f.listErr
}
func (f *fakeNotifStore) NotificationCounts(context.Context, string, string) (int, int, error) {
	return len(f.rows), len(f.rows), f.countErr
}
func (f *fakeNotifStore) MarkNotificationRead(_ context.Context, id, user, ws string) error {
	f.marked = append(f.marked, [3]string{id, user, ws})
	return nil
}
func (f *fakeNotifStore) MarkAllNotificationsRead(_ context.Context, user, ws string) error {
	f.markedAll = append(f.markedAll, [2]string{user, ws})
	return nil
}
func (f *fakeNotifStore) FindMember(_ context.Context, ws, key string) (store.Member, bool, error) {
	m, ok := f.members[ws+"|"+key]
	return m, ok, nil
}
func (f *fakeNotifStore) InviteesOf(context.Context, string, time.Time) ([]store.Invitee, error) {
	return f.invitees, nil
}
func (f *fakeNotifStore) DeleteNotificationsAbout(_ context.Context, user, typ, id string) error {
	f.deleted = append(f.deleted, [3]string{user, typ, id})
	return nil
}
func (f *fakeNotifStore) MarkNotificationsAboutRead(_ context.Context, user, ws, typ, id string) error {
	f.about = append(f.about, [4]string{user, ws, typ, id})
	return nil
}

type pushed struct {
	ws, user string
	n        *pb.Notification
}

type fakePush struct{ got []pushed }

func (p *fakePush) SendNotification(ws, user string, n *pb.Notification) {
	p.got = append(p.got, pushed{ws, user, n})
}

func TestNotificationService_ListBoundsThePage(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, 25}, {-3, 25}, {10, 10}, {50, 50}, {51, 25}} {
		f := &fakeNotifStore{}
		_, err := NewNotificationService(f, nil).List(context.Background(), "u", "ws", tc.in, -5)
		require.NoError(t, err)
		assert.Equal(t, tc.want, f.gotLimit, "limit %d", tc.in)
		assert.Equal(t, 0, f.gotOffset, "a negative offset starts at the top")
		assert.Equal(t, "ws", f.gotWS, "the list is for the caller's workspace")
	}
}

// Without both a user and a workspace there is nothing to scope to: refused, and
// the store is never asked.
func TestNotificationService_NoUserOrNoWorkspaceIsDeniedAndTouchesNothing(t *testing.T) {
	for _, who := range [][2]string{{"", "ws"}, {"u", ""}, {"", ""}} {
		f := &fakeNotifStore{}
		s := NewNotificationService(f, nil)
		ctx := context.Background()

		_, err := s.List(ctx, who[0], who[1], 10, 0)
		assert.ErrorIs(t, err, ErrAccessDenied, "%v", who)
		_, err = s.UnreadCount(ctx, who[0], who[1])
		assert.ErrorIs(t, err, ErrAccessDenied)
		assert.ErrorIs(t, s.MarkRead(ctx, who[0], who[1], "n1"), ErrAccessDenied)
		assert.ErrorIs(t, s.MarkAllRead(ctx, who[0], who[1]), ErrAccessDenied)
		assert.ErrorIs(t, s.MarkAboutRead(ctx, who[0], who[1], "workspace_invitation", "i1"), ErrAccessDenied)
		assert.Empty(t, f.marked)
		assert.Empty(t, f.markedAll)
		assert.Empty(t, f.about)
	}
}

func TestNotificationService_MarkingIsScopedToTheCallerAndWorkspace(t *testing.T) {
	f := &fakeNotifStore{}
	s := NewNotificationService(f, nil)
	ctx := context.Background()
	require.NoError(t, s.MarkRead(ctx, "alice", "ws-1", "n1"))
	assert.Equal(t, [][3]string{{"n1", "alice", "ws-1"}}, f.marked)
	require.NoError(t, s.MarkAllRead(ctx, "alice", "ws-1"))
	assert.Equal(t, [][2]string{{"alice", "ws-1"}}, f.markedAll)
	require.NoError(t, s.MarkAboutRead(ctx, "alice", "ws-1", "workspace_invitation", "i1"))
	assert.Equal(t, [][4]string{{"alice", "ws-1", "workspace_invitation", "i1"}}, f.about)
	assert.ErrorIs(t, s.MarkAboutRead(ctx, "alice", "ws-1", "", "i1"), ErrInvalidInput)
}

func TestNotificationService_StoreFailuresAreReturnedNotSwallowed(t *testing.T) {
	boom := errors.New("db down")
	_, err := NewNotificationService(&fakeNotifStore{countErr: boom}, nil).List(context.Background(), "u", "w", 10, 0)
	assert.ErrorIs(t, err, boom, "a failed count must not be reported as zero")
	_, err = NewNotificationService(&fakeNotifStore{listErr: boom}, nil).List(context.Background(), "u", "w", 10, 0)
	assert.ErrorIs(t, err, boom)
	_, err = NewNotificationService(&fakeNotifStore{countErr: boom}, nil).UnreadCount(context.Background(), "u", "w")
	assert.ErrorIs(t, err, boom)
}

func members() map[string]store.Member {
	return map[string]store.Member{
		"ws|node-req":   {UserID: "u-req", Name: "Hoa"},
		"ws|u-req":      {UserID: "u-req", Name: "Hoa"},
		"ws|node-boss":  {UserID: "u-boss", Name: "Vinh"},
		"other|node-me": {UserID: "u-req", Name: "Hoa"},
	}
}

func TestCreateNotification_ResolvesPeopleStoresFactsAndPushesInTheWorkspace(t *testing.T) {
	f := &fakeNotifStore{members: members()}
	p := &fakePush{}
	err := NewNotificationService(f, p).CreateNotification(context.Background(), NewNotification{
		WorkspaceID: "ws", Type: "approval_rejected", Recipient: "node-req", Actor: "node-boss",
		TargetType: "approval", TargetID: "r1", TargetName: "Tạm ứng", Params: map[string]string{"reason": "no budget", "empty": ""},
	})
	require.NoError(t, err)

	require.Len(t, f.rows, 1)
	n := f.rows[0]
	assert.Equal(t, "u-req", n.UserID, "the recipient is the person, not their node")
	assert.Equal(t, "u-boss", n.ActorUserID)
	assert.Equal(t, "Vinh", n.ActorName, "the name comes from the workspace's own membership")
	assert.Equal(t, "Tạm ứng", n.TargetName)
	assert.Equal(t, map[string]string{"reason": "no budget"}, n.Params, "empty params are not stored")

	require.Len(t, p.got, 1)
	assert.Equal(t, "ws", p.got[0].ws, "pushed within the workspace only")
	assert.Equal(t, "u-req", p.got[0].user)
	assert.Equal(t, n.ID, p.got[0].n.Id)
	assert.Equal(t, "Vinh", p.got[0].n.ActorName)
	assert.False(t, p.got[0].n.Read)
}

func TestCreateNotification_RecipientOutsideTheWorkspaceGetsNothing(t *testing.T) {
	f := &fakeNotifStore{members: members()}
	p := &fakePush{}
	// node-me is a member of "other", not of "ws".
	err := NewNotificationService(f, p).CreateNotification(context.Background(), NewNotification{
		WorkspaceID: "ws", Type: "asset_assigned", Recipient: "node-me", TargetName: "Laptop",
	})
	assert.ErrorIs(t, err, ErrNotAMember)
	assert.Empty(t, f.rows)
	assert.Empty(t, p.got, "nothing is pushed for a notification that was not stored")
}

// A person who is not (or is no longer) in the workspace is not named: the
// notification stands without an actor rather than showing an id.
func TestCreateNotification_AnUnknownActorIsLeftOut(t *testing.T) {
	f := &fakeNotifStore{members: members()}
	require.NoError(t, NewNotificationService(f, nil).CreateNotification(context.Background(), NewNotification{
		WorkspaceID: "ws", Type: "asset_assigned", Recipient: "u-req", Actor: "node-from-elsewhere", TargetName: "Laptop",
	}))
	require.Len(t, f.rows, 1)
	assert.Empty(t, f.rows[0].ActorUserID)
	assert.Empty(t, f.rows[0].ActorName)
}

func TestCreateNotification_NobodyIsToldOfTheirOwnAct(t *testing.T) {
	f := &fakeNotifStore{members: members()}
	p := &fakePush{}
	// The recipient is named by user id and the actor by node: still one person.
	err := NewNotificationService(f, p).CreateNotification(context.Background(), NewNotification{
		WorkspaceID: "ws", Type: "asset_assigned", Recipient: "u-req", Actor: "node-req", TargetName: "Laptop",
	})
	require.NoError(t, err)
	assert.Empty(t, f.rows)
	assert.Empty(t, p.got)
}

func TestCreateNotification_RefusesWhatCannotBePlaced(t *testing.T) {
	s := NewNotificationService(&fakeNotifStore{members: members()}, nil)
	for _, in := range []NewNotification{
		{Type: "t", Recipient: "u-req"},
		{WorkspaceID: "ws", Recipient: "u-req"},
		{WorkspaceID: "ws", Type: "t"},
	} {
		assert.ErrorIs(t, s.CreateNotification(context.Background(), in), ErrInvalidInput, "%+v", in)
	}
}

func TestCreateNotification_PushesOnlyAfterTheRowIsStored(t *testing.T) {
	failing := &fakeNotifStore{members: members(), insertErr: errors.New("db down")}
	p := &fakePush{}
	err := NewNotificationService(failing, p).CreateNotification(context.Background(), NewNotification{
		WorkspaceID: "ws", Type: "asset_assigned", Recipient: "u-req",
	})
	require.Error(t, err)
	assert.Empty(t, p.got)
}

func TestCreateNotification_WithoutAPusherStillStores(t *testing.T) {
	f := &fakeNotifStore{members: members()}
	require.NoError(t, NewNotificationService(f, nil).CreateNotification(context.Background(), NewNotification{
		WorkspaceID: "ws", Type: "asset_assigned", Recipient: "u-req",
	}))
	assert.Len(t, f.rows, 1)
}

func TestNotifyInvitation_TellsTheInviteePersonallyAndReplacesAnEarlierNotice(t *testing.T) {
	f := &fakeNotifStore{
		members:  map[string]store.Member{"ws-new|node-boss": {UserID: "u-boss", Name: "Vinh"}},
		invitees: []store.Invitee{{UserID: "u-guest", WorkspaceID: "ws-new", WorkspaceName: "NovaPay", InviterNode: "node-boss"}},
	}
	p := &fakePush{}
	require.NoError(t, NewNotificationService(f, p).NotifyInvitation(context.Background(), "inv-1"))

	require.Len(t, f.rows, 1)
	n := f.rows[0]
	assert.Equal(t, "u-guest", n.UserID)
	assert.Equal(t, store.TypeWorkspaceInvitation, n.Type)
	assert.Equal(t, "ws-new", n.WorkspaceID)
	assert.Equal(t, "NovaPay", n.TargetName)
	assert.Equal(t, "inv-1", n.TargetID)
	assert.Equal(t, "Vinh", n.ActorName)
	assert.Equal(t, [][3]string{{"u-guest", store.TypeWorkspaceInvitation, "inv-1"}}, f.deleted, "a refreshed invitation replaces its notice")
	require.Len(t, p.got, 1)
	assert.Equal(t, "", p.got[0].ws, "personal: every session of the invitee, whichever workspace it has open")
	assert.Equal(t, "u-guest", p.got[0].user)
}

// An address with no verified account reaches nobody, and nothing says so.
func TestNotifyInvitation_NobodyReachedMeansNothingHappens(t *testing.T) {
	f := &fakeNotifStore{}
	p := &fakePush{}
	require.NoError(t, NewNotificationService(f, p).NotifyInvitation(context.Background(), "inv-1"))
	assert.Empty(t, f.rows)
	assert.Empty(t, p.got)
	assert.ErrorIs(t, NewNotificationService(f, p).NotifyInvitation(context.Background(), ""), ErrInvalidInput)
}
