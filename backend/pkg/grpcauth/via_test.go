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

// viaPolicy restricts Check, for a user caller, to requests sent by workspace.
func viaPolicy() ServerPolicy {
	return ServerPolicy{UserVia: map[string]ServiceRule{checkMethod: ServiceOnly("test", "workspace")}}
}

var someUser = Caller{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"}

func check(c grpc_health_v1.HealthClient, ctx context.Context) error {
	_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	return err
}

func wantStatus(t *testing.T, what string, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("%s: code = %v, want %v (err: %v)", what, got, want, err)
	}
}

func TestUserViaAdmitsAUserSentByAListedService(t *testing.T) {
	var got context.Context
	c := startServer(t, viaPolicy(), "workspace", &got)
	if err := check(c, WithCaller(context.Background(), someUser)); err != nil {
		t.Fatalf("user through workspace refused: %v", err)
	}
	if CallerFrom(got) != someUser {
		t.Fatalf("handler saw %+v want %+v", CallerFrom(got), someUser)
	}
	if ServiceFrom(got) != "" {
		t.Fatalf("a user request must not read as a service request, got %q", ServiceFrom(got))
	}
}

func TestUserViaRefusesAUserSentByAnUnlistedService(t *testing.T) {
	c := startServer(t, viaPolicy(), "document", nil)
	wantStatus(t, "user through document", check(c, WithCaller(context.Background(), someUser)), codes.PermissionDenied)
}

// A client that names no service sends no proof of where the call comes from,
// which a restricted method refuses rather than guesses.
func TestUserViaRefusesAUserWithNoCallingService(t *testing.T) {
	c := startServer(t, viaPolicy(), "", nil)
	wantStatus(t, "user with no service", check(c, WithCaller(context.Background(), someUser)), codes.PermissionDenied)
}

// A method the policy does not restrict keeps admitting any user, from anywhere.
func TestUserViaLeavesOtherMethodsOpenToUsers(t *testing.T) {
	p := ServerPolicy{UserVia: map[string]ServiceRule{"/other/Method": ServiceOnly("test", "workspace")}}
	for _, svc := range []string{"", "document"} {
		c := startServer(t, p, svc, nil)
		if err := check(c, WithCaller(context.Background(), someUser)); err != nil {
			t.Fatalf("user through %q refused on an unrestricted method: %v", svc, err)
		}
	}
}

// A service identity is governed by ServiceOK alone: being listed for users
// on a method does not let the service call it without one.
func TestUserViaDoesNotAdmitABareService(t *testing.T) {
	c := startServer(t, viaPolicy(), "workspace", nil)
	wantStatus(t, "bare workspace", check(c, context.Background()), codes.Unauthenticated)
}

// The calling-service token cannot be supplied by the code making the call:
// the client replaces whatever is on the outgoing context, so a caller cannot
// borrow another service's name.
func TestClientDropsAForgedCallingServiceToken(t *testing.T) {
	forged, err := mint(active.Load(), claims{SVC: "workspace"}, checkMethod)
	if err != nil {
		t.Fatal(err)
	}
	ctx := metadata.AppendToOutgoingContext(WithCaller(context.Background(), someUser), KeyService, forged)

	c := startServer(t, viaPolicy(), "document", nil)
	wantStatus(t, "document with a forged workspace token", check(c, ctx), codes.PermissionDenied)
	anon := startServer(t, viaPolicy(), "", nil)
	wantStatus(t, "no service with a forged workspace token", check(anon, ctx), codes.PermissionDenied)
}

// rawHealth serves health under p and returns a client that signs nothing, so
// a test sets every identity header itself.
func rawHealth(t *testing.T, p ServerPolicy) grpc_health_v1.HealthClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer(ServerOptions(p)...)
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return grpc_health_v1.NewHealthClient(conn)
}

func mustMint(t *testing.T, c claims, method string) string {
	t.Helper()
	tok, err := mint(active.Load(), c, method)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestServerVerifiesTheCallingServiceToken(t *testing.T) {
	c := rawHealth(t, viaPolicy())
	user := mustMint(t, claims{UID: someUser.UserID, NID: someUser.NGACNodeID}, checkMethod)
	with := func(via string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), KeyIdentity, user, KeyService, via)
	}

	if err := check(c, with(mustMint(t, claims{SVC: "workspace"}, checkMethod))); err != nil {
		t.Fatalf("valid user and calling-service tokens refused: %v", err)
	}
	for name, via := range map[string]string{
		"unsigned":           "eyJzdmMiOiJ3b3Jrc3BhY2UifQ.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		"for another method": mustMint(t, claims{SVC: "workspace"}, "/other/Method"),
		"naming a user":      mustMint(t, claims{UID: "u2", NID: "n2"}, checkMethod),
		"garbage":            "not-a-token",
	} {
		wantStatus(t, "calling-service token "+name, check(c, with(via)), codes.Unauthenticated)
	}

	// Verified on every method, not only restricted ones: a bad proof of origin
	// is a forged request wherever it arrives.
	open := rawHealth(t, ServerPolicy{})
	bad := metadata.AppendToOutgoingContext(context.Background(), KeyIdentity, user, KeyService, "not-a-token")
	wantStatus(t, "bad calling-service token on an open method", check(open, bad), codes.Unauthenticated)
}

func TestStreamAppliesUserVia(t *testing.T) {
	c := startStreamServer(t, ServerPolicy{UserVia: map[string]ServiceRule{watchMethod: ServiceOnly("test", "workspace")}})
	// startStreamServer's client names no service.
	wantStatus(t, "stream user with no service", watchErr(t, c, WithCaller(context.Background(), someUser)), codes.PermissionDenied)
}
