//go:build realpolicy

package testutil_test

import (
	"context"
	"testing"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

// The contacts-visibility model (docs/specs/contacts-visibility) on the real
// policy service: its one rule is a prohibition, and no service creates a
// prohibition anywhere else. This proves the service persists it, applies it on
// the single and the batch RPC, and lets go of it when it is removed, with the
// nodes and edges built through the write API exactly as the workspace service
// will build them.
//
//	Mgmt ── Directory ─┬─ DirectoryOpen ◄─ Members {read} ── Dir_a, Dir_b
//	                   ├─ DirectoryOwners ◄─ Members {read}
//	                   └─ DirPeer_a, DirPeer_b ◄─ DeptPeers_a/b {read}
//	DirectoryLimited (under the policy class) ◄ Role "limited"
//	prohibition: DirectoryLimited, read, [DirectoryOpen]
func TestContactsVisibilityProhibitionOnTheRealPolicy(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	ws := testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)
	id := ngac.WorkspaceID(ws.WorkspaceID)

	node := func(name, nodeType string, parents ...string) string {
		return testutil.NewNode(t, write, name, nodeType, parents...)
	}
	directory := node(ngac.DirectoryOAName(id), ngac.TypeOA, ws.Mgmt)
	open := node(ngac.DirectoryOpenOAName(id), ngac.TypeOA, directory)
	owners := node(ngac.DirectoryOwnersOAName(id), ngac.TypeOA, directory)
	limited := node(ngac.DirectoryLimitedUAName(id), ngac.TypeUA, ws.PC)
	testutil.Associate(t, write, ws.Members, open, []string{ngac.OpRead})
	testutil.Associate(t, write, ws.Members, owners, []string{ngac.OpRead})

	type dept struct{ dir, peer, peers string }
	mkDept := func(label string) dept {
		d := ngac.DeptID(ws.WorkspaceID + "-" + label)
		x := dept{
			dir:   node(ngac.DeptDirOAName(d), ngac.TypeOA, open),
			peer:  node(ngac.DeptPeerOAName(d), ngac.TypeOA, directory),
			peers: node(ngac.DeptPeersUAName(d), ngac.TypeUA, ws.PC),
		}
		testutil.Associate(t, write, x.peers, x.peer, []string{ngac.OpRead})
		return x
	}
	a, b := mkDept("a"), mkDept("b")

	limitedRole := node("Role_rp_limited_"+ws.WorkspaceID, ngac.TypeUA, ws.PC, limited)
	member := testutil.NewUser(t, write, ws.Members, a.peers)
	lim := testutil.NewUser(t, write, ws.Members, a.peers, limitedRole)
	owner := testutil.NewUser(t, write, ws.Owners, a.peers)
	foreign := testutil.NewUser(t, write, other.Members)

	// The workspace's rule. Registered for removal after the nodes, so it goes first.
	ruleName := ngac.DirectoryLimitedProhibitionName(id)
	createRule := func() {
		t.Helper()
		_, err := write.CreateProhibition(testutil.PolicyAdmin(context.Background()), &policypb.CreateProhibitionRequest{
			Name: ruleName, SubjectId: limited, Operations: []string{ngac.OpRead}, TargetOaIds: []string{open},
		})
		if err != nil {
			t.Fatalf("create the directory rule: %v", err)
		}
	}
	removeRule := func() {
		t.Helper()
		if _, err := write.RemoveProhibition(testutil.PolicyAdmin(context.Background()), &policypb.RemoveProhibitionRequest{Name: ruleName}); err != nil {
			t.Fatalf("remove the directory rule: %v", err)
		}
	}
	t.Cleanup(func() {
		// Prohibitions have no foreign key to the graph: a failed test must not leave one behind.
		_, _ = write.RemoveProhibition(testutil.PolicyAdmin(context.Background()), &policypb.RemoveProhibitionRequest{Name: ruleName})
	})

	batch := func(user string, oas ...string) map[string]bool {
		t.Helper()
		resp, err := read.BatchCheckAccess(testutil.PolicyCaller(context.Background(), user), &policypb.BatchCheckAccessRequest{
			UserNodeId: user, ObjectIds: oas, Operations: []string{ngac.OpRead},
		})
		if err != nil {
			t.Fatalf("batch check for %s: %v", user, err)
		}
		out := map[string]bool{}
		for _, oa := range oas {
			out[oa] = resp.GetResults()[oa].GetPermissions()[ngac.OpRead]
		}
		return out
	}
	// expect asserts a read decision on both RPCs.
	expect := func(why, user, oa string, want bool) {
		t.Helper()
		if got := testutil.Allowed(t, read, user, oa, ngac.OpRead); got != want {
			t.Errorf("%s: CheckAccess = %v, want %v", why, got, want)
		}
		if got := batch(user, oa)[oa]; got != want {
			t.Errorf("%s: BatchCheckAccess = %v, want %v", why, got, want)
		}
	}

	// Before the rule exists, a limited role changes nothing: it carries no association.
	expect("no rule yet: the limited person reads another department", lim, b.dir, true)

	createRule()

	// Deny: the rule overrides what Members grants, for the limited role only.
	expect("limited reads another department", lim, b.dir, false)
	expect("limited reads their own department through Open", lim, a.dir, false)
	expect("someone in another workspace reads this one", foreign, a.dir, false)
	expect("a member reads a peer OA they are not under", member, b.peer, false)
	// Allow, same fixture: everyone else, the limited person's own tree and the owners.
	expect("a member reads another department", member, b.dir, true)
	expect("a member reads the owners", member, owners, true)
	expect("limited reads their own peers", lim, a.peer, true)
	expect("limited still reads the owners", lim, owners, true)
	expect("the owner reads the open tree", owner, b.dir, true)
	if !testutil.Allowed(t, read, owner, directory, ngac.OpManage) {
		t.Error("the owner must manage the directory")
	}
	if testutil.Allowed(t, read, lim, directory, ngac.OpManage) {
		t.Error("a limited member must not manage the directory")
	}
	// One batch, several objects: the rule is applied per object.
	got := batch(lim, a.dir, b.dir, owners, a.peer, b.peer)
	for oa, want := range map[string]bool{a.dir: false, b.dir: false, owners: true, a.peer: true, b.peer: false} {
		if got[oa] != want {
			t.Errorf("mixed batch for the limited person: %s = %v, want %v", oa, got[oa], want)
		}
	}

	// Removing the rule gives the open tree back at once.
	removeRule()
	expect("rule removed: the limited person reads another department again", lim, b.dir, true)
	expect("rule removed: another workspace still reads nothing", foreign, a.dir, false)
}
