package grpcauth

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// captureLog redirects the default logger for one test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestRecoveryHidesThePanicFromTheCallerAndLogsIt(t *testing.T) {
	logs := captureLog(t)
	_, err := Recovery(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/x.Y/Z"},
		func(context.Context, any) (any, error) { panic("pq: password authentication failed") })

	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if strings.Contains(err.Error(), "pq:") {
		t.Errorf("panic text reached the caller: %v", err)
	}
	if !strings.Contains(logs.String(), "pq: password authentication failed") {
		t.Errorf("panic not logged: %s", logs)
	}
}

func TestLoggingRecordsTheErrorOfAFailedCall(t *testing.T) {
	logs := captureLog(t)
	boom := status.Error(codes.NotFound, "no such thing")
	_, err := Logging(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/x.Y/Z"},
		func(context.Context, any) (any, error) { return nil, boom })

	if err != boom {
		t.Fatalf("Logging must pass the error through unchanged, got %v", err)
	}
	for _, want := range []string{"grpc call failed", "/x.Y/Z", "NotFound", "no such thing"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log lacks %q: %s", want, logs)
		}
	}
}

// Every server built from ServerOptions survives a panicking interceptor or
// handler and answers a generic Internal, without its main asking for it.
func TestServerOptionsRecoverFromAPanic(t *testing.T) {
	captureLog(t)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	panicking := func(context.Context, any, *grpc.UnaryServerInfo, grpc.UnaryHandler) (any, error) {
		panic("secret internals")
	}
	gs := grpc.NewServer(ServerOptions(ServerPolicy{Exempt: HealthExempt()}, panicking)...)
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := Dial(lis.Addr().String(), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	_, err = grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal (err %v)", status.Code(err), err)
	}
	if strings.Contains(err.Error(), "secret internals") {
		t.Errorf("panic text reached the client: %v", err)
	}
}

func TestDialForwardsTheCaller(t *testing.T) {
	var seen Caller
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	capture := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		seen = CallerFrom(ctx)
		return h(ctx, req)
	}
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(ServerInterceptor(ServerPolicy{}), capture))
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := Dial(lis.Addr().String(), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	want := Caller{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"}
	ctx := WithCaller(context.Background(), want)
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if seen != want {
		t.Errorf("server saw %+v, want %+v", seen, want)
	}
}
