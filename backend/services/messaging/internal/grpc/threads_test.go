package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/messaging"
)

// seedThread writes a channel holding a root message with one reply, and
// returns the channel, the root message and a third user to act as caller.
func seedThread(t *testing.T, pool *pgxpool.Pool) (chID, rootID, caller string) {
	t.Helper()
	ctx := context.Background()
	starter, replier := getTwoTestUserIDs(t, pool)
	caller = getTestUserID(t, pool)
	chID = insertTestChannel(t, pool, "threads", "workspace", getTestWorkspaceID(t, pool))
	rootID = chID + "-root"
	now := time.Now()
	_, err := pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, created_at) VALUES ($1, $2, $3, 'root', $4)`,
		rootID, chID, starter, now.Add(-time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, parent_message_id, created_at) VALUES ($1, $2, $3, 're', $4, $5)`,
		rootID+"-reply", chID, replier, rootID, now)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET reply_count = 1, last_reply_at = $2 WHERE id = $1`, rootID, now)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id = $1`, rootID)
		pool.Exec(ctx, `DELETE FROM messages WHERE parent_message_id = $1`, rootID)
		cleanTestData(t, pool, chID)
	})
	return chID, rootID, caller
}

func TestThreadRPCs_DeniedWithoutRead(t *testing.T) {
	srv, pool := setupTestServerWithPolicy(t, &mockPolicyReadDeny{})
	chID, rootID, user := seedThread(t, pool)
	ctx := asCaller(user, "ngac-stranger")

	calls := map[string]func() error{
		"FollowThread":   func() error { _, err := srv.FollowThread(ctx, &pb.ThreadRequest{MessageId: rootID}); return err },
		"UnfollowThread": func() error { _, err := srv.UnfollowThread(ctx, &pb.ThreadRequest{MessageId: rootID}); return err },
		"MarkThreadRead": func() error { _, err := srv.MarkThreadRead(ctx, &pb.ThreadRequest{MessageId: rootID}); return err },
		"ListThreads":    func() error { _, err := srv.ListThreads(ctx, &pb.ListThreadsRequest{ChannelId: chID}); return err },
	}
	for name, call := range calls {
		assert.Equal(t, codes.PermissionDenied, status.Code(call()), name)
	}
	var rows int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM thread_follows WHERE root_message_id = $1 AND user_id = $2`, rootID, user).Scan(&rows))
	assert.Zero(t, rows, "a denied call records nothing")
}

func TestThreadRPCs_AllowedWithRead(t *testing.T) {
	srv, pool := setupTestServer(t)
	chID, rootID, user := seedThread(t, pool)
	ctx := asCaller(user, "ngac-reader")

	_, err := srv.FollowThread(ctx, &pb.ThreadRequest{MessageId: rootID})
	require.NoError(t, err)
	list, err := srv.ListThreads(ctx, &pb.ListThreadsRequest{ChannelId: chID, Filter: "following"})
	require.NoError(t, err)
	require.Len(t, list.Messages, 1)
	assert.True(t, list.Messages[0].Following)

	_, err = srv.MarkThreadRead(ctx, &pb.ThreadRequest{MessageId: rootID})
	require.NoError(t, err)
	_, err = srv.UnfollowThread(ctx, &pb.ThreadRequest{MessageId: rootID})
	require.NoError(t, err)

	_, err = srv.ListThreads(ctx, &pb.ListThreadsRequest{ChannelId: chID, Filter: "nonsense"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = srv.FollowThread(ctx, &pb.ThreadRequest{MessageId: "no-such-message"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}
