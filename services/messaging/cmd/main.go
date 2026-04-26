package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	mgrpc "ngac-platform/services/messaging/internal/grpc"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
)

func main() {
	ctx := context.Background()
	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	authAddr := envOr("AUTH_SERVICE_ADDR", "localhost:50052")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	port := envOr("GRPC_PORT", "50055")
	wsPort := envOr("WS_PORT", "8081")

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

	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Auth Service: %v", err)
	}
	defer authConn.Close()

	hub := mgrpc.NewHub()

	// Start WebSocket server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.HandleWebSocket(jwtSecret))
	go func() {
		log.Printf("WebSocket server listening on :%s", wsPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%s", wsPort), mux); err != nil {
			log.Fatalf("WebSocket server error: %v", err)
		}
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterMessagingServiceServer(srv, mgrpc.NewMessagingServer(
		pool, policypb.NewPolicyServiceClient(policyConn),
		authpb.NewAuthServiceClient(authConn), hub,
	))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	log.Printf("Messaging Service listening on :%s (gRPC) and :%s (WebSocket)", port, wsPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
