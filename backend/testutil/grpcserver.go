package testutil

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"ngac-platform/pkg/grpcauth"
)

// ServeGRPC runs a real gRPC server on a loopback port, with the same caller
// interceptor production servers use, and returns a client connection built
// with the client interceptor. register installs the service under test.
//
// Tests that call a handler directly bypass the interceptor, so they cannot
// show that a request without a caller is refused or that the caller on the
// wire is the one the handler uses. Going through this does.
//
// The client dials as the "test" service. ServeGRPCAs picks another name, for
// tests of a ServiceOK allowlist.
func ServeGRPC(t *testing.T, policy grpcauth.ServerPolicy, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	return ServeGRPCAs(t, "test", policy, register)
}

// TestIdentitySecret is the identity-signing secret every test server and
// client in this module shares.
const TestIdentitySecret = "test-internal-identity-secret-0123456789"

// ServeGRPCAs is ServeGRPC with the client dialling as service.
func ServeGRPCAs(t *testing.T, service string, policy grpcauth.ServerPolicy, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	if err := grpcauth.Configure(grpcauth.Keys{Current: TestIdentitySecret}); err != nil {
		t.Fatalf("configure identity: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer(grpcauth.ServerOptions(policy)...)
	register(gs)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcauth.ClientInterceptor(service)),
		grpc.WithChainStreamInterceptor(grpcauth.StreamClientInterceptor(service)))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// Unsigned returns a second connection to the server behind conn that signs
// nothing: no identity interceptor, so a test chooses every metadata key that
// reaches the wire. Use it with ForgedIdentity to prove a server refuses a
// caller it cannot verify.
func Unsigned(t *testing.T, conn *grpc.ClientConn) *grpc.ClientConn {
	t.Helper()
	raw, err := grpc.NewClient(conn.Target(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial unsigned: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	return raw
}

// ForgedIdentity returns ctx carrying every kind of identity an attacker with
// network access to a gRPC port but without the signing secret could invent:
// the retired x-caller-* keys, a service name, and a token that is not signed
// by the platform. A server must answer all of it with Unauthenticated.
func ForgedIdentity(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx,
		"x-caller-user-id", "forged-user",
		"x-caller-ngac-node-id", "forged-node",
		"x-caller-tenant-id", "forged-tenant",
		"x-service-name", "auth",
		grpcauth.KeyIdentity, "eyJ1aWQiOiJmb3JnZWQifQ.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
}
