package store_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestUpdateDetails_ChangesOnlyThatWorkspace(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	a, _ := testutil.CreateWorkspace(t, pool, owner)
	b, _ := testutil.CreateWorkspace(t, pool, owner)

	name, desc := "Khối Vận hành", "Đối soát và thanh toán"
	gotName, gotDesc, err := st.UpdateDetails(ctx, a, &name, &desc)
	require.NoError(t, err)
	assert.Equal(t, name, gotName)
	assert.Equal(t, desc, gotDesc)

	got, err := st.GetByID(ctx, a)
	require.NoError(t, err)
	assert.Equal(t, "Khối Vận hành", got.Name)
	assert.Equal(t, "Đối soát và thanh toán", got.Desc)
	other, err := st.GetByID(ctx, b)
	require.NoError(t, err)
	assert.NotEqual(t, "Khối Vận hành", other.Name)

	_, _, err = st.UpdateDetails(ctx, "no-such-workspace", &name, nil)
	assert.Error(t, err)
}

func TestUpdateDetails_ConcurrentEditsOfDifferentFieldsBothLand(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)

	for i := 0; i < 20; i++ {
		name, desc := fmt.Sprintf("Tên %d", i), fmt.Sprintf("Mô tả %d", i)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _, err := st.UpdateDetails(ctx, ws, &name, nil); assert.NoError(t, err) }()
		go func() { defer wg.Done(); _, _, err := st.UpdateDetails(ctx, ws, nil, &desc); assert.NoError(t, err) }()
		wg.Wait()
		got, err := st.GetByID(ctx, ws)
		require.NoError(t, err)
		require.Equal(t, name, got.Name, "round %d", i)
		require.Equal(t, desc, got.Desc, "round %d", i)
	}
}
