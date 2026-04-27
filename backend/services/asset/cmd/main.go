package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	assetpb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	agrpc "ngac-platform/services/asset/internal/grpc"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/rest"
	"ngac-platform/services/asset/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	kafkaBrokers := envOr("KAFKA_BROKERS", "localhost:19092")
	port := envOr("GRPC_PORT", "50056")
	restPort := envOr("REST_PORT", "8080")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")

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

	producer, err := events.NewProducer(strings.Split(kafkaBrokers, ","))
	if err != nil {
		slog.Warn("kafka unavailable, event streaming disabled", "error", err)
	}
	if producer != nil {
		defer producer.Close()
	}

	assetStore := store.New(pool)
	policyRead := policypb.NewPolicyReadServiceClient(policyConn)
	policyWrite := policypb.NewPolicyWriteServiceClient(policyConn)

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
	assetTypeSrv := agrpc.NewAssetTypeServer(assetStore, policyRead, policyWrite)
	assetSrv := agrpc.NewAssetServer(assetStore, policyRead, policyWrite, producer)
	assetReqSrv := agrpc.NewAssetRequestServer(assetStore, policyRead, policyWrite, producer)

	assetpb.RegisterAssetTypeServiceServer(srv, assetTypeSrv)
	assetpb.RegisterAssetServiceServer(srv, assetSrv)
	assetpb.RegisterAssetRequestServiceServer(srv, assetReqSrv)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	restHandler := rest.NewHandler(assetSrv, assetTypeSrv, assetReqSrv)
	restHandler.RegisterRoutes(e, jwtSecret)

	// Start both servers
	go func() {
		slog.Info("asset gRPC listening", "port", port)
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("asset REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	gracefulShutdown(srv, healthSrv, e, cancel)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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

func gracefulShutdown(srv *grpc.Server, healthSrv *health.Server, echoSrv *echo.Echo, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig)

	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	echoSrv.Shutdown(shutdownCtx)

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

func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	code := status.Code(err)
	attrs := []any{"method", info.FullMethod, "duration_ms", time.Since(start).Milliseconds(), "code", code.String()}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Warn("grpc call failed", attrs...)
	} else {
		slog.Debug("grpc call", attrs...)
	}
	return resp, err
}

func recoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered", "method", info.FullMethod, "panic", fmt.Sprintf("%v", r))
			err = status.Errorf(13, "internal server error")
		}
	}()
	return handler(ctx, req)
}
