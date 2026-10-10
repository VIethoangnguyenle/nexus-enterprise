package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/realtime"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/domain"
	driveGRPC "ngac-platform/services/drive/internal/grpc"
	"ngac-platform/services/drive/internal/rest"
	"ngac-platform/services/drive/internal/store"
)

func main() {
	bootstrap.InitLogger()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := bootstrap.PolicyAddr()
	policyReadAddr := bootstrap.Env("POLICY_READ_SERVICE_ADDR", policyAddr)
	docAddr := bootstrap.Env("DOCUMENT_SERVICE_ADDR", "localhost:50054")
	grpcPort := bootstrap.Env("GRPC_PORT", "50057")
	restPort := bootstrap.Env("REST_PORT", "8080")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	policyWriteConn := dial(policyAddr)
	policyReadConn := dial(policyReadAddr)
	docConn := dial(docAddr)

	driveStore := store.NewStore(db)
	svc := domain.NewService(
		driveStore,
		policypb.NewPolicyReadServiceClient(policyReadConn),
		policypb.NewPolicyWriteServiceClient(policyWriteConn),
		docpb.NewDocumentStorageServiceClient(docConn),
	)

	rt := realtime.Connect("drive")
	defer rt.Close()
	svc.SetEmitter(rt)
	srv := driveGRPC.NewServer(svc)

	gs := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	pb.RegisterDriveServiceServer(gs, srv)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(gs, healthSrv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	// REST server (client-facing); calls the domain service, not the gRPC server
	e := httputil.NewEcho("drive", echomw.Logger())
	restHandler := rest.NewHandler(rest.WithOwnerNames(svc, driveStore), policypb.NewPolicyReadServiceClient(policyReadConn))
	restHandler.RegisterRoutes(e, jwtSecret)

	// Start both servers
	go func() {
		slog.Info("drive gRPC listening", "port", grpcPort)
		if err := gs.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("drive REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	bootstrap.Shutdown{GRPC: gs, Health: healthSrv, HTTP: []bootstrap.HTTPServer{e}, Cancel: cancel}.Wait()
}

// dial opens a connection to another service, or exits: drive cannot serve
// without its policy and document peers.
func dial(addr string) *grpc.ClientConn {
	conn, err := grpcauth.Dial(addr, "drive")
	if err != nil {
		slog.Error("failed to dial", "address", addr, "error", err)
		os.Exit(1)
	}
	return conn
}
