package domain

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/asset/internal/store"
)

func settingsDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func ptrTo[T any](v T) *T { return &v }

// settingsFixture is a workspace with one active member and one outsider.
type settingsFixture struct {
	svc               *AssetService
	ws, member, other string
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, settingsDBURL())
	require.NoError(t, err)
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	sfx := uuid.New().String()[:8]
	f := &settingsFixture{ws: "aset-ws-" + sfx, member: "aset-m-" + sfx, other: "aset-o-" + sfx}
	exec := func(q string, args ...any) {
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`INSERT INTO users (id, username, password) VALUES ($1, $1, ''), ($2, $2, '')`, f.member, f.other)
	exec(`INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization')`, f.ws, f.member)
	exec(`INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2)`, f.ws, f.member)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM asset_user_settings WHERE workspace_id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM users WHERE id IN ($1, $2)`, f.member, f.other)
	})
	f.svc = NewAssetService(store.New(pool), nil, nil)
	return f
}

func (f *settingsFixture) as(userID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: userID, NGACNodeID: "n-" + userID, TenantID: f.ws})
}

// A client that sends only some settings must not reset the others.
func TestAssetSettingsSaveKeepsFieldsTheBodyLeavesOut(t *testing.T) {
	f := newSettingsFixture(t)
	ctx := f.as(f.member)
	_, err := f.svc.UpdateAssetSettings(ctx, f.member, f.ws, &AssetSettingsPatch{
		DefaultTab: ptrTo("requests"), Sort: ptrTo("status"),
		NotifyRequests: ptrTo(false), NotifyMaintenance: ptrTo(false),
	})
	require.NoError(t, err)

	got, err := f.svc.UpdateAssetSettings(ctx, f.member, f.ws, &AssetSettingsPatch{Sort: ptrTo("updated")})
	require.NoError(t, err)
	want := AssetSettings{DefaultTab: "requests", Sort: "updated"}
	assert.Equal(t, want, *got)
	stored, err := f.svc.GetAssetSettings(ctx, f.member, f.ws)
	require.NoError(t, err)
	assert.Equal(t, want, *stored)

	_, err = f.svc.UpdateAssetSettings(ctx, f.member, f.ws, &AssetSettingsPatch{})
	require.NoError(t, err)
	stored, err = f.svc.GetAssetSettings(ctx, f.member, f.ws)
	require.NoError(t, err)
	assert.Equal(t, want, *stored, "an empty body changes nothing")
}

func TestAssetSettingsFirstPartialSaveStartsFromTheDefaults(t *testing.T) {
	f := newSettingsFixture(t)
	got, err := f.svc.UpdateAssetSettings(f.as(f.member), f.member, f.ws, &AssetSettingsPatch{NotifyMaintenance: ptrTo(false)})
	require.NoError(t, err)
	want := *DefaultAssetSettings()
	want.NotifyMaintenance = false
	assert.Equal(t, want, *got)
}

func TestAssetSettingsMergedValueIsStillCheckedAndOutsidersAreDenied(t *testing.T) {
	f := newSettingsFixture(t)
	ctx := f.as(f.member)
	_, err := f.svc.UpdateAssetSettings(ctx, f.member, f.ws, &AssetSettingsPatch{DefaultTab: ptrTo("history")})
	assert.True(t, errors.Is(err, ErrInvalidInput), "got %v", err)
	_, err = f.svc.UpdateAssetSettings(ctx, f.member, f.ws, &AssetSettingsPatch{Sort: ptrTo("")})
	assert.True(t, errors.Is(err, ErrInvalidInput), "an explicit empty value is not a left-out field: %v", err)
	stored, err := f.svc.GetAssetSettings(ctx, f.member, f.ws)
	require.NoError(t, err)
	assert.Equal(t, *DefaultAssetSettings(), *stored, "a rejected save stores nothing")

	_, err = f.svc.UpdateAssetSettings(f.as(f.other), f.other, f.ws, &AssetSettingsPatch{Sort: ptrTo("name")})
	assert.True(t, errors.Is(err, ErrAccessDenied), "got %v", err)
	_, err = f.svc.UpdateAssetSettings(f.as(f.other), f.other, f.ws, &AssetSettingsPatch{})
	assert.True(t, errors.Is(err, ErrAccessDenied), "an empty body is still checked: %v", err)
}
