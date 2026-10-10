package grpcauth

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestCallerFromEmptyContext(t *testing.T) {
	if got := CallerFrom(context.Background()); got != (Caller{}) {
		t.Fatalf("want zero caller, got %+v", got)
	}
}

func TestCallerRoundTripThroughContext(t *testing.T) {
	want := Caller{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"}
	if got := CallerFrom(WithCaller(context.Background(), want)); got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestCallerAuthenticatedNeedsUserAndNode(t *testing.T) {
	cases := map[string]struct {
		c    Caller
		want bool
	}{
		"both":       {Caller{UserID: "u", NGACNodeID: "n"}, true},
		"no node":    {Caller{UserID: "u"}, false},
		"no user":    {Caller{NGACNodeID: "n"}, false},
		"zero":       {Caller{}, false},
		"tenantonly": {Caller{TenantID: "t"}, false},
	}
	for name, tc := range cases {
		if got := tc.c.Authenticated(); got != tc.want {
			t.Errorf("%s: got %v want %v", name, got, tc.want)
		}
	}
}

// startServer runs a health server behind the server interceptor and returns a
// client connection built with the client interceptor.
func startServer(t *testing.T, policy ServerPolicy, svc string, handlerCtx *context.Context) grpc_health_v1.HealthClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		ServerInterceptor(policy),
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			if handlerCtx != nil {
				*handlerCtx = ctx
			}
			return h(ctx, req)
		},
	))
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(ClientInterceptor(svc)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return grpc_health_v1.NewHealthClient(conn)
}

const checkMethod = "/grpc.health.v1.Health/Check"

func TestServerRejectsMissingCaller(t *testing.T) {
	c := startServer(t, ServerPolicy{}, "", nil)
	_, err := c.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestServerPassesCallerToHandler(t *testing.T) {
	var got context.Context
	c := startServer(t, ServerPolicy{}, "", &got)
	want := Caller{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"}
	if _, err := c.Check(WithCaller(context.Background(), want), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if CallerFrom(got) != want {
		t.Fatalf("handler saw %+v want %+v", CallerFrom(got), want)
	}
}

func TestServerRejectsIncompleteCaller(t *testing.T) {
	c := startServer(t, ServerPolicy{}, "", nil)
	_, err := c.Check(WithCaller(context.Background(), Caller{UserID: "u1"}), &grpc_health_v1.HealthCheckRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestServerExemptMethodNeedsNothing(t *testing.T) {
	c := startServer(t, ServerPolicy{Exempt: map[string]string{checkMethod: "health probe"}}, "", nil)
	if _, err := c.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("exempt method rejected: %v", err)
	}
}

func TestServerServiceMethodNeedsServiceIdentity(t *testing.T) {
	p := ServerPolicy{ServiceOK: map[string]string{checkMethod: "bootstrap"}}

	anon := startServer(t, p, "", nil)
	if _, err := anon.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no identity: want Unauthenticated, got %v", err)
	}

	var got context.Context
	named := startServer(t, p, "auth", &got)
	if _, err := named.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatalf("service identity rejected: %v", err)
	}
	if ServiceFrom(got) != "auth" {
		t.Fatalf("service = %q", ServiceFrom(got))
	}
}

func TestServiceIdentityDoesNotUnlockOtherMethods(t *testing.T) {
	c := startServer(t, ServerPolicy{ServiceOK: map[string]string{"/other/Method": "x"}}, "auth", nil)
	if _, err := c.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestClientDoesNotForwardForgedOutgoingMetadata(t *testing.T) {
	var got context.Context
	c := startServer(t, ServerPolicy{}, "", &got)
	real := Caller{UserID: "real", NGACNodeID: "real-node"}
	// A caller smuggling its own metadata must lose to the verified context.
	ctx := metadata.AppendToOutgoingContext(WithCaller(context.Background(), real),
		KeyUserID, "forged", KeyNGACNodeID, "forged-node")
	if _, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if CallerFrom(got) != real {
		t.Fatalf("got %+v want %+v", CallerFrom(got), real)
	}
}

func TestClientStripsForgedMetadataWithoutCaller(t *testing.T) {
	c := startServer(t, ServerPolicy{}, "", nil)
	ctx := metadata.AppendToOutgoingContext(context.Background(), KeyUserID, "forged", KeyNGACNodeID, "forged-node")
	if _, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

// startStreamServer serves health (Watch is a server stream) behind the
// options every service uses, so the stream path is tested as wired.
func startStreamServer(t *testing.T, p ServerPolicy) grpc_health_v1.HealthClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer(ServerOptions(p)...)
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(ClientInterceptor("")))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return grpc_health_v1.NewHealthClient(conn)
}

func watchErr(t *testing.T, c grpc_health_v1.HealthClient, ctx context.Context) error {
	t.Helper()
	stream, err := c.Watch(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}
	_, err = stream.Recv()
	return err
}

func TestStreamRejectsMissingCaller(t *testing.T) {
	c := startStreamServer(t, ServerPolicy{})
	if err := watchErr(t, c, context.Background()); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

func TestStreamAdmitsCaller(t *testing.T) {
	c := startStreamServer(t, ServerPolicy{})
	ctx := metadata.AppendToOutgoingContext(context.Background(), KeyUserID, "u1", KeyNGACNodeID, "n1")
	if err := watchErr(t, c, ctx); err != nil {
		t.Fatalf("want first watch update, got %v", err)
	}
}

func TestStreamHealthWatchIsExempt(t *testing.T) {
	c := startStreamServer(t, ServerPolicy{Exempt: HealthExempt()})
	if err := watchErr(t, c, context.Background()); err != nil {
		t.Fatalf("want first watch update, got %v", err)
	}
}

func TestClientRefusesIncompleteCaller(t *testing.T) {
	c := startServer(t, ServerPolicy{Exempt: map[string]string{checkMethod: "test"}}, "svc", nil)
	// A caller with a user but no node must fail, not be downgraded to the
	// service identity, or a lost node id would silently become service rights.
	ctx := WithCaller(context.Background(), Caller{UserID: "u1"})
	if _, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}
