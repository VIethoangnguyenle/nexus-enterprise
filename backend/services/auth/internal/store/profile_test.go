package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/store"
)

func str(s string) *string { return &s }

// profileFixture is one throwaway account in a pool the test cleans up.
type profileFixture struct {
	ctx  context.Context
	pool *pgxpool.Pool
	st   *store.Store
	n    int64
}

func newProfileFixture(t *testing.T) *profileFixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL())
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx), "test database must be reachable")
	t.Cleanup(pool.Close)
	return &profileFixture{ctx: ctx, pool: pool, st: store.New(pool), n: time.Now().UnixNano()}
}

func (f *profileFixture) node(tag string) string { return fmt.Sprintf("pfn-%s-%d", tag, f.n) }

func (f *profileFixture) user(t *testing.T, tag, email string) string {
	t.Helper()
	id := fmt.Sprintf("pf-%s-%d", tag, f.n)
	node := f.node(tag)
	_, err := f.pool.Exec(f.ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'U', '{}')`, node)
	require.NoError(t, err)
	require.NoError(t, f.st.CreateUser(f.ctx, id, "pf"+tag+fmt.Sprint(f.n), "", node, email, "u-"+id, "Initial Name", ""))
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE user_id = $1`, id)
		f.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
		f.pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, node)
	})
	return id
}

func TestAccountState_NewAccountIsUnverifiedAndOwesAProfile(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "state", fmt.Sprintf("state%d@example.vn", f.n))

	u, err := f.st.GetUserByID(f.ctx, id)
	require.NoError(t, err)
	assert.False(t, u.EmailVerified)
	assert.False(t, u.ProfileCompleted, "a new account has not been asked for its name yet")

	changed, err := f.st.MarkEmailVerified(f.ctx, id)
	require.NoError(t, err)
	require.True(t, changed)

	byEmail, err := f.st.GetUserByEmail(f.ctx, fmt.Sprintf("STATE%d@example.vn", f.n))
	require.NoError(t, err)
	assert.True(t, byEmail.EmailVerified, "every lookup reports the proof, not only GetUserByID")
}

func TestUpdateProfile_OnlyTouchesTheFieldsSent(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "patch", "")

	_, err := f.pool.Exec(f.ctx, `UPDATE users SET department = 'Tài chính', avatar_url = 'https://cdn.example.vn/a.png' WHERE id = $1`, id)
	require.NoError(t, err)
	require.NoError(t, f.st.UpdateProfile(f.ctx, id, store.ProfileChanges{Title: str("Kế toán"), Location: str("Hà Nội")}))
	require.NoError(t, f.st.UpdateProfile(f.ctx, id, store.ProfileChanges{DisplayName: str("Phạm Thuý An")}))

	var name, title, dept, loc, avatar string
	require.NoError(t, f.pool.QueryRow(f.ctx,
		`SELECT display_name, COALESCE(title,''), COALESCE(department,''), COALESCE(location,''), COALESCE(avatar_url,'') FROM users WHERE id = $1`, id,
	).Scan(&name, &title, &dept, &loc, &avatar))
	assert.Equal(t, "Phạm Thuý An", name)
	assert.Equal(t, "Kế toán", title, "a field that was not sent stays as it was")
	assert.Equal(t, "Hà Nội", loc)
	assert.Equal(t, "Tài chính", dept, "the profile never writes the department")
	assert.Equal(t, "https://cdn.example.vn/a.png", avatar, "nor the avatar")

	require.NoError(t, f.st.UpdateProfile(f.ctx, id, store.ProfileChanges{Title: str("")}))
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT COALESCE(title,'') FROM users WHERE id = $1`, id).Scan(&title))
	assert.Equal(t, "", title, "an empty string that was sent clears the field")
}

func TestUpdateProfile_MarksTheProfileDoneOnce(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "done", "")

	require.NoError(t, f.st.UpdateProfile(f.ctx, id, store.ProfileChanges{DisplayName: str("An")}))
	u, err := f.st.GetUserByID(f.ctx, id)
	require.NoError(t, err)
	assert.True(t, u.ProfileCompleted)

	var first time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT profile_completed_at FROM users WHERE id = $1`, id).Scan(&first))
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, f.st.UpdateProfile(f.ctx, id, store.ProfileChanges{Title: str("x")}))
	var second time.Time
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT profile_completed_at FROM users WHERE id = $1`, id).Scan(&second))
	assert.True(t, first.Equal(second), "the first completion is kept")
}

func TestUpdateProfile_UnknownUserIsNotFound(t *testing.T) {
	f := newProfileFixture(t)
	err := f.st.UpdateProfile(f.ctx, "no-such-user", store.ProfileChanges{DisplayName: str("x")})
	assert.ErrorIs(t, err, store.ErrNoSuchUser)
}

func TestListWorkspaceSummaries_RoleAndHeadcount(t *testing.T) {
	f := newProfileFixture(t)
	me := f.user(t, "me", "")
	other := f.user(t, "other", "")
	gone := f.user(t, "gone", "")

	mkWorkspace := func(tag string) string {
		id := fmt.Sprintf("pfw-%s-%d", tag, f.n)
		_, err := f.pool.Exec(f.ctx, `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $2, $3)`, id, "Khối "+tag, me)
		require.NoError(t, err)
		t.Cleanup(func() {
			f.pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE tenant_id = $1`, id)
			f.pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, id)
		})
		return id
	}
	mine := mkWorkspace("mine")
	shared := mkWorkspace("shared")
	notMine := mkWorkspace("notmine")

	require.NoError(t, f.st.InsertTenantUser(f.ctx, mine, me, "owner", "active", f.node("me")))
	require.NoError(t, f.st.InsertTenantUser(f.ctx, shared, me, "member", "active", f.node("me")))
	require.NoError(t, f.st.InsertTenantUser(f.ctx, shared, other, "owner", "active", f.node("other")))
	require.NoError(t, f.st.InsertTenantUser(f.ctx, shared, gone, "member", "disabled", f.node("gone")))
	require.NoError(t, f.st.InsertTenantUser(f.ctx, notMine, other, "owner", "active", f.node("other")))

	got, err := f.st.ListWorkspaceSummaries(f.ctx, me)
	require.NoError(t, err)

	byName := map[string]store.WorkspaceSummary{}
	for _, w := range got {
		byName[w.Name] = w
	}
	require.Len(t, byName, 2, "only the workspaces this person is an active member of")
	assert.Equal(t, store.WorkspaceSummary{ID: mine, Name: "Khối mine", Role: "owner", MemberCount: 1}, byName["Khối mine"])
	_, err = f.pool.Exec(f.ctx, `UPDATE workspaces SET domain = $2 WHERE id = $1`, shared, fmt.Sprintf("novapay-%d.vn", f.n))
	require.NoError(t, err)
	again, err := f.st.ListWorkspaceSummaries(f.ctx, me)
	require.NoError(t, err)
	for _, w := range again {
		if w.ID == shared {
			assert.Equal(t, fmt.Sprintf("novapay-%d.vn", f.n), w.Domain, "a claimed company domain is named")
		} else {
			assert.Empty(t, w.Domain, "a personal workspace has none")
		}
	}
	assert.Equal(t, store.WorkspaceSummary{ID: shared, Name: "Khối shared", Role: "member", MemberCount: 2}, byName["Khối shared"],
		"a disabled member is not counted")
	assert.NotContains(t, byName, "Khối notmine")
}

func TestGetUserByID_CarriesTheProfile(t *testing.T) {
	f := newProfileFixture(t)
	id := f.user(t, "prof", "")
	require.NoError(t, f.st.UpdateProfile(f.ctx, id, store.ProfileChanges{Title: str("Kế toán"), Location: str("Hà Nội")}))
	u, err := f.st.GetUserByID(f.ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "Kế toán", u.Title)
	assert.Equal(t, "Hà Nội", u.Location)
}

// directoryFixture is a workspace with three active people, one suspended
// person and one person in another workspace.
type directoryFixture struct {
	*profileFixture
	ws, other string
	dept      string
}

func newDirectoryFixture(t *testing.T) *directoryFixture {
	t.Helper()
	f := newProfileFixture(t)
	d := &directoryFixture{profileFixture: f}
	d.ws = fmt.Sprintf("dirws-%d", f.n)
	d.other = fmt.Sprintf("dirws2-%d", f.n)
	d.dept = fmt.Sprintf("dept-%d", f.n)
	owner := f.user(t, "own", "")
	for _, id := range []string{d.ws, d.other} {
		_, err := f.pool.Exec(f.ctx, `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, id, owner)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		for _, id := range []string{d.ws, d.other} {
			f.pool.Exec(context.Background(), `DELETE FROM departments WHERE workspace_id = $1`, id)
			f.pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE tenant_id = $1`, id)
			f.pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, id)
		}
	})
	// "Đối soát" exists only in this workspace.
	_, err := f.pool.Exec(f.ctx, `INSERT INTO departments (id, workspace_id, name, ngac_ua_id) VALUES ($1, $2, 'Đối soát', $3)`, d.dept, d.ws, f.node("own"))
	require.NoError(t, err)
	return d
}

func (d *directoryFixture) join(t *testing.T, tag, name, status string, completed bool, department string, ws string) string {
	t.Helper()
	id := d.user(t, tag, "")
	_, err := d.pool.Exec(d.ctx, `UPDATE users SET display_name = $2, title = $3 WHERE id = $1`, id, name, "Chuyên viên "+tag)
	require.NoError(t, err)
	if completed {
		_, err = d.pool.Exec(d.ctx, `UPDATE users SET profile_completed_at = now() WHERE id = $1`, id)
		require.NoError(t, err)
	}
	var dep any
	if department != "" {
		dep = department
	}
	_, err = d.pool.Exec(d.ctx, `INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id, department_id) VALUES ($1, $2, 'member', $3, $4, $5)`,
		ws, id, status, d.node(tag), dep)
	require.NoError(t, err)
	return id
}

func TestListContacts_NamesTheAdminAssignedDepartmentAndNoHandleAsAName(t *testing.T) {
	d := newDirectoryFixture(t)
	lan := d.join(t, "lan", "Nguyễn Thu Lan", "active", true, d.dept, d.ws)
	// Never asked for a name: the display name is only the handle made from the address.
	handle := d.join(t, "han", "hoa.le.novapay", "active", false, "", d.ws)
	// Her own profile says a department, which nobody but an administrator may set.
	_, err := d.pool.Exec(d.ctx, `UPDATE users SET department = 'Tự khai' WHERE id = $1`, lan)
	require.NoError(t, err)

	got, total, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	byID := map[string]store.User{}
	for _, u := range got {
		byID[u.ID] = u
	}
	assert.Equal(t, "Đối soát", byID[lan].Department, "the department is the one an administrator assigned in this workspace")
	assert.Equal(t, "Nguyễn Thu Lan", byID[lan].DisplayName)
	assert.Equal(t, "", byID[handle].DisplayName, "a handle is not sent as a name")
	assert.Equal(t, "", byID[handle].Department, "no assignment, no department, whatever the person wrote about themselves")
}

func TestListContacts_OnlyActiveMembersOfThisWorkspace(t *testing.T) {
	d := newDirectoryFixture(t)
	d.join(t, "a1", "An", "active", true, "", d.ws)
	d.join(t, "b1", "Bình", "disabled", true, "", d.ws)
	d.join(t, "c1", "Chi", "invited", true, "", d.ws)
	d.join(t, "d1", "Dũng", "active", true, "", d.other)

	got, total, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, got, 1)
	assert.Equal(t, "An", got[0].DisplayName)
}

func TestListContacts_PagesByCursorAndReportsTheTrueTotal(t *testing.T) {
	d := newDirectoryFixture(t)
	for i := 0; i < 7; i++ {
		d.join(t, fmt.Sprintf("p%d", i), fmt.Sprintf("Người %02d", i), "active", true, "", d.ws)
	}

	first, total, next, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 3})
	require.NoError(t, err)
	assert.Equal(t, 7, total, "the total counts everyone, not the page")
	require.Len(t, first, 3)
	require.NotNil(t, next, "more people follow")

	second, total2, next2, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 3, After: next})
	require.NoError(t, err)
	assert.Equal(t, 7, total2, "and it is the same on every page")
	require.Len(t, second, 3)
	require.NotNil(t, next2)
	last, _, next3, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 3, After: next2})
	require.NoError(t, err)
	require.Len(t, last, 1)
	assert.Nil(t, next3, "the last page has no next cursor")

	seen := map[string]bool{}
	var order []string
	for _, page := range [][]store.User{first, second, last} {
		for _, u := range page {
			assert.False(t, seen[u.ID], "a person appears on one page only")
			seen[u.ID] = true
			order = append(order, u.DisplayName)
		}
	}
	assert.Len(t, seen, 7)
	assert.IsIncreasing(t, order, "pages continue in one order")
}

func TestListContacts_AnExactMultipleOfThePageSizeEndsCleanly(t *testing.T) {
	d := newDirectoryFixture(t)
	for i := 0; i < 4; i++ {
		d.join(t, fmt.Sprintf("q%d", i), fmt.Sprintf("Bạn %02d", i), "active", true, "", d.ws)
	}
	_, _, next, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 4})
	require.NoError(t, err)
	assert.Nil(t, next, "four people in pages of four: there is no fifth page to offer")
}

func TestListContacts_APersonWhoJoinsBetweenPagesMovesNothingAlreadyRead(t *testing.T) {
	d := newDirectoryFixture(t)
	for i := 0; i < 4; i++ {
		d.join(t, fmt.Sprintf("m%d", i), fmt.Sprintf("Bạn %02d", i+1), "active", true, "", d.ws)
	}
	first, _, next, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 2})
	require.NoError(t, err)
	require.NotNil(t, next)
	d.join(t, "new", "Bạn 00", "active", true, "", d.ws) // sorts before everything already read

	rest, total, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Limit: 10, After: next})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	for _, u := range rest {
		for _, read := range first {
			assert.NotEqual(t, read.ID, u.ID, "nobody is listed twice")
		}
	}
	assert.Len(t, rest, 2, "the two people after the cursor")
}

func TestListContacts_Filters(t *testing.T) {
	d := newDirectoryFixture(t)
	d.join(t, "f1", "Nguyễn Thu Lan", "active", true, d.dept, d.ws)
	d.join(t, "f2", "Lê Quang Vinh", "active", true, "", d.ws)
	_, err := d.pool.Exec(d.ctx, `UPDATE users SET location = 'Hà Nội' WHERE id = (SELECT user_id FROM tenant_users WHERE tenant_id = $1 AND ngac_node_id = $2)`, d.ws, d.node("f2"))
	require.NoError(t, err)

	byDept, total, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Department: "Đối soát", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "Nguyễn Thu Lan", byDept[0].DisplayName)

	byPlace, _, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Location: "Hà Nội", Limit: 50})
	require.NoError(t, err)
	require.Len(t, byPlace, 1)
	assert.Equal(t, "Lê Quang Vinh", byPlace[0].DisplayName)

	bySearch, total, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Search: "quang", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "Lê Quang Vinh", bySearch[0].DisplayName)

	wildcard, total, _, err := d.st.ListContactsByWorkspace(d.ctx, d.ws, store.ContactFilter{Search: "%", Limit: 50})
	require.NoError(t, err)
	assert.Zero(t, total, "a % in the search is a character, not a wildcard")
	assert.Empty(t, wildcard)
}

func TestGetTenantUser_CarriesTheAssignedDepartmentName(t *testing.T) {
	d := newDirectoryFixture(t)
	d.join(t, "g1", "An", "active", true, d.dept, d.ws)
	id := d.join(t, "g2", "Bình", "active", true, "", d.ws)

	with, err := d.st.GetTenantUser(d.ctx, d.ws, mustUserOf(t, d, "g1"))
	require.NoError(t, err)
	assert.Equal(t, "Đối soát", with.DepartmentName)
	without, err := d.st.GetTenantUser(d.ctx, d.ws, id)
	require.NoError(t, err)
	assert.Equal(t, "", without.DepartmentName)
}

func mustUserOf(t *testing.T, d *directoryFixture, tag string) string {
	t.Helper()
	var id string
	require.NoError(t, d.pool.QueryRow(d.ctx, `SELECT user_id FROM tenant_users WHERE tenant_id = $1 AND ngac_node_id = $2`, d.ws, d.node(tag)).Scan(&id))
	return id
}
