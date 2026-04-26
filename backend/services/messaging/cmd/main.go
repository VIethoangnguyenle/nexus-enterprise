package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/events"
	mgrpc "ngac-platform/services/messaging/internal/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/2")
	kafkaBrokers := envOr("KAFKA_BROKERS", "localhost:19092")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	authAddr := envOr("AUTH_SERVICE_ADDR", "localhost:50052")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	port := envOr("GRPC_PORT", "50055")
	wsPort := envOr("WS_PORT", "8081")

	pool, err := connectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpc.NewClient(policyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()

	authConn, err := grpc.NewClient(authAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("failed to connect to auth service", "address", authAddr, "error", err)
		os.Exit(1)
	}
	defer authConn.Close()

	rdb, err := connectRedis(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, local-only hub", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	hub := mgrpc.NewHub(rdb)
	defer hub.Close()

	// Start WebSocket server with graceful shutdown support
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/ws", hub.HandleWebSocket(jwtSecret))
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

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor,
			recoveryInterceptor,
		),
	)
	pb.RegisterMessagingServiceServer(srv, mgrpc.NewMessagingServer(
		pool,
		policypb.NewPolicyServiceClient(policyConn),
		authpb.NewAuthServiceClient(authConn),
		hub,
		producer,
	))

	notifSrv := mgrpc.NewNotificationServer(pool, hub)
	pb.RegisterNotificationServiceServer(srv, notifSrv)

	// Start Kafka consumer for asset events → notifications
	consumer, err := events.NewConsumer(strings.Split(kafkaBrokers, ","), notifSrv)
	if err != nil {
		slog.Warn("kafka consumer unavailable, notifications from asset events disabled", "error", err)
	}
	if consumer != nil {
		defer consumer.Close()
	}

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Graceful shutdown for both gRPC and WebSocket
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)

		healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		cancel()

		// Shutdown WebSocket server
		wsCtx, wsCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer wsCancel()
		if err := wsServer.Shutdown(wsCtx); err != nil {
			slog.Warn("websocket server shutdown error", "error", err)
		}

		// Shutdown gRPC server
		stopped := make(chan struct{})
		go func() {
			srv.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
			slog.Info("server stopped gracefully")
		case <-time.After(15 * time.Second):
			slog.Warn("graceful stop timed out, forcing stop")
			srv.Stop()
		}
	}()

	slog.Info("messaging service listening", "grpc_port", port, "ws_port", wsPort)
	if err := srv.Serve(lis); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// connectRedis creates a Redis client from a URL and verifies connectivity.
func connectRedis(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("pinging redis: %w", err)
	}
	slog.Info("redis connected", "addr", opts.Addr, "db", opts.DB)
	return rdb, nil
}

func connectDB(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 5
	cfg.MaxConnLifetime = 5 * time.Minute
	cfg.MaxConnIdleTime = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return pool, nil
}

func loggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	code := status.Code(err)
	attrs := []any{"method", info.FullMethod, "duration_ms", time.Since(start).Milliseconds(), "code", code.String()}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Warn("grpc call failed", attrs...)
	} else {
		slog.Debug("grpc call", attrs...)
	}
	return resp, err
}

func recoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered", "method", info.FullMethod, "panic", fmt.Sprintf("%v", r))
			err = status.Errorf(13, "internal server error")
		}
	}()
	return handler(ctx, req)
}
