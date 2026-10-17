package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	_ "time/tzdata" // zone data for event time zones, whatever the image carries

	echomw "github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/domain"
	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/rest"
	"ngac-platform/services/calendar/internal/store"
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
	policyAddr := bootstrap.PolicyAddr()
	port := bootstrap.Env("GRPC_PORT", "50059")
	restPort := bootstrap.Env("REST_PORT", "8187")
	jwtSecret := bootstrap.Env("JWT_SECRET", httputil.DevJWTSecret)
	if err := httputil.RequireJWTSecret(jwtSecret); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	pool, err := bootstrap.ConnectDB(ctx, dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyConn, err := grpcauth.Dial(policyAddr, "calendar")
	if err != nil {
		slog.Error("failed to connect to policy service", "address", policyAddr, "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()

	// A broker that is down costs the realtime nudge, never a request.
	frames := realtime.Connect("calendar")
	defer frames.Close()

	// Notices to the people an event concerns, and reminders. Without a broker the
	// calendar still works; nobody is notified.
	notices, err := events.NewProducer(strings.Split(bootstrap.Env("KAFKA_BROKERS", "localhost:19092"), ","))
	if err != nil {
		slog.Warn("kafka unavailable, calendar notifications disabled", "error", err)
	}
	defer notices.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		slog.Error("failed to listen", "port", port, "error", err)
		os.Exit(1)
	}

	// The calendar has no gRPC surface of its own yet; the server carries the
	// health probe the container orchestration uses.
	srv := grpc.NewServer(grpcauth.ServerOptions(grpcauth.ServerPolicy{Exempt: grpcauth.HealthExempt()})...)
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	calendars := domain.NewService(
		store.New(pool),
		policypb.NewPolicyReadServiceClient(policyConn),
		policypb.NewPolicyWriteServiceClient(policyConn),
		frames,
	)
	if notices != nil {
		calendars.SetNotices(notices)
	}
	e := httputil.NewEcho("calendar", echomw.Logger())
	rest.NewHandler(calendars).RegisterRoutes(e, jwtSecret)

	go func() {
		slog.Info("calendar gRPC listening", "port", port)
		if err := srv.Serve(lis); err != nil {
			slog.Error("grpc server exited", "error", err)
		}
	}()
	go func() {
		slog.Info("calendar REST listening", "port", restPort)
		if err := e.Start(fmt.Sprintf(":%s", restPort)); err != nil {
			slog.Info("rest server stopped", "error", err)
		}
	}()

	// Due reminders are sent every 30 seconds until shutdown, and the pool is not
	// closed under a pass that is still running.
	reminders := make(chan struct{})
	go func() {
		defer close(reminders)
		calendars.RunReminders(ctx)
	}()

	bootstrap.Shutdown{GRPC: srv, Health: healthSrv, HTTP: []bootstrap.HTTPServer{e}, Cancel: cancel}.Wait()
	<-reminders
}
