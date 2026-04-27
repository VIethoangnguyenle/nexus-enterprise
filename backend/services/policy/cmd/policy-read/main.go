package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	"ngac-platform/services/policy/internal/ngac"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/0")
	port := envOr("GRPC_PORT", "50061")

	pool, err := connectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	graph := ngac.NewGraph()
	store := ngac.NewStore(pool, graph)

	if err := store.LoadGraph(ctx); err != nil {
		slog.Error("failed to load graph", "error", err)
		os.Exit(1)
	}
	slog.Info("graph loaded", "nodes", len(graph.Nodes))

	ce := ngac.NewConstraintEngine()
	if os.Getenv("ENABLE_WEEKDAY_CONSTRAINT") == "true" {
		ce.Register(ngac.WeekdayOnlyConstraint)
		slog.Info("weekday-only constraint enabled")
	}

	rdb, err := connectRedis(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, L1 caching disabled", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	cte := ngac.NewCTEEvaluator(pool)
	materialized := ngac.NewMaterializedAccess(pool)
	versionTracker := ngac.NewVersionTracker(pool)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor,
			recoveryInterceptor,
		),
	)

	readServer := pgrpc.NewReadServer(store, ce, rdb, cte, materialized, versionTracker)
	pb.RegisterPolicyReadServiceServer(srv, readServer)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)
		healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		cancel()
		stopped := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
			slog.Info("server stopped gracefully")
		case <-time.After(15 * time.Second):
			slog.Warn("graceful stop timed out, forcing stop")
			srv.Stop()
		}
	}()

	slog.Info("policy-read service listening", "port", port)
	if err := srv.Serve(lis); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func connectRedis(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("pinging redis: %w", err)
	}
	return rdb, nil
}

func connectDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
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

func loggingInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)
	code := status.Code(err)
	attrs := []any{
		"method", info.FullMethod,
		"duration_ms", duration.Milliseconds(),
		"code", code.String(),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Warn("grpc call failed", attrs...)
	} else {
		slog.Debug("grpc call", attrs...)
	}
	return resp, err
}

func recoveryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in grpc handler",
				"method", info.FullMethod,
				"panic", fmt.Sprintf("%v", r),
			)
			err = status.Errorf(2, "internal server error")
		}
	}()
	return handler(ctx, req)
}
