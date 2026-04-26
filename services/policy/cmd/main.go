package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/services/policy/internal/ngac"
	pgrpc "ngac-platform/services/policy/internal/grpc"
	pb "ngac-platform/proto/policy"
)

func main() {
	ctx := context.Background()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable"
	}
	port := os.Getenv("GRPC_PORT")
	if port == "" {
		port = "50051"
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	graph := ngac.NewGraph()
	store := ngac.NewStore(pool, graph)

	// Initialize schema
	if err := store.InitSchema(ctx); err != nil {
		log.Fatalf("Failed to init schema: %v", err)
	}
	log.Println("Schema initialized")

	// Load graph from DB
	if err := store.LoadGraph(ctx); err != nil {
		log.Fatalf("Failed to load graph: %v", err)
	}
	log.Printf("Graph loaded: %d nodes", len(graph.Nodes))

	// Register constraint engine
	ce := ngac.NewConstraintEngine()
	ce.Register(ngac.WeekdayOnlyConstraint)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterPolicyServiceServer(srv, pgrpc.NewPolicyServer(store, ce))

	// Register health check
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	log.Printf("Policy Service listening on :%s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
