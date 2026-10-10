package testutil

import (
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"ngac-platform/pkg/grpcauth"
)

// ServeGRPC runs a real gRPC server on a loopback port, with the same caller
// interceptor production servers use, and returns a client connection built
// with the client interceptor. register installs the service under test.
//
// Tests that call a handler directly bypass the interceptor, so they cannot
// show that a request without a caller is refused or that the caller on the
// wire is the one the handler uses. Going through this does.
func ServeGRPC(t *testing.T, policy grpcauth.ServerPolicy, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
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
		grpc.WithChainUnaryInterceptor(grpcauth.ClientInterceptor("test")))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
