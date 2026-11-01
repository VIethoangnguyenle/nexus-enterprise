//go:build realpolicy

package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// These tests run the workspace service's administration guards against the real
// policy service. Removing a member needs invite on Mgmt; removing someone who
// holds manage on any area needs manage on Mgmt. Deleting a folder deletes a
// folder, never one of the workspace's own containers.

type guardWorld struct {
	*wsWorld
	assets string
	// callers: invite is invite-only; admin holds manage on Mgmt.
	invite, admin wsPerson
	// targets holding manage on one area each; plain holds nothing.
	byArea map[ngac.Area]wsPerson
	plain  wsPerson
}

func (w *guardWorld) isMember(p wsPerson) bool {
	w.t.Helper()
	_, write := testutil.StartPolicy(w.t)
	ok, err := policyclient.IsWorkspaceMember(testutil.PolicyAdmin(context.Background()), write, p.node, ngac.WorkspaceID(w.g.WorkspaceID))
	require.NoError(w.t, err)
	return ok
}

// roleHolder is a member who also sits in a role that holds ops on oa.
func (w *guardWorld) roleHolder(oa string, ops ...string) wsPerson {
	w.t.Helper()
	_, write := testutil.StartPolicy(w.t)
	role := testutil.NewNode(w.t, write, "Role_rp-"+uuid.NewString(), ngac.TypeUA, w.g.PC)
	testutil.Associate(w.t, write, role, oa, ops)
	return w.person(w.g.Members, role)
}

func newGuardWorld(t *testing.T) *guardWorld {
	t.Helper()
	w := &guardWorld{wsWorld: newWSWorld(t), byArea: map[ngac.Area]wsPerson{}}
	_, write := testutil.StartPolicy(t)
	w.assets = testutil.NewNode(t, write, ngac.AssetsOAName(ngac.WorkspaceID(w.g.WorkspaceID)), ngac.TypeOA, w.g.PC)

	w.invite = w.roleHolder(w.g.Mgmt, ngac.OpInvite)
	w.admin = w.roleHolder(w.g.Mgmt, ngac.OpInvite, ngac.OpManage)
	w.byArea[ngac.AreaManagement] = w.roleHolder(w.g.Mgmt, ngac.OpManage)
	w.byArea[ngac.AreaDocuments] = w.roleHolder(w.g.Documents, ngac.OpManage)
	w.byArea[ngac.AreaChannels] = w.roleHolder(w.g.Channels, ngac.OpManage)
	w.byArea[ngac.AreaAssets] = w.roleHolder(w.assets, ngac.OpManage)
	w.plain = w.person(w.g.Members)
	return w
}

// failingBatch answers every single check but fails the batch one.
type failingBatch struct {
	policypb.PolicyReadServiceClient
}

func (failingBatch) BatchCheckAccess(context.Context, *policypb.BatchCheckAccessRequest, ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	return nil, errors.New("policy unavailable")
}

// A person with invite alone removes ordinary members, not anyone who holds
// manage on an area; the same removals succeed for a holder of manage on Mgmt.
func TestRealPolicy_RemovingAManagerNeedsManage(t *testing.T) {
	w := newGuardWorld(t)
	ws := w.g.WorkspaceID
	read, _ := testutil.StartPolicy(t)

	// The fixture is meaningful: every target really holds manage on its area.
	areaOA := map[ngac.Area]string{ngac.AreaManagement: w.g.Mgmt, ngac.AreaDocuments: w.g.Documents, ngac.AreaChannels: w.g.Channels, ngac.AreaAssets: w.assets}
	for area, p := range w.byArea {
		require.True(t, testutil.Allowed(t, read, p.node, areaOA[area], ngac.OpManage), string(area))
	}
	require.False(t, testutil.Allowed(t, read, w.invite.node, w.g.Mgmt, ngac.OpManage))
	require.True(t, testutil.Allowed(t, read, w.invite.node, w.g.Mgmt, ngac.OpInvite))

	for area, target := range w.byArea {
		err := w.svc.RemoveMember(w.as(w.invite), w.invite.node, ws, target.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, "invite-only removes the manager of "+string(area))
		assert.True(t, w.isMember(target), "the manager of "+string(area)+" stays")
	}

	require.NoError(t, w.svc.RemoveMember(w.as(w.invite), w.invite.node, ws, w.plain.node), "invite-only removes a plain member")
	assert.False(t, w.isMember(w.plain))

	for area, target := range w.byArea {
		require.NoError(t, w.svc.RemoveMember(w.as(w.admin), w.admin.node, ws, target.node), "a manager of Mgmt removes the manager of "+string(area))
		assert.False(t, w.isMember(target))
	}
}

// parentsOf lists the IDs the node is directly assigned to.
func (w *guardWorld) parentsOf(id string) []string {
	w.t.Helper()
	_, write := testutil.StartPolicy(w.t)
	ps, err := write.GetParents(testutil.PolicyAdmin(context.Background()), &policypb.GetParentsRequest{NodeId: id})
	require.NoError(w.t, err)
	var out []string
	for _, n := range ps.GetNodes() {
		out = append(out, n.Id)
	}
	return out
}

// Removing a member removes a person, nothing else. A role restricted in the
// directory, a sub-department, the Members and Owners UAs, a node of another
// workspace and an ID that names nothing are refused, by an invite-only member and
// by an owner alike, and every edge they had stays: detaching the role from the
// restriction would lift its prohibition, detaching the department would cut it
// off its parent. The paired allow: the same callers still remove a person.
func TestRealPolicy_RemoveMemberRemovesOnlyPeople(t *testing.T) {
	type target struct {
		id      string
		parents []string
	}
	w := newGuardWorld(t)
	_, write := testutil.StartPolicy(t)
	limited := testutil.NewNode(t, write, ngac.DirectoryLimitedUAName(ngac.WorkspaceID(w.g.WorkspaceID)), ngac.TypeUA, w.g.PC)
	role := testutil.NewNode(t, write, ngac.RoleUAName(ngac.RoleID("rp-"+uuid.NewString())), ngac.TypeUA, w.g.PC, limited)
	parentDept := testutil.NewNode(t, write, ngac.DeptUAName(ngac.DeptID("rp-"+uuid.NewString())), ngac.TypeUA, w.g.PC)
	childDept := testutil.NewNode(t, write, ngac.DeptUAName(ngac.DeptID("rp-"+uuid.NewString())), ngac.TypeUA, parentDept)
	w.person(w.g.Members, role, childDept)
	other := testutil.SeedWorkspaceGraph(t, write)
	ids := map[string]string{
		"a role restricted in the directory": role,
		"the directory restriction":          limited,
		"a sub-department":                   childDept,
		"a top department":                   parentDept,
		"the Members UA":                     w.g.Members,
		"the Owners UA":                      w.g.Owners,
		"another workspace's Members UA":     other.Members,
		"another workspace's person":         w.outsider.node,
		"a node that does not exist":         uuid.NewString(),
	}
	out := map[string]target{}
	for name, id := range ids {
		out[name] = target{id: id, parents: w.parentsOf(id)}
	}
	require.Contains(t, out["a role restricted in the directory"].parents, limited, "the fixture is meaningful")
	require.Contains(t, out["a sub-department"].parents, parentDept, "the fixture is meaningful")

	for name, tg := range out {
		for caller, p := range map[string]wsPerson{"invite-only": w.invite, "owner": w.owner} {
			err := w.svc.RemoveMember(w.as(p), p.node, w.g.WorkspaceID, tg.id)
			assert.ErrorIs(t, err, domain.ErrNotFound, caller+" removing "+name)
			assert.ElementsMatch(t, tg.parents, w.parentsOf(tg.id), caller+" removing "+name+": every edge stays")
		}
	}

	require.NoError(t, w.svc.RemoveMember(w.as(w.invite), w.invite.node, w.g.WorkspaceID, w.plain.node), "invite-only removes a person")
	assert.False(t, w.isMember(w.plain))
	co := w.person(w.g.Members)
	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), w.owner.node, w.g.WorkspaceID, co.node), "an owner removes a person")
	assert.False(t, w.isMember(co))
}

// If the policy cannot answer whether the target holds manage, the removal is
// refused; with it answering, the same removal goes through.
func TestRealPolicy_RemovingAMemberIsRefusedWhenManageCannotBeChecked(t *testing.T) {
	w := newGuardWorld(t)
	ws := w.g.WorkspaceID
	_, write := testutil.StartPolicy(t)
	read, _ := testutil.StartPolicy(t)

	st := store.New(w.pool)
	broken := domain.NewService(st, st, failingBatch{read}, write, nil, nil)
	err := broken.RemoveMember(w.as(w.invite), w.invite.node, ws, w.plain.node)
	assert.ErrorIs(t, err, domain.ErrUnavailable, "refused as unavailable, not as a denial")
	assert.NotErrorIs(t, err, domain.ErrAccessDenied)
	assert.True(t, w.isMember(w.plain), "nothing detached")

	require.NoError(t, w.svc.RemoveMember(w.as(w.invite), w.invite.node, ws, w.plain.node))
	assert.False(t, w.isMember(w.plain))
}

// An owner who is removed still takes an owner plus manage, as before.
func TestRealPolicy_RemovingAnOwnerStillNeedsAnOwner(t *testing.T) {
	w := newGuardWorld(t)
	ws := w.g.WorkspaceID
	co := w.person(w.g.Owners)
	err := w.svc.RemoveMember(w.as(w.admin), w.admin.node, ws, co.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a manager who is not an owner")
	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), w.owner.node, ws, co.node), "an owner")
}

func (w *guardWorld) nodePresent(id string) bool {
	w.t.Helper()
	_, write := testutil.StartPolicy(w.t)
	desc, err := write.GetDescendants(testutil.PolicyAdmin(context.Background()), &policypb.GetDescendantsRequest{NodeId: w.g.PC})
	require.NoError(w.t, err)
	for _, n := range desc.GetNodes() {
		if n.Id == id {
			return true
		}
	}
	return false
}

// createOA makes an OA under parent with the workspace_id property folders and
// asset OAs carry.
func (w *guardWorld) createOA(name, workspaceID, parent string, typ ...string) string {
	w.t.Helper()
	props := map[string]string{"workspace_id": workspaceID}
	if len(typ) > 0 {
		props[ngac.PropType] = typ[0]
	}
	_, write := testutil.StartPolicy(w.t)
	n, err := write.CreateNode(testutil.PolicyAdmin(context.Background()), &policypb.CreateNodeRequest{
		Name: name, NodeType: ngac.TypeOA, Properties: props,
	})
	require.NoError(w.t, err)
	w.t.Cleanup(func() {
		_, _ = write.DeleteNode(testutil.PolicyAdmin(context.Background()), &policypb.DeleteNodeRequest{NodeId: n.GetId()})
	})
	testutil.Assign(w.t, write, n.GetId(), parent)
	return n.GetId()
}

// DeleteFolder deletes a folder. The workspace's own containers (Mgmt, Channels,
// Documents), a group's content OA, an asset type's OA, and a folder of another
// workspace are refused, and stay; a real folder is deleted.
func TestRealPolicy_DeleteFolderDeletesOnlyFolders(t *testing.T) {
	ids := func(w *guardWorld) map[string]string {
		ws := ngac.WorkspaceID(w.g.WorkspaceID)
		return map[string]string{
			"Mgmt":                              w.g.Mgmt,
			"Channels":                          w.g.Channels,
			"Documents":                         w.g.Documents,
			"Assets":                            w.assets,
			"a group's content":                 w.createOA(ngac.ChannelContentOAName(ngac.ChannelID("rp-"+uuid.NewString())), w.g.WorkspaceID, w.g.Channels),
			"an asset type":                     w.createOA(ngac.AssetTypeOAName(ws, ngac.AssetTypeID("rp-"+uuid.NewString())), w.g.WorkspaceID, w.assets),
			"a Documents drive folder":          w.createOA(ngac.FolderNodeName(ngac.FolderID("rp-"+uuid.NewString())), w.g.WorkspaceID, w.g.Documents, "drive_folder"),
			"a group's drive folder":            w.createOA(ngac.FolderNodeName(ngac.FolderID("rp-"+uuid.NewString())), w.g.WorkspaceID, w.g.Channels, "drive_folder"),
			"a folder naming another workspace": w.createOA(ngac.FolderNodeName(ngac.FolderID("rp-"+uuid.NewString())), "someone-elses-workspace", w.g.Documents),
		}
	}
	names := []string{"Mgmt", "Channels", "Documents", "Assets", "a group's content", "an asset type", "a Documents drive folder", "a group's drive folder", "a folder naming another workspace"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			w := newGuardWorld(t)
			target := ids(w)[name]
			require.True(t, w.nodePresent(target))
			err := w.svc.DeleteFolder(w.as(w.owner), w.owner.node, w.g.WorkspaceID, target)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.True(t, w.nodePresent(target), "the node stays")
		})
	}

	t.Run("a folder of another workspace", func(t *testing.T) {
		w := newGuardWorld(t)
		_, write := testutil.StartPolicy(t)
		other := testutil.SeedWorkspaceGraph(t, write)
		theirs := w.createOA(ngac.FolderNodeName(ngac.FolderID("rp-"+uuid.NewString())), other.WorkspaceID, other.PC)
		err := w.svc.DeleteFolder(w.as(w.owner), w.owner.node, w.g.WorkspaceID, theirs)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		desc, derr := write.GetDescendants(testutil.PolicyAdmin(context.Background()), &policypb.GetDescendantsRequest{NodeId: other.PC})
		require.NoError(t, derr)
		var found bool
		for _, n := range desc.GetNodes() {
			found = found || n.Id == theirs
		}
		assert.True(t, found, "their folder stays")
	})

	t.Run("a real folder", func(t *testing.T) {
		w := newGuardWorld(t)
		f, err := w.svc.CreateFolder(w.as(w.owner), w.owner.node, w.g.WorkspaceID, "Reports", "")
		require.NoError(t, err)
		require.True(t, w.nodePresent(f.NGACNodeID))
		require.NoError(t, w.svc.DeleteFolder(w.as(w.owner), w.owner.node, w.g.WorkspaceID, f.NGACNodeID))
		assert.False(t, w.nodePresent(f.NGACNodeID), "the folder is gone")
	})
}

// A folder made before folders were typed (no type at all) is still an admin
// folder and deletable; the listing shows admin folders and nothing else.
func TestRealPolicy_ListAndDeleteTreatAdminFoldersAlike(t *testing.T) {
	w := newGuardWorld(t)
	ws := w.g.WorkspaceID
	made, err := w.svc.CreateFolder(w.as(w.owner), w.owner.node, ws, "Reports", "")
	require.NoError(t, err)
	legacy := w.createOA(ngac.FolderNodeName(ngac.FolderID("rp-"+uuid.NewString())), ws, w.g.PC)
	drive := w.createOA(ngac.FolderNodeName(ngac.FolderID("rp-"+uuid.NewString())), ws, w.g.Documents, "drive_folder")

	list, err := w.svc.ListFolders(w.as(w.member), w.member.node, ws)
	require.NoError(t, err)
	var ids []string
	for _, f := range list {
		ids = append(ids, f.NGACNodeID)
	}
	assert.ElementsMatch(t, []string{made.NGACNodeID, legacy}, ids, "admin folders only: not Mgmt, Documents, Channels, nor a drive folder")

	require.NoError(t, w.svc.DeleteFolder(w.as(w.owner), w.owner.node, ws, legacy), "an untyped folder of the workspace")
	assert.False(t, w.nodePresent(legacy))
	assert.True(t, w.nodePresent(drive))
}
