// Package bootstrap is the start-up and shut-down code every service's main
// shares: reading configuration from the environment, opening the database
// pool, and stopping gracefully on a signal. Redis lives in bootstrap/redisconn
// so that only the services that use it link the client.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// InitLogger makes JSON on stdout the default logger.
func InitLogger() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
}

// Env returns the environment variable key, or fallback when it is unset or
// empty.
func Env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// EnvAlias is Env for a variable that was renamed. key is the current name;
// deprecated are the older names still honoured, in order, for one release.
// Using one logs a warning that names the replacement, so the deployment that
// still sets it finds out before the alias goes away.
func EnvAlias(key string, deprecated []string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	for _, old := range deprecated {
		if v := os.Getenv(old); v != "" {
			slog.Warn("deprecated environment variable; it will stop being read in the next release",
				"variable", old, "use", key)
			return v
		}
	}
	return fallback
}

// PolicyAddrEnv is the one name for the policy service's address. POLICY_ADDR
// is the old name approval used; it is still read, with a deprecation warning.
const PolicyAddrEnv = "POLICY_SERVICE_ADDR"

// PolicyAddr returns the policy service's gRPC address.
func PolicyAddr() string {
	return EnvAlias(PolicyAddrEnv, []string{"POLICY_ADDR"}, "localhost:50051")
}

// ConnectDB opens a pgx pool with the pool settings every service uses and
// verifies it can reach the database.
func ConnectDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 5 * time.Minute
	cfg.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

// GRPCServer is the part of *grpc.Server that Shutdown stops.
type GRPCServer interface {
	GracefulStop()
	Stop()
}

// HTTPServer is anything that drains in-flight requests on Shutdown: an
// *echo.Echo or an *http.Server.
type HTTPServer interface {
	Shutdown(ctx context.Context) error
}

// Shutdown describes what to stop, in order, when the process is asked to.
type Shutdown struct {
	// GRPC is stopped gracefully, then forcibly if Timeout runs out.
	GRPC GRPCServer
	// Health, if set, is marked NOT_SERVING first so load balancers drain.
	Health *health.Server
	// HTTP servers are drained before the gRPC server.
	HTTP []HTTPServer
	// Cancel, if set, cancels the process context (stopping background work).
	Cancel context.CancelFunc
	// Cleanup, if set, runs after Cancel and before any server stops.
	Cleanup func()
	// Timeout bounds the whole drain. Zero means 15 seconds.
	Timeout time.Duration
	// Signals delivers the stop request. Nil means SIGINT or SIGTERM.
	Signals <-chan os.Signal
}

// Wait blocks until a stop signal arrives, then stops everything described by
// s and returns.
func (s Shutdown) Wait() {
	sigCh := s.Signals
	if sigCh == nil {
		real := make(chan os.Signal, 1)
		signal.Notify(real, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(real)
		sigCh = real
	}
	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig)

	if s.Health != nil {
		s.Health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	}
	if s.Cancel != nil {
		s.Cancel()
	}
	if s.Cleanup != nil {
		s.Cleanup()
	}

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, h := range s.HTTP {
		if err := h.Shutdown(ctx); err != nil {
			slog.Warn("http shutdown error", "error", err)
		}
	}

	if s.GRPC == nil {
		return
	}
	stopped := make(chan struct{})
	go func() {
		s.GRPC.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		slog.Info("server stopped gracefully")
	case <-ctx.Done():
		slog.Warn("graceful stop timed out, forcing stop")
		s.GRPC.Stop()
	}
}
