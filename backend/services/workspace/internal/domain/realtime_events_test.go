package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
)

// probe records events and, at the instant each is emitted, how much had been
// written, so a test can tell an announcement made after the change from one
// made before it.
type probe struct {
	realtime.Recorder
	written func() int
	seen    []int
}

func (p *probe) Emit(e realtime.Event) {
	p.seen = append(p.seen, p.written())
	p.Recorder.Emit(e)
}

func withProbe(f *adminFixture) *probe {
	p := &probe{written: func() int {
		return len(f.write.mutations) + len(f.depts.mutations) + len(f.wsStore.mutations) + len(f.dir.removed)
	}}
	f.svc = f.svc.WithEmitter(p)
	return p
}

type announced struct {
	domain, kind string
	ids          []string
}

func kinds(p *probe) []announced {
	var out []announced
	for _, e := range p.Events() {
		out = append(out, announced{e.Domain, e.Kind, e.IDs})
	}
	return out
}

func TestWorkspaceChanges_AnnounceOnceAfterTheyAreWritten(t *testing.T) {
	cases := []struct {
		name string
		do   func(f *adminFixture) error
		want []announced
	}{
		{"remove member", func(f *adminFixture) error {
			f.read.grants[grant{owner, mgmt1, ngac.OpInvite}] = true
			return f.svc.RemoveMember(ctx(), owner, ws1, target)
		}, []announced{
			{realtime.DomainWorkspace, realtime.KindMemberRemoved, []string{target}},
			{realtime.DomainPermission, realtime.KindChanged, nil},
		}},
		{"leave", func(f *adminFixture) error { return f.svc.LeaveWorkspace(ctx(), member, ws1) },
			[]announced{{realtime.DomainWorkspace, realtime.KindMemberRemoved, []string{member}}}},
		{"assign role", func(f *adminFixture) error { return f.svc.AssignMemberRole(ctx(), owner, ws1, target, roleRead) }, []announced{
			{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{target, roleRead}},
			{realtime.DomainPermission, realtime.KindChanged, nil},
		}},
		{"unassign role", func(f *adminFixture) error { return f.svc.UnassignMemberRole(ctx(), manager, ws1, target, roleRead) }, []announced{
			{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{target, roleRead}},
			{realtime.DomainPermission, realtime.KindChanged, nil},
		}},
		{"change role grants", func(f *adminFixture) error {
			_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite})
			return err
		}, []announced{{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{roleRead}}}},
		{"transfer ownership", func(f *adminFixture) error { return f.svc.TransferOwnership(ctx(), owner, ws1, member) }, []announced{
			{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{member}},
			{realtime.DomainPermission, realtime.KindChanged, nil},
		}},
		{"remove owner", func(f *adminFixture) error { return f.svc.RemoveOwner(ctx(), owner, ws1, target) }, []announced{
			{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{target}},
			{realtime.DomainPermission, realtime.KindChanged, nil},
		}},
		{"delete role", func(f *adminFixture) error { return f.svc.DeleteRole(ctx(), owner, ws1, role1) },
			[]announced{{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{role1}}}},
		{"member department", func(f *adminFixture) error {
			return f.svc.UpdateMemberDepartment(ctx(), manager, ws1, target, "dept-root-1")
		}, []announced{
			{realtime.DomainWorkspace, realtime.KindDepartmentChanged, []string{"dept-root-1", target}},
			{realtime.DomainPermission, realtime.KindChanged, nil},
		}},
		{"rename department", func(f *adminFixture) error {
			_, err := f.svc.UpdateDepartment(ctx(), owner, ws1, "dept-root-1", "Kinh doanh")
			return err
		}, []announced{{realtime.DomainWorkspace, realtime.KindDepartmentChanged, []string{"dept-root-1"}}}},
		{"move department", func(f *adminFixture) error {
			_, err := f.svc.MoveDepartment(ctx(), manager, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-child-1"})
			return err
		}, []announced{{realtime.DomainWorkspace, realtime.KindDepartmentChanged, []string{"dept-child-1"}}}},
		{"delete department", func(f *adminFixture) error { return f.svc.DeleteDepartment(ctx(), owner, ws1, "dept-child-1") },
			[]announced{{realtime.DomainWorkspace, realtime.KindDepartmentChanged, []string{"dept-child-1"}}}},
		{"workspace details", func(f *adminFixture) error {
			_, err := f.svc.UpdateWorkspaceDetails(ctx(), manager, ws1, ptr("Khối mới"), nil)
			return err
		}, []announced{{realtime.DomainWorkspace, realtime.KindUpdated, []string{ws1}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAdminFixture(t)
			p := withProbe(f)

			require.NoError(t, tc.do(f))

			got := kinds(p)
			require.Len(t, got, len(tc.want))
			for i, w := range tc.want {
				assert.Equal(t, w.domain, got[i].domain)
				assert.Equal(t, w.kind, got[i].kind)
				if w.ids != nil {
					assert.Equal(t, w.ids, got[i].ids)
				}
			}
			for _, e := range p.Events() {
				assert.Equal(t, ws1, e.WorkspaceID)
				assert.Equal(t, ws1, e.TenantID, "the audience is the workspace's own tenant")
				assert.NoError(t, e.Validate())
				if e.Domain == realtime.DomainPermission {
					assert.NotEmpty(t, e.UserNodeIDs, "the permission notice names the person")
				}
			}
			for i, n := range p.seen {
				assert.Positive(t, n, "event %d was announced before anything was written", i)
			}
		})
	}
}

func TestWorkspaceChanges_ActorComesFromTheVerifiedCaller(t *testing.T) {
	f := newAdminFixture(t)
	p := withProbe(f)
	c := grpcauth.WithCaller(ctx(), grpcauth.Caller{UserID: "user-owner", NGACNodeID: owner, TenantID: "some-other-tenant"})

	require.NoError(t, f.svc.AssignMemberRole(c, owner, ws1, target, roleRead))

	require.NotEmpty(t, p.Events())
	for _, e := range p.Events() {
		assert.Equal(t, "user-owner", e.ActorUserID)
		assert.Equal(t, ws1, e.TenantID, "not the tenant of the token the admin happens to hold")
	}
}

func TestWorkspaceChanges_NoGrantChangeAnnouncesNothing(t *testing.T) {
	f := newAdminFixture(t)
	p := withProbe(f)
	// roleRead already holds exactly read on Documents.
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead})
	require.NoError(t, err)
	assert.Empty(t, p.Events())
}

func TestWorkspaceChanges_RefusedOrFailedAnnounceNothing(t *testing.T) {
	denied := map[string]func(f *adminFixture, caller string) error{
		"RemoveMember":     func(f *adminFixture, c string) error { return f.svc.RemoveMember(ctx(), c, ws1, target) },
		"AssignMemberRole": func(f *adminFixture, c string) error { return f.svc.AssignMemberRole(ctx(), c, ws1, target, roleRead) },
		"UnassignMemberRole": func(f *adminFixture, c string) error {
			return f.svc.UnassignMemberRole(ctx(), c, ws1, target, roleRead)
		},
		"SetRolePermissions": func(f *adminFixture, c string) error {
			_, err := f.svc.SetRolePermissions(ctx(), c, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite})
			return err
		},
		"TransferOwnership": func(f *adminFixture, c string) error { return f.svc.TransferOwnership(ctx(), c, ws1, member) },
		"RemoveOwner":       func(f *adminFixture, c string) error { return f.svc.RemoveOwner(ctx(), c, ws1, target) },
		"DeleteRole":        func(f *adminFixture, c string) error { return f.svc.DeleteRole(ctx(), c, ws1, role1) },
		"CreateRole": func(f *adminFixture, c string) error {
			_, err := f.svc.CreateRole(ctx(), c, ws1, "Reviewer")
			return err
		},
		"UpdateMemberDepartment": func(f *adminFixture, c string) error {
			return f.svc.UpdateMemberDepartment(ctx(), c, ws1, target, "dept-root-1")
		},
		"CreateDepartment": func(f *adminFixture, c string) error {
			_, err := f.svc.CreateDepartment(ctx(), c, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Mới"})
			return err
		},
		"UpdateDepartment": func(f *adminFixture, c string) error {
			_, err := f.svc.UpdateDepartment(ctx(), c, ws1, "dept-root-1", "Tên mới")
			return err
		},
		"MoveDepartment": func(f *adminFixture, c string) error {
			_, err := f.svc.MoveDepartment(ctx(), c, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-child-1"})
			return err
		},
		"DeleteDepartment": func(f *adminFixture, c string) error { return f.svc.DeleteDepartment(ctx(), c, ws1, "dept-child-1") },
		"UpdateWorkspaceDetails": func(f *adminFixture, c string) error {
			_, err := f.svc.UpdateWorkspaceDetails(ctx(), c, ws1, ptr("Tên khác"), nil)
			return err
		},
		"LeaveWorkspace": func(f *adminFixture, c string) error { return f.svc.LeaveWorkspace(ctx(), c, ws1) },
	}
	for name, call := range denied {
		for _, caller := range []string{outsider, otherOwner, ""} {
			f := newAdminFixture(t)
			p := withProbe(f)
			err := call(f, caller)
			require.Error(t, err, "%s as %q", name, caller)
			assert.Empty(t, p.Events(), "%s refused to %q must announce nothing", name, caller)
		}
	}

	t.Run("not found and invalid input", func(t *testing.T) {
		f := newAdminFixture(t)
		p := withProbe(f)
		assert.Error(t, f.svc.AssignMemberRole(ctx(), owner, ws1, "nope", roleRead))
		assert.Error(t, f.svc.DeleteRole(ctx(), owner, ws1, "nope"))
		assert.Error(t, f.svc.UpdateMemberDepartment(ctx(), owner, ws1, target, "no-such-dept"))
		_, err := f.svc.UpdateWorkspaceDetails(ctx(), manager, ws1, ptr("   "), nil)
		assert.Error(t, err)
		_, err = f.svc.UpdateDepartment(ctx(), owner, ws1, "dept-root-1", "")
		assert.Error(t, err)
		assert.Empty(t, p.Events())
	})

	t.Run("the last owner stays", func(t *testing.T) {
		f := newAdminFixture(t)
		f.read.children[owners1] = []*policypb.NGACNode{node(owner, "owner", ngac.TypeU)}
		p := withProbe(f)
		assert.ErrorIs(t, f.svc.LeaveWorkspace(ctx(), owner, ws1), domain.ErrLastOwner)
		assert.Error(t, f.svc.RemoveOwner(ctx(), owner, ws1, owner))
		assert.Empty(t, p.Events())
	})

	t.Run("a policy write that fails", func(t *testing.T) {
		f := newAdminFixture(t)
		failing := &failingAssign{fakePolicyWrite: f.write}
		svc := domain.NewService(f.wsStore, f.depts, f.read, failing, nil, nil).WithDirectory(f.dir)
		p := &probe{written: func() int { return 1 }}
		svc = svc.WithEmitter(p)
		assert.Error(t, svc.AssignMemberRole(ctx(), owner, ws1, target, roleRead))
		assert.Error(t, svc.DeleteDepartment(ctx(), owner, ws1, "dept-root-1"))
		assert.Empty(t, p.Events())
	})

	t.Run("a department move that is undone", func(t *testing.T) {
		f := newAdminFixture(t)
		f.read.parents[target] = []*policypb.NGACNode{node("ua-dept-other-1", "d", ngac.TypeUA)}
		failing := &failingRemove{fakePolicyWrite: f.write, ua: "ua-dept-other-1"}
		p := &probe{written: func() int { return 1 }}
		svc := domain.NewService(f.wsStore, f.depts, f.read, failing, nil, nil).WithDirectory(f.dir).WithEmitter(p)
		require.Error(t, svc.UpdateMemberDepartment(ctx(), owner, ws1, target, "dept-root-1"))
		assert.Empty(t, p.Events(), "the new assignment was taken back; nobody changed department")
	})
}

func TestCreateWorkspaceEntities_AnnounceAfterProvisioning(t *testing.T) {
	f := newAdminFixture(t)
	p := withProbe(f)

	role, err := f.svc.CreateRole(ctx(), owner, ws1, "Reviewer")
	require.NoError(t, err)
	dept, err := f.svc.CreateDepartment(ctx(), owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Mới"})
	require.NoError(t, err)

	got := kinds(p)
	require.Len(t, got, 2)
	assert.Equal(t, announced{realtime.DomainWorkspace, realtime.KindRoleChanged, []string{role.ID}}, got[0])
	assert.Equal(t, announced{realtime.DomainWorkspace, realtime.KindDepartmentChanged, []string{dept.ID}}, got[1])
	for _, n := range p.seen {
		assert.Positive(t, n)
	}
}

func TestCreateRole_FailedProvisioningAnnouncesNothing(t *testing.T) {
	f := newAdminFixture(t)
	p := &probe{written: func() int { return 1 }}
	svc := domain.NewService(f.wsStore, f.depts, f.read, &failingAssign{fakePolicyWrite: f.write}, nil, nil).WithEmitter(p)
	_, err := svc.CreateRole(ctx(), owner, ws1, "Reviewer")
	require.Error(t, err)
	assert.Empty(t, p.Events())
}

func TestAcceptInvitation_AnnouncesTheNewMemberOnce(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{}) // before the probe: inviting announces too
	p := withProbe(f.adminFixture)

	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err)

	assert.Equal(t, []announced{{realtime.DomainWorkspace, realtime.KindInvitationAccepted, []string{newbie}}}, kinds(p))
	assert.Positive(t, p.seen[0])
}

func TestAcceptInvitation_RefusedOrUndoneAnnouncesNothing(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{}) // before the probe: inviting announces too
	p := withProbe(f.adminFixture)

	_, err := f.svc.AcceptInvitation(ctx(), "someone-else", newbie, id)
	require.Error(t, err)
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, "", id)
	require.Error(t, err)

	f.dir.ensure = errors.New("directory down") // joining is undone
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.Error(t, err)

	assert.Empty(t, p.Events())
}

func TestServiceWithoutAnEmitterStillWorks(t *testing.T) {
	f := newAdminFixture(t)
	require.NoError(t, f.svc.AssignMemberRole(context.Background(), owner, ws1, target, roleRead))
}
