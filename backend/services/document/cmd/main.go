package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/document"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	dgrpc "ngac-platform/services/document/internal/grpc"
	"ngac-platform/services/document/internal/rest"
	"ngac-platform/services/document/internal/storage"
	"ngac-platform/services/document/internal/texts"
)

func main() {
	bootstrap.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	grpcPort := bootstrap.Env("GRPC_PORT", "50054")
	restPort := bootstrap.Env("REST_PORT", "8080")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	driveAddr := bootstrap.Env("DRIVE_SERVICE_ADDR", "localhost:50057")
	policyAddr := bootstrap.PolicyAddr()

	// MinIO configuration
	minioEndpoint := bootstrap.Env("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := bootstrap.Env("MINIO_ACCESS_KEY", "ngac-admin")
	minioSecretKey := bootstrap.Env("MINIO_SECRET_KEY", "ngac-secret-key")
	minioUseSSL := bootstrap.Env("MINIO_USE_SSL", "false") == "true"
	// Where browsers reach the store, for presigned URLs. MINIO_PUBLIC_SECURE=true
	// signs https URLs (production, behind TLS); unset keeps plain http for dev.
	minioPublicEndpoint := bootstrap.Env("MINIO_PUBLIC_ENDPOINT", "localhost/storage")
	minioPublicSecure := bootstrap.Env("MINIO_PUBLIC_SECURE", "false") == "true"

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
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
	presignClient, err := storage.NewPresignClient(
		storage.PublicEndpoint{Host: minioPublicEndpoint, Secure: minioPublicSecure}, minioAccessKey, minioSecretKey)
	if err != nil {
		slog.Error("failed to create minio presign client", "endpoint", minioPublicEndpoint, "error", err)
		os.Exit(1)
	}
	slog.Info("minio presign client initialized", "endpoint", minioPublicEndpoint, "secure", minioPublicSecure)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	pb.RegisterDocumentStorageServiceServer(srv, dgrpc.NewDocumentStorageServer(storage.New(minioClient, presignClient)))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Connect to Drive Service for legacy document endpoint proxying
	var driveClient drivepb.DriveServiceClient
	driveConn, err := grpcauth.Dial(driveAddr, "document")
	if err != nil {
		slog.Warn("drive service unavailable for document proxy", "address", driveAddr, "error", err)
	} else {
		defer driveConn.Close()
		driveClient = drivepb.NewDriveServiceClient(driveConn)
	}

	// Documents written in the app are authorized by the policy service.
	policyConn, err := grpcauth.Dial(policyAddr, "document")
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()
	textService := texts.NewService(texts.NewStore(pool), policypb.NewPolicyReadServiceClient(policyConn))
	rt := realtime.Connect("document")
	defer rt.Close()
	textService.SetEmitter(rt)

	// REST server (client-facing)
	e := httputil.NewEcho("document", echomw.Logger())
	restHandler := rest.NewHandler(driveClient, textService)
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

	bootstrap.Shutdown{GRPC: srv, Health: healthSrv, HTTP: []bootstrap.HTTPServer{e}, Cancel: cancel}.Wait()
}
