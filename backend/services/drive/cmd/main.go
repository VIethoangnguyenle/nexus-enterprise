package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	driveGRPC "ngac-platform/services/drive/internal/grpc"
	"ngac-platform/services/drive/internal/rest"
)

func main() {
	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	policyReadAddr := envOr("POLICY_READ_SERVICE_ADDR", policyAddr)
	docAddr := envOr("DOCUMENT_SERVICE_ADDR", "localhost:50054")
	grpcPort := envOr("GRPC_PORT", "50057")
	restPort := envOr("REST_PORT", "8080")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	policyWriteConn := dial(policyAddr)
	policyReadConn := dial(policyReadAddr)
	docConn := dial(docAddr)

	srv := driveGRPC.NewDriveServer(
		db,
		policypb.NewPolicyReadServiceClient(policyReadConn),
		policypb.NewPolicyWriteServiceClient(policyWriteConn),
		docpb.NewDocumentStorageServiceClient(docConn),
	)

	gs := grpc.NewServer()
	pb.RegisterDriveServiceServer(gs, srv)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(gs, healthSrv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	// REST server (client-facing) — delegates to gRPC server
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	restHandler := rest.NewHandler(srv)
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

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutting down drive service")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	e.Shutdown(shutdownCtx)
	gs.GracefulStop()
}

func dial(addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial %s: %v", addr, err)
	}
	return conn
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
