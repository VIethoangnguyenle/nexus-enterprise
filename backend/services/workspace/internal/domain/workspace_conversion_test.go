package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
)

func personalFixture(t *testing.T) *adminFixture {
	t.Helper()
	f := newAdminFixture(t)
	f.wsStore.ws[ws1].Type = domain.TypePersonal
	return f
}

func TestConvertToOrganization_TheOwnerConvertsAndItIsRecordedAndAnnounced(t *testing.T) {
	f := personalFixture(t)
	p := withProbe(f)

	got, err := f.svc.ConvertToOrganization(context.Background(), owner, ws1)

	require.NoError(t, err)
	assert.Equal(t, "Lê Thị Hoa", got.ConvertedByName)
	assert.False(t, got.ConvertedAt.IsZero())
	assert.Equal(t, domain.TypeOrganization, f.wsStore.ws[ws1].Type)
	assert.NotEqual(t, domain.TypeOrganization, f.wsStore.ws[ws2].Type, "another workspace is untouched")
	require.Equal(t, []announced{{realtime.DomainWorkspace, realtime.KindUpdated, []string{ws1}}}, kinds(p))
	assert.Equal(t, []int{1}, p.seen, "announced after the change was written")

	d, err := f.svc.WorkspaceDetails(context.Background(), owner, ws1)
	require.NoError(t, err)
	assert.Equal(t, domain.TypeOrganization, d.Type)
	assert.True(t, d.IsOwner)
	assert.Equal(t, "Lê Thị Hoa, Nguyễn Thu Lan", d.OwnerName)
	assert.Equal(t, "Lê Thị Hoa", d.ConvertedByName)
	require.NotNil(t, d.ConvertedAt)

	_, err = f.svc.ConvertToOrganization(context.Background(), owner, ws1)
	assert.ErrorIs(t, err, domain.ErrAlreadyOrganization, "one way, and once")
}

func TestConvertToOrganization_OnlyTheOwnerWhoManages(t *testing.T) {
	cases := map[string]string{
		"manages but is not the owner": manager,
		"plain member":                 member,
		"inviter":                      inviter,
		"owner of another workspace":   otherOwner,
		"outsider":                     outsider,
		"no caller":                    "",
	}
	for name, caller := range cases {
		t.Run(name, func(t *testing.T) {
			f := personalFixture(t)
			p := withProbe(f)

			_, err := f.svc.ConvertToOrganization(context.Background(), caller, ws1)

			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrAccessDenied)
			assert.Equal(t, domain.TypePersonal, f.wsStore.ws[ws1].Type)
			assert.False(t, f.mutated())
			assert.Empty(t, p.Events())
		})
	}
}

func TestConvertToOrganization_AnOwnerWhoManagesConvertsEvenIfNotTheCreator(t *testing.T) {
	f := personalFixture(t)
	// target was made an owner later (the Owners group holds owner and target).
	f.read.grants[grant{target, mgmt1, "manage"}] = true

	_, err := f.svc.ConvertToOrganization(context.Background(), target, ws1)

	require.NoError(t, err)
	assert.Equal(t, domain.TypeOrganization, f.wsStore.ws[ws1].Type)
}

func TestConvertToOrganization_AnOwnerWithoutManageIsRefused(t *testing.T) {
	f := personalFixture(t)

	_, err := f.svc.ConvertToOrganization(context.Background(), target, ws1) // owner, no manage grant

	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, f.mutated())
}

func TestConvertToOrganization_UnreadableOwnersRefuse(t *testing.T) {
	f := personalFixture(t)
	f.write.writerChildren = map[string][]*policypb.NGACNode{pc1: nil} // no Owners group to be found

	_, err := f.svc.ConvertToOrganization(context.Background(), owner, ws1)

	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, f.mutated())
}

// Who the owners are is read, and the type changed, while the owner lock is held,
// so a conversion cannot interleave with an owner being removed.
func TestConvertToOrganization_DecidedUnderTheOwnerLock(t *testing.T) {
	f := personalFixture(t)
	var ownersReadLocked []bool
	f.write.onChildren = func(node string) {
		if node == owners1 {
			ownersReadLocked = append(ownersReadLocked, f.wsStore.locked)
		}
	}

	_, err := f.svc.ConvertToOrganization(context.Background(), owner, ws1)

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, f.wsStore.convertedLocked, "written under the lock")
	require.NotEmpty(t, ownersReadLocked)
	assert.Equal(t, []bool{true}, ownersReadLocked[:1], "owners read under the lock")
}

func TestConvertToOrganization_AnOwnerRemovedBeforeTheLockIsRefused(t *testing.T) {
	f := personalFixture(t)
	// The owner is removed between the permission check and the lock being taken.
	f.write.writerChildren = map[string][]*policypb.NGACNode{}
	f.wsStore.beforeLock = func() {
		f.write.writerChildren[owners1] = []*policypb.NGACNode{node(target, "target", "U")}
	}

	_, err := f.svc.ConvertToOrganization(context.Background(), owner, ws1)

	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, domain.TypePersonal, f.wsStore.ws[ws1].Type)
}

func TestConvertToOrganization_AlreadyAnOrganizationIsAConflict(t *testing.T) {
	f := newAdminFixture(t)
	f.wsStore.ws[ws1].Type = domain.TypeOrganization
	p := withProbe(f)

	_, err := f.svc.ConvertToOrganization(context.Background(), owner, ws1)

	assert.ErrorIs(t, err, domain.ErrAlreadyOrganization)
	assert.False(t, f.mutated())
	assert.Empty(t, p.Events())

	// Someone who may not convert is told so, not that it is an organization.
	_, err = f.svc.ConvertToOrganization(context.Background(), manager, ws1)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestConvertToOrganization_UnknownWorkspaceIsNotFound(t *testing.T) {
	f := personalFixture(t)

	_, err := f.svc.ConvertToOrganization(context.Background(), owner, "no-such-workspace")

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestConvertToOrganization_LosingTheRaceIsAConflict(t *testing.T) {
	f := personalFixture(t)
	// The row was personal when read, and converted by someone else before the write.
	f.wsStore.ws[ws1].Type = domain.TypePersonal
	racing := &racingStore{fakeWSStore: f.wsStore}
	f.svc = domain.NewService(racing, f.depts, f.read, f.write, nil, nil).WithDirectory(f.dir)

	_, err := f.svc.ConvertToOrganization(context.Background(), owner, ws1)

	assert.ErrorIs(t, err, domain.ErrAlreadyOrganization)
}

// racingStore lets another conversion land between the domain's read and write.
type racingStore struct{ *fakeWSStore }

func (r *racingStore) ConvertToOrganization(ctx context.Context, id, by string) (*store.Conversion, error) {
	if _, err := r.fakeWSStore.ConvertToOrganization(ctx, id, by); err != nil {
		return nil, err
	}
	return r.fakeWSStore.ConvertToOrganization(ctx, id, by)
}

func TestWorkspaceDetails_TellsTypeOwnerAndConversion(t *testing.T) {
	f := personalFixture(t)

	d, err := f.svc.WorkspaceDetails(context.Background(), member, ws1)
	require.NoError(t, err)
	assert.Equal(t, domain.TypePersonal, d.Type)
	assert.False(t, d.IsOwner)
	assert.Equal(t, "Lê Thị Hoa, Nguyễn Thu Lan", d.OwnerName, "a name, never an id")
	assert.Empty(t, d.ConvertedByName)
	assert.Nil(t, d.ConvertedAt)
	assert.Equal(t, "Acme", d.Name, "existing fields stay")

	d, err = f.svc.WorkspaceDetails(context.Background(), owner, ws1)
	require.NoError(t, err)
	assert.True(t, d.IsOwner)
}

func TestUpdateWorkspaceDetails_AnswersWithTypeAndOwnerToo(t *testing.T) {
	f := personalFixture(t)

	d, err := f.svc.UpdateWorkspaceDetails(context.Background(), owner, ws1, ptr("Mới"), nil)

	require.NoError(t, err)
	assert.Equal(t, domain.TypePersonal, d.Type)
	assert.True(t, d.IsOwner)
	assert.Equal(t, "Lê Thị Hoa, Nguyễn Thu Lan", d.OwnerName)
}
