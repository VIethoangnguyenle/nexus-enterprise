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
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
	wgrpc "ngac-platform/services/workspace/internal/grpc"
	"ngac-platform/services/workspace/internal/rest"
	"ngac-platform/services/workspace/internal/store"
)

func main() {
	bootstrap.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := bootstrap.PolicyAddr()
	// Same convention as drive: authorization reads may go to a read replica;
	// unset, they go to the primary policy service.
	policyReadAddr := bootstrap.Env("POLICY_READ_SERVICE_ADDR", policyAddr)
	driveAddr := bootstrap.Env("DRIVE_SERVICE_ADDR", "localhost:50057")
	grpcPort := bootstrap.Env("GRPC_PORT", "50053")
	restPort := bootstrap.Env("REST_PORT", "8080")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	// MinIO configuration
	minioEndpoint := bootstrap.Env("MINIO_ENDPOINT", "localhost:9000")
	minioAccessKey := bootstrap.Env("MINIO_ACCESS_KEY", "ngac-admin")
	minioSecretKey := bootstrap.Env("MINIO_SECRET_KEY", "ngac-secret-key")
	minioUseSSL := bootstrap.Env("MINIO_USE_SSL", "false") == "true"

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpcauth.Dial(policyAddr, "workspace")
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()

	policyReadConn := policyConn
	if policyReadAddr != policyAddr {
		policyReadConn, err = grpcauth.Dial(policyReadAddr, "workspace")
		if err != nil {
			slog.Error("failed to connect to policy read service", "address", policyReadAddr, "error", err)
			os.Exit(1)
		}
		defer policyReadConn.Close()
	}

	// Initialize MinIO client
	minioClient, err := minio.New(minioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioAccessKey, minioSecretKey, ""),
		Secure: minioUseSSL,
	})
	if err != nil {
		slog.Warn("failed to create minio client, continuing without bucket creation", "error", err)
		minioClient = nil
	} else {
		slog.Info("minio client initialized", "endpoint", minioEndpoint)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	// Connect to Drive Service (optional)
	var driveClient drivepb.DriveServiceClient
	driveConn, err := grpcauth.Dial(driveAddr, "workspace")
	if err != nil {
		slog.Warn("drive service unavailable, workspace drives disabled", "address", driveAddr, "error", err)
	} else {
		defer driveConn.Close()
		driveClient = drivepb.NewDriveServiceClient(driveConn)
	}

	// --- Wire clean architecture layers ---
	// The read client is the PDP: every admin route checks the caller against it
	// (see domain/authz.go). It must be wired — a nil client would panic, and the
	// domain treats any error from it as DENY.
	policyReadClient := policypb.NewPolicyReadServiceClient(policyReadConn)
	policyWriteClient := policypb.NewPolicyWriteServiceClient(policyConn)

	rt := realtime.Connect("workspace")
	defer rt.Close()

	wsStore := store.New(pool)
	wsSvc := domain.NewService(wsStore, wsStore, policyReadClient, policyWriteClient, minioClient, driveClient).WithDirectory(wsStore).WithInvitations(wsStore).WithEmitter(rt)
	wsSrv := wgrpc.NewWorkspaceServer(wsSvc)
	pb.RegisterWorkspaceServiceServer(srv, wsSrv)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := httputil.NewEcho("workspace", echomw.Logger())
	restHandler := rest.NewHandler(wsSvc)
	restHandler.RegisterRoutes(e, jwtSecret)

	// Admin organization management endpoints
	adminHandler := rest.NewAdminHandler(wsSvc)
	adminAPI := e.Group("/api", httputil.JWTMiddleware(jwtSecret))
	adminHandler.RegisterAdminRoutes(adminAPI)

	// Start both servers
	go func() {
		slog.Info("workspace gRPC listening", "port", grpcPort)
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("workspace REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	bootstrap.Shutdown{GRPC: srv, Health: healthSrv, HTTP: []bootstrap.HTTPServer{e}, Cancel: cancel}.Wait()
}
