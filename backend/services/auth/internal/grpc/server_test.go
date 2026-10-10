package grpc_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/auth"
	"ngac-platform/services/auth/internal/domain"
	agrpc "ngac-platform/services/auth/internal/grpc"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = os.Getenv("REDIS_ADDR")
	}
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 14})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		t.Skipf("test redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

// serveWith runs the real handler over a real database; callers carry a service
// identity so the lookups are admitted.
func serveWith(t *testing.T, rdb *redis.Client) (pb.AuthServiceClient, string, string, string) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	userID, nodeID := testutil.CreateUser(t, pool)
	svc := domain.NewService(store.New(pool), rdb, nil, nil, nil, nil)
	conn := testutil.ServeGRPC(t, agrpc.AuthPolicy(), func(s *grpc.Server) {
		pb.RegisterAuthServiceServer(s, agrpc.NewAuthServer(svc, rdb))
	})
	var username string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT username FROM users WHERE id = $1`, userID).Scan(&username))
	return pb.NewAuthServiceClient(conn), userID, nodeID, username
}

func asService(ctx context.Context) context.Context {
	return grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: "svc-user", NGACNodeID: "svc-node"})
}

func TestLookups_ReturnTheAccountAndNothingSecret(t *testing.T) {
	c, userID, nodeID, username := serveWith(t, nil)
	ctx := asService(context.Background())

	byID, err := c.GetUserByID(ctx, &pb.GetUserByIDRequest{UserId: userID})
	require.NoError(t, err)
	assert.Equal(t, userID, byID.Id)
	assert.Equal(t, username, byID.Username)
	assert.Equal(t, nodeID, byID.NgacNodeId)

	byNode, err := c.GetUserByNGACNodeID(ctx, &pb.GetUserByNGACNodeIDRequest{NgacNodeId: nodeID})
	require.NoError(t, err)
	assert.Equal(t, userID, byNode.Id)
}

func TestLookups_AnUnknownAccountIsNotFound(t *testing.T) {
	c, _, _, _ := serveWith(t, nil)
	ctx := asService(context.Background())

	_, err := c.GetUserByID(ctx, &pb.GetUserByIDRequest{UserId: "no-such-user"})
	assert.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.GetUserByNGACNodeID(ctx, &pb.GetUserByNGACNodeIDRequest{NgacNodeId: "no-such-node"})
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestRevokeToken_BlacklistsUntilItExpires(t *testing.T) {
	rdb := testRedisClient(t)
	c, _, _, _ := serveWith(t, rdb)
	ctx := asService(context.Background())
	jti := "jti-" + time.Now().Format("150405.000000000")

	before, err := c.IsTokenRevoked(ctx, &pb.IsTokenRevokedRequest{Jti: jti})
	require.NoError(t, err)
	assert.False(t, before.Revoked)

	res, err := c.RevokeToken(ctx, &pb.RevokeTokenRequest{Jti: jti, ExpiresAtUnix: time.Now().Add(time.Hour).Unix()})
	require.NoError(t, err)
	assert.True(t, res.Revoked)
	after, err := c.IsTokenRevoked(ctx, &pb.IsTokenRevokedRequest{Jti: jti})
	require.NoError(t, err)
	assert.True(t, after.Revoked)

	ttl, err := rdb.TTL(context.Background(), "jwt:blacklist:"+jti).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 30*time.Minute, "the entry lives as long as the token would have")
	assert.LessOrEqual(t, ttl, time.Hour)
	t.Cleanup(func() { rdb.Del(context.Background(), "jwt:blacklist:"+jti) })

	other, err := c.IsTokenRevoked(ctx, &pb.IsTokenRevokedRequest{Jti: jti + "-other"})
	require.NoError(t, err)
	assert.False(t, other.Revoked, "one token's revocation does not touch another")
}

func TestRevokeToken_AnExpiredTokenNeedsNoEntry(t *testing.T) {
	rdb := testRedisClient(t)
	c, _, _, _ := serveWith(t, rdb)
	jti := "expired-" + time.Now().Format("150405.000000000")

	res, err := c.RevokeToken(asService(context.Background()), &pb.RevokeTokenRequest{Jti: jti, ExpiresAtUnix: time.Now().Add(-time.Minute).Unix()})
	require.NoError(t, err)
	assert.True(t, res.Revoked)
	n, err := rdb.Exists(context.Background(), "jwt:blacklist:"+jti).Result()
	require.NoError(t, err)
	assert.Zero(t, n)
}

// Without a blacklist there is nothing to revoke into: the call says so rather
// than reporting a revocation that did not happen; the check cannot say
// "revoked" either.
func TestRevokeToken_WithoutRedisIsUnavailableNotSuccess(t *testing.T) {
	c, _, _, _ := serveWith(t, nil)
	ctx := asService(context.Background())

	_, err := c.RevokeToken(ctx, &pb.RevokeTokenRequest{Jti: "x", ExpiresAtUnix: time.Now().Add(time.Hour).Unix()})
	assert.Equal(t, codes.Unavailable, status.Code(err))
	res, err := c.IsTokenRevoked(ctx, &pb.IsTokenRevokedRequest{Jti: "x"})
	require.NoError(t, err)
	assert.False(t, res.Revoked)
}

func TestRevokeToken_RequiresACaller(t *testing.T) {
	c, _, _, _ := serveWith(t, nil)
	_, err := c.RevokeToken(context.Background(), &pb.RevokeTokenRequest{Jti: "x"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
