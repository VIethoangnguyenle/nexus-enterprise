package grpcauth

import (
	"context"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func ring(current, previous string) *keyring {
	kr := &keyring{current: []byte(current)}
	if previous != "" {
		kr.previous = []byte(previous)
	}
	return kr
}

var (
	userClaims = claims{UID: "u1", NID: "n1", TID: "t1"}
	keysA      = ring("secret-A-0123456789abcdef", "")
)

// clockAt pins the package clock for one test.
func clockAt(t *testing.T, at time.Time) {
	t.Helper()
	prev := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = prev })
}

// forge signs an arbitrary payload under kr, the way an attacker holding the
// secret (or a buggy minter) could, so the claim checks are tested on their own.
func forge(kr *keyring, c claims) string {
	body, _ := json.Marshal(c)
	payload := enc.EncodeToString(body)
	return payload + "." + enc.EncodeToString(sign(kr.current, payload))
}

func TestTokenRoundTripUser(t *testing.T) {
	tok, err := mint(keysA, userClaims, checkMethod)
	if err != nil {
		t.Fatal(err)
	}
	got, err := verify(keysA, tok, checkMethod)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.UID != "u1" || got.NID != "n1" || got.TID != "t1" || got.SVC != "" || got.AUD != checkMethod || got.JTI == "" {
		t.Fatalf("claims = %+v", got)
	}
}

func TestTokenRoundTripService(t *testing.T) {
	tok, _ := mint(keysA, claims{SVC: "auth"}, checkMethod)
	got, err := verify(keysA, tok, checkMethod)
	if err != nil || got.SVC != "auth" || got.UID != "" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestTokensAreUnique(t *testing.T) {
	a, _ := mint(keysA, userClaims, checkMethod)
	b, _ := mint(keysA, userClaims, checkMethod)
	if a == b {
		t.Fatal("two tokens for the same call are identical; the nonce is not random")
	}
}

func TestTokenWithTamperedPayloadIsRefused(t *testing.T) {
	tok, _ := mint(keysA, userClaims, checkMethod)
	payload, mac, _ := strings.Cut(tok, ".")
	body, _ := enc.DecodeString(payload)
	var c claims
	if err := json.Unmarshal(body, &c); err != nil {
		t.Fatal(err)
	}
	c.UID, c.NID = "admin", "admin-node"
	tampered, _ := json.Marshal(c)
	if _, err := verify(keysA, enc.EncodeToString(tampered)+"."+mac, checkMethod); err == nil {
		t.Fatal("a payload edited after signing was accepted")
	}
}

func TestTokenWithTamperedSignatureIsRefused(t *testing.T) {
	tok, _ := mint(keysA, userClaims, checkMethod)
	flipped := tok[:len(tok)-1]
	if strings.HasSuffix(tok, "A") {
		flipped += "B"
	} else {
		flipped += "A"
	}
	for name, bad := range map[string]string{
		"flipped last char": flipped,
		"no signature":      strings.Split(tok, ".")[0] + ".",
		"no separator":      strings.ReplaceAll(tok, ".", ""),
		"extra part":        tok + ".AAAA",
		"empty":             "",
		"not base64":        "!!!.???",
	} {
		if _, err := verify(keysA, bad, checkMethod); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestExpiredTokenIsRefusedAfterSkew(t *testing.T) {
	base := time.Unix(1_800_000_000, 0)
	clockAt(t, base)
	tok, _ := mint(keysA, userClaims, checkMethod)

	for name, tc := range map[string]struct {
		at   time.Duration
		want bool
	}{
		"fresh":                       {0, true},
		"last second of ttl":          {tokenTTL, true},
		"inside skew after ttl":       {tokenTTL + clockSkew - time.Second, true},
		"past skew after ttl":         {tokenTTL + clockSkew + time.Second, false},
		"a day later":                 {24 * time.Hour, false},
		"issuer clock slightly ahead": {-(clockSkew - time.Second), true},
		"issuer clock far ahead":      {-(clockSkew + 5*time.Second), false},
	} {
		clockAt(t, base.Add(tc.at))
		_, err := verify(keysA, tok, checkMethod)
		if (err == nil) != tc.want {
			t.Errorf("%s: err = %v, want accepted = %v", name, err, tc.want)
		}
	}
}

func TestTokenForAnotherMethodIsRefused(t *testing.T) {
	tok, _ := mint(keysA, userClaims, checkMethod)
	if _, err := verify(keysA, tok, watchMethod); err == nil {
		t.Fatal("a token minted for one method verified for another")
	}
}

func TestTokenUnderAnotherSecretIsRefused(t *testing.T) {
	tok, _ := mint(ring("secret-B-0123456789abcdef", ""), userClaims, checkMethod)
	if _, err := verify(keysA, tok, checkMethod); err == nil {
		t.Fatal("a token signed with another secret was accepted")
	}
}

func TestPreviousSecretVerifiesButNeverSigns(t *testing.T) {
	oldRing := ring("secret-OLD-0123456789abcdef", "")
	rotated := ring("secret-NEW-0123456789abcdef", "secret-OLD-0123456789abcdef")

	oldTok, _ := mint(oldRing, userClaims, checkMethod)
	if _, err := verify(rotated, oldTok, checkMethod); err != nil {
		t.Fatalf("token signed with the previous secret refused: %v", err)
	}
	newTok, _ := mint(rotated, userClaims, checkMethod)
	if _, err := verify(rotated, newTok, checkMethod); err != nil {
		t.Fatalf("token signed with the current secret refused: %v", err)
	}
	// A service that has not rotated yet does not know the new secret, and the
	// rotated one must have signed with the new one, not the previous.
	if _, err := verify(oldRing, newTok, checkMethod); err == nil {
		t.Fatal("minting used the previous secret")
	}
	// Dropping the previous secret ends acceptance of the old one.
	if _, err := verify(ring("secret-NEW-0123456789abcdef", ""), oldTok, checkMethod); err == nil {
		t.Fatal("old secret still accepted after rotation finished")
	}
}

func TestMalformedClaimsAreRefusedEvenWhenSigned(t *testing.T) {
	clockAt(t, time.Unix(1_800_000_000, 0))
	ok := func() claims {
		return claims{UID: "u", NID: "n", AUD: checkMethod, IAT: now().Unix(), EXP: now().Add(tokenTTL).Unix(), JTI: "j"}
	}
	if _, err := verify(keysA, forge(keysA, ok()), checkMethod); err != nil {
		t.Fatalf("baseline refused: %v", err)
	}
	for name, mod := range map[string]func(*claims){
		"user and service":  func(c *claims) { c.SVC = "auth" },
		"user without node": func(c *claims) { c.NID = "" },
		"node without user": func(c *claims) { c.UID = "" },
		"no identity":       func(c *claims) { c.UID, c.NID = "", "" },
		"lifetime too long": func(c *claims) { c.EXP = c.IAT + int64(time.Hour/time.Second) },
	} {
		c := ok()
		mod(&c)
		if _, err := verify(keysA, forge(keysA, c), checkMethod); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A signed payload with a field this version does not know is refused
	// rather than silently dropped.
	payload := enc.EncodeToString([]byte(`{"uid":"u","nid":"n","admin":true,"aud":"` + checkMethod + `","iat":1800000000,"exp":1800000060,"jti":"j"}`))
	unknown := payload + "." + enc.EncodeToString(sign(keysA.current, payload))
	if _, err := verify(keysA, unknown, checkMethod); err == nil {
		t.Error("unknown field accepted")
	}
}

func TestConfigureValidatesSecrets(t *testing.T) {
	defer func(prev *keyring) { active.Store(prev) }(active.Load())
	if err := Configure(Keys{Current: strings.Repeat("k", minSecretLen-1)}); err == nil {
		t.Error("31-byte secret accepted")
	}
	if err := Configure(Keys{Current: strings.Repeat("k", minSecretLen)}); err != nil {
		t.Errorf("32-byte secret refused: %v", err)
	}
	if err := Configure(Keys{Current: "short"}); err == nil {
		t.Error("short secret accepted")
	}
	if err := Configure(Keys{Current: testSecret, Previous: "short"}); err == nil {
		t.Error("short previous secret accepted")
	}
	if err := Configure(Keys{Current: testSecret, Previous: testSecret}); err != nil {
		t.Errorf("valid keys refused: %v", err)
	}
	if active.Load().previous != nil {
		t.Error("previous equal to current should be dropped")
	}
}

// --- over the wire ---------------------------------------------------------

// rawServer serves health behind the full server options and returns a client
// with no interceptors, so the test controls exactly which metadata is sent.
func rawServer(t *testing.T, p ServerPolicy) grpc_health_v1.HealthClient {
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
	t.Cleanup(func() { _ = conn.Close() })
	return grpc_health_v1.NewHealthClient(conn)
}

func withToken(t *testing.T, c claims, method string) context.Context {
	t.Helper()
	tok, err := mint(active.Load(), c, method)
	if err != nil {
		t.Fatal(err)
	}
	return metadata.AppendToOutgoingContext(context.Background(), KeyIdentity, tok)
}

func wantCode(t *testing.T, what string, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("%s: code = %v, want %v (err %v)", what, got, want, err)
	}
}

func TestServerIgnoresRawCallerMetadata(t *testing.T) {
	c := rawServer(t, ServerPolicy{ServiceOK: map[string]ServiceRule{checkMethod: ServiceOnly("x", "auth")}})
	ctx := metadata.AppendToOutgoingContext(context.Background(),
		rawUserID, "admin", rawNodeID, "admin-node", "x-caller-tenant-id", "t", rawSvc, "auth")
	_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	wantCode(t, "raw x-caller-* and x-service-name", err, codes.Unauthenticated)
}

func TestServerRefusesWrongAndGarbageTokens(t *testing.T) {
	c := rawServer(t, ServerPolicy{})
	req := &grpc_health_v1.HealthCheckRequest{}

	_, err := c.Check(metadata.AppendToOutgoingContext(context.Background(), KeyIdentity, "garbage"), req)
	wantCode(t, "garbage token", err, codes.Unauthenticated)

	wrong, _ := mint(ring("some-other-secret-0123456789", ""), userClaims, checkMethod)
	_, err = c.Check(metadata.AppendToOutgoingContext(context.Background(), KeyIdentity, wrong), req)
	wantCode(t, "wrong secret", err, codes.Unauthenticated)

	_, err = c.Check(withToken(t, userClaims, checkMethod), req)
	if err != nil {
		t.Fatalf("valid token refused: %v", err)
	}
}

func TestServerRefusesExpiredToken(t *testing.T) {
	c := rawServer(t, ServerPolicy{})
	ctx := withToken(t, userClaims, checkMethod)
	clockAt(t, time.Now().Add(tokenTTL+clockSkew+10*time.Second))
	_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	wantCode(t, "expired token", err, codes.Unauthenticated)
}

func TestServerRefusesATokenReplayedOnAnotherMethod(t *testing.T) {
	c := rawServer(t, ServerPolicy{})
	ctx := withToken(t, userClaims, watchMethod) // minted for Watch, sent to Check
	_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	wantCode(t, "token for another method", err, codes.Unauthenticated)
}

func TestServerWithoutConfiguredSecretRefuses(t *testing.T) {
	c := rawServer(t, ServerPolicy{Exempt: map[string]string{watchMethod: "probe"}})
	ctx := withToken(t, userClaims, checkMethod)
	prev := active.Swap(nil)
	t.Cleanup(func() { active.Store(prev) })
	_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	wantCode(t, "unconfigured server", err, codes.Unauthenticated)
}

func TestServiceTokenOnlyOnMethodsThatAllowThatService(t *testing.T) {
	p := ServerPolicy{ServiceOK: map[string]ServiceRule{checkMethod: ServiceOnly("signup", "auth")}}
	c := rawServer(t, p)
	req := &grpc_health_v1.HealthCheckRequest{}

	_, err := c.Check(withToken(t, claims{SVC: "auth"}, checkMethod), req)
	if err != nil {
		t.Fatalf("allowlisted service refused: %v", err)
	}
	_, err = c.Check(withToken(t, claims{SVC: "workspace"}, checkMethod), req)
	wantCode(t, "service not on the allowlist", err, codes.Unauthenticated)

	// Method not in ServiceOK at all.
	other := rawServer(t, ServerPolicy{ServiceOK: map[string]ServiceRule{"/other/Method": ServiceOnly("x", "auth")}})
	_, err = other.Check(withToken(t, claims{SVC: "auth"}, checkMethod), req)
	wantCode(t, "service on a method not in ServiceOK", err, codes.Unauthenticated)
}

func TestServiceNameFromTokenReachesHandler(t *testing.T) {
	var got string
	lis, _ := net.Listen("tcp", "127.0.0.1:0")
	gs := grpc.NewServer(grpc.ChainUnaryInterceptor(
		ServerInterceptor(ServerPolicy{ServiceOK: map[string]ServiceRule{checkMethod: ServiceOnly("x", "auth")}}),
		func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			got = ServiceFrom(ctx)
			return h(ctx, req)
		}))
	grpc_health_v1.RegisterHealthServer(gs, health.NewServer())
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)
	conn, err := Dial(lis.Addr().String(), "auth")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := grpc_health_v1.NewHealthClient(conn).Check(context.Background(), &grpc_health_v1.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	if got != "auth" {
		t.Fatalf("service = %q", got)
	}
}

func TestStreamRefusesRawMetadataAndReplayedTokens(t *testing.T) {
	c := rawServer(t, ServerPolicy{})

	raw := metadata.AppendToOutgoingContext(context.Background(), rawUserID, "u", rawNodeID, "n")
	wantCode(t, "stream with raw metadata", watchErr(t, c, raw), codes.Unauthenticated)

	replay := withToken(t, userClaims, checkMethod) // a unary method's token on the stream
	wantCode(t, "stream with a token for another method", watchErr(t, c, replay), codes.Unauthenticated)

	if err := watchErr(t, c, withToken(t, userClaims, watchMethod)); err != nil {
		t.Fatalf("stream with its own token refused: %v", err)
	}
}

func TestClientWithoutSecretFailsTheCall(t *testing.T) {
	c := startServer(t, ServerPolicy{}, "svc", nil)
	prev := active.Swap(nil)
	t.Cleanup(func() { active.Store(prev) })
	_, err := c.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	wantCode(t, "client with no secret", err, codes.Unauthenticated)
}

func TestClientSendsNoIdentityWithoutCallerOrService(t *testing.T) {
	c := startServer(t, ServerPolicy{ServiceOK: map[string]ServiceRule{checkMethod: ServiceOnly("x", "auth")}}, "", nil)
	_, err := c.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	wantCode(t, "anonymous client", err, codes.Unauthenticated)
}

func TestOversizedAndMisshapenTokensAreRefusedBeforeDecoding(t *testing.T) {
	good, _ := mint(keysA, userClaims, checkMethod)
	if len(good) > maxTokenLen/2 {
		t.Fatalf("a real token is %d bytes; the %d cap leaves no headroom", len(good), maxTokenLen)
	}
	payload, mac, _ := strings.Cut(good, ".")
	for name, bad := range map[string]string{
		"over the cap":  strings.Repeat("A", maxTokenLen) + "." + mac,
		"mac too short": payload + "." + mac[:macLen-1],
		"mac too long":  payload + "." + mac + "A",
		"huge garbage":  strings.Repeat("A", 1<<20),
	} {
		if _, err := verify(keysA, bad, checkMethod); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if len(mac) != macLen {
		t.Fatalf("a real MAC is %d characters, macLen says %d", len(mac), macLen)
	}
}

// Headers past the limit are refused by the server before any interceptor
// runs; an ordinary signed call, whose only extra header is the token, is not
// near it.
func TestServerRefusesOversizedHeaders(t *testing.T) {
	c := rawServer(t, ServerPolicy{})
	req := &grpc_health_v1.HealthCheckRequest{}

	ctx := metadata.AppendToOutgoingContext(withToken(t, userClaims, checkMethod), "x-padding", strings.Repeat("a", 2*maxHeaderListSize))
	if _, err := c.Check(ctx, req); err == nil {
		t.Fatal("a request with 32 KiB of headers was served")
	}
	if _, err := c.Check(withToken(t, userClaims, checkMethod), req); err != nil {
		t.Fatalf("ordinary signed call refused: %v", err)
	}
	tok, _ := mint(active.Load(), claims{UID: strings.Repeat("u", 36), NID: strings.Repeat("n", 36), TID: strings.Repeat("t", 36)},
		"/workspace.WorkspaceService/ListWorkspaceMembersWithAVeryLongMethodName")
	if len(tok) > maxTokenLen/2 {
		t.Errorf("token with three UUIDs and a long method is %d bytes", len(tok))
	}
}
