package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

func TestNotifications_RoundTripAndOwnership(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	ctx := context.Background()
	alice, _ := testutil.CreateUser(t, pool)
	bob, _ := testutil.CreateUser(t, pool)

	base := time.Now().Add(-time.Hour)
	for i, n := range []*store.Notification{
		{ID: "n-" + alice + "-1", UserID: alice, Type: "mention", Title: "first", CreatedAt: base},
		{ID: "n-" + alice + "-2", UserID: alice, Type: "asset_approved", Title: "second", Body: "b", EntityType: "asset", EntityID: "a-1", CreatedAt: base.Add(time.Minute)},
		{ID: "n-" + bob + "-1", UserID: bob, Type: "mention", Title: "bobs", CreatedAt: base},
	} {
		require.NoError(t, st.InsertNotification(ctx, n), "insert %d", i)
	}

	list, err := st.ListNotifications(ctx, alice, 10, 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "second", list[0].Title, "newest first")
	assert.Equal(t, "asset", list[0].EntityType)
	assert.Equal(t, "", list[1].EntityType, "NULL reads back as empty")

	page, err := st.ListNotifications(ctx, alice, 1, 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "first", page[0].Title)

	total, unread, err := st.NotificationCounts(ctx, alice)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, 2, unread)

	// Bob cannot mark Alice's notification read.
	require.NoError(t, st.MarkNotificationRead(ctx, "n-"+alice+"-1", bob))
	_, unread, err = st.NotificationCounts(ctx, alice)
	require.NoError(t, err)
	assert.Equal(t, 2, unread, "another user's mark changes nothing")

	require.NoError(t, st.MarkNotificationRead(ctx, "n-"+alice+"-1", alice))
	_, unread, _ = st.NotificationCounts(ctx, alice)
	assert.Equal(t, 1, unread)

	require.NoError(t, st.MarkAllNotificationsRead(ctx, alice))
	_, unread, _ = st.NotificationCounts(ctx, alice)
	assert.Equal(t, 0, unread)
	_, bobUnread, _ := st.NotificationCounts(ctx, bob)
	assert.Equal(t, 1, bobUnread, "marking all read is the caller's own")
}
