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

	"ngac-platform/services/auth/internal/auth"
	agrpc "ngac-platform/services/auth/internal/grpc"
	"ngac-platform/services/auth/internal/store"
	pb "ngac-platform/proto/auth"
	policypb "ngac-platform/proto/policy"
)

func main() {
	ctx := context.Background()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	port := envOr("GRPC_PORT", "50052")

	auth.SetJWTSecret(jwtSecret)

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Connect to Policy Service
	policyConn, err := grpc.NewClient(policyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Policy Service: %v", err)
	}
	defer policyConn.Close()
	policyClient := policypb.NewPolicyServiceClient(policyConn)

	s := store.New(pool)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterAuthServiceServer(srv, agrpc.NewAuthServer(s, policyClient))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	log.Printf("Auth Service listening on :%s", port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
