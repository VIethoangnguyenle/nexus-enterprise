package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

// refreshPolicy answers the membership questions of one workspace, "ws-1", and
// can be switched off to behave as a policy service that does not answer.
type refreshPolicy struct {
	policypb.PolicyWriteServiceClient
	down atomic.Bool
}

func (p *refreshPolicy) check() error {
	if p.down.Load() {
		return status.Error(codes.Unavailable, "policy: connection refused")
	}
	return nil
}

func (p *refreshPolicy) FindNodeByName(_ context.Context, in *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: "n-" + in.Name, Name: in.Name, NodeType: in.NodeType}, p.check()
}

func (p *refreshPolicy) IsAssigned(context.Context, *policypb.IsAssignedRequest, ...grpc.CallOption) (*policypb.BoolResponse, error) {
	return &policypb.BoolResponse{Value: true}, p.check()
}

func (p *refreshPolicy) GetNode(_ context.Context, in *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: in.NodeId, NodeType: ngac.TypeU}, p.check()
}

func (p *refreshPolicy) GetAncestors(context.Context, *policypb.GetAncestorsRequest, ...grpc.CallOption) (*policypb.NodeList, error) {
	name := ngac.MembersUAName("ws-1")
	return &policypb.NodeList{Nodes: []*policypb.NGACNode{{Id: "n-" + name, Name: name, NodeType: ngac.TypeUA}}}, p.check()
}

type refreshApp struct {
	e      *echo.Echo
	st     *acctStore
	svc    *domain.Service
	policy *refreshPolicy
}

func newRefreshApp(t *testing.T) *refreshApp {
	t.Helper()
	t.Setenv("APP_ENV", "dev")
	auth.SetJWTSecret(testJWTSecret)
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Fatal("REDIS_ADDR must be set: this test needs Redis")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })

	st := newAcctStore()
	pol := &refreshPolicy{}
	svc := domain.NewService(st, rdb, acctRead{}, pol, &acctWS{}, acctMsg{})
	e := echo.New()
	NewHandler(svc).RegisterRoutes(e, testJWTSecret)
	return &refreshApp{e: e, st: st, svc: svc, policy: pol}
}

// sessionIn starts a session for a new member of ws-1 and returns its refresh token.
func (a *refreshApp) sessionIn(t *testing.T, id string) string {
	t.Helper()
	u := a.st.add(id, "")
	a.st.members[id] = append(a.st.members[id], store.WorkspaceSummary{ID: "ws-1", Name: "Acme", Role: "member"})
	tok, err := a.svc.AttachRefreshToken(context.Background(), domain.RefreshIdentity{
		UserID: id, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: "ws-1", SessionID: "sess-" + id,
	})
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func (a *refreshApp) refresh(token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: token})
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

func cookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == refreshCookieName {
			return c
		}
	}
	return nil
}

// An outage of the policy answers 503 and leaves the cookie alone: clearing it, or
// spending the token, would sign the person out for something that is not their
// doing. Once the policy is back the same cookie refreshes.
func TestRefresh_AnOutageIsA503AndKeepsTheCookie(t *testing.T) {
	a := newRefreshApp(t)
	token := a.sessionIn(t, uniq("u"))

	a.policy.down.Store(true)
	rec := a.refresh(token)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d (%s), want 503", rec.Code, rec.Body)
	}
	if c := cookieOf(rec); c != nil {
		t.Errorf("the response touched the refresh cookie (%q, MaxAge %d); it must leave it alone", c.Value, c.MaxAge)
	}
	if got := decode(t, rec)["code"]; got != "unavailable" {
		t.Errorf("code = %v, want unavailable", got)
	}

	a.policy.down.Store(false)
	rec = a.refresh(token)
	if rec.Code != http.StatusOK {
		t.Fatalf("after the outage status %d (%s), want 200: the token must not have been spent", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if body["access_token"] == "" || body["tenant_id"] != "ws-1" {
		t.Errorf("body = %v, want an access token for ws-1", body)
	}
	if _, present := body["moved"]; present {
		t.Errorf("body = %v: an ordinary refresh must not say the person was moved", body)
	}
	if c := cookieOf(rec); c == nil || c.Value == "" || c.Value == token {
		t.Errorf("the cookie must carry the rotated token, got %+v", c)
	}
}

// The same endpoint still refuses what it must refuse, and still clears the cookie then.
func TestRefresh_ARejectedTokenIsA401AndClearsTheCookie(t *testing.T) {
	a := newRefreshApp(t)

	rec := a.refresh("never-issued")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
	if c := cookieOf(rec); c == nil || c.MaxAge >= 0 {
		t.Errorf("a rejected token's cookie must be cleared, got %+v", c)
	}
}

// A person removed from the tenant of their token stays signed in: the answer names the
// tenant they land in (none here) so the client can follow.
func TestRefresh_ARemovedMemberGetsASessionWithoutTheTenant(t *testing.T) {
	a := newRefreshApp(t)
	id := uniq("gone")
	token := a.sessionIn(t, id)
	a.st.status[id] = "removed"

	rec := a.refresh(token)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d (%s), want 200", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if tenant, present := body["tenant_id"]; !present || tenant != "" {
		t.Errorf("tenant_id = %v (present %v), want an empty tenant", tenant, present)
	}
	if body["moved"] != true {
		t.Errorf("moved = %v, want true: the move was because the person no longer belongs", body["moved"])
	}
	if c := cookieOf(rec); c == nil || c.Value == "" || c.MaxAge < 0 {
		t.Errorf("the session goes on with a new cookie, got %+v", c)
	}
}
