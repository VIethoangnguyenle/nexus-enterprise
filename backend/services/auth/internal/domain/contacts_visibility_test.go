package domain_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

// scopeWorkspace is the workspace service as auth sees it: it answers GetDirectoryScope
// with a prepared scope or error, and remembers who asked.
type scopeWorkspace struct {
	workspacepb.WorkspaceServiceClient
	mu     sync.Mutex
	scope  *workspacepb.DirectoryScope
	err    error
	calls  int
	caller grpcauth.Caller
	asked  string
}

func (f *scopeWorkspace) GetDirectoryScope(ctx context.Context, req *workspacepb.GetDirectoryScopeRequest, _ ...grpc.CallOption) (*workspacepb.DirectoryScope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.caller = grpcauth.CallerFrom(ctx)
	f.asked = req.WorkspaceId
	if f.err != nil {
		return nil, f.err
	}
	return f.scope, nil
}

// contactsStore is the fake world with a directory: it hands back prepared rows and
// remembers the filter it was given, so a test can read the rule the domain applied.
type contactsStore struct {
	*fakeWorld
	rows       []store.User
	filter     *store.ContactFilter
	listCalls  int
	resolved   []store.ResolvedPerson
	resolveWS  string
	resolveIDs [][]string
	resolveErr error
}

func (s *contactsStore) ListContactsByWorkspace(_ context.Context, _ string, f store.ContactFilter) ([]store.User, int, *store.ContactCursor, error) {
	s.listCalls++
	s.filter = &f
	return s.rows, len(s.rows), nil, nil
}

func (s *contactsStore) ResolvePeopleByWorkspace(_ context.Context, ws string, userIDs, nodeIDs []string) ([]store.ResolvedPerson, error) {
	s.resolveWS = ws
	s.resolveIDs = [][]string{userIDs, nodeIDs}
	return s.resolved, s.resolveErr
}

type contactsWorld struct {
	svc    *domain.Service
	st     *contactsStore
	ws     *scopeWorkspace
	tenant string
	caller *store.User
}

// newContactsWorld is a workspace with one caller who is an active member in the row and
// in the graph, and the workspace service answering with scope.
func newContactsWorld(t *testing.T, scope *workspacepb.DirectoryScope) *contactsWorld {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	w := newFakeWorld()
	st := &contactsStore{fakeWorld: w}
	ws := &scopeWorkspace{scope: scope}
	c := &contactsWorld{
		svc: domain.NewService(st, nil, &fakePolicyRead{}, &fakePolicyWrite{w: w}, ws, &fakeMessaging{w: w}),
		st:  st, ws: ws, tenant: w.addTenant("Khối", ""), caller: w.addUser("caller@example.vn"),
	}
	seedMember(t, w, c.tenant, c.caller.ID, c.caller.NGACNodeID)
	return c
}

func (c *contactsWorld) list(q domain.ContactQuery) (*domain.ContactPage, error) {
	return c.svc.ListContacts(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, q)
}

// limitedScope is a caller limited to department "d1", who may see the owners.
func limitedScope() *workspacepb.DirectoryScope {
	return &workspacepb.DirectoryScope{
		Ready: true, Limited: true,
		VisibleDepartmentIds: []string{"d1"}, OwnersVisible: true, OwnerNodeIds: []string{"node-owner"},
	}
}

func TestListContacts_TheScopeBecomesTheRuleOfTheQuery(t *testing.T) {
	c := newContactsWorld(t, limitedScope())
	c.st.rows = []store.User{{ID: "p1", DisplayName: "An"}}

	page, err := c.list(domain.ContactQuery{})
	require.NoError(t, err)

	// allow: the rows come back
	require.Len(t, page.Contacts, 1)
	// the caller is the one the workspace service was asked about, as that caller
	assert.Equal(t, c.tenant, c.ws.asked)
	assert.Equal(t, grpcauth.Caller{UserID: c.caller.ID, NGACNodeID: c.caller.NGACNodeID}, c.ws.caller)
	// deny: the store is told to narrow, to exactly what the workspace granted, and never to All
	sc := c.st.filter.Scope
	assert.False(t, sc.All, "a limited caller is not given the whole workspace")
	assert.Equal(t, []string{"d1"}, sc.DepartmentIDs)
	assert.False(t, sc.NoDepartment)
	assert.True(t, sc.OwnersVisible)
	assert.Equal(t, []string{"node-owner"}, sc.OwnerNodeIDs)
	assert.Equal(t, c.caller.ID, sc.CallerID)
	assert.Equal(t, "limited", page.Visibility)
}

func TestListContacts_AnAdminAndAWorkspaceWithoutRulesSeeEveryone(t *testing.T) {
	for name, scope := range map[string]*workspacepb.DirectoryScope{
		"admin":        {Ready: true, Admin: true},
		"not ready":    {Ready: false},
		"ready, rules": {Ready: true, VisibleDepartmentIds: []string{"d1"}, NoDepartmentVisible: true},
	} {
		t.Run(name, func(t *testing.T) {
			c := newContactsWorld(t, scope)
			page, err := c.list(domain.ContactQuery{})
			require.NoError(t, err)
			assert.Equal(t, "all", page.Visibility)
			if name == "ready, rules" {
				// the same fixture shape as the limited case but not limited: still narrowed to its departments
				assert.False(t, c.st.filter.Scope.All)
				assert.Equal(t, []string{"d1"}, c.st.filter.Scope.DepartmentIDs)
				return
			}
			assert.True(t, c.st.filter.Scope.All, "the whole workspace")
		})
	}
}

// Every way the scope can fail to arrive ends in a refusal, and no row is read: the full
// directory is never the fallback.
func TestListContacts_FailsClosedWhenTheScopeIsNotGranted(t *testing.T) {
	cases := map[string]struct {
		err  error
		want error
	}{
		"workspace says the caller is not a member": {status.Error(codes.PermissionDenied, "not a member"), domain.ErrAccessDenied},
		"workspace unavailable":                     {status.Error(codes.Unavailable, "down"), domain.ErrUnavailable},
		"workspace times out":                       {status.Error(codes.DeadlineExceeded, "slow"), domain.ErrUnavailable},
		"workspace fails":                           {status.Error(codes.Internal, "boom"), domain.ErrUnavailable},
		"identity not accepted":                     {status.Error(codes.Unauthenticated, "who"), domain.ErrUnavailable},
		"a plain error":                             {errors.New("connection refused"), domain.ErrUnavailable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			c := newContactsWorld(t, limitedScope())
			c.st.rows = []store.User{{ID: "p1", DisplayName: "An"}}
			c.ws.err = tc.err

			page, err := c.list(domain.ContactQuery{})
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, page, "no rows for a caller whose scope is unknown")
			assert.Zero(t, c.st.listCalls, "the directory was not read")
		})
	}
}

func TestListContacts_NoWorkspaceServiceIsUnavailableNotEveryone(t *testing.T) {
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	w := newFakeWorld()
	st := &contactsStore{fakeWorld: w, rows: []store.User{{ID: "p1"}}}
	svc := domain.NewService(st, nil, &fakePolicyRead{}, &fakePolicyWrite{w: w}, nil, &fakeMessaging{w: w})
	tenant, u := w.addTenant("Khối", ""), w.addUser("a@example.vn")
	seedMember(t, w, tenant, u.ID, u.NGACNodeID)

	_, err := svc.ListContacts(context.Background(), u.ID, u.NGACNodeID, tenant, domain.ContactQuery{})
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.Zero(t, st.listCalls)
}

func TestListContacts_AnEmptyAnswerIsNotEveryone(t *testing.T) {
	c := newContactsWorld(t, nil)
	_, err := c.list(domain.ContactQuery{})
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.Zero(t, c.st.listCalls)
}

// A row that is not an active member never reaches the workspace, whatever the graph says.
func TestListContacts_OnlyAnActiveRowAsksTheWorkspace(t *testing.T) {
	c := newContactsWorld(t, limitedScope())
	_, err := c.list(domain.ContactQuery{})
	require.NoError(t, err, "allow: the active caller of the same fixture")
	require.Equal(t, 1, c.ws.calls)

	for _, status := range []string{"removed", "disabled", "invited"} {
		w := c.st.fakeWorld
		w.mu.Lock()
		w.members[c.tenant+"|"+c.caller.ID].Status = status
		w.mu.Unlock()
		_, err := c.list(domain.ContactQuery{})
		assert.ErrorIs(t, err, domain.ErrAccessDenied, status)
	}
	assert.Equal(t, 1, c.ws.calls, "the workspace was not asked for a refused caller")
}

func TestListContacts_NoRowIsNotAMember(t *testing.T) {
	c := newContactsWorld(t, limitedScope())
	stranger := c.st.fakeWorld.addUser("stranger@example.vn")
	_, err := c.svc.ListContacts(context.Background(), stranger.ID, stranger.NGACNodeID, c.tenant, domain.ContactQuery{})
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Zero(t, c.ws.calls)
	assert.Zero(t, c.st.listCalls)
}

func TestListContacts_DepartmentIDAndSearchPassThrough(t *testing.T) {
	c := newContactsWorld(t, limitedScope())
	_, err := c.list(domain.ContactQuery{DepartmentID: "d1", Search: " lan ", Department: " Kinh doanh "})
	require.NoError(t, err)
	assert.Equal(t, "d1", c.st.filter.DepartmentID)
	assert.Equal(t, "lan", c.st.filter.Search)
	assert.Equal(t, "Kinh doanh", c.st.filter.Department)
}

// Roles ride the answer: the ones the workspace let the caller see, held by that person.
func TestListContacts_RolesAreThoseTheScopeShowsAndThePersonHolds(t *testing.T) {
	scope := limitedScope()
	scope.Roles = []*workspacepb.DirectoryRole{
		{Id: "owners-1", Kind: "owners", HolderNodeIds: []string{"node-owner"}},
		{Id: "r-fin", Name: "Kế toán trưởng", Kind: "custom", HolderNodeIds: []string{"node-p1", "node-p2"}},
		{Id: "r-other", Name: "Thủ quỹ", Kind: "custom", HolderNodeIds: []string{"node-p9"}},
	}
	c := newContactsWorld(t, scope)
	c.st.rows = []store.User{
		{ID: "p1", NGACNodeID: "node-p1", DepartmentID: "d1"},
		{ID: "o", NGACNodeID: "node-owner"},
		{ID: "p3", NGACNodeID: "node-p3"},
	}

	page, err := c.list(domain.ContactQuery{})
	require.NoError(t, err)
	byID := map[string]domain.ContactInfo{}
	for _, p := range page.Contacts {
		byID[p.UserID] = p
	}
	assert.Equal(t, []domain.ContactRole{{ID: "r-fin", Name: "Kế toán trưởng", Kind: "custom"}}, byID["p1"].Roles)
	assert.Equal(t, "d1", byID["p1"].DepartmentID)
	assert.Equal(t, []domain.ContactRole{{ID: "owners-1", Kind: "owners"}}, byID["o"].Roles, "the owners group carries no name")
	assert.Empty(t, byID["p3"].Roles, "deny: a role this person does not hold")
	for _, p := range page.Contacts {
		for _, r := range p.Roles {
			assert.NotEqual(t, "Thủ quỹ", r.Name, "a role of someone not on the page is not attached to anyone")
		}
	}
}

func TestListContacts_PagingStillValidatedBeforeAnythingIsAsked(t *testing.T) {
	c := newContactsWorld(t, limitedScope())
	for _, q := range []domain.ContactQuery{
		{Limit: domain.MaxContactsLimit + 1},
		{Search: strings.Repeat("a", 101)},
		{Cursor: "%%%"},
	} {
		_, err := c.list(q)
		assert.ErrorIs(t, err, domain.ErrInvalidInput)
	}
	assert.Zero(t, c.st.listCalls)
}

// --- the resolver ---

func TestResolvePeople_ReturnsNameAndAvatarOnlyAndAsksNoWorkspace(t *testing.T) {
	c := newContactsWorld(t, nil) // a workspace service that would answer nil: it must not be asked
	c.st.resolved = []store.ResolvedPerson{{UserID: "gone", NGACNodeID: "node-gone", DisplayName: "Đã Rời", AvatarURL: "/a/gone"}}

	got, err := c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, []string{"gone", "ghost"}, []string{"node-x"})
	require.NoError(t, err)
	assert.Equal(t, []domain.ResolvedPerson{{UserID: "gone", NGACNodeID: "node-gone", DisplayName: "Đã Rời", AvatarURL: "/a/gone"}}, got)
	assert.Equal(t, c.tenant, c.st.resolveWS, "the lookup is bound to the caller's workspace")
	assert.Equal(t, [][]string{{"gone", "ghost"}, {"node-x"}}, c.st.resolveIDs)
	assert.Zero(t, c.ws.calls, "names keep working when the workspace service is down")
}

func TestResolvePeople_OnlyAnActiveGraphMemberMayAsk(t *testing.T) {
	c := newContactsWorld(t, nil)
	c.st.resolved = []store.ResolvedPerson{{UserID: "p", DisplayName: "Ai Đó"}}
	ask := func(userID, node string) ([]domain.ResolvedPerson, error) {
		return c.svc.ResolvePeople(context.Background(), userID, node, c.tenant, []string{"p"}, nil)
	}

	got, err := ask(c.caller.ID, c.caller.NGACNodeID)
	require.NoError(t, err, "allow: the member of the same fixture")
	require.Len(t, got, 1)
	c.st.resolveWS = ""

	// an active row but not under Members in the graph
	notInGraph := c.st.fakeWorld.addUser("nograph@example.vn")
	require.NoError(t, c.st.fakeWorld.InsertTenantUser(context.Background(), c.tenant, notInGraph.ID, "member", "active", notInGraph.NGACNodeID))
	c.st.fakeWorld.detach(notInGraph.NGACNodeID, c.tenant)
	got, err = ask(notInGraph.ID, notInGraph.NGACNodeID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "deny: row without graph membership")
	assert.Nil(t, got)

	// no row and no graph
	stranger := c.st.fakeWorld.addUser("stranger2@example.vn")
	_, err = ask(stranger.ID, stranger.NGACNodeID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "deny: not a member at all")

	// a removed member, even one still in the graph
	for _, status := range []string{"removed", "disabled", "invited"} {
		c.st.fakeWorld.mu.Lock()
		c.st.fakeWorld.members[c.tenant+"|"+c.caller.ID].Status = status
		c.st.fakeWorld.mu.Unlock()
		_, err = ask(c.caller.ID, c.caller.NGACNodeID)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, status)
	}
	assert.Empty(t, c.st.resolveWS, "no denied caller reached the lookup")
}

func TestResolvePeople_PolicyDownIsUnavailableNotDenied(t *testing.T) {
	c := newContactsWorld(t, nil)
	c.st.fakeWorld.policyDown = errors.New("policy: connection refused")
	_, err := c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, []string{"p"}, nil)
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.NotErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, c.st.resolveWS)
}

func TestResolvePeople_AtMostTwoHundredIdsInAll(t *testing.T) {
	c := newContactsWorld(t, nil)
	ids := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		ids = append(ids, "u"+strings.Repeat("x", i%5)+string(rune('a'+i%26)))
	}
	_, err := c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, ids, nil)
	assert.NoError(t, err, "allow: exactly 200")

	_, err = c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, ids[:100], append(append([]string{}, ids[:100]...), "one-more"))
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "deny: 201 across both lists")
	_, err = c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, "", []string{"a"}, nil)
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "no workspace")
}

func TestResolvePeople_AskingForNobodyAsksNothing(t *testing.T) {
	c := newContactsWorld(t, nil)
	got, err := c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Empty(t, c.st.resolveWS)
}

func TestResolvePeople_StoreFailureIsNotAnEmptyAnswer(t *testing.T) {
	c := newContactsWorld(t, nil)
	c.st.resolveErr = errors.New("db down")
	_, err := c.svc.ResolvePeople(context.Background(), c.caller.ID, c.caller.NGACNodeID, c.tenant, []string{"p"}, nil)
	assert.Error(t, err)
}

// A workspace service that predates GetDirectoryScope is an outage with its own name in the
// log, so an operator sees "deploy workspace first" and not a generic failure; the caller
// still gets "unavailable", never the directory.
func TestListContacts_AWorkspaceTooOldIsLoggedByName(t *testing.T) {
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	c := newContactsWorld(t, limitedScope())
	c.ws.err = status.Error(codes.Unimplemented, "unknown method GetDirectoryScope")
	_, err := c.list(domain.ContactQuery{})
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.Zero(t, c.st.listCalls)
	assert.Contains(t, buf.String(), "workspace service too old")

	// deny-side control: an ordinary outage is not reported as an old workspace
	buf.Reset()
	c.ws.err = status.Error(codes.Unavailable, "down")
	_, err = c.list(domain.ContactQuery{})
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.NotContains(t, buf.String(), "workspace service too old")
}

func TestOwnersRoleKindIsTheSharedConstant(t *testing.T) {
	assert.Equal(t, "owners", ngac.RoleKindOwners)
}
