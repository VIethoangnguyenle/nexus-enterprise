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
	"google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/ngac"
	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/approval"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/events"
	approvalGRPC "ngac-platform/services/approval/internal/grpc"
	"ngac-platform/services/approval/internal/rest"
	"ngac-platform/services/approval/internal/store"
)

func main() {
	bootstrap.InitLogger()
	if err := bootstrap.ConfigureInternalIdentity(); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable")
	grpcPort := bootstrap.Env("GRPC_PORT", "50058")
	restPort := bootstrap.Env("REST_PORT", "8080")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	policyAddr := bootstrap.PolicyAddr()
	kafkaBrokers := bootstrap.Env("KAFKA_BROKERS", "localhost:19092")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Connect to Policy Service for scope resolution and access checks
	policyConn, err := grpcauth.Dial(policyAddr, "approval")
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()
	policyClient := newPolicyAdapter(policypb.NewPolicyReadServiceClient(policyConn))

	// Tenant schema resolver — caches tenant_id → schema_name lookups
	resolver := httputil.NewTenantSchemaResolver(db)

	st := store.NewStore(db)
	svc := domain.NewService(st, policyClient)
	srv := approvalGRPC.NewServer(svc)

	gs := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	pb.RegisterApprovalServiceServer(gs, srv)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(gs, healthSrv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	// REST server with tenant schema middleware
	e := httputil.NewEcho("approval", echomw.Logger())

	// Start event producer (graceful degradation if Kafka unavailable)
	brokers := strings.Split(kafkaBrokers, ",")
	producer, err := events.NewProducer(brokers)
	if err != nil {
		slog.Warn("approval event producer unavailable — real-time events disabled", "error", err)
	}

	restHandler := rest.NewHandler(svc, resolver, producer).WithNames(st)
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

	bootstrap.Shutdown{
		GRPC:   gs,
		Health: healthSrv,
		HTTP:   []bootstrap.HTTPServer{e},
		Cancel: cancel,
		Cleanup: func() {
			if consumer != nil {
				consumer.Close()
			}
			if producer != nil {
				producer.Close()
			}
		},
	}.Wait()
}

// policyGRPCAdapter wraps PolicyReadServiceClient to implement domain.PolicyClient.
type policyGRPCAdapter struct {
	client policypb.PolicyReadServiceClient
	checks *policyclient.Client
}

func newPolicyAdapter(c policypb.PolicyReadServiceClient) *policyGRPCAdapter {
	return &policyGRPCAdapter{client: c, checks: policyclient.New(c)}
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

// CheckAccess asks the policy service; an error is never an allow.
func (a *policyGRPCAdapter) CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) (bool, error) {
	allowed, err := a.checks.Check(ctx, userNodeID, objectNodeID, operation)
	if err != nil {
		return false, fmt.Errorf("policy check access: %w", err)
	}
	return allowed, nil
}

// GetAncestors delegates to the Policy Service: every node above the given one.
func (a *policyGRPCAdapter) GetAncestors(ctx context.Context, nodeID string) ([]string, error) {
	resp, err := a.client.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: nodeID})
	if err != nil {
		return nil, fmt.Errorf("policy get ancestors: %w", err)
	}
	ids := make([]string, 0, len(resp.GetNodes()))
	for _, n := range resp.GetNodes() {
		ids = append(ids, n.GetId())
	}
	return ids, nil
}

// GetMembers returns the people (U nodes) beneath a role or department.
func (a *policyGRPCAdapter) GetMembers(ctx context.Context, uaNodeID string) ([]string, error) {
	resp, err := a.client.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: uaNodeID})
	if err != nil {
		return nil, fmt.Errorf("policy get descendants: %w", err)
	}
	var ids []string
	for _, n := range resp.GetNodes() {
		if n.GetNodeType() == ngac.TypeU {
			ids = append(ids, n.GetId())
		}
	}
	return ids, nil
}
