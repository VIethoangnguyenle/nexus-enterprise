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
	return serveAuthAs(t, "messaging")
}

// serveAuthAs dials as service. "messaging" is the only service the auth
// policy accepts without a user.
func serveAuthAs(t *testing.T, service string) pb.AuthServiceClient {
	t.Helper()
	conn := testutil.ServeGRPCAs(t, service, agrpc.AuthPolicy(), func(s *grpc.Server) {
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
}

// People sign in with Google or a one-time code, over REST. The service has no
// password RPC and no way to list accounts, so there is nothing to attack or to
// enumerate over gRPC either.
func TestNoPasswordOrListingRPCsExist(t *testing.T) {
	methods := pb.AuthService_ServiceDesc.Methods
	have := map[string]bool{}
	for _, m := range methods {
		have[m.MethodName] = true
	}
	for _, gone := range []string{"Register", "Login", "Signup", "Signin", "ListUsers"} {
		if have[gone] {
			t.Errorf("AuthService still has %s", gone)
		}
	}
	if !have["GetUserByID"] || !have["GetUserByNGACNodeID"] {
		t.Error("the lookups other services use must remain")
	}
}

func TestIsTokenRevokedAcceptsAServiceIdentity(t *testing.T) {
	c := serveAuth(t) // dials as the messaging service

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

func TestIsTokenRevokedRefusesOtherServicesAndForgedIdentity(t *testing.T) {
	req := &pb.IsTokenRevokedRequest{Jti: "j"}
	if _, err := serveAuthAs(t, "drive").IsTokenRevoked(context.Background(), req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("from drive: want Unauthenticated, got %v", err)
	}

	conn := testutil.ServeGRPC(t, agrpc.AuthPolicy(), func(s *grpc.Server) {
		pb.RegisterAuthServiceServer(s, agrpc.NewAuthServer(nil, nil))
	})
	forged := testutil.ForgedIdentity(context.Background())
	if _, err := pb.NewAuthServiceClient(testutil.Unsigned(t, conn)).IsTokenRevoked(forged, req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("forged identity: want Unauthenticated, got %v", err)
	}
}
