package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

const testJWTSecret = "rest-account-test-secret-0123456789abcdef"

// acctStore is the slice of the account store these routes touch. Anything else
// panics on the embedded nil interface, which is how a test learns a route
// reached further than it should.
type acctStore struct {
	domain.AuthStore
	mu       sync.Mutex
	users    map[string]*store.User
	changes  map[string]store.ProfileChanges
	members  map[string][]store.WorkspaceSummary // user id -> workspaces
	byEmail  map[string]string
	verified map[string]bool
	// status overrides a membership's status ("active" by default), department
	// names the administrator-assigned department by "user|workspace".
	status      map[string]string
	department  map[string]string
	contacts    []store.User
	contactsTot int
	contactsNxt *store.ContactCursor
	lastFilter  *store.ContactFilter
}

func newAcctStore() *acctStore {
	return &acctStore{
		users: map[string]*store.User{}, changes: map[string]store.ProfileChanges{},
		members: map[string][]store.WorkspaceSummary{}, byEmail: map[string]string{}, verified: map[string]bool{},
		status: map[string]string{}, department: map[string]string{},
	}
}

func (s *acctStore) add(id, email string) *store.User {
	u := &store.User{ID: id, Username: "u-" + id, NGACNodeID: "node-" + id, Email: email, DisplayName: "u-" + id}
	s.users[id] = u
	if email != "" {
		s.byEmail[strings.ToLower(email)] = id
	}
	return u
}

func (s *acctStore) GetUserByID(_ context.Context, id string) (*store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u := s.users[id]; u != nil {
		cp := *u
		cp.EmailVerified = s.verified[id]
		return &cp, nil
	}
	return nil, nil
}

func (s *acctStore) GetUserByEmail(_ context.Context, email string) (*store.User, error) {
	s.mu.Lock()
	id := s.byEmail[strings.ToLower(email)]
	s.mu.Unlock()
	if id == "" {
		return nil, nil
	}
	return s.GetUserByID(context.Background(), id)
}

func (s *acctStore) UpdateProfile(_ context.Context, id string, ch store.ProfileChanges) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.users[id] == nil {
		return store.ErrNoSuchUser
	}
	s.changes[id] = ch
	s.users[id].ProfileCompleted = true
	if ch.DisplayName != nil {
		s.users[id].DisplayName = *ch.DisplayName
	}
	return nil
}

func (s *acctStore) ListWorkspaceSummaries(_ context.Context, id string) ([]store.WorkspaceSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.WorkspaceSummary(nil), s.members[id]...), nil
}

func (s *acctStore) ListTenantsByUser(context.Context, string) ([]store.TenantMembership, error) {
	return nil, nil
}

func (s *acctStore) GetTenantUser(_ context.Context, tenantID, userID string) (*store.TenantMembership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.members[userID] {
		if w.ID == tenantID {
			st := "active"
			if v, ok := s.status[userID]; ok {
				st = v
			}
			return &store.TenantMembership{TenantID: tenantID, TenantName: w.Name, UserID: userID, Role: w.Role, Status: st,
				DepartmentName: s.department[userID+"|"+tenantID]}, nil
		}
	}
	return nil, nil
}

func (s *acctStore) InsertTenantUser(_ context.Context, tenantID, userID, role, _, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[userID] = append(s.members[userID], store.WorkspaceSummary{ID: tenantID, Name: "created", Role: role, MemberCount: 1})
	return nil
}

func (s *acctStore) ListContactsByWorkspace(_ context.Context, _ string, f store.ContactFilter) ([]store.User, int, *store.ContactCursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastFilter = &f
	if s.contacts != nil {
		return s.contacts, s.contactsTot, s.contactsNxt, nil
	}
	return []store.User{{ID: "c1", Username: "colleague", DisplayName: "Đồng nghiệp", Email: "c@example.test"}}, 1, nil, nil
}

func (s *acctStore) MarkEmailVerified(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.verified[id] {
		return false, nil
	}
	s.verified[id] = true
	return true, nil
}

func (s *acctStore) ClearPassword(context.Context, string) error { return nil }

type acctWS struct {
	workspacepb.WorkspaceServiceClient
	n   int
	err error
}

func (w *acctWS) CreateWorkspace(_ context.Context, req *workspacepb.CreateWorkspaceRequest, _ ...grpc.CallOption) (*workspacepb.Workspace, error) {
	if w.err != nil {
		return nil, w.err
	}
	w.n++
	id := fmt.Sprintf("ws-%d", w.n)
	return &workspacepb.Workspace{Id: id, Name: req.Name, PcNodeId: "pc-" + id, OwnersUaId: "o-" + id, MembersUaId: "m-" + id}, nil
}

type acctMsg struct {
	messagingpb.MessagingServiceClient
}

func (acctMsg) CreateChannel(context.Context, *messagingpb.CreateChannelRequest, ...grpc.CallOption) (*messagingpb.Channel, error) {
	return &messagingpb.Channel{Id: "ch"}, nil
}

type acctRead struct {
	policypb.PolicyReadServiceClient
}

func (acctRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: "n-" + req.Name, Name: req.Name, NodeType: req.NodeType}, nil
}

type acctApp struct {
	e     *echo.Echo
	svc   *domain.Service
	st    *acctStore
	ws    *acctWS
	token func(userID string) string
}

func newAcctApp(t *testing.T, withRedis bool, opts *domain.OTPOptions) *acctApp {
	t.Helper()
	t.Setenv("APP_ENV", "dev")
	auth.SetJWTSecret(testJWTSecret)
	var rdb *redis.Client
	if withRedis {
		addr := os.Getenv("REDIS_ADDR")
		if addr == "" {
			t.Fatal("REDIS_ADDR must be set: this test needs Redis")
		}
		rdb = redis.NewClient(&redis.Options{Addr: addr})
		if err := rdb.Ping(context.Background()).Err(); err != nil {
			t.Fatalf("redis: %v", err)
		}
		t.Cleanup(func() { rdb.Close() })
	}
	st := newAcctStore()
	ws := &acctWS{}
	svc := domain.NewService(st, rdb, acctRead{}, testutil.NewFakePolicyWrite(), ws, acctMsg{})
	if opts != nil {
		svc.ConfigureOTP(*opts)
	}
	e := echo.New()
	NewHandler(svc).RegisterRoutes(e, testJWTSecret)
	return &acctApp{
		e: e, svc: svc, st: st, ws: ws,
		token: func(userID string) string {
			u := st.users[userID]
			tok, _, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, "")
			if err != nil {
				t.Fatal(err)
			}
			return tok
		},
	}
}

func (a *acctApp) do(method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	return m
}

func uniq(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

// --- PATCH /api/me/profile ---

func TestProfile_RequiresASession(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	for _, tok := range []string{"", "not-a-token"} {
		if rec := a.do(http.MethodPatch, "/api/me/profile", tok, `{"display_name":"x"}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", tok, rec.Code)
		}
	}
	if len(a.st.changes) != 0 {
		t.Error("nothing may be written without a session")
	}
}

func TestProfile_WritesOnlyTheFieldsSent(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	rec := a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), `{"display_name":"  Phạm Thuý An "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	ch := a.st.changes["u1"]
	if ch.DisplayName == nil || *ch.DisplayName != "Phạm Thuý An" {
		t.Errorf("display name = %v", ch.DisplayName)
	}
	if ch.Title != nil || ch.Location != nil {
		t.Errorf("fields that were not sent must stay untouched: %+v", ch)
	}
}

func TestProfile_SendingAnEmptyStringClearsAField(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	if rec := a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), `{"title":""}`); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if ch := a.st.changes["u1"]; ch.Title == nil || *ch.Title != "" {
		t.Errorf("title = %v, want a pointer to the empty string", ch.Title)
	}
}

func TestProfile_CannotNameAnotherPerson(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("me", "me@example.test")
	a.st.add("victim", "victim@example.test")

	body := `{"display_name":"Hacked","user_id":"victim","id":"victim","ngac_node_id":"node-victim"}`
	if rec := a.do(http.MethodPatch, "/api/me/profile", a.token("me"), body); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if _, touched := a.st.changes["victim"]; touched {
		t.Fatal("the person edited is the token's, never one named in the body")
	}
	if a.st.changes["me"].DisplayName == nil {
		t.Error("the caller's own profile is what changes")
	}
}

func TestProfile_RefusesBadInputWith400AndWritesNothing(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	for name, body := range map[string]string{
		"no fields":        `{}`,
		"blank name":       `{"display_name":"   "}`,
		"name too long":    `{"display_name":"` + strings.Repeat("a", 81) + `"}`,
		"not json":         `display_name=x`,
		"wrong field type": `{"display_name":5}`,
	} {
		rec := a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
	if len(a.st.changes) != 0 {
		t.Errorf("a refused update wrote %+v", a.st.changes)
	}
}

// --- GET /api/me ---

func TestMe_ReportsVerificationAndProfileState(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")

	me := decode(t, a.do(http.MethodGet, "/api/me", a.token("u1"), ""))["user"].(map[string]any)
	if me["email_verified"] != false || me["needs_profile"] != true {
		t.Errorf("new unverified account: %v", me)
	}

	a.st.verified["u1"] = true
	_ = a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), `{"display_name":"An"}`)
	me = decode(t, a.do(http.MethodGet, "/api/me", a.token("u1"), ""))["user"].(map[string]any)
	if me["email_verified"] != true || me["needs_profile"] != false || me["display_name"] != "An" {
		t.Errorf("verified, profile saved: %v", me)
	}
}

// --- /api/me/workspaces ---

func TestMyWorkspaces_RequireASession(t *testing.T) {
	a := newAcctApp(t, false, nil)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		if rec := a.do(method, "/api/me/workspaces", "", `{"name":"x"}`); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without a token: %d, want 401", method, rec.Code)
		}
	}
	if a.ws.n != 0 {
		t.Error("an unauthenticated request created a workspace")
	}
}

func TestMyWorkspaces_ListsOnlyTheCallersWithRoleAndCount(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("me", "me@example.test")
	a.st.add("other", "other@example.test")
	a.st.members["me"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối Vận hành", Role: "owner", MemberCount: 64, Domain: "novapay.vn"}}
	a.st.members["other"] = []store.WorkspaceSummary{{ID: "w2", Name: "Nhóm khác", Role: "owner", MemberCount: 3}}

	rec := a.do(http.MethodGet, "/api/me/workspaces", a.token("me"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	list := decode(t, rec)["workspaces"].([]any)
	if len(list) != 1 {
		t.Fatalf("got %d workspaces: %v", len(list), list)
	}
	w := list[0].(map[string]any)
	if w["id"] != "w1" || w["name"] != "Khối Vận hành" || w["role"] != "owner" || w["member_count"] != float64(64) || w["domain"] != "novapay.vn" {
		t.Errorf("workspace = %v", w)
	}
}

func TestMyWorkspaces_NoMembershipsIsAnEmptyListNotNull(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("me", "me@example.test")
	rec := a.do(http.MethodGet, "/api/me/workspaces", a.token("me"), "")
	if !strings.Contains(rec.Body.String(), `"workspaces":[]`) {
		t.Errorf("body = %s, want an empty array so the client can map over it", rec.Body)
	}
}

func TestMyWorkspaces_CreateTakesAName_AndNothingElse(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("me", "me@example.test")
	a.st.verified["me"] = true

	// A slug, a domain or an owner in the body are not inputs.
	rec := a.do(http.MethodPost, "/api/me/workspaces", a.token("me"),
		`{"name":" Tổ Đối soát ","slug":"to-doi-soat","domain":"acme.com","owner_id":"someone-else"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	got := decode(t, rec)
	if got["name"] != "Tổ Đối soát" || got["role"] != "owner" || got["id"] == "" || got["member_count"] != float64(1) {
		t.Errorf("created = %v", got)
	}
	if len(a.st.members["me"]) != 1 || a.st.members["me"][0].Role != "owner" {
		t.Errorf("the caller is the owner: %+v", a.st.members)
	}
	if len(a.st.members["someone-else"]) != 0 {
		t.Error("owner_id in the body must be ignored")
	}
}

func TestMyWorkspaces_CreateNeedsAVerifiedAddress(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("fixed", "fixed@example.test") // signed in with the test-only code: unverified
	a.st.add("phone", "")

	for _, id := range []string{"fixed", "phone"} {
		rec := a.do(http.MethodPost, "/api/me/workspaces", a.token(id), `{"name":"Tổ Đối soát"}`)
		if rec.Code != http.StatusForbidden || decode(t, rec)["code"] != "email_unverified" {
			t.Errorf("%s: %d %s, want 403 email_unverified", id, rec.Code, rec.Body)
		}
	}
	if a.ws.n != 0 {
		t.Errorf("%d workspaces were created", a.ws.n)
	}
	a.st.verified["fixed"] = true
	if rec := a.do(http.MethodPost, "/api/me/workspaces", a.token("fixed"), `{"name":"Tổ Đối soát"}`); rec.Code != http.StatusCreated {
		t.Errorf("after verification: %d %s", rec.Code, rec.Body)
	}
}

func TestMyWorkspaces_CreateRefusesBadNames(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("me", "me@example.test")
	a.st.verified["me"] = true
	for name, body := range map[string]string{
		"missing":  `{}`,
		"empty":    `{"name":""}`,
		"blank":    `{"name":"  "}`,
		"too long": `{"name":"` + strings.Repeat("x", 81) + `"}`,
		"newline":  `{"name":"a\nb"}`,
		"not json": `name=x`,
	} {
		if rec := a.do(http.MethodPost, "/api/me/workspaces", a.token("me"), body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
	if a.ws.n != 0 {
		t.Errorf("%d workspaces were created for refused names", a.ws.n)
	}
}

func TestMyWorkspaces_CreateIsLimitedAndSaysWhenToRetry(t *testing.T) {
	a := newAcctApp(t, true, nil)
	id := uniq("limit")
	a.st.add(id, id+"@example.test")
	a.st.verified[id] = true

	for i := 0; i < domain.WorkspaceCreateLimit; i++ {
		if rec := a.do(http.MethodPost, "/api/me/workspaces", a.token(id), `{"name":"Nhóm"}`); rec.Code != http.StatusCreated {
			t.Fatalf("creation %d: %d %s", i+1, rec.Code, rec.Body)
		}
	}
	rec := a.do(http.MethodPost, "/api/me/workspaces", a.token(id), `{"name":"Nhóm nữa"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", rec.Code)
	}
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs <= 0 {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	body := decode(t, rec)
	if body["code"] != "rate_limited" || body["retry_after_seconds"] != float64(secs) {
		t.Errorf("body = %v", body)
	}
	if a.ws.n != domain.WorkspaceCreateLimit {
		t.Errorf("workspaces created = %d, want %d", a.ws.n, domain.WorkspaceCreateLimit)
	}
}

// --- errors never leak internals ---

func TestMapError_InternalFailuresHideTheirDetail(t *testing.T) {
	secret := "dial tcp 10.0.0.5:25: connect: connection refused (smtp password hunter2)"
	he := mapError(fmt.Errorf("deliver otp code: %s", secret))
	if he.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", he.Code)
	}
	raw, _ := json.Marshal(he.Message)
	if strings.Contains(string(raw), "10.0.0.5") || strings.Contains(string(raw), "hunter2") || strings.Contains(string(raw), "deliver") {
		t.Errorf("a 500 must not echo the underlying error, got %s", raw)
	}
}

func TestMapError_StatusesAndCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
		{domain.ErrOTPInvalid, http.StatusUnauthorized, "otp_invalid"},
		{domain.ErrOTPExpired, http.StatusUnauthorized, "otp_expired"},
		{domain.ErrTooManyAttempts, http.StatusTooManyRequests, "otp_too_many_attempts"},
		{domain.ErrOTPRateLimited, http.StatusTooManyRequests, "otp_rate_limited"},
		{domain.ErrRateLimited, http.StatusTooManyRequests, "rate_limited"},
		{domain.ErrEmailUnverified, http.StatusForbidden, "email_unverified"},
		{domain.ErrOTPUnavailable, http.StatusServiceUnavailable, "otp_unavailable"},
		{domain.ErrInvalidInput, http.StatusBadRequest, "invalid_input"},
		{domain.ErrNotFound, http.StatusNotFound, "not_found"},
		{domain.ErrAccessDenied, http.StatusForbidden, "access_denied"},
		{domain.ErrUserExists, http.StatusConflict, "already_exists"},
	} {
		he := mapError(tc.err)
		raw, _ := json.Marshal(he.Message)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatalf("%v: message %s is not an object: %v", tc.err, raw, err)
		}
		if he.Code != tc.status || body["code"] != tc.code || body["message"] == "" {
			t.Errorf("%v -> %d %v, want %d code %q", tc.err, he.Code, body, tc.status, tc.code)
		}
	}
}

// --- sign-in responses do not reveal accounts ---

func TestOTP_VerifyFailuresDoNotDependOnWhetherTheAccountExists(t *testing.T) {
	a := newAcctApp(t, true, &domain.OTPOptions{FixedCode: "999999"})
	known := uniq("known") + "@example.test"
	a.st.add(uniq("k"), known)
	unknown := uniq("unknown") + "@example.test"

	wrong := func(email string) (int, map[string]any) {
		rec := a.do(http.MethodPost, "/api/auth/otp/request", "", fmt.Sprintf(`{"identifier":%q,"type":"email"}`, email))
		if rec.Code != http.StatusOK {
			t.Fatalf("request for %s: %d %s", email, rec.Code, rec.Body)
		}
		sid := decode(t, rec)["session_id"].(string)
		rec = a.do(http.MethodPost, "/api/auth/otp/verify", "", fmt.Sprintf(`{"session_id":%q,"code":"000000"}`, sid))
		return rec.Code, decode(t, rec)
	}
	kc, kb := wrong(known)
	uc, ub := wrong(unknown)
	if kc != uc || kb["code"] != ub["code"] || kb["message"] != ub["message"] || kb["attempts_left"] != ub["attempts_left"] {
		t.Errorf("a wrong code answers differently for a known (%d %v) and an unknown (%d %v) address", kc, kb, uc, ub)
	}
	if kc != http.StatusUnauthorized || kb["code"] != "otp_invalid" || kb["attempts_left"] != float64(4) {
		t.Errorf("wrong code: %d %v, want 401 otp_invalid with 4 tries left", kc, kb)
	}
}

func TestOTP_RequestAnswersTheSameForKnownAndUnknownAddresses(t *testing.T) {
	a := newAcctApp(t, true, &domain.OTPOptions{FixedCode: "999999"})
	known := uniq("known") + "@example.test"
	a.st.add(uniq("k"), known)

	shape := func(email string) (int, map[string]any) {
		rec := a.do(http.MethodPost, "/api/auth/otp/request", "", fmt.Sprintf(`{"identifier":%q,"type":"email"}`, email))
		m := decode(t, rec)
		delete(m, "session_id")
		return rec.Code, m
	}
	kc, km := shape(known)
	uc, um := shape(uniq("unknown") + "@example.test")
	if kc != uc || fmt.Sprint(km) != fmt.Sprint(um) {
		t.Errorf("known: %d %v, unknown: %d %v", kc, km, uc, um)
	}
}

func TestOTP_RateLimitedRequestCarriesRetryAfter(t *testing.T) {
	a := newAcctApp(t, true, &domain.OTPOptions{FixedCode: "999999"})
	email := uniq("burst") + "@example.test"
	body := fmt.Sprintf(`{"identifier":%q,"type":"email"}`, email)
	for i := 0; i < 5; i++ {
		if rec := a.do(http.MethodPost, "/api/auth/otp/request", "", body); rec.Code != http.StatusOK {
			t.Fatalf("request %d: %d", i+1, rec.Code)
		}
	}
	rec := a.do(http.MethodPost, "/api/auth/otp/request", "", body)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", rec.Code)
	}
	secs, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || secs <= 0 || secs > 15*60 {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	if decode(t, rec)["code"] != "otp_rate_limited" {
		t.Errorf("body = %s", rec.Body)
	}
}

func TestOTP_VerifyReportsAccountState(t *testing.T) {
	a := newAcctApp(t, true, &domain.OTPOptions{FixedCode: "999999"})
	addr := uniq("state") + "@example.test"
	a.st.add("state-user", addr)

	rec := a.do(http.MethodPost, "/api/auth/otp/request", "", fmt.Sprintf(`{"identifier":%q,"type":"email"}`, addr))
	sid := decode(t, rec)["session_id"].(string)
	rec = a.do(http.MethodPost, "/api/auth/otp/verify", "", fmt.Sprintf(`{"session_id":%q,"code":"999999"}`, sid))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	user := body["user"].(map[string]any)
	if user["email_verified"] != false || body["needs_profile"] != true || body["is_new_user"] != false {
		t.Errorf("fixed code on an existing, unverified account: user=%v body=%v", user, body)
	}
	if body["access_token"] == "" {
		t.Error("a session is issued")
	}
}

// providers advertises whether a code delivered here would prove the address.
func TestProviders_ReportsWhetherACodeProvesTheAddress(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts domain.OTPOptions
		want bool
	}{
		{"fixed test code", domain.OTPOptions{FixedCode: "999999"}, false},
		{"log sender", domain.OTPOptions{Sender: domain.LogSender{}}, false},
		{"real sender", domain.OTPOptions{Sender: ownerSender{}}, true},
		{"nothing", domain.OTPOptions{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{svc: otpTestService(true, &tc.opts)}
			if got := providersBody(t, h)["otp_proves_email"]; got != tc.want {
				t.Errorf("otp_proves_email = %v, want %v", got, tc.want)
			}
		})
	}
}

type ownerSender struct{}

func (ownerSender) SendCode(context.Context, string, string, string) error { return nil }
func (ownerSender) DeliversToOwner() bool                                  { return true }

func TestRetryAfterHelperRoundTrips(t *testing.T) {
	if _, ok := domain.RetryAfter(errors.New("x")); ok {
		t.Error("a plain error has no retry time")
	}
}

// --- who may read the directory ---

func TestContacts_OnlyMembersOfTheWorkspaceMayListIt(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("member", "m@example.test")
	a.st.add("outsider", "o@example.test")
	a.st.members["member"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối Vận hành", Role: "member"}}

	if rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("member"), ""); rec.Code != http.StatusOK {
		t.Fatalf("a member: status %d: %s", rec.Code, rec.Body)
	}
	rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("outsider"), "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a person outside the workspace: status %d, want 403", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "c@example.test") || strings.Contains(rec.Body.String(), "Đồng nghiệp") {
		t.Errorf("the refusal leaked the directory: %s", rec.Body)
	}
	if rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: status %d, want 401", rec.Code)
	}
}

// Nothing in the product lists accounts or looks one up by username, and either
// would let any signed-in person walk the user table.
func TestUserDirectoryRoutesDoNotExist(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	for _, path := range []string{"/api/users", "/api/users/lookup?username=u-u1"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			rec := a.do(method, path, a.token("u1"), "")
			if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
				t.Errorf("%s %s answered %d: %s", method, path, rec.Code, rec.Body)
			}
		}
	}
}

// --- fields a person may not write ---

func TestProfile_RefusesAnAvatarUntilThereIsWhereToKeepOne(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	for _, body := range []string{
		`{"avatar_url":"https://cdn.example.test/a.png"}`,
		`{"avatar_url":""}`,
		`{"display_name":"Phạm Thuý An","avatar_url":"javascript:alert(1)"}`,
	} {
		rec := a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), body)
		if rec.Code != http.StatusBadRequest || decode(t, rec)["code"] != "avatar_not_supported" {
			t.Errorf("%s: %d %s, want 400 avatar_not_supported", body, rec.Code, rec.Body)
		}
	}
	if len(a.st.changes) != 0 {
		t.Errorf("a refused request wrote %+v, including its valid half", a.st.changes)
	}
}

func TestProfile_RefusesADepartmentThePersonChoseThemselves(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	for _, body := range []string{`{"department":"Ban giám đốc"}`, `{"department":""}`, `{"display_name":"An","department":"Kế toán"}`} {
		rec := a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), body)
		if rec.Code != http.StatusBadRequest || decode(t, rec)["code"] != "department_not_editable" {
			t.Errorf("%s: %d %s, want 400 department_not_editable", body, rec.Code, rec.Body)
		}
	}
	if len(a.st.changes) != 0 {
		t.Errorf("a refused request wrote %+v", a.st.changes)
	}
}

func TestProfile_TitleAndLocationStillSave(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	rec := a.do(http.MethodPatch, "/api/me/profile", a.token("u1"), `{"title":"Kế toán","location":"Hà Nội"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	ch := a.st.changes["u1"]
	if ch.Title == nil || *ch.Title != "Kế toán" || ch.Location == nil || *ch.Location != "Hà Nội" {
		t.Errorf("changes = %+v", ch)
	}
}

// --- the directory ---

func contactsApp(t *testing.T) *acctApp {
	t.Helper()
	a := newAcctApp(t, false, nil)
	a.st.add("member", "m@example.test")
	a.st.members["member"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối", Role: "member"}}
	return a
}

// The wire the directory screens read: `cursor` in, `next_cursor` and the true
// `total` out. The frontend's useContacts follows next_cursor until it is gone.
func TestContacts_PagesByCursor_AndTheTotalIsTheStores(t *testing.T) {
	a := contactsApp(t)
	a.st.contacts = []store.User{{ID: "c1", Username: "lan", DisplayName: "Nguyễn Thu Lan", Department: "Đối soát"}}
	a.st.contactsTot = 123
	a.st.contactsNxt = &store.ContactCursor{Key: "nguyễn thu lan", ID: "c1"}

	rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts?limit=10&department=%C4%90%E1%BB%91i+so%C3%A1t&location=H%C3%A0+N%E1%BB%99i&search=lan", a.token("member"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	body := decode(t, rec)
	if body["total"] != float64(123) {
		t.Errorf("total = %v, want the store's 123, not the page's length", body["total"])
	}
	next, _ := body["next_cursor"].(string)
	if next == "" {
		t.Fatalf("a full directory has a next_cursor: %v", body)
	}
	if _, has := body["page"]; has {
		t.Error("there is no page number: the directory is paged by cursor")
	}
	f := a.st.lastFilter
	if f == nil || f.Limit != 10 || f.After != nil || f.Department != "Đối soát" || f.Location != "Hà Nội" || f.Search != "lan" {
		t.Errorf("first request filter = %+v", f)
	}

	// The second page: what the first handed out is what the store is asked to continue from.
	a.st.contactsNxt = nil
	rec = a.do(http.MethodGet, "/api/workspaces/w1/contacts?limit=10&cursor="+next, a.token("member"), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if f := a.st.lastFilter; f.After == nil || f.After.ID != "c1" || f.After.Key != "nguyễn thu lan" {
		t.Errorf("second request continues from %+v", f.After)
	}
	if _, has := decode(t, rec)["next_cursor"]; has {
		t.Error("the last page carries no next_cursor")
	}
}

func TestContacts_DefaultsToFiftyAndNoCursorOnTheOnlyPage(t *testing.T) {
	a := contactsApp(t)
	body := decode(t, a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token("member"), ""))
	if f := a.st.lastFilter; f == nil || f.Limit != 50 || f.After != nil {
		t.Errorf("filter = %+v", f)
	}
	if _, has := body["next_cursor"]; has || body["total"] != float64(1) {
		t.Errorf("body = %v", body)
	}
}

func TestContacts_RefusesPagingThatIsNotPaging(t *testing.T) {
	a := contactsApp(t)
	notJSON := base64.RawURLEncoding.EncodeToString([]byte("not json"))
	noID := base64.RawURLEncoding.EncodeToString([]byte(`{"k":"a"}`))
	for _, q := range []string{
		"limit=0", "limit=-5", "limit=abc", "limit=201", "limit=1000000",
		"cursor=%25%25", "cursor=" + notJSON, "cursor=" + noID, "cursor=" + strings.Repeat("A", 600),
		"search=" + strings.Repeat("x", 101),
	} {
		a.st.lastFilter = nil
		rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts?"+q, a.token("member"), "")
		if rec.Code != http.StatusBadRequest || decode(t, rec)["code"] != "invalid_input" {
			t.Errorf("?%s: %d %s, want 400 invalid_input", q, rec.Code, rec.Body)
		}
		if a.st.lastFilter != nil {
			t.Errorf("?%s reached the store", q)
		}
	}
}

func TestContacts_ASuspendedOrUnacceptedMemberCannotReadTheDirectory(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("gone", "g@example.test")
	a.st.add("waiting", "w@example.test")
	a.st.members["gone"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối", Role: "member"}}
	a.st.members["waiting"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối", Role: "member"}}
	a.st.status["gone"] = "disabled"
	a.st.status["waiting"] = "invited"

	for _, id := range []string{"gone", "waiting"} {
		rec := a.do(http.MethodGet, "/api/workspaces/w1/contacts", a.token(id), "")
		if rec.Code != http.StatusForbidden || decode(t, rec)["code"] != "access_denied" {
			t.Errorf("%s: %d %s, want 403 access_denied", id, rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "Đồng nghiệp") {
			t.Errorf("%s: the refusal leaked the directory", id)
		}
	}
	if a.st.lastFilter != nil {
		t.Error("the store was asked for a directory that was refused")
	}
}

// --- the person's own record, for Settings ---

func TestMe_CarriesTheProfileAndTheAssignedDepartment(t *testing.T) {
	a := newAcctApp(t, false, nil)
	u := a.st.add("u1", "a@example.test")
	u.Title, u.Location, u.AvatarURL = "Kế toán", "Hà Nội", "https://cdn.example.test/a.png"
	a.st.members["u1"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối", Role: "member"}, {ID: "w2", Name: "Dự án", Role: "owner"}}
	a.st.department["u1|w1"] = "Đối soát"

	body := decode(t, a.do(http.MethodGet, "/api/me?workspace=w1", a.token("u1"), ""))
	user := body["user"].(map[string]any)
	if user["title"] != "Kế toán" || user["location"] != "Hà Nội" || user["avatar_url"] != "https://cdn.example.test/a.png" {
		t.Errorf("user = %v", user)
	}
	if body["current_tenant"].(map[string]any)["department"] != "Đối soát" {
		t.Errorf("current_tenant = %v", body["current_tenant"])
	}
	other := decode(t, a.do(http.MethodGet, "/api/me?workspace=w2", a.token("u1"), ""))
	if other["current_tenant"].(map[string]any)["department"] != "" {
		t.Errorf("a workspace with no assignment has no department: %v", other["current_tenant"])
	}
}

func TestMe_AWorkspaceTheCallerDoesNotBelongToIsNotDescribed(t *testing.T) {
	a := newAcctApp(t, false, nil)
	a.st.add("u1", "a@example.test")
	a.st.members["u1"] = []store.WorkspaceSummary{{ID: "w1", Name: "Khối", Role: "member"}}
	a.st.status["u1"] = "disabled"

	for _, ws := range []string{"w1", "someone-elses", "no-such"} {
		body := decode(t, a.do(http.MethodGet, "/api/me?workspace="+ws, a.token("u1"), ""))
		if _, ok := body["current_tenant"]; ok {
			t.Errorf("workspace %q: current_tenant = %v, want none", ws, body["current_tenant"])
		}
	}
}
