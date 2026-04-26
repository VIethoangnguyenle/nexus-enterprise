package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	wgrpc "ngac-platform/services/workspace/internal/grpc"
	policypb "ngac-platform/proto/policy"
	pb "ngac-platform/proto/workspace"
)

func main() {
	ctx := context.Background()
	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	port := envOr("GRPC_PORT", "50053")

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	policyConn, err := grpc.NewClient(policyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Policy Service: %v", err)
	}
	defer policyConn.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterWorkspaceServiceServer(srv, wgrpc.NewWorkspaceServer(pool, policypb.NewPolicyServiceClient(policyConn)))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	log.Printf("Workspace Service listening on :%s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
