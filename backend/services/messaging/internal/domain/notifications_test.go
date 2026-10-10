package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

type fakeNotifStore struct {
	rows      []*store.Notification
	insertErr error
	listErr   error
	countErr  error
	gotLimit  int
	gotOffset int
	marked    [][2]string
	markedAll []string
}

func (f *fakeNotifStore) InsertNotification(_ context.Context, n *store.Notification) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.rows = append(f.rows, n)
	return nil
}
func (f *fakeNotifStore) ListNotifications(_ context.Context, _ string, limit, offset int) ([]*store.Notification, error) {
	f.gotLimit, f.gotOffset = limit, offset
	return f.rows, f.listErr
}
func (f *fakeNotifStore) NotificationCounts(context.Context, string) (int, int, error) {
	return len(f.rows), len(f.rows), f.countErr
}
func (f *fakeNotifStore) MarkNotificationRead(_ context.Context, id, user string) error {
	f.marked = append(f.marked, [2]string{id, user})
	return nil
}
func (f *fakeNotifStore) MarkAllNotificationsRead(_ context.Context, user string) error {
	f.markedAll = append(f.markedAll, user)
	return nil
}

type fakePush struct {
	user string
	n    *pb.Notification
}

func (p *fakePush) SendNotification(user string, n *pb.Notification) { p.user, p.n = user, n }

func TestNotificationService_ListBoundsThePage(t *testing.T) {
	for _, tc := range []struct{ in, want int }{{0, 25}, {-3, 25}, {10, 10}, {50, 50}, {51, 25}} {
		f := &fakeNotifStore{}
		_, err := NewNotificationService(f, nil).List(context.Background(), "u", tc.in, -5)
		require.NoError(t, err)
		assert.Equal(t, tc.want, f.gotLimit, "limit %d", tc.in)
		assert.Equal(t, 0, f.gotOffset, "a negative offset starts at the top")
	}
}

func TestNotificationService_NoUserIsDeniedAndTouchesNothing(t *testing.T) {
	f := &fakeNotifStore{}
	s := NewNotificationService(f, nil)
	ctx := context.Background()

	_, err := s.List(ctx, "", 10, 0)
	assert.ErrorIs(t, err, ErrAccessDenied)
	_, err = s.UnreadCount(ctx, "")
	assert.ErrorIs(t, err, ErrAccessDenied)
	assert.ErrorIs(t, s.MarkRead(ctx, "", "n1"), ErrAccessDenied)
	assert.ErrorIs(t, s.MarkAllRead(ctx, ""), ErrAccessDenied)
	assert.Empty(t, f.marked)
	assert.Empty(t, f.markedAll)
}

func TestNotificationService_MarkReadIsScopedToTheCaller(t *testing.T) {
	f := &fakeNotifStore{}
	s := NewNotificationService(f, nil)
	require.NoError(t, s.MarkRead(context.Background(), "alice", "n1"))
	assert.Equal(t, [][2]string{{"n1", "alice"}}, f.marked)
}

func TestNotificationService_StoreFailuresAreReturnedNotSwallowed(t *testing.T) {
	boom := errors.New("db down")
	_, err := NewNotificationService(&fakeNotifStore{countErr: boom}, nil).List(context.Background(), "u", 10, 0)
	assert.ErrorIs(t, err, boom, "a failed count must not be reported as zero")
	_, err = NewNotificationService(&fakeNotifStore{listErr: boom}, nil).List(context.Background(), "u", 10, 0)
	assert.ErrorIs(t, err, boom)
	_, err = NewNotificationService(&fakeNotifStore{countErr: boom}, nil).UnreadCount(context.Background(), "u")
	assert.ErrorIs(t, err, boom)
}

func TestNotificationService_CreatePushesOnlyAfterTheRowIsStored(t *testing.T) {
	f := &fakeNotifStore{}
	p := &fakePush{}
	require.NoError(t, NewNotificationService(f, p).CreateNotification(context.Background(), "alice", "mention", "t", "b", "asset", "a-1"))
	require.Len(t, f.rows, 1)
	assert.Equal(t, "alice", p.user)
	assert.Equal(t, f.rows[0].ID, p.n.Id)
	assert.Equal(t, "asset", p.n.EntityType)
	assert.False(t, p.n.Read)

	failing := &fakeNotifStore{insertErr: errors.New("db down")}
	p2 := &fakePush{}
	err := NewNotificationService(failing, p2).CreateNotification(context.Background(), "alice", "mention", "t", "b", "", "")
	require.Error(t, err)
	assert.Nil(t, p2.n, "nothing is pushed for a notification that was not stored")
}

func TestNotificationService_CreateWithoutAPusherStillStores(t *testing.T) {
	f := &fakeNotifStore{}
	require.NoError(t, NewNotificationService(f, nil).CreateNotification(context.Background(), "alice", "mention", "t", "", "", ""))
	assert.Len(t, f.rows, 1)
}
