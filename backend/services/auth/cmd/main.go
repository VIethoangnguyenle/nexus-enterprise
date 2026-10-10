package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/auth"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/googleauth"
	agrpc "ngac-platform/services/auth/internal/grpc"
	"ngac-platform/services/auth/internal/rest"
	"ngac-platform/services/auth/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := envOr("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := envOr("REDIS_URL", "redis://localhost:6379/1")
	policyAddr := envOr("POLICY_SERVICE_ADDR", "localhost:50051")
	workspaceAddr := envOr("WORKSPACE_SERVICE_ADDR", "localhost:50053")
	messagingAddr := envOr("MESSAGING_SERVICE_ADDR", "localhost:50055")
	jwtSecret := envOr("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	grpcPort := envOr("GRPC_PORT", "50052")
	restPort := envOr("REST_PORT", "8080")

	auth.SetJWTSecret(jwtSecret)

	pool, err := connectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpc.NewClient(policyAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcauth.ClientInterceptor("auth")))
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()
	policyRead := policypb.NewPolicyReadServiceClient(policyConn)
	policyWrite := policypb.NewPolicyWriteServiceClient(policyConn)

	// Workspace gRPC client (for auto-provisioning on register)
	var wsClient workspacepb.WorkspaceServiceClient
	wsConn, err := grpc.NewClient(workspaceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcauth.ClientInterceptor("auth")))
	if err != nil {
		slog.Warn("workspace service unavailable, auto-provision disabled", "address", workspaceAddr, "error", err)
	} else {
		defer wsConn.Close()
		wsClient = workspacepb.NewWorkspaceServiceClient(wsConn)
	}

	// Messaging gRPC client (for auto-provisioning #general channel)
	var msgClient messagingpb.MessagingServiceClient
	msgConn, err := grpc.NewClient(messagingAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(grpcauth.ClientInterceptor("auth")))
	if err != nil {
		slog.Warn("messaging service unavailable, auto-provision disabled", "address", messagingAddr, "error", err)
	} else {
		defer msgConn.Close()
		msgClient = messagingpb.NewMessagingServiceClient(msgConn)
	}

	st := store.New(pool)

	rdb, err := connectRedis(ctx, redisURL)
	if err != nil {
		slog.Warn("redis unavailable, jwt blacklist disabled", "error", err)
	}
	if rdb != nil {
		defer rdb.Close()
	}

	// Domain service — shared by gRPC and REST handlers
	svc := domain.NewService(st, rdb, policyRead, policyWrite, wsClient, msgClient)
	otpOpts, err := otpOptions(jwtSecret)
	if err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	svc.ConfigureOTP(otpOpts)

	// gRPC server (service-to-service)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcPort))
	if err != nil {
		slog.Error("failed to listen", "port", grpcPort, "error", err)
		os.Exit(1)
	}

	srv := grpc.NewServer(grpcauth.ServerOptions(agrpc.AuthPolicy(), loggingInterceptor, recoveryInterceptor)...)
	pb.RegisterAuthServiceServer(srv, agrpc.NewAuthServer(svc, rdb))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.LoggerWithConfig(echomw.LoggerConfig{
		// The access log records the full URI, and the Google callback's query
		// carries the one-time authorization code. The handler logs the
		// outcome of that request itself, without the code.
		Skipper: func(c echo.Context) bool {
			return c.Request().URL.Path == "/api/auth/google/callback"
		},
	}))
	e.Use(echomw.Recover())
	restHandler := rest.NewHandler(svc)
	restHandler.EnableGoogle(googleOptions(rdb))
	restHandler.RegisterRoutes(e, jwtSecret)

	// Start both servers
	go func() {
		slog.Info("auth gRPC listening", "port", grpcPort)
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("auth REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	gracefulShutdown(srv, healthSrv, e, cancel)
}

// envOr returns the environment variable value or a fallback default.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var otpCodeFormat = regexp.MustCompile(`^[0-9]{6}$`)

// otpOptions configures OTP sign-in from the environment.
//
// AUTH_FIXED_OTP_CODE selects the mode. Unset → the documented test-only
// fixed code "999999"; any six digits → that fixed code; set but EMPTY →
// random codes, delivered by a CodeSender. The only sender today is the
// dev-only LogSender (APP_ENV=dev or AUTH_DEV_OTP=1); without it random-code
// OTP is disabled and /api/auth/providers reports otp=false.
func otpOptions(jwtSecret string) (domain.OTPOptions, error) {
	// Derive the at-rest HMAC key from the shared JWT secret so every auth
	// instance verifies codes the others issued, without a new secret to manage.
	key := sha256.Sum256([]byte("auth-otp-code-hmac\x00" + jwtSecret))
	opts := domain.OTPOptions{Secret: key[:]}

	fixed, set := os.LookupEnv("AUTH_FIXED_OTP_CODE")
	if !set {
		fixed = domain.DefaultFixedOTPCode
	}
	fixed = strings.TrimSpace(fixed)
	if fixed != "" {
		if !otpCodeFormat.MatchString(fixed) {
			return opts, fmt.Errorf("AUTH_FIXED_OTP_CODE must be six digits, or empty to disable the fixed code")
		}
		opts.FixedCode = fixed
		slog.Warn("OTP fixed-code TEST MODE is on: every OTP sign-in accepts the configured code. " +
			"Set AUTH_FIXED_OTP_CODE= (empty) to use random delivered codes.")
		return opts, nil
	}

	if domain.DevOTPMode() {
		opts.Sender = domain.LogSender{}
		slog.Info("OTP random codes delivered to the log (dev mode only)")
	} else {
		slog.Warn("OTP sign-in disabled: fixed code is off and no code sender is configured")
	}
	return opts, nil
}

// googleOptions configures "Sign in with Google" from the environment.
// Without GOOGLE_CLIENT_ID (or without Redis, which holds in-flight sign-ins)
// the feature is off: /api/auth/google/start answers 503 and
// /api/auth/providers reports it unavailable, so the login page hides it.
func googleOptions(rdb *redis.Client) rest.GoogleOptions {
	opts := rest.GoogleOptions{
		AppBaseURL: strings.TrimRight(envOr("APP_BASE_URL", "http://localhost:5173"), "/"),
	}
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	if clientID == "" {
		slog.Info("google sign-in disabled: GOOGLE_CLIENT_ID not set")
		return opts
	}
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientSecret == "" {
		slog.Warn("google sign-in disabled: GOOGLE_CLIENT_SECRET not set")
		return opts
	}
	if rdb == nil {
		slog.Warn("google sign-in disabled: redis unavailable")
		return opts
	}
	redirectURL := envOr("GOOGLE_REDIRECT_URL", "http://localhost:5173/api/auth/google/callback")

	opts.Provider = googleauth.New(googleauth.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	})
	opts.Flows = googleauth.NewRedisFlowStore(rdb)
	// Never log the secret; the client ID and URLs are not sensitive.
	slog.Info("google sign-in enabled", "redirect_url", redirectURL, "app_base_url", opts.AppBaseURL)
	return opts
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

// connectDB creates a pgxpool with production-ready pool configuration.
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

// gracefulShutdown waits for SIGINT/SIGTERM and drains in-flight requests.
func gracefulShutdown(srv *grpc.Server, healthSrv *health.Server, echoSrv *echo.Echo, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig)

	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown Echo REST server
	if err := echoSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("echo shutdown error", "error", err)
	}

	// Graceful stop gRPC server
	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
		slog.Info("server stopped gracefully")
	case <-shutdownCtx.Done():
		slog.Warn("graceful stop timed out, forcing stop")
		srv.Stop()
	}
}

// loggingInterceptor logs every gRPC call with method, duration, and status code.
func loggingInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	code := status.Code(err)
	attrs := []any{
		"method", info.FullMethod,
		"duration_ms", time.Since(start).Milliseconds(),
		"code", code.String(),
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
		slog.Warn("grpc call failed", attrs...)
	} else {
		slog.Debug("grpc call", attrs...)
	}
	return resp, err
}

// recoveryInterceptor catches panics in handlers and returns Internal error.
func recoveryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in grpc handler",
				"method", info.FullMethod,
				"panic", fmt.Sprintf("%v", r),
			)
			err = status.Errorf(13, "internal server error")
		}
	}()
	return handler(ctx, req)
}
