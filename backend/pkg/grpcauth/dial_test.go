package grpcauth

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// healthServer starts a plain gRPC server with the health service on a free port.
func healthServer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	return lis.Addr().String()
}

// waitForState waits until conn reports want, or fails the test after limit.
func waitForState(t *testing.T, conn *grpc.ClientConn, want connectivity.State, limit time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	for s := conn.GetState(); s != want; s = conn.GetState() {
		if !conn.WaitForStateChange(ctx, s) {
			t.Fatalf("channel state = %v after %v, want %v", conn.GetState(), limit, want)
		}
	}
}

// A channel that idles has to rebuild its resolver and connection on the next
// call; in production that first call after half an hour without traffic waited
// about 8 s. Channels opened by Dial therefore never idle.
func TestDialChannelsNeverIdle(t *testing.T) {
	if dialIdleTimeout != 0 {
		t.Fatalf("dialIdleTimeout = %v, want 0 (idleness disabled)", dialIdleTimeout)
	}
}

// Dial applies dialIdleTimeout: with a short one the channel does go idle, so
// the zero above is what keeps production channels connected.
func TestDialAppliesTheIdleTimeout(t *testing.T) {
	prev := dialIdleTimeout
	dialIdleTimeout = 100 * time.Millisecond
	t.Cleanup(func() { dialIdleTimeout = prev })

	conn, err := Dial(healthServer(t), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	waitForState(t, conn, connectivity.Idle, 3*time.Second)
}

// The first call after a service starts must not pay for the connection
// either: Dial connects at once instead of waiting for that call.
func TestDialConnectsWithoutWaitingForACall(t *testing.T) {
	conn, err := Dial(healthServer(t), "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	waitForState(t, conn, connectivity.Ready, 3*time.Second)
}

// A channel that takes longer than slowConnect to become ready is logged with
// its target and the wait, so a slow reconnect shows in the log.
func TestDialLogsASlowConnect(t *testing.T) {
	logs := &lockedBuffer{}
	prevLog := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prevLog) })
	prev := slowConnect
	slowConnect = 0
	t.Cleanup(func() { slowConnect = prev })

	addr := healthServer(t)
	conn, err := Dial(addr, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	waitForState(t, conn, connectivity.Ready, 3*time.Second)

	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(logs.String(), "grpc channel slow to become ready") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	for _, want := range []string{"grpc channel slow to become ready", addr, "waited_ms"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log lacks %q: %s", want, logs)
		}
	}
}

// lockedBuffer is a log sink the channel watcher's goroutine and the test can
// share.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A channel whose server went away comes back by itself once the server is up
// again, without waiting for a call: a dropped connection leaves pick_first
// idle, and the next call would otherwise pay for reconnecting.
func TestDialReconnectsWithoutWaitingForACall(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	first := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(first, health.NewServer())
	go func() { _ = first.Serve(lis) }()

	conn, err := Dial(addr, "test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	waitForState(t, conn, connectivity.Ready, 3*time.Second)

	first.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if !conn.WaitForStateChange(ctx, connectivity.Ready) {
		t.Fatal("channel still ready after its server stopped")
	}
	lis2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	second := grpc.NewServer()
	grpc_health_v1.RegisterHealthServer(second, health.NewServer())
	go func() { _ = second.Serve(lis2) }()
	t.Cleanup(second.Stop)

	waitForState(t, conn, connectivity.Ready, 5*time.Second)
}
