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
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/document"
	drivepb "ngac-platform/proto/drive"
	dgrpc "ngac-platform/services/document/internal/grpc"
	"ngac-platform/services/document/internal/rest"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	grpcPort := envOr("GRPC_PORT", "50054")
	restPort := envOr("REST_PORT", "8080")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	driveAddr := envOr("DRIVE_SERVICE_ADDR", "localhost:50057")

	// MinIO configuration
	minioEndpoint := envOr("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := envOr("MINIO_ACCESS_KEY", "ngac-admin")
	minioSecretKey := envOr("MINIO_SECRET_KEY", "ngac-secret-key")
	minioUseSSL := envOr("MINIO_USE_SSL", "false") == "true"
	minioPublicEndpoint := envOr("MINIO_PUBLIC_ENDPOINT", "localhost/storage")

	pool, err := connectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize internal MinIO client (for server-side ops: StatObject, PutObject, CopyObject)
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: minioUseSSL,
	})
	if err != nil {
		slog.Error("failed to create minio client", "endpoint", minioEndpoint, "error", err)
		os.Exit(1)
	}
	slog.Info("minio internal client initialized", "endpoint", minioEndpoint)

	// Initialize presign MinIO client for generating presigned URLs with the public endpoint.
	presignClient, err := minio.New(minioPublicEndpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure:       false,
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		slog.Error("failed to create minio presign client", "endpoint", minioPublicEndpoint, "error", err)
		os.Exit(1)
	}
	slog.Info("minio presign client initialized", "endpoint", minioPublicEndpoint)

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
	pb.RegisterDocumentStorageServiceServer(srv, dgrpc.NewDocumentStorageServer(pool, minioClient, presignClient))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Connect to Drive Service for legacy document endpoint proxying
	var driveClient drivepb.DriveServiceClient
	driveConn, err := grpc.NewClient(driveAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Warn("drive service unavailable for document proxy", "address", driveAddr, "error", err)
	} else {
		defer driveConn.Close()
		driveClient = drivepb.NewDriveServiceClient(driveConn)
	}

	// REST server (client-facing)
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	restHandler := rest.NewHandler(driveClient)
	restHandler.RegisterRoutes(e, jwtSecret)

	// Start both servers
	go func() {
		slog.Info("document gRPC listening", "port", grpcPort)
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("document REST listening", "port", restPort)
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
