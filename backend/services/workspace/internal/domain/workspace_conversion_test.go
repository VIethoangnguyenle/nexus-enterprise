package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/domain"
)

func personalFixture(t *testing.T) *adminFixture {
	t.Helper()
	f := newAdminFixture(t)
	f.wsStore.ws[ws1].Type = domain.TypePersonal
	return f
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
