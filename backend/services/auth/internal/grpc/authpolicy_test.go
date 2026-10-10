package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/auth"
	agrpc "ngac-platform/services/auth/internal/grpc"
	"ngac-platform/testutil"
)

func serveAuth(t *testing.T) pb.AuthServiceClient {
	t.Helper()
	conn := testutil.ServeGRPC(t, agrpc.AuthPolicy(), func(s *grpc.Server) {
		pb.RegisterAuthServiceServer(s, agrpc.NewAuthServer(nil, nil))
	})
	return pb.NewAuthServiceClient(conn)
}

func TestUserLookupsRequireACaller(t *testing.T) {
	c := serveAuth(t)

	if _, err := c.GetUserByID(context.Background(), &pb.GetUserByIDRequest{UserId: "u"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetUserByID: want Unauthenticated, got %v", err)
	}
	if _, err := c.GetUserByNGACNodeID(context.Background(), &pb.GetUserByNGACNodeIDRequest{NgacNodeId: "n"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("GetUserByNGACNodeID: want Unauthenticated, got %v", err)
	}
	if _, err := c.ListUsers(context.Background(), &pb.ListUsersRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("ListUsers: want Unauthenticated, got %v", err)
	}
}

// Login presents credentials; with no caller it still reaches the handler. An
// empty request is rejected there as invalid input, never by the interceptor.
func TestCredentialRPCsAreReachableWithoutACaller(t *testing.T) {
	c := serveAuth(t)

	_, err := c.Login(context.Background(), &pb.LoginRequest{})
	if status.Code(err) == codes.Unauthenticated && status.Convert(err).Message() == "caller identity required" {
		t.Fatalf("Login refused by the caller interceptor: %v", err)
	}
}

func TestIsTokenRevokedAcceptsAServiceIdentity(t *testing.T) {
	c := serveAuth(t) // test client dials with the "test" service identity

	resp, err := c.IsTokenRevoked(context.Background(), &pb.IsTokenRevokedRequest{Jti: "j"})
	if err != nil {
		t.Fatalf("IsTokenRevoked: %v", err)
	}
	if resp.Revoked {
		t.Fatal("unknown token reported revoked")
	}
}

func TestPolicyKeepsRevokeTokenBehindACaller(t *testing.T) {
	c := serveAuth(t)

	if _, err := c.RevokeToken(context.Background(), &pb.RevokeTokenRequest{Jti: "j"}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("RevokeToken: want Unauthenticated, got %v", err)
	}
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})
	if _, err := c.RevokeToken(ctx, &pb.RevokeTokenRequest{Jti: "j"}); status.Code(err) == codes.Unauthenticated {
		t.Fatalf("RevokeToken with a caller was refused: %v", err)
	}
}
