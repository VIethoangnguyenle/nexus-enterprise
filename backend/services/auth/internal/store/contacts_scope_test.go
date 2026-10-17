package store_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/store"
)

// allPeople is the scope of a caller who may see everyone.
var allPeople = store.ContactScope{All: true}

// scopeFixture is one workspace with two departments and the people of the visibility
// rules: a caller in "Kinh doanh" (the one department they may see), a colleague of theirs,
// someone in the hidden "Nhân sự", an owner who sits in that hidden department, and a
// person with no department. Every person has an e-mail so a search can try to find them.
type scopeFixture struct {
	*directoryFixture
	open, hidden                               string // department ids
	caller, colleague, hiddenOne, owner, loose string // user ids
	ownerNode, hiddenNode                      string
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	d := newDirectoryFixture(t)
	s := &scopeFixture{directoryFixture: d, open: d.dept, hidden: fmt.Sprintf("dept-h-%d", d.n)}
	_, err := d.pool.Exec(d.ctx, `INSERT INTO departments (id, workspace_id, name, ngac_ua_id) VALUES ($1, $2, 'Nhân sự', $3)`, s.hidden, d.ws, d.node("own"))
	require.NoError(t, err)
	s.caller = d.join(t, "cal", "Cao Anh Lễ", "active", true, s.open, d.ws)
	s.colleague = d.join(t, "col", "Đỗ Thu Hà", "active", true, s.open, d.ws)
	s.hiddenOne = d.join(t, "hid", "Hồ Minh Nhân", "active", true, s.hidden, d.ws)
	s.owner = d.join(t, "own2", "Quản Trị Viên", "active", true, s.hidden, d.ws)
	s.loose = d.join(t, "lse", "Lưu Văn Rảnh", "active", true, "", d.ws)
	for tag, id := range map[string]string{"cal": s.caller, "col": s.colleague, "hid": s.hiddenOne, "own2": s.owner, "lse": s.loose} {
		_, err := d.pool.Exec(d.ctx, `UPDATE users SET email = $2 WHERE id = $1`, id, tag+fmt.Sprint(d.n)+"@example.vn")
		require.NoError(t, err)
	}
	s.ownerNode, s.hiddenNode = d.node("own2"), d.node("hid")
	return s
}

// limitedToOwnDepartment is what a caller limited to "Kinh doanh" is given: their
// department, the owners, and themselves.
func (s *scopeFixture) limitedToOwnDepartment() store.ContactScope {
	return store.ContactScope{
		CallerID: s.caller, DepartmentIDs: []string{s.open},
		OwnerNodeIDs: []string{s.ownerNode}, OwnersVisible: true,
	}
}

func (s *scopeFixture) list(t *testing.T, f store.ContactFilter) (ids []string, total int) {
	t.Helper()
	f.Limit = 50
	got, total, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, f)
	require.NoError(t, err)
	for _, u := range got {
		ids = append(ids, u.ID)
	}
	return ids, total
}

// A person in a department the caller may not see is not listed, not counted and not found
// by searching for their name or their e-mail; the people the caller may see still are.
func TestListContacts_AHiddenPersonIsNeverListedCountedOrSearched(t *testing.T) {
	s := newScopeFixture(t)
	scope := s.limitedToOwnDepartment()

	ids, total := s.list(t, store.ContactFilter{Scope: scope})
	assert.NotContains(t, ids, s.hiddenOne, "deny: a person of another department")
	assert.NotContains(t, ids, s.loose, "deny: no-department people were not made visible")
	assert.Equal(t, 3, total, "the count follows the same rule as the list")
	// allow, same fixture
	assert.ElementsMatch(t, []string{s.caller, s.colleague, s.owner}, ids, "own department, the owners and the caller")

	for _, q := range []string{"Hồ Minh Nhân", "hid" + fmt.Sprint(s.n) + "@example.vn", "Chuyên viên hid"} {
		got, n := s.list(t, store.ContactFilter{Scope: scope, Search: q})
		assert.Empty(t, got, "deny: searching %q must not find a hidden person", q)
		assert.Zero(t, n, "and must not count one")
	}
	got, n := s.list(t, store.ContactFilter{Scope: scope, Search: "Đỗ Thu Hà"})
	assert.Equal(t, []string{s.colleague}, got, "allow: search works inside the scope")
	assert.Equal(t, 1, n)
}

// Whether people with no department are visible, and whether the owners are, are separate
// switches; each is shown on the same fixture both ways.
func TestListContacts_NoDepartmentAndOwnersAreOwnSwitches(t *testing.T) {
	s := newScopeFixture(t)

	sc := s.limitedToOwnDepartment()
	sc.OwnersVisible = false
	ids, _ := s.list(t, store.ContactFilter{Scope: sc})
	assert.NotContains(t, ids, s.owner, "deny: an owner of a hidden department when owners are not visible")
	assert.Contains(t, ids, s.colleague)

	sc = s.limitedToOwnDepartment()
	ids, _ = s.list(t, store.ContactFilter{Scope: sc})
	assert.Contains(t, ids, s.owner, "allow: the same owner when owners are visible, wherever their department is")
	assert.NotContains(t, ids, s.hiddenOne, "and their department-mate is still hidden")

	sc.NoDepartment = true
	ids, total := s.list(t, store.ContactFilter{Scope: sc})
	assert.Contains(t, ids, s.loose, "allow: people with no department once they are visible")
	assert.Equal(t, 4, total)
}

// The caller always finds themselves, even in a department the scope leaves out.
func TestListContacts_TheCallerAlwaysSeesThemselves(t *testing.T) {
	s := newScopeFixture(t)
	sc := store.ContactScope{CallerID: s.hiddenOne}
	ids, total := s.list(t, store.ContactFilter{Scope: sc})
	assert.Equal(t, []string{s.hiddenOne}, ids, "allow: own row only; deny: nobody else")
	assert.Equal(t, 1, total)
}

// A filter nobody scoped shows nobody: the zero value must not read as "everyone".
func TestListContacts_AnUnscopedFilterShowsNobodyAndAllShowsEveryone(t *testing.T) {
	s := newScopeFixture(t)
	ids, total := s.list(t, store.ContactFilter{})
	assert.Empty(t, ids, "deny: the zero scope")
	assert.Zero(t, total)

	ids, total = s.list(t, store.ContactFilter{Scope: allPeople})
	assert.Len(t, ids, 5, "allow: the scope of someone who may see everyone")
	assert.Equal(t, 5, total)
}

// The scope never reaches a person of another workspace or one who is not active.
func TestListContacts_ScopeDoesNotWidenToOtherWorkspacesOrInactiveRows(t *testing.T) {
	s := newScopeFixture(t)
	elsewhere := s.join(t, "els", "Ở Nơi Khác", "active", true, "", s.other)
	gone := s.join(t, "gon", "Đã Rời", "removed", true, s.open, s.ws)

	ids, _ := s.list(t, store.ContactFilter{Scope: allPeople})
	assert.NotContains(t, ids, elsewhere, "deny: another workspace")
	assert.NotContains(t, ids, gone, "deny: a removed member")
	assert.Contains(t, ids, s.colleague, "allow: an active one of this workspace")
}

func TestListContacts_FiltersByDepartmentIDAndReturnsIt(t *testing.T) {
	s := newScopeFixture(t)
	got, total, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: allPeople, DepartmentID: s.hidden, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	for _, u := range got {
		assert.Equal(t, s.hidden, u.DepartmentID)
	}
	got, _, _, err = s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: allPeople, Limit: 50})
	require.NoError(t, err)
	byID := map[string]store.User{}
	for _, u := range got {
		byID[u.ID] = u
	}
	assert.Equal(t, s.open, byID[s.caller].DepartmentID)
	assert.Equal(t, "", byID[s.loose].DepartmentID, "no assignment, no department id")
}

// The resolver gives a name and a picture to any member of the workspace, whatever the
// status of the row; it never reaches outside the workspace.
func TestResolvePeople_AnyStatusOfThisWorkspaceOnly(t *testing.T) {
	s := newScopeFixture(t)
	for _, u := range []string{s.caller, s.hiddenOne} {
		_, err := s.pool.Exec(s.ctx, `UPDATE users SET avatar_url = '/avatars/'||id WHERE id = $1`, u)
		require.NoError(t, err)
	}
	removed := s.join(t, "rem", "Đã Rời Đi", "removed", true, s.open, s.ws)
	disabled := s.join(t, "dis", "Tạm Khoá", "disabled", true, "", s.ws)
	invited := s.join(t, "inv", "Mới Mời", "invited", true, "", s.ws)
	foreign := s.join(t, "frn", "Người Ngoài", "active", true, "", s.other)

	got, err := s.st.ResolvePeopleByWorkspace(s.ctx, s.ws, []string{s.hiddenOne, removed, disabled, invited, foreign, "no-such-id"}, nil)
	require.NoError(t, err)
	byID := map[string]store.ResolvedPerson{}
	for _, p := range got {
		byID[p.UserID] = p
	}
	assert.NotContains(t, byID, foreign, "deny: a user of another workspace is omitted")
	assert.NotContains(t, byID, "no-such-id", "deny: unknown ids are omitted silently")
	// allow, same call: every status of this workspace, hidden included
	for id, name := range map[string]string{s.hiddenOne: "Hồ Minh Nhân", removed: "Đã Rời Đi", disabled: "Tạm Khoá", invited: "Mới Mời"} {
		assert.Equal(t, name, byID[id].DisplayName)
	}
	assert.Equal(t, "/avatars/"+s.hiddenOne, byID[s.hiddenOne].AvatarURL)
	assert.Equal(t, s.node("hid"), byID[s.hiddenOne].NGACNodeID)
}

func TestResolvePeople_ByNodeAndByUserIDWithoutDuplicates(t *testing.T) {
	s := newScopeFixture(t)
	foreignNode := s.node("frn")
	s.join(t, "frn", "Người Ngoài", "active", true, "", s.other)

	got, err := s.st.ResolvePeopleByWorkspace(s.ctx, s.ws, []string{s.caller}, []string{s.node("cal"), s.node("col"), foreignNode})
	require.NoError(t, err)
	var ids []string
	for _, p := range got {
		ids = append(ids, p.UserID)
	}
	assert.ElementsMatch(t, []string{s.caller, s.colleague}, ids, "allow: by id and by node, once each; deny: the node of another workspace's user")
}

// A name is empty until the person has saved their profile: a handle is not a name.
func TestResolvePeople_NoHandleAsAName(t *testing.T) {
	s := newScopeFixture(t)
	handle := s.join(t, "hnd", "hoa.le.novapay", "active", false, "", s.ws)
	got, err := s.st.ResolvePeopleByWorkspace(s.ctx, s.ws, []string{handle, s.caller}, nil)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, p := range got {
		if p.UserID == handle {
			assert.Equal(t, "", p.DisplayName)
		} else {
			assert.Equal(t, "Cao Anh Lễ", p.DisplayName)
		}
	}
}

func TestResolvePeople_NothingAskedNothingReturned(t *testing.T) {
	s := newScopeFixture(t)
	got, err := s.st.ResolvePeopleByWorkspace(s.ctx, s.ws, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// A department outside the scope is not named anywhere in the answer, even on the rows the
// caller may see (an owner who sits in it, the caller themselves when it is not theirs to
// see): not as a name, not as an id, and not as something to filter or search by.
func TestListContacts_AnOutOfScopeDepartmentIsNotNamedOnAVisibleRow(t *testing.T) {
	s := newScopeFixture(t)
	scope := s.limitedToOwnDepartment()

	got, _, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: scope, Limit: 50})
	require.NoError(t, err)
	byID := map[string]store.User{}
	for _, u := range got {
		byID[u.ID] = u
	}
	require.Contains(t, byID, s.owner, "the owner is visible")
	assert.Equal(t, "", byID[s.owner].Department, "deny: the owner's hidden department name")
	assert.Equal(t, "", byID[s.owner].DepartmentID, "deny: the owner's hidden department id")
	assert.Equal(t, "Đối soát", byID[s.colleague].Department, "allow: a department in scope is named")
	assert.Equal(t, s.open, byID[s.colleague].DepartmentID)

	for name, f := range map[string]store.ContactFilter{
		"by id":             {DepartmentID: s.hidden},
		"by name":           {Department: "Nhân sự"},
		"by search of name": {Search: "Nhân sự"},
	} {
		f.Scope, f.Limit = scope, 50
		rows, total, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, f)
		require.NoError(t, err, name)
		assert.Empty(t, rows, "deny: %s must not confirm a hidden department", name)
		assert.Zero(t, total, name)
	}

	// allow: the same queries under a scope that sees everything find the owner
	for name, f := range map[string]store.ContactFilter{
		"by id":             {DepartmentID: s.hidden},
		"by name":           {Department: "Nhân sự"},
		"by search of name": {Search: "Nhân sự"},
	} {
		f.Scope, f.Limit = allPeople, 50
		rows, total, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, f)
		require.NoError(t, err, name)
		var ids []string
		for _, u := range rows {
			ids = append(ids, u.ID)
		}
		assert.Contains(t, ids, s.owner, name)
		assert.Equal(t, 2, total, name)
	}
}

// The caller's own row names their own department even when the scope leaves it out.
func TestListContacts_TheCallersOwnDepartmentIsNamedToThem(t *testing.T) {
	s := newScopeFixture(t)
	got, _, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: store.ContactScope{CallerID: s.hiddenOne}, Limit: 50})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Nhân sự", got[0].Department)
	assert.Equal(t, s.hidden, got[0].DepartmentID)
}

// Paging a limited scope visits exactly the scope, once each, with a stable total.
func TestListContacts_LimitedScopePagesWithinTheScope(t *testing.T) {
	s := newScopeFixture(t)
	for i := 0; i < 4; i++ {
		s.join(t, fmt.Sprintf("ex%d", i), fmt.Sprintf("Thêm %02d", i), "active", true, s.open, s.ws)
	}
	scope := s.limitedToOwnDepartment()
	var seen []string
	var after *store.ContactCursor
	for page := 0; page < 10; page++ {
		got, total, next, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: scope, Limit: 3, After: after})
		require.NoError(t, err)
		assert.Equal(t, 7, total, "the total is the scope's, on every page")
		for _, u := range got {
			seen = append(seen, u.ID)
		}
		if next == nil {
			break
		}
		after = next
	}
	assert.Len(t, seen, 7)
	assert.NotContains(t, seen, s.hiddenOne, "deny: no page reaches the hidden department")
	assert.NotContains(t, seen, s.loose)
	assert.Contains(t, seen, s.owner, "allow: the owner is on some page")
}

// A cursor is only a place to continue from: a forged one can skip rows but never add one
// the scope excludes.
func TestListContacts_AForgedCursorCannotWidenTheScope(t *testing.T) {
	s := newScopeFixture(t)
	scope := s.limitedToOwnDepartment()
	forged := []*store.ContactCursor{
		{Key: "", ID: ""},
		{Key: "a", ID: s.hiddenOne},
		{Key: "hồ minh nhân", ID: "x"},
		{Key: "' OR true --", ID: "x"},
	}
	for _, c := range forged {
		got, total, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: scope, Limit: 50, After: c})
		require.NoError(t, err)
		assert.Equal(t, 3, total, "the count ignores where the page starts")
		for _, u := range got {
			assert.Contains(t, []string{s.caller, s.colleague, s.owner}, u.ID, "cursor %+v added %s", c, u.ID)
		}
	}
	// allow: an honest first page of the same fixture
	got, _, _, err := s.st.ListContactsByWorkspace(s.ctx, s.ws, store.ContactFilter{Scope: scope, Limit: 50, After: forged[0]})
	require.NoError(t, err)
	assert.Len(t, got, 3)
}
