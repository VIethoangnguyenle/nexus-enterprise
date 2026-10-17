package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/domain"
	"ngac-platform/services/calendar/internal/policyfake"
	"ngac-platform/services/calendar/internal/store"
)

// End to end through the real routes, JWT middleware, domain and store, against
// the test database. Only the policy service is faked (policyfake), and its nodes
// are mirrored into ngac_nodes because the calendar tables hold foreign keys onto
// them. The test is skipped when the database is not there.

func dbURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable"
}

type flow struct {
	t       *testing.T
	pool    *pgxpool.Pool
	e       *echo.Echo
	w       *policyfake.World
	sfx     string
	ws      string
	users   map[string]string // short name -> users.id
	nodes   map[string]string // short name -> user node id
	chat    string            // a private conversation alice is in
	dm      string
	nodeIDs []string
	mu      sync.Mutex
	secret  string
}

func newFlow(t *testing.T) *flow {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL())
	if err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `SELECT 1 FROM calendars LIMIT 0`); err != nil {
		t.Skipf("calendar schema not applied (make db-migrate): %v", err)
	}

	f := &flow{t: t, pool: pool, w: policyfake.NewWorld(), sfx: uuid.New().String()[:8], secret: httputil.DevJWTSecret,
		users: map[string]string{}, nodes: map[string]string{}}
	f.ws = "cal-ws-" + f.sfx
	f.w.IDPrefix = f.sfx + "-"
	f.w.OnNodeCreated = func(n *policypb.NGACNode) error { return f.insertNode(n.Id, n.Name, n.NodeType) }
	f.w.OnNodeDeleted = func(id string) {
		f.mu.Lock()
		defer f.mu.Unlock()
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, id)
	}

	exec := func(q string, args ...any) {
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	seed := func(id, name, typ string) {
		require.NoError(t, f.insertNode(id, name, typ))
		f.w.Seed(id, name, typ, nil)
	}
	wid := ngac.WorkspaceID(f.ws)
	seed("pc-"+f.sfx, ngac.PCName(wid), ngac.TypePC)
	seed("members-"+f.sfx, ngac.MembersUAName(wid), ngac.TypeUA)
	seed("owners-"+f.sfx, ngac.OwnersUAName(wid), ngac.TypeUA)
	f.w.Assign("members-"+f.sfx, "pc-"+f.sfx)
	f.w.Assign("owners-"+f.sfx, "pc-"+f.sfx)

	people := []struct{ key, name string }{{"alice", "Alice Nguyen"}, {"bob", "Bob Tran"}, {"carol", "Carol Le"}, {"gus", "Gus Outsider"}}
	for _, p := range people {
		id := "cal-u-" + p.key + "-" + f.sfx
		f.users[p.key], f.nodes[p.key] = id, "unode-"+p.key+"-"+f.sfx
		seed(f.nodes[p.key], "U_"+id, ngac.TypeU)
		exec(`INSERT INTO users (id, username, password, display_name) VALUES ($1, $1, '', $2)`, id, p.name)
		if p.key != "gus" {
			f.w.Assign(f.nodes[p.key], "members-"+f.sfx)
		}
	}
	exec(`INSERT INTO workspaces (id, name, owner_id, type, ngac_pc_id) VALUES ($1, 'Acme Calendar Test', $2, 'organization', $3)`,
		f.ws, f.users["alice"], "pc-"+f.sfx)
	for _, k := range []string{"alice", "bob", "carol"} {
		exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`,
			f.ws, f.users[k], f.nodes[k])
	}

	// A private conversation alice is in, and a direct message.
	f.chat, f.dm = "cal-ch-"+f.sfx, "cal-dm-"+f.sfx
	seed("ch-oa-"+f.sfx, ngac.ChannelContentOAName(ngac.ChannelID(f.chat)), ngac.TypeOA)
	seed("ch-ua-"+f.sfx, ngac.ChannelMembersUAName(ngac.ChannelID(f.chat)), ngac.TypeUA)
	seed("dm-oa-"+f.sfx, ngac.ChannelContentOAName(ngac.ChannelID(f.dm)), ngac.TypeOA)
	f.w.Assocs[[2]string{"ch-ua-" + f.sfx, "ch-oa-" + f.sfx}] = ngac.ChannelMemberOps()
	f.w.Assocs[[2]string{"ch-ua-" + f.sfx, "dm-oa-" + f.sfx}] = ngac.ChannelMemberOps()
	f.w.Assign(f.nodes["alice"], "ch-ua-"+f.sfx)
	exec(`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id) VALUES ($1, 'Design team', 'private', $2, $3, $4)`,
		f.chat, f.ws, "ch-oa-"+f.sfx, "ch-ua-"+f.sfx)
	exec(`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id) VALUES ($1, 'Alice, Bob', 'dm', $2, $3, $4)`,
		f.dm, f.ws, "dm-oa-"+f.sfx, "ch-ua-"+f.sfx)
	exec(`INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2), ($3, $2)`, f.chat, f.nodes["alice"], f.dm)

	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM calendars WHERE workspace_id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM channels WHERE workspace_id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM tenant_users WHERE tenant_id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, f.ws)
		for _, id := range f.users {
			pool.Exec(c, `DELETE FROM users WHERE id = $1`, id)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, id := range f.nodeIDs {
			pool.Exec(c, `DELETE FROM ngac_nodes WHERE id = $1`, id)
		}
		pool.Exec(c, `DELETE FROM ngac_nodes WHERE id LIKE $1`, "node-"+f.sfx+"-%")
	})

	f.e = httputil.NewEcho("calendar-test")
	NewHandler(domain.NewService(store.New(pool), f.w.Read(), f.w.Write(), nil)).RegisterRoutes(f.e, f.secret)
	return f
}

func (f *flow) insertNode(id, name, typ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`, id, name, typ)
	if err == nil {
		f.nodeIDs = append(f.nodeIDs, id)
	}
	return err
}

// token signs a session for a person scoped to a workspace.
func (f *flow) token(who, tenant string) string {
	claims := &httputil.Claims{UserID: f.users[who], NGACNodeID: f.nodes[who], TenantID: tenant,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(f.secret))
	require.NoError(f.t, err)
	return s
}

// do sends a request as who in the flow's workspace ("" sends no token).
func (f *flow) do(method, path, who, body string) (int, []byte) {
	return f.doIn(method, path, who, f.ws, body)
}

func (f *flow) doIn(method, path, who, tenant, body string) (int, []byte) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if who != "" {
		req.Header.Set("Authorization", "Bearer "+f.token(who, tenant))
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func (f *flow) base() string { return "/api/workspaces/" + f.ws + "/calendar" }

type calJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Access  string `json:"access"`
	Color   int    `json:"color"`
	Visible bool   `json:"visible"`
	IsMine  bool   `json:"isMine"`
	Owner   *struct {
		NodeID      string `json:"nodeId"`
		DisplayName string `json:"displayName"`
		Hue         int    `json:"hue"`
	} `json:"owner"`
	Channel *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channel"`
}

func (f *flow) list(who string) []calJSON {
	f.t.Helper()
	code, body := f.do(http.MethodGet, f.base()+"/calendars", who, "")
	require.Equal(f.t, http.StatusOK, code, string(body))
	var out struct {
		Calendars []calJSON `json:"calendars"`
	}
	require.NoError(f.t, json.Unmarshal(body, &out))
	return out.Calendars
}

func (f *flow) count(q string, args ...any) int {
	var n int
	require.NoError(f.t, f.pool.QueryRow(context.Background(), q, args...).Scan(&n))
	return n
}

func TestListCreatesCalendarsOnceAndLeaksNoNodeIDs(t *testing.T) {
	f := newFlow(t)
	first := f.list("alice")
	require.Len(t, first, 2)
	again := f.list("alice")
	require.Len(t, again, 2)
	assert.Equal(t, first[0].ID, again[0].ID)
	assert.Equal(t, first[1].ID, again[1].ID)
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM calendars WHERE workspace_id = $1`, f.ws))
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM ngac_nodes WHERE name LIKE 'Cal\_%' AND id LIKE $1`, "node-"+f.sfx+"-%"),
		"one OA per calendar, however often it is opened")

	assert.Equal(t, "personal", first[0].Kind)
	assert.Equal(t, "Alice Nguyen", first[0].Name)
	assert.Equal(t, "manage", first[0].Access)
	assert.True(t, first[0].IsMine)
	require.NotNil(t, first[0].Owner)
	assert.Equal(t, "Alice Nguyen", first[0].Owner.DisplayName)
	assert.Equal(t, first[0].Color, first[0].Owner.Hue, "a personal calendar is drawn in its owner's hue")
	assert.Equal(t, f.nodes["alice"], first[0].Owner.NodeID)
	assert.Nil(t, first[0].Channel)
	assert.Equal(t, "workspace", first[1].Kind)
	assert.Equal(t, "read", first[1].Access)
	assert.Equal(t, -1, first[1].Color, "the workspace calendar uses the accent colour")
	assert.False(t, first[1].IsMine)
	assert.Nil(t, first[1].Owner)

	_, body := f.do(http.MethodGet, f.base()+"/calendars", "alice", "")
	assert.NotContains(t, string(body), "node-"+f.sfx, "no calendar OA id may reach a client")
	assert.NotContains(t, strings.ToLower(string(body)), "ngac")
}

func TestConcurrentFirstUseMakesOneCalendar(t *testing.T) {
	f := newFlow(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, body := f.do(http.MethodGet, f.base()+"/calendars", "alice", "")
			assert.Equal(t, http.StatusOK, code, string(body))
		}()
	}
	wg.Wait()
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM calendars WHERE workspace_id = $1`, f.ws))
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM ngac_nodes WHERE name LIKE 'Cal\_%' AND id LIKE $1`, "node-"+f.sfx+"-%"),
		"the races that lost must leave no node of their own")
}

func TestShareFlow(t *testing.T) {
	f := newFlow(t)
	var mine calJSON
	for _, c := range f.list("alice") {
		if c.Kind == "personal" {
			mine = c
		}
	}
	share := func(who, grantee, level string) (int, []byte) {
		return f.do(http.MethodPut, f.base()+"/calendars/"+mine.ID+"/shares/"+f.nodes[grantee], who, `{"level":"`+level+`"}`)
	}

	// A colleague with no share sees busy ranges only.
	for _, c := range f.list("bob") {
		assert.NotEqual(t, mine.ID, c.ID, "an unshared calendar is not on a colleague's list")
	}

	code, body := share("alice", "bob", "view")
	require.Equal(t, http.StatusOK, code, string(body))
	var got struct {
		Level   string `json:"level"`
		Grantee struct {
			DisplayName string `json:"displayName"`
			Hue         int    `json:"hue"`
		} `json:"grantee"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "view", got.Level)
	assert.Equal(t, "Bob Tran", got.Grantee.DisplayName)

	access := func(who string) string {
		for _, c := range f.list(who) {
			if c.ID == mine.ID {
				return c.Access
			}
		}
		return "absent"
	}
	assert.Equal(t, "read", access("bob"))
	code, _ = share("alice", "bob", "edit")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "write", access("bob"))
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendar_shares WHERE calendar_id = $1`, mine.ID), "a changed level replaces the share")

	code, body = f.do(http.MethodGet, f.base()+"/calendars/"+mine.ID+"/shares", "alice", "")
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(body), "Bob Tran")
	assert.NotContains(t, string(body), f.users["bob"], "the account id is not shown")

	// Deny paths. A workspace colleague who was not shared with, and the grantee
	// at "edit", can neither read the shares nor change them.
	code, _ = share("carol", "carol", "view")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = share("bob", "carol", "view")
	assert.Equal(t, http.StatusForbidden, code, "edit does not carry share")
	code, _ = f.do(http.MethodGet, f.base()+"/calendars/"+mine.ID+"/shares", "carol", "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodDelete, f.base()+"/calendars/"+mine.ID+"/shares/"+f.nodes["bob"], "carol", "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = share("alice", "bob", "root")
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = share("alice", "gus", "view")
	assert.Equal(t, http.StatusForbidden, code, "a person outside the workspace gets nothing")
	code, _ = share("alice", "alice", "view")
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendar_shares WHERE calendar_id = $1`, mine.ID))

	// Revoked: the next decision is DENY, and a colleague falls back to busy ranges.
	code, _ = f.do(http.MethodDelete, f.base()+"/calendars/"+mine.ID+"/shares/"+f.nodes["bob"], "alice", "")
	require.Equal(t, http.StatusNoContent, code)
	assert.Equal(t, "freebusy", access("bob"))
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_shares WHERE calendar_id = $1`, mine.ID))
}

func TestSubscriptions(t *testing.T) {
	f := newFlow(t)
	code, body := f.do(http.MethodPost, f.base()+"/subscriptions", "alice", `{"userNodeId":"`+f.nodes["bob"]+`"}`)
	require.Equal(t, http.StatusOK, code, string(body))
	var bobs calJSON
	require.NoError(t, json.Unmarshal(body, &bobs))
	assert.Equal(t, "Bob Tran", bobs.Name)
	assert.Equal(t, "freebusy", bobs.Access)

	code, _ = f.do(http.MethodPost, f.base()+"/subscriptions", "alice", `{"userNodeId":"`+f.nodes["gus"]+`"}`)
	assert.Equal(t, http.StatusForbidden, code, "someone outside the workspace cannot be subscribed to")
	code, _ = f.do(http.MethodPost, f.base()+"/subscriptions", "alice", `{}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = f.do(http.MethodPost, f.base()+"/subscriptions", "alice", `not json`)
	assert.Equal(t, http.StatusBadRequest, code)

	code, body = f.do(http.MethodPut, f.base()+"/calendars/"+bobs.ID+"/subscription", "alice", `{"color":7,"visible":false}`)
	require.Equal(t, http.StatusOK, code, string(body))
	var set calJSON
	require.NoError(t, json.Unmarshal(body, &set))
	assert.Equal(t, 7, set.Color)
	assert.False(t, set.Visible)
	found := false
	for _, c := range f.list("alice") {
		if c.ID == bobs.ID {
			found = true
			assert.Equal(t, 7, c.Color)
			assert.False(t, c.Visible)
		}
	}
	assert.True(t, found)
	code, _ = f.do(http.MethodPut, f.base()+"/calendars/"+bobs.ID+"/subscription", "alice", `{"color":500}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	code, _ = f.do(http.MethodDelete, f.base()+"/subscriptions/"+bobs.ID, "alice", "")
	assert.Equal(t, http.StatusNoContent, code)
	for _, c := range f.list("alice") {
		assert.NotEqual(t, bobs.ID, c.ID)
	}
}

func TestGroupCalendar(t *testing.T) {
	f := newFlow(t)
	path := f.base() + "/channels/" + f.chat + "/calendar"

	code, body := f.do(http.MethodPost, path, "alice", "")
	require.Equal(t, http.StatusOK, code, string(body))
	var cal calJSON
	require.NoError(t, json.Unmarshal(body, &cal))
	assert.Equal(t, "channel", cal.Kind)
	assert.Equal(t, "Design team", cal.Name)
	require.NotNil(t, cal.Channel)
	assert.Equal(t, f.chat, cal.Channel.ID)
	assert.Equal(t, "Design team", cal.Channel.Name)
	assert.Equal(t, "write", cal.Access)

	code, body = f.do(http.MethodPost, path, "alice", "")
	require.Equal(t, http.StatusOK, code)
	var again calJSON
	require.NoError(t, json.Unmarshal(body, &again))
	assert.Equal(t, cal.ID, again.ID)
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendars WHERE kind = 'channel' AND workspace_id = $1`, f.ws))

	listed := false
	for _, c := range f.list("alice") {
		listed = listed || c.ID == cal.ID
	}
	assert.True(t, listed, "a member finds the group calendar on their list")
	for _, c := range f.list("carol") {
		assert.NotEqual(t, cal.ID, c.ID, "a workspace colleague outside the group does not")
	}

	// Deny: not in the group; and a direct message has no calendar.
	code, _ = f.do(http.MethodPost, path, "carol", "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodPost, f.base()+"/channels/"+f.dm+"/calendar", "alice", "")
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendars WHERE channel_id = $1`, f.dm))
	code, _ = f.do(http.MethodPost, f.base()+"/channels/nope/calendar", "alice", "")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestWorkspaceAndSessionBoundaries(t *testing.T) {
	f := newFlow(t)
	code, _ := f.do(http.MethodGet, f.base()+"/calendars", "", "")
	assert.Equal(t, http.StatusUnauthorized, code)

	// A token scoped to another workspace cannot read this one's calendars.
	code, _ = f.doIn(http.MethodGet, f.base()+"/calendars", "alice", "some-other-workspace", "")
	assert.Equal(t, http.StatusForbidden, code)

	// Someone outside the workspace is refused even with a token naming it.
	code, _ = f.do(http.MethodGet, f.base()+"/calendars", "gus", "")
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendars WHERE workspace_id = $1`, f.ws), "refused callers create nothing")

	// A calendar that does not exist.
	code, _ = f.do(http.MethodGet, f.base()+"/calendars/missing/shares", "alice", "")
	assert.Equal(t, http.StatusNotFound, code)
}
