package domain_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
)

// ---------------------------------------------------------------------------
// Directory fake: what the users table and tenant_users would answer.
// ---------------------------------------------------------------------------

type fakeDirectory struct {
	profiles map[string]*store.Profile // by node ID
	listed   []string                  // EnsureTenantUser calls: tenant/node
	removed  []string                  // RemoveTenantUser calls: tenant/node
	ensure   error
	profErr  error
}

func (d *fakeDirectory) ProfilesByNodeIDs(_ context.Context, _ string, ids []string) (map[string]*store.Profile, error) {
	if d.profErr != nil {
		return nil, d.profErr
	}
	out := map[string]*store.Profile{}
	for _, id := range ids {
		if p, ok := d.profiles[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

func (d *fakeDirectory) EnsureTenantUser(_ context.Context, tenant, _, node string) error {
	d.listed = append(d.listed, tenant+"/"+node)
	return d.ensure
}

func (d *fakeDirectory) RemoveTenantUser(_ context.Context, tenant, node string) error {
	d.removed = append(d.removed, tenant+"/"+node)
	return nil
}

const (
	uuidLike = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
	newbie   = "u-newbie"
)

var uuidRE = regexp.MustCompile(uuidLike)

// adminFixture is the shared fixture with a people directory wired in.
type adminFixture struct {
	*fixture
	dir *fakeDirectory
}

func newAdminFixture(t *testing.T) *adminFixture {
	t.Helper()
	f := newFixture(t)
	dir := &fakeDirectory{
		profiles: map[string]*store.Profile{
			target: {UserID: "user-target", NodeID: target, Username: "lan", DisplayName: "Nguyễn Thu Lan", Email: "lan@novapay.vn", Title: "Chuyên viên", Status: "active"},
			owner:  {UserID: "user-owner", NodeID: owner, Username: "hoa", DisplayName: "Lê Thị Hoa", Email: "hoa@novapay.vn", Status: "active"},
		},
	}
	f.svc = f.svc.WithDirectory(dir)
	f.read.nodes[newbie] = node(newbie, "newbie", ngac.TypeU)
	return &adminFixture{fixture: f, dir: dir}
}

func ctx() context.Context { return context.Background() }

// The callers a denied admin call is tried with: one step short of the grant.
func nonManagers() []string { return []string{member, inviter, outsider, otherOwner, ""} }

// ---------------------------------------------------------------------------
// Permission areas
// ---------------------------------------------------------------------------

func TestListPermissionAreas_ListsEachAreaWithItsOperations(t *testing.T) {
	f := newAdminFixture(t)
	areas, err := f.svc.ListPermissionAreas(ctx(), member, ws1)
	require.NoError(t, err)

	got := map[ngac.Area][]string{}
	for _, a := range areas {
		got[a.Area] = a.Operations
	}
	for _, a := range ngac.Areas() {
		assert.Equal(t, ngac.AreaOps(a), got[a], "operations of %s come from the ngac package", a)
	}
	require.Len(t, areas, len(ngac.Areas()))
	assert.Equal(t, ngac.Areas()[0], areas[0].Area, "order is the ngac package's")
}

func TestListPermissionAreas_OmitsAnAreaThisWorkspaceDoesNotHave(t *testing.T) {
	f := newAdminFixture(t)
	// A workspace that has never used assets has no Assets OA.
	children := f.read.children[pc1]
	kept := children[:0:0]
	for _, n := range children {
		if n.Id != assets1 {
			kept = append(kept, n)
		}
	}
	f.read.children[pc1] = kept

	areas, err := f.svc.ListPermissionAreas(ctx(), member, ws1)
	require.NoError(t, err)
	for _, a := range areas {
		assert.NotEqual(t, ngac.AreaAssets, a.Area)
	}
	assert.Len(t, areas, len(ngac.Areas())-1)
}

func TestListPermissionAreas_DeniesNonMembers(t *testing.T) {
	for _, caller := range []string{outsider, otherOwner, ""} {
		f := newAdminFixture(t)
		_, err := f.svc.ListPermissionAreas(ctx(), caller, ws1)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
	}
}

// ---------------------------------------------------------------------------
// Role list and detail
// ---------------------------------------------------------------------------

func TestListRoleSummaries_NamesRolesByDisplayNameAndFlagsSystemRoles(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children[roleEdit] = []*policypb.NGACNode{node(target, "target", ngac.TypeU), node(leaver, "leaver", ngac.TypeU)}

	roles, err := f.svc.ListRoleSummaries(ctx(), member, ws1)
	require.NoError(t, err)

	byKind := map[domain.RoleKind][]*domain.RoleSummary{}
	for _, r := range roles {
		byKind[r.Kind] = append(byKind[r.Kind], r)
		assert.False(t, uuidRE.MatchString(r.Name), "no id as a name: %q", r.Name)
		assert.False(t, strings.HasPrefix(r.Name, "Role_"), "the node name is not a role's name: %q", r.Name)
	}
	require.Len(t, byKind[domain.RoleOwners], 1)
	require.Len(t, byKind[domain.RoleMembers], 1)
	assert.Equal(t, 2, byKind[domain.RoleOwners][0].MemberCount)
	names := map[string]int{}
	for _, r := range byKind[domain.RoleCustom] {
		names[r.Name] = r.MemberCount
	}
	assert.Equal(t, 2, names["Biên tập"], "the display name, with the people who hold it")
	assert.Contains(t, names, "Người đọc")
	assert.Contains(t, names, "Editor", "an older role whose node name is its name still reads")
}

func TestListRoleSummaries_DeniesNonMembers(t *testing.T) {
	for _, caller := range []string{outsider, otherOwner, ""} {
		f := newAdminFixture(t)
		_, err := f.svc.ListRoleSummaries(ctx(), caller, ws1)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
	}
}

func TestGetRoleDetail_ShowsMembersAndGrantsByArea(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children[roleEdit] = []*policypb.NGACNode{node(target, "target", ngac.TypeU)}
	// A grant on something that is not an area (a folder) is not shown as one.
	f.read.assocs[roleEdit] = append(f.read.assocs[roleEdit], &policypb.Association{Id: "as-x", UaId: roleEdit, OaId: folder1, Operations: []string{ngac.OpRead}})

	d, err := f.svc.GetRoleDetail(ctx(), manager, ws1, roleEdit)
	require.NoError(t, err)

	assert.Equal(t, "Biên tập", d.Name)
	assert.Equal(t, domain.RoleCustom, d.Kind)
	require.Len(t, d.Members, 1)
	assert.Equal(t, "Nguyễn Thu Lan", d.Members[0].DisplayName)
	require.Len(t, d.Grants, 1)
	assert.Equal(t, ngac.AreaDocuments, d.Grants[0].Area)
	assert.Equal(t, []string{ngac.OpRead, ngac.OpWrite}, d.Grants[0].Operations)
}

func TestGetRoleDetail_SystemRolesAreReadable(t *testing.T) {
	f := newAdminFixture(t)
	f.read.assocs[members1] = []*policypb.Association{{Id: "as-m", UaId: members1, OaId: docs1, Operations: ngac.MemberDocumentOps()}}

	d, err := f.svc.GetRoleDetail(ctx(), manager, ws1, members1)
	require.NoError(t, err)
	assert.Equal(t, domain.RoleMembers, d.Kind)
	require.Len(t, d.Grants, 1)
	assert.Contains(t, d.Grants[0].Operations, ngac.OpShare, "members hold share on Documents and the screen has to show it")
}

func TestGetRoleDetail_Denies(t *testing.T) {
	for _, caller := range nonManagers() {
		f := newAdminFixture(t)
		_, err := f.svc.GetRoleDetail(ctx(), caller, ws1, roleEdit)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
	}
	f := newAdminFixture(t)
	f.read.checkErr = errors.New("policy down")
	_, err := f.svc.GetRoleDetail(ctx(), owner, ws1, roleEdit)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "an unreachable PDP denies")
}

func TestGetRoleDetail_OtherWorkspacesAndNonRolesAreNotFound(t *testing.T) {
	for name, id := range map[string]string{"foreign UA": owners2, "an OA": docs1, "unknown": "nope", "a department UA": "ua-dept-root-1"} {
		f := newAdminFixture(t)
		_, err := f.svc.GetRoleDetail(ctx(), owner, ws1, id)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
	}
}

func TestGetRoleDetail_FailsClosedWhenGrantsCannotBeRead(t *testing.T) {
	f := newAdminFixture(t)
	f.read.assocErr = errors.New("policy down")
	_, err := f.svc.GetRoleDetail(ctx(), owner, ws1, roleEdit)
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrNotFound)
}

// ---------------------------------------------------------------------------
// Setting a role's permissions
// ---------------------------------------------------------------------------

func TestSetRolePermissions_DeniesWithoutManageAndWritesNothing(t *testing.T) {
	for _, caller := range nonManagers() {
		f := newAdminFixture(t)
		_, err := f.svc.SetRolePermissions(ctx(), caller, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead})
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
		assert.Empty(t, f.write.mutations, caller)
	}
}

func TestSetRolePermissions_DeniesWhenPolicyCannotAnswer(t *testing.T) {
	f := newAdminFixture(t)
	f.read.checkErr = errors.New("policy down")
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead})
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)
}

func TestSetRolePermissions_OwnerSetsTheOperationsOfOneArea(t *testing.T) {
	f := newAdminFixture(t)
	g, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite, ngac.OpShare})
	require.NoError(t, err)
	assert.Equal(t, ngac.AreaDocuments, g.Area)
	assert.Equal(t, []string{ngac.OpRead, ngac.OpWrite, ngac.OpShare}, g.Operations)

	require.Len(t, f.write.assocs, 1)
	assert.Equal(t, roleRead, f.write.assocs[0].UaId)
	assert.Equal(t, docs1, f.write.assocs[0].OaId, "the OA is resolved from the area inside this workspace")
	assert.Equal(t, []string{ngac.OpRead, ngac.OpWrite, ngac.OpShare}, f.write.assocs[0].Operations)
}

func TestSetRolePermissions_EmptySetRemovesTheGrant(t *testing.T) {
	f := newAdminFixture(t)
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"RemoveAssociation " + roleRead + "->" + docs1}, f.write.mutations)
	assert.Empty(t, f.write.assocs)
}

func TestSetRolePermissions_EmptySetWhereNothingIsGrantedChangesNothing(t *testing.T) {
	f := newAdminFixture(t)
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaChannels, nil)
	require.NoError(t, err)
	assert.Empty(t, f.write.mutations)
}

func TestSetRolePermissions_CallerCannotAddWhatTheyDoNotHold(t *testing.T) {
	// delegator holds manage on Mgmt but only read on Documents.
	f := newAdminFixture(t)
	_, err := f.svc.SetRolePermissions(ctx(), delegator, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations, "one op not held rejects the whole request")

	_, err = f.svc.SetRolePermissions(ctx(), delegator, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpShare})
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestSetRolePermissions_CallerMayKeepOrNarrowWhatIsAlreadyGranted(t *testing.T) {
	// roleEdit already has read and write. delegator holds only read, yet
	// narrowing to read adds nothing, and re-saving the same set adds nothing.
	f := newAdminFixture(t)
	_, err := f.svc.SetRolePermissions(ctx(), delegator, ws1, roleEdit, ngac.AreaDocuments, []string{ngac.OpRead})
	require.NoError(t, err)
	_, err = f.svc.SetRolePermissions(ctx(), delegator, ws1, roleEdit, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite})
	require.NoError(t, err)
}

func TestSetRolePermissions_ChecksEachAddedOperationAgainstTheAreasOA(t *testing.T) {
	f := newAdminFixture(t)
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpShare})
	require.NoError(t, err)
	var saw []string
	for _, c := range f.read.checks {
		if c.ObjectNodeId == docs1 && c.UserNodeId == owner {
			saw = append(saw, c.Operation)
		}
	}
	assert.ElementsMatch(t, []string{ngac.OpShare}, saw, "only the operation being added needs the caller to hold it")
}

func TestSetRolePermissions_RejectsOperationsTheAreaDoesNotOffer(t *testing.T) {
	f := newAdminFixture(t)
	for name, tc := range map[string]struct {
		area ngac.Area
		ops  []string
	}{
		"approve on documents":  {ngac.AreaDocuments, []string{ngac.OpApprove}},
		"upload on documents":   {ngac.AreaDocuments, []string{ngac.OpUpload}},
		"upload on management":  {ngac.AreaManagement, []string{ngac.OpUpload}},
		"unknown operation":     {ngac.AreaDocuments, []string{"superuser"}},
		"read on management":    {ngac.AreaManagement, []string{ngac.OpRead}},
		"one valid, one stray":  {ngac.AreaChannels, []string{ngac.OpRead, ngac.OpApprove}},
		"unknown area":          {"payroll", []string{ngac.OpRead}},
		"an empty area name":    {"", []string{ngac.OpRead}},
		"create_channel on doc": {ngac.AreaDocuments, []string{ngac.OpCreateChannel}},
	} {
		_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, tc.area, tc.ops)
		assert.ErrorIs(t, err, domain.ErrInvalidInput, name)
	}
	assert.Empty(t, f.write.mutations)
}

func TestSetRolePermissions_OnlyCustomRolesOfThisWorkspaceCanBeEdited(t *testing.T) {
	for name, id := range map[string]string{
		"the Owners UA":   owners1,
		"the Members UA":  members1,
		"a foreign role":  owners2,
		"a department UA": "ua-dept-root-1",
		"an OA":           docs1,
		"unknown":         "nope",
	} {
		f := newAdminFixture(t)
		_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, id, ngac.AreaDocuments, []string{ngac.OpRead})
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		assert.Empty(t, f.write.mutations, name)
	}
}

func TestSetRolePermissions_AreaMissingInThisWorkspaceIsNotFound(t *testing.T) {
	f := newAdminFixture(t)
	kept := []*policypb.NGACNode{}
	for _, n := range f.read.children[pc1] {
		if n.Id != assets1 {
			kept = append(kept, n)
		}
	}
	f.read.children[pc1] = kept
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaAssets, []string{ngac.OpRead})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Empty(t, f.write.mutations)
}

func TestSetRolePermissions_DuplicateOperationsAreCollapsed(t *testing.T) {
	f := newAdminFixture(t)
	g, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpWrite, ngac.OpRead, ngac.OpRead})
	require.NoError(t, err)
	assert.Equal(t, []string{ngac.OpRead, ngac.OpWrite}, g.Operations, "canonical order, once each")
}

// ---------------------------------------------------------------------------
// Assigning and removing a role
// ---------------------------------------------------------------------------

func TestAssignMemberRole_DeniesWithoutManage(t *testing.T) {
	for _, caller := range nonManagers() {
		f := newAdminFixture(t)
		err := f.svc.AssignMemberRole(ctx(), caller, ws1, target, role1)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
		assert.Empty(t, f.write.mutations, caller)
	}
}

func TestAssignMemberRole_AddsOneAssignmentAndKeepsTheRest(t *testing.T) {
	f := newAdminFixture(t)
	require.NoError(t, f.svc.AssignMemberRole(ctx(), owner, ws1, target, roleRead))
	assert.Equal(t, []string{"CreateAssignment " + target + "->" + roleRead}, f.write.mutations,
		"adding a role must not detach the person from anything else")
}

func TestAssignMemberRole_TargetMustAlreadyBeAMember(t *testing.T) {
	// Without this, manage would be a way to bring anyone into the workspace
	// without the invite operation.
	for name, id := range map[string]string{"a user of another workspace": otherOwner, "a user in no workspace": outsider, "an unknown node": "nope", "a role UA": roleRead} {
		f := newAdminFixture(t)
		err := f.svc.AssignMemberRole(ctx(), owner, ws1, id, role1)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		assert.Empty(t, f.write.mutations, name)
	}
}

func TestAssignMemberRole_OnlyCustomRolesOfThisWorkspace(t *testing.T) {
	for name, id := range map[string]string{"Owners UA": owners1, "Members UA": members1, "foreign UA": owners2, "unknown": "nope"} {
		f := newAdminFixture(t)
		err := f.svc.AssignMemberRole(ctx(), owner, ws1, target, id)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		assert.Empty(t, f.write.mutations, name)
	}
}

func TestAssignMemberRole_CannotHandOutWhatTheCallerDoesNotHold(t *testing.T) {
	// roleEdit confers read and write on Documents; delegator holds only read there.
	f := newAdminFixture(t)
	err := f.svc.AssignMemberRole(ctx(), delegator, ws1, target, roleEdit)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)

	// roleRead confers only what delegator holds.
	require.NoError(t, f.svc.AssignMemberRole(ctx(), delegator, ws1, target, roleRead))
	assert.Equal(t, []string{"CreateAssignment " + target + "->" + roleRead}, f.write.mutations)
}

func TestAssignMemberRole_ManagerWithNoRightsOnTheAreaCannotDelegateIt(t *testing.T) {
	f := newAdminFixture(t)
	err := f.svc.AssignMemberRole(ctx(), manager, ws1, target, roleRead)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)
}

func TestAssignMemberRole_FailsClosedWhenGrantsCannotBeRead(t *testing.T) {
	f := newAdminFixture(t)
	f.read.assocErr = errors.New("policy down")
	err := f.svc.AssignMemberRole(ctx(), owner, ws1, target, roleRead)
	require.Error(t, err)
	assert.Empty(t, f.write.mutations, "unknown grants are not assumed to be none")
}

func TestUnassignMemberRole_DeniesWithoutManageAndChecksScope(t *testing.T) {
	for _, caller := range nonManagers() {
		f := newAdminFixture(t)
		err := f.svc.UnassignMemberRole(ctx(), caller, ws1, target, role1)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
		assert.Empty(t, f.write.mutations, caller)
	}
	f := newAdminFixture(t)
	assert.ErrorIs(t, f.svc.UnassignMemberRole(ctx(), owner, ws1, target, owners1), domain.ErrNotFound, "never the Owners UA")
	assert.ErrorIs(t, f.svc.UnassignMemberRole(ctx(), owner, ws1, otherOwner, role1), domain.ErrNotFound)
	assert.Empty(t, f.write.mutations)
}

func TestUnassignMemberRole_RemovesOnlyThatAssignment(t *testing.T) {
	f := newAdminFixture(t)
	require.NoError(t, f.svc.UnassignMemberRole(ctx(), manager, ws1, target, roleRead))
	assert.Equal(t, []string{"RemoveAssignment " + target + "->" + roleRead}, f.write.mutations,
		"removing a role does not need the caller to hold what it confers")
}

// ---------------------------------------------------------------------------
// The people table
// ---------------------------------------------------------------------------

func TestListMemberDirectory_DeniesWithoutManage(t *testing.T) {
	for _, caller := range nonManagers() {
		f := newAdminFixture(t)
		_, err := f.svc.ListMemberDirectory(ctx(), caller, ws1)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
	}
}

func TestListMemberDirectory_JoinsNamesRolesDepartmentAndOwnerFlag(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children[roleEdit] = []*policypb.NGACNode{node(target, "target", ngac.TypeU)}
	f.read.children["ua-dept-root-1"] = []*policypb.NGACNode{node(target, "target", ngac.TypeU)}

	people, err := f.svc.ListMemberDirectory(ctx(), manager, ws1)
	require.NoError(t, err)

	by := map[string]*domain.MemberView{}
	for _, p := range people {
		by[p.NodeID] = p
	}
	require.Contains(t, by, target)
	lan := by[target]
	assert.Equal(t, "Nguyễn Thu Lan", lan.DisplayName)
	assert.Equal(t, "lan@novapay.vn", lan.Email)
	assert.Equal(t, "Chuyên viên", lan.Title)
	assert.True(t, lan.Owner, "target is in the Owners UA in this fixture")
	require.Len(t, lan.Roles, 1)
	assert.Equal(t, "Biên tập", lan.Roles[0].Name)
	require.NotNil(t, lan.Department)
	assert.Equal(t, "Root", lan.Department.Name)
	assert.Equal(t, "dept-root-1", lan.Department.ID)
	assert.Equal(t, "user-target", lan.UserID, "the colour key is carried; the node ID is not for display")

	hoa := by[owner]
	require.NotNil(t, hoa)
	assert.True(t, hoa.Owner)
	assert.Nil(t, hoa.Department)
	assert.Empty(t, hoa.Roles)
}

func TestListMemberDirectory_NeverFallsBackToAnIdentifier(t *testing.T) {
	f := newAdminFixture(t)
	// Nobody has a profile row; a U node's own name is all there is.
	f.dir.profiles = map[string]*store.Profile{}
	f.read.descendants[pc1] = append(f.read.descendants[pc1], &policypb.NGACNode{
		Id: "4b0f6c7e-1111-4222-8333-444455556666", Name: "U_4b0f6c7e-1111-4222-8333-444455556666", NodeType: ngac.TypeU,
		Properties: map[string]string{ngac.PropDisplayName: "dung.pham"},
	})
	people, err := f.svc.ListMemberDirectory(ctx(), owner, ws1)
	require.NoError(t, err)
	for _, p := range people {
		assert.NotEmpty(t, p.DisplayName)
		assert.False(t, uuidRE.MatchString(p.DisplayName), "a name is never an id: %q", p.DisplayName)
		assert.False(t, strings.HasPrefix(p.DisplayName, "U_"), "nor the node name: %q", p.DisplayName)
	}
}

func TestListMemberDirectory_NodeNameWithoutDisplayNameIsReplacedByANeutralWord(t *testing.T) {
	f := newAdminFixture(t)
	f.dir.profiles = map[string]*store.Profile{}
	id := "4b0f6c7e-1111-4222-8333-444455556666"
	f.read.descendants[pc1] = []*policypb.NGACNode{{Id: id, Name: "U_" + id, NodeType: ngac.TypeU}}
	people, err := f.svc.ListMemberDirectory(ctx(), owner, ws1)
	require.NoError(t, err)
	require.Len(t, people, 1)
	assert.Equal(t, "Thành viên", people[0].DisplayName)
}

func TestListMemberDirectory_ProfileFailureStillAnswers(t *testing.T) {
	f := newAdminFixture(t)
	f.dir.profErr = errors.New("db down")
	people, err := f.svc.ListMemberDirectory(ctx(), owner, ws1)
	require.NoError(t, err)
	assert.NotEmpty(t, people, "names fall back to what the graph holds")
}

// ---------------------------------------------------------------------------
// Removing a person
// ---------------------------------------------------------------------------

func TestRemoveMember_DropsTheListingToo(t *testing.T) {
	f := newAdminFixture(t)
	require.NoError(t, f.svc.RemoveMember(ctx(), inviter, ws1, leaver))
	assert.Equal(t, []string{ws1 + "/" + leaver}, f.dir.removed)
}

func TestRemoveMember_AnOwnerNeedsManageNotJustInvite(t *testing.T) {
	// target is in the Owners UA in this fixture. Invite lets you bring people
	// in and send them out, not demote the people who run the workspace.
	f := newAdminFixture(t)
	err := f.svc.RemoveMember(ctx(), inviter, ws1, target)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)
	assert.Empty(t, f.dir.removed)
}

func TestRemoveMember_TheLastOwnerStays(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children[owners1] = []*policypb.NGACNode{node(owner, "owner", ngac.TypeU)}
	f.read.grants[grant{owner, mgmt1, ngac.OpInvite}] = true
	err := f.svc.RemoveMember(ctx(), owner, ws1, owner)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, f.write.mutations)
}

func TestRemoveMember_ManageHolderMayRemoveANonLastOwner(t *testing.T) {
	f := newAdminFixture(t)
	f.read.grants[grant{owner, mgmt1, ngac.OpInvite}] = true
	require.NoError(t, f.svc.RemoveMember(ctx(), owner, ws1, target))
	assert.Equal(t, []string{ws1 + "/" + target}, f.dir.removed)
}

// ---------------------------------------------------------------------------
// A person's department
// ---------------------------------------------------------------------------

func TestUpdateMemberDepartment_TargetMustBeAMember(t *testing.T) {
	for name, id := range map[string]string{"another workspace": otherOwner, "no workspace": outsider, "unknown": "nope"} {
		f := newAdminFixture(t)
		err := f.svc.UpdateMemberDepartment(ctx(), owner, ws1, id, "dept-root-1")
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		assert.False(t, f.mutated(), name)
	}
}

func TestUpdateMemberDepartment_MovesOutOfTheOldDepartment(t *testing.T) {
	f := newAdminFixture(t)
	f.read.parents[target] = []*policypb.NGACNode{node("ua-dept-other-1", "d", ngac.TypeUA), node(owners1, "o", ngac.TypeUA)}

	require.NoError(t, f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1"))

	assert.Equal(t, []string{
		"CreateAssignment " + target + "->ua-dept-root-1",
		"RemoveAssignment " + target + "->ua-dept-other-1",
	}, f.write.mutations, "a person is in one department; Owners is not touched")
	assert.Equal(t, []string{"user-dept " + ws1 + " " + target + " dept-root-1"}, f.depts.mutations)
}

func TestUpdateMemberDepartment_EmptyDepartmentLeavesTheCurrentOne(t *testing.T) {
	f := newAdminFixture(t)
	f.read.parents[target] = []*policypb.NGACNode{node("ua-dept-other-1", "d", ngac.TypeUA)}

	require.NoError(t, f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, ""))

	assert.Equal(t, []string{"RemoveAssignment " + target + "->ua-dept-other-1"}, f.write.mutations)
	assert.Equal(t, []string{"user-dept " + ws1 + " " + target + " "}, f.depts.mutations)
}

func TestUpdateMemberDepartment_CannotHandOutWhatTheDepartmentConfersAndTheCallerLacks(t *testing.T) {
	f := newAdminFixture(t)
	f.read.assocs["ua-dept-root-1"] = []*policypb.Association{{Id: "as-d", UaId: "ua-dept-root-1", OaId: docs1, Operations: []string{ngac.OpWrite}}}

	err := f.svc.UpdateMemberDepartment(ctx(), delegator, ws1, target, "dept-root-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, f.mutated())

	require.NoError(t, f.svc.UpdateMemberDepartment(ctx(), owner, ws1, target, "dept-root-1"))
}

func TestUpdateMemberDepartment_FailedDetachUndoesTheNewAssignment(t *testing.T) {
	f := newAdminFixture(t)
	f.read.parents[target] = []*policypb.NGACNode{node("ua-dept-other-1", "d", ngac.TypeUA)}
	failing := &failingRemove{fakePolicyWrite: f.write, ua: "ua-dept-other-1"}
	svc := domain.NewService(f.wsStore, f.depts, f.read, failing, nil, nil).WithDirectory(f.dir)

	err := svc.UpdateMemberDepartment(ctx(), owner, ws1, target, "dept-root-1")
	require.Error(t, err)
	assert.Contains(t, f.write.mutations, "RemoveAssignment "+target+"->ua-dept-root-1", "the new assignment is taken back")
	assert.Empty(t, f.depts.mutations, "the row follows the graph, which did not change")
}

type failingRemove struct {
	*fakePolicyWrite
	ua string
}

func (f *failingRemove) RemoveAssignment(c context.Context, req *policypb.RemoveAssignmentRequest, o ...grpc.CallOption) (*policypb.Empty, error) {
	if req.ParentId == f.ua {
		return nil, errors.New("policy down")
	}
	return f.fakePolicyWrite.RemoveAssignment(c, req, o...)
}

// ---------------------------------------------------------------------------
// Deleting a department keeps the graph whole
// ---------------------------------------------------------------------------

func TestDeleteDepartment_MovesSubDepartmentsAndPeopleUpBeforeTheNodeGoes(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children["ua-dept-child-1"] = []*policypb.NGACNode{node(target, "target", ngac.TypeU)}

	require.NoError(t, f.svc.DeleteDepartment(ctx(), owner, ws1, "dept-child-1"))
	// dept-child-1 sits under dept-root-1: its people go there.
	assert.Equal(t, []string{
		"CreateAssignment " + target + "->ua-dept-root-1",
		"DeleteNode ua-dept-child-1",
	}, f.write.mutations)

	f2 := newAdminFixture(t)
	// dept-root-1 is a root with a child: the child goes to the PC, so it still reaches the workspace.
	require.NoError(t, f2.svc.DeleteDepartment(ctx(), owner, ws1, "dept-root-1"))
	assert.Equal(t, []string{
		"CreateAssignment ua-dept-child-1->" + pc1,
		"DeleteNode ua-dept-root-1",
	}, f2.write.mutations)
}

func TestDeleteDepartment_StopsBeforeDeletingIfTheGraphCannotBeKept(t *testing.T) {
	f := newAdminFixture(t)
	failing := &failingAssign{fakePolicyWrite: f.write}
	svc := domain.NewService(f.wsStore, f.depts, f.read, failing, nil, nil).WithDirectory(f.dir)
	err := svc.DeleteDepartment(ctx(), owner, ws1, "dept-root-1")
	require.Error(t, err)
	for _, m := range f.write.mutations {
		assert.NotContains(t, m, "DeleteNode", "nothing is deleted when the children could not be moved")
	}
	assert.Empty(t, f.depts.mutations, "and the rows are untouched")
}

type failingAssign struct{ *fakePolicyWrite }

func (f *failingAssign) CreateAssignment(context.Context, *policypb.CreateAssignmentRequest, ...grpc.CallOption) (*policypb.Assignment, error) {
	return nil, errors.New("policy down")
}

// ---------------------------------------------------------------------------
// No path grants across workspaces
// ---------------------------------------------------------------------------

// The only way left to change what a role holds is SetRolePermissions, which
// takes the area (resolved inside the route's workspace) and a role of that
// workspace. A caller who administers workspace B cannot reach workspace A's
// attributes through B's route, nor B's attributes through A's.
func TestSetRolePermissions_CannotReachAnotherWorkspace(t *testing.T) {
	f := newAdminFixture(t)
	// otherOwner administers ws-2 only. ws-1's role and ws-1's Members/Owners UAs
	// are not nodes of ws-2.
	for name, id := range map[string]string{"a role of ws-1": roleRead, "ws-1 Members UA": members1, "ws-1 Owners UA": owners1} {
		_, err := f.svc.SetRolePermissions(ctx(), otherOwner, ws2, id, ngac.AreaManagement, []string{ngac.OpManage})
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
	}
	// And the other way: ws-1's owner has no authority on ws-2's route.
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws2, roleRead, ngac.AreaManagement, []string{ngac.OpManage})
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)
	assert.Empty(t, f.write.assocs)
}

// A role manager must not be able to demote Owners on Mgmt: Owners is not a
// role, so no remaining path writes its associations.
func TestSetRolePermissions_CannotEditOwnersOrMembers(t *testing.T) {
	f := newAdminFixture(t)
	for _, id := range []string{owners1, members1} {
		_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, id, ngac.AreaManagement, nil)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	}
	assert.Empty(t, f.write.mutations)
}

// ---------------------------------------------------------------------------
// Owners: only an Owner changes who the owners are
// ---------------------------------------------------------------------------

func TestOwnerChanges_NeedAnOwnerNotJustManage(t *testing.T) {
	for name, call := range map[string]func(f *adminFixture, caller string) error{
		"TransferOwnership": func(f *adminFixture, c string) error { return f.svc.TransferOwnership(ctx(), c, ws1, leaver) },
		"RemoveOwner":       func(f *adminFixture, c string) error { return f.svc.RemoveOwner(ctx(), c, ws1, target) },
		"RemoveMember(owner)": func(f *adminFixture, c string) error {
			f.read.grants[grant{c, mgmt1, ngac.OpInvite}] = true
			return f.svc.RemoveMember(ctx(), c, ws1, target)
		},
	} {
		// manager holds manage (and, for the last case, invite) on Mgmt but is not in the Owners UA.
		f := newAdminFixture(t)
		err := call(f, manager)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name)
		assert.Empty(t, f.write.mutations, name)

		f = newAdminFixture(t)
		require.NoError(t, call(f, owner), name)
	}
}

func TestTransferOwnership_NewOwnerMustAlreadyBelong(t *testing.T) {
	for name, id := range map[string]string{"another workspace": otherOwner, "no workspace": outsider, "unknown": "nope", "a role": roleRead} {
		f := newAdminFixture(t)
		err := f.svc.TransferOwnership(ctx(), owner, ws1, id)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		assert.Empty(t, f.write.mutations, name)
	}
}

// Two owners removing each other at once must not both succeed: each sees two
// owners, so without one lock around check and removal both pass the last-owner
// rule and the workspace is left with none.
func TestRemoveOwner_ConcurrentRemovalsLeaveAnOwner(t *testing.T) {
	for round := 0; round < 20; round++ {
		f := newAdminFixture(t)
		// owner and target are the two owners; target may also remove, as an Owner with manage.
		f.read.grants[grant{target, mgmt1, ngac.OpManage}] = true
		state := &ownersState{owners: map[string]bool{owner: true, target: true}}
		write := &liveOwnersWrite{fakePolicyWrite: f.write, st: state, ownersUA: owners1}
		svc := domain.NewService(f.wsStore, f.depts, f.read, write, nil, nil).WithDirectory(f.dir)

		var wg sync.WaitGroup
		errs := make([]error, 2)
		wg.Add(2)
		go func() { defer wg.Done(); errs[0] = svc.RemoveOwner(ctx(), owner, ws1, target) }()
		go func() { defer wg.Done(); errs[1] = svc.RemoveOwner(ctx(), target, ws1, owner) }()
		wg.Wait()

		state.mu.Lock()
		left := len(state.owners)
		state.mu.Unlock()
		assert.Equal(t, 1, left, "round %d: exactly one owner remains (errs %v)", round, errs)
	}
}

type ownersState struct {
	mu     sync.Mutex
	owners map[string]bool
}

// liveOwnersWrite answers the Owners UA's children from live state, and takes
// its time doing it so another caller can slip in between a check and its action.
type liveOwnersWrite struct {
	*fakePolicyWrite
	st       *ownersState
	ownersUA string
}

func (w *liveOwnersWrite) GetChildren(c context.Context, req *policypb.GetChildrenRequest, o ...grpc.CallOption) (*policypb.NodeList, error) {
	if req.NodeId != w.ownersUA {
		return w.fakePolicyWrite.GetChildren(c, req, o...)
	}
	w.st.mu.Lock()
	var nodes []*policypb.NGACNode
	for id := range w.st.owners {
		nodes = append(nodes, node(id, id, ngac.TypeU))
	}
	w.st.mu.Unlock()
	time.Sleep(2 * time.Millisecond)
	return &policypb.NodeList{Nodes: nodes}, nil
}

func (w *liveOwnersWrite) RemoveAssignment(c context.Context, req *policypb.RemoveAssignmentRequest, o ...grpc.CallOption) (*policypb.Empty, error) {
	if req.ParentId == w.ownersUA {
		w.st.mu.Lock()
		delete(w.st.owners, req.ChildId)
		w.st.mu.Unlock()
	}
	return w.fakePolicyWrite.RemoveAssignment(c, req, o...)
}

// ---------------------------------------------------------------------------
// Delegation follows inheritance: a sub-department confers its parents' grants
// ---------------------------------------------------------------------------

func withParentGrant(f *adminFixture) {
	// dept-root-1 grants write on Documents; dept-child-1 sits under it and
	// has no grant of its own, so its people inherit the parent's.
	f.read.assocs["ua-dept-root-1"] = []*policypb.Association{{Id: "as-p", UaId: "ua-dept-root-1", OaId: docs1, Operations: []string{ngac.OpWrite}}}
	f.read.ancestors["ua-dept-child-1"] = []*policypb.NGACNode{node("ua-dept-root-1", "root", ngac.TypeUA), node(pc1, "pc", ngac.TypePC)}
	f.read.ancestors["ua-dept-root-1"] = []*policypb.NGACNode{node(pc1, "pc", ngac.TypePC)}
}

func TestUpdateMemberDepartment_InheritedGrantsAreDelegated(t *testing.T) {
	f := newAdminFixture(t)
	withParentGrant(f)
	// delegator holds only read on Documents: joining the sub-department would hand them write.
	err := f.svc.UpdateMemberDepartment(ctx(), delegator, ws1, delegator, "dept-child-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, f.mutated())

	f = newAdminFixture(t)
	withParentGrant(f)
	require.NoError(t, f.svc.UpdateMemberDepartment(ctx(), owner, ws1, target, "dept-child-1"))
}

func TestMoveDepartment_UnderAParentIsADelegationOfItsWholeChain(t *testing.T) {
	// Moving dept-other-1 (which may contain the caller) under dept-child-1 puts
	// its people under dept-root-1 and dept-child-1: everything either grants.
	f := newAdminFixture(t)
	withParentGrant(f)
	_, err := f.svc.MoveDepartment(ctx(), delegator, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-other-1", NewParentID: "dept-child-1"})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, f.mutated(), "nothing is detached or attached: %v", f.write.mutations)

	f = newAdminFixture(t)
	withParentGrant(f)
	_, err = f.svc.MoveDepartment(ctx(), owner, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-other-1", NewParentID: "dept-child-1"})
	require.NoError(t, err)
}

func TestMoveDepartment_ToTheRootGrantsNothingNew(t *testing.T) {
	f := newAdminFixture(t)
	withParentGrant(f)
	_, err := f.svc.MoveDepartment(ctx(), manager, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-child-1"})
	require.NoError(t, err)
}

func TestDelegation_UnreadableAncestorsAreNotAssumedEmpty(t *testing.T) {
	f := newAdminFixture(t)
	f.read.ancErr = errors.New("policy down")
	err := f.svc.AssignMemberRole(ctx(), owner, ws1, target, roleRead)
	require.Error(t, err)
	assert.Empty(t, f.write.mutations)
}

// Which operations a role "already has" is read from the writer, not from a
// replica that may be behind: otherwise an operation the replica still shows
// would be treated as kept and skip the check that the caller holds it.
func TestSetRolePermissions_KeptOperationsAreReadFromTheWriter(t *testing.T) {
	f := newAdminFixture(t)
	// The replica still shows write (it has not applied a narrowing yet); the writer has read only.
	f.read.assocs[roleRead] = []*policypb.Association{{Id: "as-1", UaId: roleRead, OaId: docs1, Operations: []string{ngac.OpRead, ngac.OpWrite}}}
	f.write.writerAssocs = map[string][]*policypb.Association{
		roleRead: {{Id: "as-1", UaId: roleRead, OaId: docs1, Operations: []string{ngac.OpRead}}},
	}
	// delegator holds read on Documents, not write: putting write back is an addition.
	_, err := f.svc.SetRolePermissions(ctx(), delegator, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.assocs)
}

// ---------------------------------------------------------------------------
// Head counts and the people listed come from the same place: the graph
// ---------------------------------------------------------------------------

func TestListDepartments_CountsPeopleFromTheGraphLikeTheTable(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children["ua-dept-root-1"] = []*policypb.NGACNode{node(target, "target", ngac.TypeU), node(leaver, "leaver", ngac.TypeU), node("ua-dept-child-1", "c", ngac.TypeUA)}
	depts, err := f.svc.ListDepartments(ctx(), member, ws1)
	require.NoError(t, err)
	counts := map[string]int{}
	for _, d := range depts {
		counts[d.ID] = d.MemberCount
	}
	assert.Equal(t, 2, counts["dept-root-1"], "two people, not the sub-department node")
	assert.Equal(t, 0, counts["dept-other-1"])

	// The table lists exactly those people in that department.
	people, err := f.svc.ListMemberDirectory(ctx(), owner, ws1)
	require.NoError(t, err)
	n := 0
	for _, p := range people {
		if p.Department != nil && p.Department.ID == "dept-root-1" {
			n++
		}
	}
	assert.Equal(t, counts["dept-root-1"], n)
}

func TestListDepartments_DoesNotAnswerWhenThePeopleCannotBeRead(t *testing.T) {
	f := newAdminFixture(t)
	f.read.ancErr = errors.New("policy down")
	_, err := f.svc.ListDepartments(ctx(), member, ws1)
	assert.Error(t, err)
}
