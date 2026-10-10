package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/bootstrap/redisconn"
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
	bootstrap.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	redisURL := bootstrap.Env("REDIS_URL", "redis://localhost:6379/1")
	policyAddr := bootstrap.PolicyAddr()
	workspaceAddr := bootstrap.Env("WORKSPACE_SERVICE_ADDR", "localhost:50053")
	messagingAddr := bootstrap.Env("MESSAGING_SERVICE_ADDR", "localhost:50055")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	grpcPort := bootstrap.Env("GRPC_PORT", "50052")
	restPort := bootstrap.Env("REST_PORT", "8080")

	auth.SetJWTSecret(jwtSecret)

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpcauth.Dial(policyAddr, "auth")
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()
	policyRead := policypb.NewPolicyReadServiceClient(policyConn)
	policyWrite := policypb.NewPolicyWriteServiceClient(policyConn)

	// Workspace gRPC client (for auto-provisioning on register)
	var wsClient workspacepb.WorkspaceServiceClient
	wsConn, err := grpcauth.Dial(workspaceAddr, "auth")
	if err != nil {
		slog.Warn("workspace service unavailable, auto-provision disabled", "address", workspaceAddr, "error", err)
	} else {
		defer wsConn.Close()
		wsClient = workspacepb.NewWorkspaceServiceClient(wsConn)
	}

	// Messaging gRPC client (for auto-provisioning #general channel)
	var msgClient messagingpb.MessagingServiceClient
	msgConn, err := grpcauth.Dial(messagingAddr, "auth")
	if err != nil {
		slog.Warn("messaging service unavailable, auto-provision disabled", "address", messagingAddr, "error", err)
	} else {
		defer msgConn.Close()
		msgClient = messagingpb.NewMessagingServiceClient(msgConn)
	}

	st := store.New(pool)

	rdb, err := redisconn.Connect(ctx, redisURL)
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

	srv := grpc.NewServer(grpcauth.ServerOptions(agrpc.AuthPolicy())...)
	pb.RegisterAuthServiceServer(srv, agrpc.NewAuthServer(svc, rdb))

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// REST server (client-facing)
	e := httputil.NewEcho("auth", echomw.LoggerWithConfig(echomw.LoggerConfig{
		// The access log records the full URI, and the Google callback's query
		// carries the one-time authorization code. The handler logs the
		// outcome of that request itself, without the code.
		Skipper: func(c echo.Context) bool {
			return c.Request().URL.Path == "/api/auth/google/callback"
		},
	}))
	// Who is asking, for the per-address limits. Nothing is trusted unless the
	// proxy networks are listed: X-Forwarded-For is a header any client writes.
	extractIP, err := rest.NewIPExtractor(strings.Split(os.Getenv("AUTH_TRUSTED_PROXIES"), ","))
	if err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}
	e.IPExtractor = extractIP
	restHandler := rest.NewHandler(svc)
	if v := os.Getenv("AUTH_PUBLIC_RATE_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			slog.Error("refusing to start: AUTH_PUBLIC_RATE_LIMIT must be a number of requests per minute (0 turns it off)")
			os.Exit(1)
		}
		restHandler.SetPublicLimit(rest.PublicLimit{Max: n, Window: time.Minute})
	}
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

	bootstrap.Shutdown{GRPC: srv, Health: healthSrv, HTTP: []bootstrap.HTTPServer{e}, Cancel: cancel}.Wait()
}

var otpCodeFormat = regexp.MustCompile(`^[0-9]{6}$`)

// otpOptions configures OTP sign-in from the environment.
//
// AUTH_FIXED_OTP_CODE selects the mode. Unset → the documented test-only
// fixed code "999999"; any six digits → that fixed code; set but EMPTY →
// random codes, delivered by a CodeSender: the SMTP sender when SMTP_HOST is
// set (codes then reach the mailbox owner and prove the address), otherwise
// the dev-only LogSender (APP_ENV=dev or AUTH_DEV_OTP=1). With neither,
// random-code OTP is disabled and /api/auth/providers reports otp=false.
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

	sender, err := smtpSender()
	if err != nil {
		return opts, err
	}
	switch {
	case sender != nil:
		opts.Sender = sender
	case domain.DevOTPMode():
		opts.Sender = domain.LogSender{}
		slog.Info("OTP random codes delivered to the log (dev mode only)")
	default:
		slog.Warn("OTP sign-in disabled: fixed code is off and no code sender is configured")
	}
	return opts, nil
}

// smtpSender builds the email sender from SMTP_HOST, SMTP_PORT (default 587),
// SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM and the optional SMTP_TLS
// ("starttls" or "tls"; default by port). It returns nil when SMTP_HOST is
// unset, and an error when it is set but the rest is unusable, so a half
// configured production never silently falls back to no sign-in.
func smtpSender() (*domain.SMTPSender, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return nil, nil
	}
	s, err := domain.NewSMTPSender(domain.SMTPConfig{
		Host:     host,
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		TLS:      os.Getenv("SMTP_TLS"),
	})
	if err != nil {
		return nil, fmt.Errorf("SMTP_HOST is set but the SMTP configuration is unusable: %w", err)
	}
	// Host and from address are not secrets; the password never is logged.
	slog.Info("OTP codes delivered by email over SMTP", "host", host, "port", os.Getenv("SMTP_PORT"))
	return s, nil
}

// googleOptions configures "Sign in with Google" from the environment.
// Without GOOGLE_CLIENT_ID (or without Redis, which holds in-flight sign-ins)
// the feature is off: /api/auth/google/start answers 503 and
// /api/auth/providers reports it unavailable, so the login page hides it.
func googleOptions(rdb *redis.Client) rest.GoogleOptions {
	opts := rest.GoogleOptions{
		AppBaseURL: strings.TrimRight(bootstrap.Env("APP_BASE_URL", "http://localhost:5173"), "/"),
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
	redirectURL := bootstrap.Env("GOOGLE_REDIRECT_URL", "http://localhost:5173/api/auth/google/callback")

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
