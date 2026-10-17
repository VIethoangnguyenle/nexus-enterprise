//go:build realpolicy

package domain_test

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// What a member may see of the workspace's people (docs/specs/contacts-visibility),
// decided by the real policy service: the scope is built from the membership check and
// PDP decisions alone, every member-readable read of people or departments follows it,
// and the administrator's rules take effect for the people concerned as soon as they are
// written. Each DENY is asserted on the same fixture as its ALLOW.

// scopeWorld is an organization workspace with this shape:
//
//	A ── A1 ── A11          H (hidden)        no department
//	  └─ A2
//
// and one person (or more) in each, an owner placed in H, a role that holds
// manage on Mgmt, and a custom role that tests limit.
type scopeWorld struct {
	*dirWorld
	a, a1, a11, a2, h *domain.DepartmentResult

	pA, pA1, pA11, pA2, pH, pNone wsPerson
	ownerInH                      wsPerson
	adminRole                     *domain.Role
	manager                       wsPerson // holds manage on Mgmt through a role, not an owner
	role                          *domain.Role
	holder                        wsPerson // member of A1 who holds `role`
	holderNoDept                  wsPerson // holds `role`, no department
	stranger                      wsPerson // member of another workspace
	ghost                         wsPerson // only under the peers UA of A1
	other                         *dirWorld
}

func newScopeWorld(t *testing.T) *scopeWorld {
	t.Helper()
	w := &scopeWorld{dirWorld: newDirWorld(t, domain.TypeOrganization)}
	w.a = w.dept("A", nil)
	w.a1 = w.dept("A1", w.a)
	w.a11 = w.dept("A11", w.a1)
	w.a2 = w.dept("A2", w.a)
	w.h = w.dept("H", nil)

	for p, d := range map[*wsPerson]*domain.DepartmentResult{
		&w.pA: w.a, &w.pA1: w.a1, &w.pA11: w.a11, &w.pA2: w.a2, &w.pH: w.h,
	} {
		*p = w.member()
		w.place(*p, d)
	}
	w.pNone = w.member()
	w.ownerInH = w.member(w.ws.OwnersUaID)
	w.place(w.ownerInH, w.h)

	var err error
	w.adminRole, err = w.svc.CreateRole(w.as(w.owner), w.owner.node, w.ws.ID, "Quản lý danh bạ")
	require.NoError(t, err)
	testutil.Associate(t, w.write, w.adminRole.ID, w.ws.MgmtOaID, []string{ngac.OpManage})
	w.manager = w.member(w.adminRole.ID)

	w.role, err = w.svc.CreateRole(w.as(w.owner), w.owner.node, w.ws.ID, "Cộng tác viên")
	require.NoError(t, err)
	w.holder = w.member(w.role.ID)
	w.place(w.holder, w.a1)
	w.holderNoDept = w.member(w.role.ID)

	w.other = newDirWorld(t, domain.TypeOrganization)
	w.stranger = w.other.member()
	w.ghost = w.newPerson(w.peers(w.a1))
	return w
}

func (w *scopeWorld) scope(p wsPerson) (*domain.DirectoryScope, error) {
	return w.svc.DirectoryScope(w.as(p), p.node, w.ws.ID)
}

func (w *scopeWorld) mustScope(p wsPerson) *domain.DirectoryScope {
	w.t.Helper()
	s, err := w.scope(p)
	require.NoError(w.t, err)
	return s
}

func (w *scopeWorld) departments(p wsPerson) []*domain.DepartmentResult {
	w.t.Helper()
	d, err := w.svc.ListDepartments(w.as(p), p.node, w.ws.ID)
	require.NoError(w.t, err)
	return d
}

func (w *scopeWorld) deptIDs(p wsPerson) []string {
	var out []string
	for _, d := range w.departments(p) {
		out = append(out, d.ID)
	}
	return out
}

func (w *scopeWorld) members(p wsPerson) []string {
	w.t.Helper()
	m, err := w.svc.ListMembers(w.as(p), p.node, w.ws.ID)
	require.NoError(w.t, err)
	return nodesOf(m)
}

func (w *scopeWorld) roleCounts(p wsPerson) map[domain.RoleKind]int {
	w.t.Helper()
	list, err := w.svc.ListRoleSummaries(w.as(p), p.node, w.ws.ID)
	require.NoError(w.t, err)
	out := map[domain.RoleKind]int{}
	for _, r := range list {
		out[r.Kind] += r.MemberCount
	}
	return out
}

// limitRaw limits a role or a department the way the rule does, written straight to the graph.
func (w *scopeWorld) limitRaw(uaID string) { testutil.Assign(w.t, w.write, uaID, w.limited()) }

func keys(m map[string]bool) []string {
	var out []string
	for k, v := range m {
		if v {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}

func sorted(ids ...string) []string { slices.Sort(ids); return ids }

// Someone who is not a member gets nothing from any of the reads, and nobody but a
// directory administrator writes a rule. The same calls succeed for the people who may.
func TestRealPolicy_Scope_NonMembersAndPlainMembersAreDenied(t *testing.T) {
	w := newScopeWorld(t)
	ws := w.ws.ID

	for name, p := range map[string]wsPerson{"another workspace's member": w.stranger, "only under a peers UA": w.ghost} {
		_, err := w.scope(p)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": scope")
		_, err = w.svc.ListDepartments(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": departments")
		_, err = w.svc.ListMembers(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": members")
		_, err = w.svc.ListRoleSummaries(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": role counts")
	}
	_, err := w.scope(w.pNone)
	assert.NoError(t, err, "a member with no department is let in")

	for name, p := range map[string]wsPerson{"stranger": w.stranger, "ghost": w.ghost, "plain member": w.pA2, "limited member": w.holder, "member with no rights": w.pNone} {
		_, err := w.svc.DirectoryRules(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": read the rules")
		assert.ErrorIs(t, w.svc.HideDepartment(w.as(p), p.node, ws, w.a1.ID), domain.ErrAccessDenied, name+": hide")
		assert.ErrorIs(t, w.svc.UnhideDepartment(w.as(p), p.node, ws, w.a1.ID), domain.ErrAccessDenied, name+": unhide")
		assert.ErrorIs(t, w.svc.LimitRole(w.as(p), p.node, ws, w.role.ID), domain.ErrAccessDenied, name+": limit a role")
		assert.ErrorIs(t, w.svc.UnlimitRole(w.as(p), p.node, ws, w.role.ID), domain.ErrAccessDenied, name+": unlimit a role")
		assert.ErrorIs(t, w.svc.LimitDepartment(w.as(p), p.node, ws, w.a2.ID), domain.ErrAccessDenied, name+": limit a department")
		assert.ErrorIs(t, w.svc.UnlimitDepartment(w.as(p), p.node, ws, w.a2.ID), domain.ErrAccessDenied, name+": unlimit a department")
	}
	assert.Equal(t, []string{w.dirName(w.a)}, w.parents(w.dir(w.a1)), "nothing was hidden by the refused calls")
	assert.False(t, w.under(w.role.ID, w.limited()), "nothing was limited by the refused calls")

	// ALLOW on the same fixture.
	for name, p := range map[string]wsPerson{"owner": w.owner, "manage role": w.manager} {
		_, err := w.svc.DirectoryRules(w.as(p), p.node, ws)
		assert.NoError(t, err, name+": read the rules")
	}
	require.NoError(t, w.svc.HideDepartment(w.as(w.manager), w.manager.node, ws, w.a1.ID), "a manage holder hides")
	require.NoError(t, w.svc.UnhideDepartment(w.as(w.manager), w.manager.node, ws, w.a1.ID))
}

// A hidden department is missing from every read of a member who is not in it, and
// present for its own people and for administrators (who see it labelled). An owner
// placed in it stays visible to everyone.
func TestRealPolicy_Scope_HiddenDepartment(t *testing.T) {
	w := newScopeWorld(t)
	w.hide(w.h)

	s := w.mustScope(w.pA1)
	assert.True(t, s.Ready)
	assert.False(t, s.Admin)
	assert.False(t, s.Limited)
	assert.Equal(t, sorted(w.a.ID, w.a1.ID, w.a11.ID, w.a2.ID), keys(s.VisibleDeptIDs), "everything but H")
	assert.True(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible)
	assert.ElementsMatch(t, []string{w.owner.node, w.ownerInH.node}, s.OwnerNodeIDs, "the owner placed in the hidden department is listed")
	assert.Empty(t, s.HiddenDeptIDs, "only administrators learn which departments are hidden")

	assert.NotContains(t, w.deptIDs(w.pA1), w.h.ID)
	for _, d := range w.departments(w.pA1) {
		assert.False(t, d.Hidden, "a member is never told which departments are hidden")
	}
	got := w.members(w.pA1)
	assert.NotContains(t, got, w.pH.node, "the hidden department's person is not listed")
	assert.Contains(t, got, w.ownerInH.node, "an owner is always listed, wherever they sit")
	for _, p := range []wsPerson{w.pA, w.pA2, w.pNone, w.owner, w.pA1} {
		assert.Contains(t, got, p.node)
	}
	counts := w.roleCounts(w.pA1)
	assert.Equal(t, len(got), counts[domain.RoleMembers], "the Members count is of the people the caller may see")
	assert.Equal(t, 2, counts[domain.RoleOwners], "both owners are counted")

	// ALLOW on the same fixture: the hidden department's own people, and an administrator.
	s = w.mustScope(w.pH)
	assert.Contains(t, keys(s.VisibleDeptIDs), w.h.ID)
	assert.Contains(t, w.deptIDs(w.pH), w.h.ID)
	assert.Contains(t, w.members(w.pH), w.pH.node)

	for name, p := range map[string]wsPerson{"owner": w.owner, "manage role": w.manager} {
		s = w.mustScope(p)
		assert.True(t, s.Admin, name)
		assert.False(t, s.Limited, name)
		assert.Equal(t, sorted(w.a.ID, w.a1.ID, w.a11.ID, w.a2.ID, w.h.ID), keys(s.VisibleDeptIDs), name+" sees every department")
		assert.Equal(t, []string{w.h.ID}, keys(s.HiddenDeptIDs), name+" is told H is hidden")
		var hidden []string
		for _, d := range w.departments(p) {
			if d.Hidden {
				hidden = append(hidden, d.ID)
			}
		}
		assert.Equal(t, []string{w.h.ID}, hidden, name+": the list labels H")
		assert.Contains(t, w.members(p), w.pH.node, name+" sees the hidden person")
		assert.Equal(t, len(w.members(p)), w.roleCounts(p)[domain.RoleMembers])
	}
}

// A limited person sees their own department's subtree, the owners and themself; people
// of any other department, the root above them and the people with no department stay out.
func TestRealPolicy_Scope_LimitedRole(t *testing.T) {
	w := newScopeWorld(t)
	w.limitRaw(w.role.ID)

	s := w.mustScope(w.holder)
	assert.True(t, s.Limited)
	assert.False(t, s.Admin)
	assert.Equal(t, sorted(w.a1.ID, w.a11.ID), keys(s.VisibleDeptIDs), "A1 and the department beneath it; neither the parent nor a sibling")
	assert.False(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible, "a limited person always sees the owners")
	assert.ElementsMatch(t, []string{w.owner.node, w.ownerInH.node}, s.OwnerNodeIDs)

	got := w.members(w.holder)
	for _, p := range []wsPerson{w.holder, w.pA1, w.pA11, w.owner, w.ownerInH} {
		assert.Contains(t, got, p.node)
	}
	for name, p := range map[string]wsPerson{"A2": w.pA2, "A": w.pA, "H": w.pH, "no department": w.pNone, "manager": w.manager} {
		assert.NotContains(t, got, p.node, name+" is not shown to a limited person")
	}
	depts := w.departments(w.holder)
	require.Len(t, depts, 2)
	for _, d := range depts {
		if d.ID == w.a1.ID {
			assert.Empty(t, d.ParentID, "the parent is not visible, so A1 has none in this person's tree")
		}
	}
	assert.Equal(t, len(got), w.roleCounts(w.holder)[domain.RoleMembers])

	// A limited person with no department sees the owners and themself.
	s = w.mustScope(w.holderNoDept)
	assert.True(t, s.Limited)
	assert.Empty(t, s.VisibleDeptIDs)
	assert.False(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible)
	assert.ElementsMatch(t, []string{w.holderNoDept.node, w.owner.node, w.ownerInH.node}, w.members(w.holderNoDept))

	// ALLOW on the same fixture: people outside the role, and an owner who holds it, see more.
	assert.Contains(t, w.members(w.pA1), w.pA2.node, "a member who is not limited sees the other departments")
	assert.False(t, w.mustScope(w.pA1).Limited)
	w.limitRaw(w.adminRole.ID)
	s = w.mustScope(w.manager)
	assert.True(t, s.Admin, "the limit is on read, the administrator's shortcut is manage")
	assert.Len(t, s.VisibleDeptIDs, 5)
	assert.Contains(t, w.members(w.manager), w.pH.node)
}

// A limited department limits its people and the people of the departments beneath it,
// and nobody else.
func TestRealPolicy_Scope_LimitedDepartment(t *testing.T) {
	w := newScopeWorld(t)
	w.limitRaw(w.a2.NGACUaID)

	s := w.mustScope(w.pA2)
	assert.True(t, s.Limited)
	assert.Equal(t, []string{w.a2.ID}, keys(s.VisibleDeptIDs))
	assert.NotContains(t, w.members(w.pA2), w.pA1.node)
	assert.Contains(t, w.members(w.pA2), w.owner.node)

	s = w.mustScope(w.pA1)
	assert.False(t, s.Limited, "a sibling is not limited by it")
	assert.Contains(t, w.members(w.pA1), w.pA2.node)
}

// The limit is the prohibition: without it a limited person would see everyone, so the
// scope refuses to answer rather than guess.
func TestRealPolicy_Scope_LimitedWithoutItsRuleIsUnavailable(t *testing.T) {
	w := newScopeWorld(t)
	w.limitRaw(w.role.ID)
	_, err := w.scope(w.holder)
	require.NoError(t, err)

	_, err = w.write.RemoveProhibition(w.admin(), &policypb.RemoveProhibitionRequest{Name: ngac.DirectoryLimitedProhibitionName(w.id)})
	require.NoError(t, err)

	_, err = w.scope(w.holder)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "limited and no prohibition: 503, never the full list")
	_, err = w.svc.ListMembers(w.as(w.holder), w.holder.node, w.ws.ID)
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	_, err = w.scope(w.pA2)
	assert.NoError(t, err, "a person who is not limited is not affected")
}

// A workspace that has no directory (it predates migration 064) has no rules: everyone sees
// everyone, the scope says it is not ready, and the rule writes wait.
func TestRealPolicy_Scope_WorkspaceWithoutDirectory(t *testing.T) {
	w := newWSWorld(t)
	ws := w.g.WorkspaceID

	_, err := w.svc.DirectoryScope(w.as(w.outsider), w.outsider.node, ws)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "membership is still checked first")

	s, err := w.svc.DirectoryScope(w.as(w.member), w.member.node, ws)
	require.NoError(t, err)
	assert.False(t, s.Ready)
	assert.False(t, s.Admin)
	assert.False(t, s.Limited)
	assert.True(t, s.NoDeptVisible)
	assert.True(t, s.OwnersVisible)

	d, err := w.svc.CreateDepartment(w.as(w.owner), w.owner.node, domain.CreateDepartmentInput{WorkspaceID: ws, Name: "Old"})
	require.NoError(t, err)
	s, err = w.svc.DirectoryScope(w.as(w.member), w.member.node, ws)
	require.NoError(t, err)
	assert.Equal(t, []string{d.ID}, keys(s.VisibleDeptIDs), "every department is visible")
	got, err := w.svc.ListMembers(w.as(w.member), w.member.node, ws)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{w.owner.node, w.member.node, w.tenantMember.node}, nodesOf(got))

	rules, err := w.svc.DirectoryRules(w.as(w.owner), w.owner.node, ws)
	require.NoError(t, err, "reading the rules is not an outage")
	assert.False(t, rules.Ready)
	assert.ErrorIs(t, w.svc.HideDepartment(w.as(w.owner), w.owner.node, ws, d.ID), domain.ErrUnavailable)
	assert.ErrorIs(t, w.svc.LimitDepartment(w.as(w.owner), w.owner.node, ws, d.ID), domain.ErrUnavailable)
	_, err = w.svc.DirectoryRules(w.as(w.member), w.member.node, ws)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "and a plain member still cannot read them")
}

// Hiding moves the department's Dir and DirPeer with their subtrees and gives its own UA
// the read; unhiding puts both back under the parent they came from, also when the parent
// is itself hidden. Each write is felt by the people concerned at once.
func TestRealPolicy_Rules_HideAndUnhideRestoreTheExactParents(t *testing.T) {
	w := newScopeWorld(t)
	ws, owner := w.ws.ID, w.owner

	// Warm the decisions: pA2 sees A1 and A11.
	require.Contains(t, keys(w.mustScope(w.pA2).VisibleDeptIDs), w.a1.ID)
	require.True(t, w.can(w.pA2, w.dir(w.a11), ngac.OpRead))

	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.a1.ID))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.dir(w.a1)))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(w.a1)))
	assert.Equal(t, []string{w.dirName(w.a1)}, w.parents(w.dir(w.a11)), "the sub-department follows")
	s := w.mustScope(w.pA2)
	assert.NotContains(t, keys(s.VisibleDeptIDs), w.a1.ID, "the write is felt at once (the cached ALLOW is dropped)")
	assert.NotContains(t, keys(s.VisibleDeptIDs), w.a11.ID)
	assert.NotContains(t, w.deptIDs(w.pA2), w.a1.ID)
	assert.NotContains(t, w.members(w.pA2), w.pA1.node)
	assert.False(t, w.can(w.pA, w.dir(w.a1), ngac.OpRead), "A's own people do not read the hidden child")
	assert.False(t, w.can(w.pA, w.peer(w.a1), ngac.OpRead), "nor its peer OA")
	// ALLOW on the same fixture: A1's people and the people beneath.
	assert.Contains(t, w.deptIDs(w.pA1), w.a1.ID)
	assert.Contains(t, w.deptIDs(w.pA11), w.a11.ID, "a person of the sub-department reads it through the department above")
	assert.Contains(t, w.members(w.pA1), w.pA1.node)
	assert.True(t, w.can(w.pA1, w.dir(w.a1), ngac.OpRead), "the department's own UA reads its hidden Dir")
	assert.True(t, w.can(w.pA11, w.dir(w.a11), ngac.OpRead), "and so do the people beneath, through the department above")
	assert.False(t, w.can(w.pA2, w.dir(w.a1), ngac.OpRead))

	// Nested: hide the child of a hidden department, then unhide in the other order.
	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.a11.ID))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.dir(w.a11)))
	require.NoError(t, w.svc.UnhideDepartment(w.as(owner), owner.node, ws, w.a11.ID))
	assert.Equal(t, []string{w.dirName(w.a1)}, w.parents(w.dir(w.a11)), "back under the hidden parent, not under Open")
	assert.Equal(t, []string{w.peerName(w.a1)}, w.parents(w.peer(w.a11)))
	assert.NotContains(t, w.deptIDs(w.pA2), w.a11.ID, "still hidden by its parent")

	require.NoError(t, w.svc.UnhideDepartment(w.as(owner), owner.node, ws, w.a1.ID))
	assert.Equal(t, []string{w.dirName(w.a)}, w.parents(w.dir(w.a1)), "the exact parent it had")
	assert.Equal(t, []string{w.peerName(w.a)}, w.parents(w.peer(w.a1)))
	assert.Equal(t, []string{w.dirName(w.a1)}, w.parents(w.dir(w.a11)))
	assocs, err := w.write.GetAssociations(w.admin(), &policypb.GetAssociationsRequest{UaId: w.a1.NGACUaID})
	require.NoError(t, err)
	for _, a := range assocs.GetAssociations() {
		assert.NotEqual(t, w.dir(w.a1), a.GetOaId(), "the department's own read of its Dir is gone with the hiding")
	}
	assert.Contains(t, keys(w.mustScope(w.pA2).VisibleDeptIDs), w.a1.ID, "visible again at once")
	assert.Contains(t, w.deptIDs(w.pA2), w.a11.ID)

	// A top-level department goes back under Open and Directory.
	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.a.ID))
	require.NoError(t, w.svc.UnhideDepartment(w.as(owner), owner.node, ws, w.a.ID))
	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(w.a)))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(w.a)))

	// Idempotent both ways.
	require.NoError(t, w.svc.UnhideDepartment(w.as(owner), owner.node, ws, w.a.ID))
	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.a2.ID))
	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.a2.ID))
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.dir(w.a2)))
}

// Limiting a role or a department is one assignment under DirectoryLimited; undoing it is
// the removal. The person's scope changes at once, and the rules listing names the subjects.
func TestRealPolicy_Rules_LimitAndUnlimit(t *testing.T) {
	w := newScopeWorld(t)
	ws, owner := w.ws.ID, w.owner

	before := w.mustScope(w.holder)
	require.False(t, before.Limited)
	require.Contains(t, keys(before.VisibleDeptIDs), w.a2.ID)

	require.NoError(t, w.svc.LimitRole(w.as(owner), owner.node, ws, w.role.ID))
	require.NoError(t, w.svc.LimitRole(w.as(owner), owner.node, ws, w.role.ID), "idempotent")
	s := w.mustScope(w.holder)
	assert.True(t, s.Limited, "limited as soon as the write returns")
	assert.Equal(t, sorted(w.a1.ID, w.a11.ID), keys(s.VisibleDeptIDs))
	assert.NotContains(t, w.members(w.holder), w.pA2.node)
	assert.False(t, w.mustScope(w.pA2).Limited, "someone outside the role is untouched")

	require.NoError(t, w.svc.LimitDepartment(w.as(owner), owner.node, ws, w.a2.ID))
	assert.True(t, w.mustScope(w.pA2).Limited)

	rules, err := w.svc.DirectoryRules(w.as(owner), owner.node, ws)
	require.NoError(t, err)
	assert.True(t, rules.Ready)
	require.Len(t, rules.LimitedRoles, 1)
	assert.Equal(t, domain.RuleSubject{ID: w.role.ID, Name: "Cộng tác viên", MemberCount: 2}, rules.LimitedRoles[0], "name, and the people who hold it")
	require.Len(t, rules.LimitedDepartments, 1)
	assert.Equal(t, domain.RuleSubject{ID: w.a2.ID, Name: "A2", ParentID: w.a.ID, MemberCount: 1}, rules.LimitedDepartments[0])
	assert.Empty(t, rules.HiddenDepartments)

	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.h.ID))
	rules, err = w.svc.DirectoryRules(w.as(owner), owner.node, ws)
	require.NoError(t, err)
	require.Len(t, rules.HiddenDepartments, 1)
	assert.Equal(t, domain.RuleSubject{ID: w.h.ID, Name: "H", MemberCount: 2}, rules.HiddenDepartments[0], "the people who would be seen only by the department and administrators")

	require.NoError(t, w.svc.UnlimitRole(w.as(owner), owner.node, ws, w.role.ID))
	require.NoError(t, w.svc.UnlimitRole(w.as(owner), owner.node, ws, w.role.ID), "idempotent")
	assert.False(t, w.mustScope(w.holder).Limited)
	assert.Contains(t, w.members(w.holder), w.pA2.node, "freed at once")
	require.NoError(t, w.svc.UnlimitDepartment(w.as(owner), owner.node, ws, w.a2.ID))
	assert.False(t, w.mustScope(w.pA2).Limited)
	rules, err = w.svc.DirectoryRules(w.as(owner), owner.node, ws)
	require.NoError(t, err)
	assert.Empty(t, rules.LimitedRoles)
	assert.Empty(t, rules.LimitedDepartments)
}

// A rule names only things of this workspace and only the right kind of thing: another
// workspace's department or role, the built-in roles, a department given as a role and an id
// that does not exist are all "not found", and the other workspace's graph is not touched.
func TestRealPolicy_Rules_RefuseForeignAndWrongSubjects(t *testing.T) {
	w := newScopeWorld(t)
	ws, owner := w.ws.ID, w.owner
	foreignDept := w.other.dept("Elsewhere", nil)
	foreignRole, err := w.other.svc.CreateRole(w.other.as(w.other.owner), w.other.owner.node, w.other.ws.ID, "Elsewhere role")
	require.NoError(t, err)
	parentsBefore := w.other.parents(w.other.dir(foreignDept))

	for name, id := range map[string]string{"another workspace's department": foreignDept.ID, "unknown": "does-not-exist", "empty": ""} {
		assert.Error(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, id), name)
		assert.ErrorIs(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, id), domain.ErrNotFound, name+": hide")
		assert.ErrorIs(t, w.svc.UnhideDepartment(w.as(owner), owner.node, ws, id), domain.ErrNotFound, name+": unhide")
		assert.ErrorIs(t, w.svc.LimitDepartment(w.as(owner), owner.node, ws, id), domain.ErrNotFound, name+": limit")
		assert.ErrorIs(t, w.svc.UnlimitDepartment(w.as(owner), owner.node, ws, id), domain.ErrNotFound, name+": unlimit")
	}
	for name, id := range map[string]string{
		"another workspace's role": foreignRole.ID, "the Owners UA": w.ws.OwnersUaID, "the Members UA": w.ws.MembersUaID,
		"a department's UA": w.a1.NGACUaID, "the Limited UA itself": w.limited(), "unknown": "does-not-exist", "empty": "",
	} {
		assert.ErrorIs(t, w.svc.LimitRole(w.as(owner), owner.node, ws, id), domain.ErrNotFound, name+": limit")
		assert.ErrorIs(t, w.svc.UnlimitRole(w.as(owner), owner.node, ws, id), domain.ErrNotFound, name+": unlimit")
	}
	assert.Equal(t, parentsBefore, w.other.parents(w.other.dir(foreignDept)), "the other workspace's directory is untouched")
	assert.False(t, w.other.under(foreignRole.ID, w.other.limited()))
	assert.False(t, w.under(w.ws.OwnersUaID, w.limited()))
	assert.False(t, w.under(w.ws.MembersUaID, w.limited()))

	// The other workspace's administrator holds manage there, and nothing here.
	assert.ErrorIs(t, w.svc.HideDepartment(w.as(w.other.owner), w.other.owner.node, ws, w.a1.ID), domain.ErrAccessDenied)
	assert.ErrorIs(t, w.svc.LimitRole(w.as(w.other.owner), w.other.owner.node, ws, w.role.ID), domain.ErrAccessDenied)

	// ALLOW on the same fixture: the right kinds in this workspace.
	require.NoError(t, w.svc.HideDepartment(w.as(owner), owner.node, ws, w.a1.ID))
	require.NoError(t, w.svc.LimitRole(w.as(owner), owner.node, ws, w.role.ID))
}

// Rules belong to an organization: a personal workspace has no Danh bạ to shape.
func TestRealPolicy_Rules_PersonalWorkspaceHasNone(t *testing.T) {
	w := newDirWorld(t, domain.TypePersonal)
	d := w.dept("Home", nil)
	rules, err := w.svc.DirectoryRules(w.as(w.owner), w.owner.node, w.ws.ID)
	require.NoError(t, err)
	assert.False(t, rules.Ready, "the tab is hidden for a personal workspace")
	assert.ErrorIs(t, w.svc.HideDepartment(w.as(w.owner), w.owner.node, w.ws.ID, d.ID), domain.ErrInvalidInput)
	assert.ErrorIs(t, w.svc.LimitDepartment(w.as(w.owner), w.owner.node, w.ws.ID, d.ID), domain.ErrInvalidInput)
	assert.Equal(t, []string{ngac.DirectoryOpenOAName(w.id)}, w.parents(w.dir(d)), "and nothing moved")
}

// onFirstRemove runs a hook on the first RemoveAssignment through the client, where a move's
// first write lands, and lets the hook hold the move there.
type onFirstRemove struct {
	policypb.PolicyWriteServiceClient
	hook func()
}

func (o *onFirstRemove) RemoveAssignment(ctx context.Context, in *policypb.RemoveAssignmentRequest, opts ...grpc.CallOption) (*policypb.Empty, error) {
	if o.hook != nil {
		h := o.hook
		o.hook = nil
		h()
	}
	return o.PolicyWriteServiceClient.RemoveAssignment(ctx, in, opts...)
}

// A department that is hidden while it is being moved stays hidden: directory writes of one
// workspace take turns, so the move does not work from a picture taken before the hide.
func TestRealPolicy_Rules_AMoveRacingAHideLeavesTheDepartmentHidden(t *testing.T) {
	w := newScopeWorld(t)
	racing := &onFirstRemove{PolicyWriteServiceClient: w.write}
	svc := domain.NewService(store.New(w.pool), store.New(w.pool), w.read, racing, nil, nil)
	ctx := w.as(w.owner)

	var fired atomic.Bool
	hidden := make(chan error, 1)
	racing.hook = func() {
		fired.Store(true)
		go func() { hidden <- svc.HideDepartment(ctx, w.owner.node, w.ws.ID, w.a2.ID) }()
		time.Sleep(600 * time.Millisecond) // long enough for an unserialised hide to finish first
	}
	_, err := svc.MoveDepartment(ctx, w.owner.node, domain.MoveDepartmentInput{WorkspaceID: w.ws.ID, DeptID: w.a2.ID})
	require.NoError(t, err)
	require.True(t, fired.Load(), "the move wrote, so the race was staged")
	require.NoError(t, <-hidden)

	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.dir(w.a2)), "hidden after the move, not put back by it")
	assert.Equal(t, []string{ngac.DirectoryOAName(w.id)}, w.parents(w.peer(w.a2)))
	assert.False(t, w.can(w.pA, w.dir(w.a2), ngac.OpRead), "a member outside it does not read it")
	assert.True(t, w.can(w.pA2, w.dir(w.a2), ngac.OpRead), "its own people do")
}
