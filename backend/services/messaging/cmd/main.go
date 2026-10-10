package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/bootstrap/redisconn"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/realtime"
	authpb "ngac-platform/proto/auth"
	drivepb "ngac-platform/proto/drive"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/events"
	mgrpc "ngac-platform/services/messaging/internal/grpc"
	"ngac-platform/services/messaging/internal/rest"
	"ngac-platform/services/messaging/internal/store"
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
	redisURL := bootstrap.Env("REDIS_URL", "redis://localhost:6379/2")
	kafkaBrokers := bootstrap.Env("KAFKA_BROKERS", "localhost:19092")
	policyAddr := bootstrap.PolicyAddr()
	authAddr := bootstrap.Env("AUTH_SERVICE_ADDR", "localhost:50052")
	driveAddr := bootstrap.Env("DRIVE_SERVICE_ADDR", "localhost:50057")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	port := bootstrap.Env("GRPC_PORT", "50055")
	wsPort := bootstrap.Env("WS_PORT", "8081")
	restPort := bootstrap.Env("REST_PORT", "8080")

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpcauth.Dial(policyAddr, "messaging")
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()

	authConn, err := grpcauth.Dial(authAddr, "messaging")
	if err != nil {
		slog.Error("failed to connect to auth service", "address", authAddr, "error", err)
		os.Exit(1)
	}
	defer authConn.Close()

	driveConn, err := grpcauth.Dial(driveAddr, "messaging")
	if err != nil {
		slog.Warn("drive service unavailable, channel drives disabled", "address", driveAddr, "error", err)
	}
	if driveConn != nil {
		defer driveConn.Close()
	}

	rdb, err := redisconn.Connect(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, local-only hub", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	var driveClient drivepb.DriveServiceClient
	if driveConn != nil {
		driveClient = drivepb.NewDriveServiceClient(driveConn)
	}

	// Wire: Store → Domain → Handler
	msgStore := store.NewStore(pool)
	domainSvc := domain.NewService(
		msgStore,
		policypb.NewPolicyReadServiceClient(policyConn),
		policypb.NewPolicyWriteServiceClient(policyConn),
		authpb.NewAuthServiceClient(authConn),
		driveClient,
	)

	// The hub authorizes every channel subscription through the domain
	// service: a WebSocket subscription is a read of the channel.
	hub := mgrpc.NewHub(rdb, domainSvc)
	defer hub.Close()
	// Following a workspace's live changes is a read of the workspace.
	hub.SetWorkspaceAccess(domainSvc)
	// Channel changes (create, rename, members) are announced like every other
	// domain's: through Redpanda, after the commit.
	rt := realtime.Connect("messaging")
	defer rt.Close()
	domainSvc.SetEmitter(rt)
	// Removing a channel member also ends their live subscriptions.
	domainSvc.SetSubscriptionRevoker(hub)

	// Start WebSocket server with graceful shutdown support
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/api/ws", hub.HandleWebSocket(jwtSecret))
	wsServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", wsPort),
		Handler:      wsMux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		slog.Info("websocket server listening", "port", wsPort)
		if err := wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("websocket server error", "error", err)
		}
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	producer, err := events.NewProducer(strings.Split(kafkaBrokers, ","))
	if err != nil {
		slog.Warn("kafka unavailable, event streaming disabled", "error", err)
	}
	if producer != nil {
		defer producer.Close()
	}

	srv := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	pb.RegisterMessagingServiceServer(srv, mgrpc.NewMessagingServer(domainSvc, hub, producer))

	notifications := domain.NewNotificationService(msgStore, hub)
	pb.RegisterNotificationServiceServer(srv, mgrpc.NewNotificationServer(notifications))

	// Start Kafka consumer for asset events → notifications
	consumer, err := events.NewConsumer(strings.Split(kafkaBrokers, ","), notifications, hub)
	if err != nil {
		slog.Warn("kafka consumer unavailable, notifications from asset events disabled", "error", err)
	}
	if consumer != nil {
		defer consumer.Close()
	}

	// Every domain's committed changes reach the hub through this consumer.
	rtConsumer, err := events.NewRealtimeConsumer(strings.Split(kafkaBrokers, ","), hub)
	if err != nil {
		slog.Warn("realtime consumer unavailable, live workspace updates disabled", "error", err)
	}
	if rtConsumer != nil {
		defer rtConsumer.Close()
	}

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := httputil.NewEcho("messaging", echomw.Logger())
	restHandler := rest.NewHandler(domainSvc, notifications, hub)
	restHandler.RegisterRoutes(e, jwtSecret)

	// Graceful shutdown for the gRPC, WebSocket and REST servers.
	go bootstrap.Shutdown{
		GRPC:   srv,
		Health: healthSrv,
		HTTP:   []bootstrap.HTTPServer{wsServer, e},
		Cancel: cancel,
	}.Wait()
	go func() {
		slog.Info("messaging REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	slog.Info("messaging service listening", "grpc_port", port, "ws_port", wsPort, "rest_port", restPort)
	if err := srv.Serve(lis); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}
