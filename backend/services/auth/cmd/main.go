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
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/auth"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	agrpc "ngac-platform/services/auth/internal/grpc"
	"ngac-platform/services/auth/internal/rest"
	"ngac-platform/services/auth/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/1")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	workspaceAddr := envOr("WORKSPACE_SERVICE_ADDR", "localhost:50053")
	messagingAddr := envOr("MESSAGING_SERVICE_ADDR", "localhost:50055")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	grpcPort := envOr("GRPC_PORT", "50052")
	restPort := envOr("REST_PORT", "8080")

	auth.SetJWTSecret(jwtSecret)

	pool, err := connectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpc.NewClient(policyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()
	policyRead := policypb.NewPolicyReadServiceClient(policyConn)
	policyWrite := policypb.NewPolicyWriteServiceClient(policyConn)

	// Workspace gRPC client (for auto-provisioning on register)
	var wsClient workspacepb.WorkspaceServiceClient
	wsConn, err := grpc.NewClient(workspaceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Warn("workspace service unavailable, auto-provision disabled", "address", workspaceAddr, "error", err)
	} else {
		defer wsConn.Close()
		wsClient = workspacepb.NewWorkspaceServiceClient(wsConn)
	}

	// Messaging gRPC client (for auto-provisioning #general channel)
	var msgClient messagingpb.MessagingServiceClient
	msgConn, err := grpc.NewClient(messagingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Warn("messaging service unavailable, auto-provision disabled", "address", messagingAddr, "error", err)
	} else {
		defer msgConn.Close()
		msgClient = messagingpb.NewMessagingServiceClient(msgConn)
	}

	st := store.New(pool)

	rdb, err := connectRedis(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, jwt blacklist disabled", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	// Domain service — shared by gRPC and REST handlers
	svc := domain.NewService(st, rdb, policyRead, policyWrite, wsClient, msgClient)

	// gRPC server (service-to-service)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor,
			recoveryInterceptor,
		),
	)
	pb.RegisterAuthServiceServer(srv, agrpc.NewAuthServer(svc, rdb))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	restHandler := rest.NewHandler(svc)
	restHandler.RegisterRoutes(e, jwtSecret)

	// Start both servers
	go func() {
		slog.Info("auth gRPC listening", "port", grpcPort)
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("auth REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	gracefulShutdown(srv, healthSrv, e, cancel)
}

// envOr returns the environment variable value or a fallback default.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// connectRedis creates a Redis client from a URL and verifies connectivity.
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
	slog.Info("redis connected", "addr", opts.Addr, "db", opts.DB)
	return rdb, nil
}

// connectDB creates a pgxpool with production-ready pool configuration.
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

// gracefulShutdown waits for SIGINT/SIGTERM and drains in-flight requests.
func gracefulShutdown(srv *grpc.Server, healthSrv *health.Server, echoSrv *echo.Echo, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig)

	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown Echo REST server
	if err := echoSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("echo shutdown error", "error", err)
	}

	// Graceful stop gRPC server
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		slog.Info("server stopped gracefully")
	case <-shutdownCtx.Done():
		slog.Warn("graceful stop timed out, forcing stop")
		srv.Stop()
	}
}

// loggingInterceptor logs every gRPC call with method, duration, and status code.
func loggingInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	code := status.Code(err)
	attrs := []any{
		"method", info.FullMethod,
		"duration_ms", time.Since(start).Milliseconds(),
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

// recoveryInterceptor catches panics in handlers and returns Internal error.
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
			err = status.Errorf(13, "internal server error")
		}
	}()
	return handler(ctx, req)
}
