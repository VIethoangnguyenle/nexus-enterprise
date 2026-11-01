package store_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/store"
)

// titleFixture is one person with an account default title, an active member of
// two organizations, a disabled member of a third, and absent from a fourth.
type titleFixture struct {
	*profileFixture
	me                          string
	wsA, wsB, disabled, foreign string
}

func newTitleFixture(t *testing.T) *titleFixture {
	t.Helper()
	f := newProfileFixture(t)
	tf := &titleFixture{profileFixture: f}
	tf.me = f.user(t, "ttl", "")
	_, err := f.pool.Exec(f.ctx, `UPDATE users SET title = 'Mặc định', profile_completed_at = now() WHERE id = $1`, tf.me)
	require.NoError(t, err)
	mk := func(tag string) string {
		id := fmt.Sprintf("ttlws-%s-%d", tag, f.n)
		_, err := f.pool.Exec(f.ctx, `INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization')`, id, tf.me)
		require.NoError(t, err)
		t.Cleanup(func() {
			f.pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE tenant_id = $1`, id)
			f.pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, id)
		})
		return id
	}
	tf.wsA, tf.wsB, tf.disabled, tf.foreign = mk("a"), mk("b"), mk("off"), mk("far")
	require.NoError(t, f.st.InsertTenantUser(f.ctx, tf.wsA, tf.me, "member", "active", f.node("ttl")))
	require.NoError(t, f.st.InsertTenantUser(f.ctx, tf.wsB, tf.me, "member", "active", f.node("ttl")))
	require.NoError(t, f.st.InsertTenantUser(f.ctx, tf.disabled, tf.me, "member", "disabled", f.node("ttl")))
	return tf
}

// rawTitle is what the membership row stores: nil when nothing was set there.
func (tf *titleFixture) rawTitle(t *testing.T, ws string) *string {
	t.Helper()
	var title *string
	require.NoError(t, tf.pool.QueryRow(tf.ctx, `SELECT title FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, ws, tf.me).Scan(&title))
	return title
}

func (tf *titleFixture) accountTitle(t *testing.T) string {
	t.Helper()
	var title string
	require.NoError(t, tf.pool.QueryRow(tf.ctx, `SELECT COALESCE(title,'') FROM users WHERE id = $1`, tf.me).Scan(&title))
	return title
}

func (tf *titleFixture) shownTitle(t *testing.T, ws string) string {
	t.Helper()
	m, err := tf.st.GetTenantUser(tf.ctx, ws, tf.me)
	require.NoError(t, err)
	require.NotNil(t, m)
	return m.Title
}

func TestWorkspaceTitle_WriteChangesOnlyThatWorkspace(t *testing.T) {
	tf := newTitleFixture(t)

	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{Title: str("Kế toán trưởng"), WorkspaceID: tf.wsA}))

	assert.Equal(t, "Kế toán trưởng", *tf.rawTitle(t, tf.wsA))
	assert.Nil(t, tf.rawTitle(t, tf.wsB), "another workspace is not touched")
	assert.Equal(t, "Mặc định", tf.accountTitle(t), "nor the account default")
	assert.Equal(t, "Kế toán trưởng", tf.shownTitle(t, tf.wsA))
	assert.Equal(t, "Mặc định", tf.shownTitle(t, tf.wsB), "a workspace where nothing was set shows the account default")
}

func TestWorkspaceTitle_ClearedInAWorkspaceShowsEmpty(t *testing.T) {
	tf := newTitleFixture(t)

	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{Title: str(""), WorkspaceID: tf.wsA}))

	require.NotNil(t, tf.rawTitle(t, tf.wsA))
	assert.Equal(t, "", tf.shownTitle(t, tf.wsA), "a title cleared here is empty here, not the default")
	assert.Equal(t, "Mặc định", tf.shownTitle(t, tf.wsB))
}

func TestWorkspaceTitle_OtherFieldsStayAccountWide(t *testing.T) {
	tf := newTitleFixture(t)

	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{
		DisplayName: str("Lê Thị Lan"), Location: str("Hà Nội"), WorkspaceID: tf.wsA,
	}))
	var name, loc string
	require.NoError(t, tf.pool.QueryRow(tf.ctx, `SELECT display_name, COALESCE(location,'') FROM users WHERE id = $1`, tf.me).Scan(&name, &loc))
	assert.Equal(t, "Lê Thị Lan", name)
	assert.Equal(t, "Hà Nội", loc)
	assert.Nil(t, tf.rawTitle(t, tf.wsA), "no title sent, none set in the workspace")
}

func TestWorkspaceTitle_ATitleSetInAWorkspaceCompletesTheProfile(t *testing.T) {
	tf := newTitleFixture(t)
	_, err := tf.pool.Exec(tf.ctx, `UPDATE users SET profile_completed_at = NULL WHERE id = $1`, tf.me)
	require.NoError(t, err)

	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{Title: str("Kế toán"), WorkspaceID: tf.wsA}))
	u, err := tf.st.GetUserByID(tf.ctx, tf.me)
	require.NoError(t, err)
	assert.True(t, u.ProfileCompleted)
}

// A workspace the person is not an active member of, or one that does not exist,
// is refused, and nothing of the request is written: not the title, not the name.
func TestWorkspaceTitle_NotAMemberIsRefusedAndWritesNothing(t *testing.T) {
	tf := newTitleFixture(t)
	for name, ws := range map[string]string{
		"another person's workspace": tf.foreign,
		"a disabled membership":      tf.disabled,
		"no such workspace":          "ttlws-none-" + fmt.Sprint(tf.n),
	} {
		t.Run(name, func(t *testing.T) {
			err := tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{
				DisplayName: str("Đổi tên"), Title: str("Giám đốc"), WorkspaceID: ws,
			})
			require.ErrorIs(t, err, store.ErrNotMember)

			var name string
			require.NoError(t, tf.pool.QueryRow(tf.ctx, `SELECT display_name FROM users WHERE id = $1`, tf.me).Scan(&name))
			assert.Equal(t, "Initial Name", name, "the refused request's other fields are not applied")
			assert.Equal(t, "Mặc định", tf.accountTitle(t))
			var n int
			require.NoError(t, tf.pool.QueryRow(tf.ctx,
				`SELECT count(*) FROM tenant_users WHERE user_id = $1 AND title IS NOT NULL`, tf.me).Scan(&n))
			assert.Zero(t, n, "no membership row got a title")
		})
	}
}

func TestWorkspaceTitle_NoWorkspaceWritesTheAccountDefault(t *testing.T) {
	tf := newTitleFixture(t)

	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{Title: str("Chuyên viên")}))

	assert.Equal(t, "Chuyên viên", tf.accountTitle(t))
	assert.Nil(t, tf.rawTitle(t, tf.wsA), "no workspace row is written")
	assert.Equal(t, "Chuyên viên", tf.shownTitle(t, tf.wsA), "a workspace with nothing set follows the default")
}

func TestWorkspaceTitle_TheDirectoryShowsAndSearchesTheWorkspaceTitle(t *testing.T) {
	tf := newTitleFixture(t)
	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{Title: str("Kế toán trưởng"), WorkspaceID: tf.wsA}))

	list := func(ws, search string) []store.User {
		got, _, _, err := tf.st.ListContactsByWorkspace(tf.ctx, ws, store.ContactFilter{Scope: allPeople, Search: search, Limit: 50})
		require.NoError(t, err)
		return got
	}
	inA := list(tf.wsA, "")
	require.Len(t, inA, 1)
	assert.Equal(t, "Kế toán trưởng", inA[0].Title)
	inB := list(tf.wsB, "")
	require.Len(t, inB, 1)
	assert.Equal(t, "Mặc định", inB[0].Title, "the other workspace shows the default")

	assert.Len(t, list(tf.wsA, "toán trưởng"), 1, "the workspace title is searched")
	assert.Empty(t, list(tf.wsA, "Mặc định"), "the default it replaced is not")
	assert.Len(t, list(tf.wsB, "Mặc định"), 1)
	assert.Empty(t, list(tf.wsB, "toán trưởng"), "another workspace's title is not searchable here")
}

// The migration copies the account title into each membership that has none, once:
// a title already set in a workspace, and an account without a title, are left alone.
func TestMigration071_BackfillsTheAccountTitleIntoEachMembership(t *testing.T) {
	tf := newTitleFixture(t)
	require.NoError(t, tf.st.UpdateProfile(tf.ctx, tf.me, store.ProfileChanges{Title: str("Riêng A"), WorkspaceID: tf.wsA}))
	bare := tf.user(t, "bare", "")
	_, err := tf.pool.Exec(tf.ctx, `UPDATE users SET title = '' WHERE id = $1`, bare)
	require.NoError(t, err)
	require.NoError(t, tf.st.InsertTenantUser(tf.ctx, tf.wsB, bare, "member", "active", tf.node("bare")))

	raw, err := os.ReadFile("../../../../../data/migrations/071_tenant_user_title.sql")
	require.NoError(t, err)
	// The file carries its own BEGIN/COMMIT; run its body inside a transaction that is
	// rolled back, so the shared test database keeps every other row as it was.
	body := strings.NewReplacer("BEGIN;", "", "COMMIT;", "").Replace(string(raw))
	tx, err := tf.pool.Begin(tf.ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	for i := 0; i < 2; i++ { // re-runnable
		_, err = tx.Exec(tf.ctx, body)
		require.NoError(t, err)
	}

	title := func(ws, user string) *string {
		var v *string
		require.NoError(t, tx.QueryRow(tf.ctx, `SELECT title FROM tenant_users WHERE tenant_id = $1 AND user_id = $2`, ws, user).Scan(&v))
		return v
	}
	require.NotNil(t, title(tf.wsA, tf.me))
	assert.Equal(t, "Riêng A", *title(tf.wsA, tf.me), "a title set in the workspace is kept")
	require.NotNil(t, title(tf.wsB, tf.me))
	assert.Equal(t, "Mặc định", *title(tf.wsB, tf.me), "the account title is copied where none was set")
	assert.Nil(t, title(tf.wsB, bare), "an account without a title copies nothing")
}
