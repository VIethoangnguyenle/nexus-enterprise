package grpc_test

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/auth"
	agrpc "ngac-platform/services/auth/internal/grpc"
)

// When the revocation list cannot be read, the answer is "try again", never
// "not revoked": a revoked token must not pass because Redis was down.
func TestIsTokenRevoked_FailsClosedWhenRedisCannotBeAsked(t *testing.T) {
	dead := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1})
	t.Cleanup(func() { dead.Close() })

	res, err := agrpc.NewAuthServer(nil, dead).IsTokenRevoked(context.Background(), &pb.IsTokenRevokedRequest{Jti: "some-jti"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("Redis down: got %v, %v; want Unavailable", res, err)
	}
}

func TestIsTokenRevoked_RefusesATokenWithoutAnID(t *testing.T) {
	res, err := agrpc.NewAuthServer(nil, nil).IsTokenRevoked(context.Background(), &pb.IsTokenRevokedRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty jti: got %v, %v; want InvalidArgument", res, err)
	}
}
