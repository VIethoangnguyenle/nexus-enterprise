package main

import (
	"context"
	"fmt"
	"log"
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
	"google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/approval"
	policypb "ngac-platform/proto/policy"
	approvalGRPC "ngac-platform/services/approval/internal/grpc"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/events"
	"ngac-platform/services/approval/internal/rest"
	"ngac-platform/services/approval/internal/store"
)

func main() {
	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable")
	grpcPort := envOr("GRPC_PORT", "50058")
	restPort := envOr("REST_PORT", "8080")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	policyAddr := envOr("POLICY_ADDR", "localhost:50051")
	kafkaBrokers := envOr("KAFKA_BROKERS", "localhost:19092")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	// Connect to Policy Service for scope resolution and access checks
	policyConn, err := grpc.NewClient(policyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connect policy service: %v", err)
	}
	defer policyConn.Close()
	policyClient := &policyGRPCAdapter{client: policypb.NewPolicyReadServiceClient(policyConn)}

	// Tenant schema resolver — caches tenant_id → schema_name lookups
	resolver := httputil.NewTenantSchemaResolver(db)

	st := store.NewStore(db)
	svc := domain.NewService(st, policyClient)
	srv := approvalGRPC.NewServer(svc)

	gs := grpc.NewServer()
	pb.RegisterApprovalServiceServer(gs, srv)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(gs, healthSrv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	// REST server with tenant schema middleware
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())

	// Start event producer (graceful degradation if Kafka unavailable)
	brokers := strings.Split(kafkaBrokers, ",")
	producer, err := events.NewProducer(brokers)
	if err != nil {
		slog.Warn("approval event producer unavailable — real-time events disabled", "error", err)
	}

	restHandler := rest.NewHandler(svc, resolver, producer)
	restHandler.RegisterRoutes(e, jwtSecret)

	// Start reconciliation consumer (graceful degradation if Kafka unavailable)
	consumer, err := events.NewReconciliationConsumer(brokers, st)
	if err != nil {
		slog.Warn("reconciliation consumer unavailable — policy changes will not auto-reconcile", "error", err)
	} else {
		go func() {
			slog.Info("reconciliation consumer started")
			consumer.Run(ctx)
		}()
	}

	// Start both servers
	go func() {
		slog.Info("approval gRPC listening", "port", grpcPort)
		if err := gs.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("approval REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	slog.Info("shutting down approval service")
	cancel()

	if consumer != nil {
		consumer.Close()
	}
	if producer != nil {
		producer.Close()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	e.Shutdown(shutdownCtx)
	gs.GracefulStop()
}

// policyGRPCAdapter wraps PolicyReadServiceClient to implement domain.PolicyClient.
type policyGRPCAdapter struct {
	client policypb.PolicyReadServiceClient
}

// ResolveAccessibleScopes delegates to the Policy Service gRPC call.
func (a *policyGRPCAdapter) ResolveAccessibleScopes(ctx context.Context, userNodeID, operation string) ([]string, error) {
	resp, err := a.client.ResolveAccessibleScopes(ctx, &policypb.ResolveAccessibleScopesRequest{
		UserNodeId: userNodeID,
		Operation:  operation,
	})
	if err != nil {
		return nil, fmt.Errorf("policy resolve scopes: %w", err)
	}
	return resp.ScopeOaIds, nil
}

// CheckAccess delegates to the Policy Service CheckAccess RPC.
func (a *policyGRPCAdapter) CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) (bool, error) {
	resp, err := a.client.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId:   userNodeID,
		ObjectNodeId: objectNodeID,
		Operation:    operation,
	})
	if err != nil {
		return false, fmt.Errorf("policy check access: %w", err)
	}
	return resp.Decision == "allow", nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
