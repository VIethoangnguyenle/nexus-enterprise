package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
)

// A read replica can lag the writer. Every graph read that feeds an
// authorization or write decision must come from the writer, so a replica that
// still shows the old graph changes nothing. In these tests the read side is
// stale and the writer is right.

func TestStaleReader_OwnerRecheckUsesTheWritersOwners(t *testing.T) {
	f := newAdminFixture(t)
	// The replica still lists two owners; the writer knows target was removed.
	f.read.children[owners1] = []*policypb.NGACNode{node(owner, owner, ngac.TypeU), node(target, target, ngac.TypeU)}
	f.write.writerChildren = map[string][]*policypb.NGACNode{owners1: {node(owner, owner, ngac.TypeU)}}

	err := f.svc.RemoveOwner(ctx(), owner, ws1, owner)
	require.Error(t, err, "the last owner stays, whatever the replica says")
	assert.Empty(t, f.write.mutations)

	err = f.svc.RemoveOwner(ctx(), owner, ws1, target)
	require.Error(t, err, "target is no longer an owner")
	assert.Empty(t, f.write.mutations)
}

func TestStaleReader_DelegationUsesTheWritersGrants(t *testing.T) {
	f := newAdminFixture(t)
	// The replica shows roleRead as granting nothing; the writer has since given it write on Documents.
	f.read.assocs[roleRead] = nil
	f.write.writerAssocs = map[string][]*policypb.Association{
		roleRead: {{Id: "as-w", UaId: roleRead, OaId: docs1, Operations: []string{ngac.OpWrite}}},
	}
	// delegator holds only read on Documents.
	err := f.svc.AssignMemberRole(ctx(), delegator, ws1, target, roleRead)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)
}

func TestStaleReader_DelegationUsesTheWritersAncestry(t *testing.T) {
	f := newAdminFixture(t)
	withParentGrant(f)
	// The replica has not seen the department move under dept-root-1 yet.
	f.read.ancestors["ua-dept-child-1"] = []*policypb.NGACNode{node(pc1, "pc", ngac.TypePC)}
	f.write.writerAncestors = map[string][]*policypb.NGACNode{
		"ua-dept-child-1": {node("ua-dept-root-1", "root", ngac.TypeUA), node(pc1, "pc", ngac.TypePC)},
	}
	err := f.svc.UpdateMemberDepartment(ctx(), delegator, ws1, delegator, "dept-child-1")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, f.mutated())
}

func TestStaleReader_MembershipUsesTheWritersAncestry(t *testing.T) {
	f := newAdminFixture(t)
	// The replica still shows member inside the workspace; the writer has removed them.
	f.write.writerAncestors = map[string][]*policypb.NGACNode{member: {}}
	_, err := f.svc.ListPermissionAreas(ctx(), member, ws1)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestStaleReader_WorkspaceScopeUsesTheWritersNodes(t *testing.T) {
	f := newAdminFixture(t)
	// The replica still lists role1 under the workspace; the writer has deleted it.
	f.write.writerDescendants = map[string][]*policypb.NGACNode{pc1: {}}
	err := f.svc.UnassignMemberRole(ctx(), owner, ws1, target, role1)
	require.Error(t, err)
	assert.Empty(t, f.write.mutations)
}
