package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	assetpb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	"ngac-platform/services/asset/internal/events"
	agrpc "ngac-platform/services/asset/internal/grpc"
	"ngac-platform/services/asset/internal/rest"
	"ngac-platform/services/asset/internal/store"
)

func main() {
	bootstrap.InitLogger()
	if err := bootstrap.ConfigureInternalIdentity(); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := bootstrap.PolicyAddr()
	kafkaBrokers := bootstrap.Env("KAFKA_BROKERS", "localhost:19092")
	port := bootstrap.Env("GRPC_PORT", "50056")
	restPort := bootstrap.Env("REST_PORT", "8080")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpcauth.Dial(policyAddr, "asset")
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

	srv := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	assetTypes := domain.NewAssetTypeService(assetStore, policyRead, policyWrite)
	assets := domain.NewAssetService(assetStore, policyRead, producer)
	assetRequests := domain.NewAssetRequestService(assetStore, policyRead, policyWrite, producer)

	assetpb.RegisterAssetTypeServiceServer(srv, agrpc.ServeAssetTypeService(assetTypes))
	assetpb.RegisterAssetServiceServer(srv, agrpc.ServeAssetService(assets))
	assetpb.RegisterAssetRequestServiceServer(srv, agrpc.ServeAssetRequestService(assetRequests))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := httputil.NewEcho("asset", echomw.Logger())
	restHandler := rest.NewHandler(assets, assetTypes, assetRequests)
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

	bootstrap.Shutdown{GRPC: srv, Health: healthSrv, HTTP: []bootstrap.HTTPServer{e}, Cancel: cancel}.Wait()
}
