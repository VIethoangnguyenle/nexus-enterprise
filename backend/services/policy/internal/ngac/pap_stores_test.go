package ngac_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

func TestOperationStore_RegisterListExistsValidate(t *testing.T) {
	_, pool := setupStore(t)
	ctx := context.Background()
	ops := ngac.NewOperationStore(pool)
	known := "op_" + uuid.NewString()[:8]
	unknown := "op_" + uuid.NewString()[:8]
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_operations WHERE name = $1", known) })

	empty, err := ops.Register(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty.Registered)

	res, err := ops.Register(ctx, []string{known})
	require.NoError(t, err)
	assert.Equal(t, []string{known}, res.Registered)

	ok, err := ops.Exists(ctx, known)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = ops.Exists(ctx, unknown)
	require.NoError(t, err)
	assert.False(t, ok)

	list, err := ops.List(ctx)
	require.NoError(t, err)
	assert.Contains(t, list, known)

	invalid, err := ops.ValidateOperations(ctx, []string{known, unknown})
	require.NoError(t, err)
	assert.Equal(t, []string{unknown}, invalid, "only the unregistered one is reported")
	none, err := ops.ValidateOperations(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestProhibitionStore_ListFiltersBySubject(t *testing.T) {
	_, pool := setupStore(t)
	ctx := context.Background()
	ps := ngac.NewProhibitionStore(pool, nil)
	sfx := uuid.NewString()[:8]
	a, b := "p-a-"+sfx, "p-b-"+sfx
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_prohibitions WHERE name = ANY($1)", []string{a, b}) })
	for name, subj := range map[string]string{a: "subj-a-" + sfx, b: "subj-b-" + sfx} {
		_, err := ps.Create(ctx, &ngac.Prohibition{Name: name, SubjectID: subj, Operations: []string{"read"}, TargetOAIDs: []string{"oa-x"}})
		require.NoError(t, err)
	}

	only, err := ps.List(ctx, "subj-a-"+sfx)
	require.NoError(t, err)
	require.Len(t, only, 1)
	assert.Equal(t, a, only[0].Name)

	all, err := ps.List(ctx, "")
	require.NoError(t, err)
	names := map[string]bool{}
	for _, p := range all {
		names[p.Name] = true
	}
	assert.True(t, names[a] && names[b])
}
