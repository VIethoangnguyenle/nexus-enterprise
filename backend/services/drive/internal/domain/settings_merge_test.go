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
	"ngac-platform/services/drive/internal/store"
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
	svc               *Service
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
	f := &settingsFixture{ws: "dset-ws-" + sfx, member: "dset-m-" + sfx, other: "dset-o-" + sfx}
	exec := func(q string, args ...any) {
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`INSERT INTO users (id, username, password) VALUES ($1, $1, ''), ($2, $2, '')`, f.member, f.other)
	exec(`INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization')`, f.ws, f.member)
	exec(`INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2)`, f.ws, f.member)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM drive_user_settings WHERE workspace_id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, f.ws)
		pool.Exec(c, `DELETE FROM users WHERE id IN ($1, $2)`, f.member, f.other)
	})
	f.svc = NewService(store.NewStore(pool), nil, nil, nil)
	return f
}

func (f *settingsFixture) as(userID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: userID, NGACNodeID: "n-" + userID, TenantID: f.ws})
}

// A client that sends only some settings must not reset the others.
func TestDriveSettingsSaveKeepsFieldsTheBodyLeavesOut(t *testing.T) {
	f := newSettingsFixture(t)
	ctx := f.as(f.member)
	_, err := f.svc.UpdateDriveSettings(ctx, f.member, f.ws, &DriveSettingsPatch{
		View: ptrTo("grid"), Sort: ptrTo("size"), FoldersFirst: ptrTo(false),
		CellularPreviews: ptrTo(false), NotifyShared: ptrTo(false),
	})
	require.NoError(t, err)

	got, err := f.svc.UpdateDriveSettings(ctx, f.member, f.ws, &DriveSettingsPatch{Sort: ptrTo("name")})
	require.NoError(t, err)
	want := DriveSettings{View: "grid", Sort: "name"}
	assert.Equal(t, want, *got)
	stored, err := f.svc.GetDriveSettings(ctx, f.member, f.ws)
	require.NoError(t, err)
	assert.Equal(t, want, *stored)

	_, err = f.svc.UpdateDriveSettings(f.as(f.member), f.member, f.ws, &DriveSettingsPatch{})
	require.NoError(t, err)
	stored, err = f.svc.GetDriveSettings(ctx, f.member, f.ws)
	require.NoError(t, err)
	assert.Equal(t, want, *stored, "an empty body changes nothing")
}

func TestDriveSettingsFirstPartialSaveStartsFromTheDefaults(t *testing.T) {
	f := newSettingsFixture(t)
	ctx := f.as(f.member)
	got, err := f.svc.UpdateDriveSettings(ctx, f.member, f.ws, &DriveSettingsPatch{NotifyShared: ptrTo(false)})
	require.NoError(t, err)
	want := *DefaultDriveSettings()
	want.NotifyShared = false
	assert.Equal(t, want, *got)
}

func TestDriveSettingsMergedValueIsStillCheckedAndOutsidersAreDenied(t *testing.T) {
	f := newSettingsFixture(t)
	ctx := f.as(f.member)
	_, err := f.svc.UpdateDriveSettings(ctx, f.member, f.ws, &DriveSettingsPatch{View: ptrTo("tiles")})
	assert.True(t, errors.Is(err, ErrInvalidInput), "got %v", err)
	_, err = f.svc.UpdateDriveSettings(ctx, f.member, f.ws, &DriveSettingsPatch{Sort: ptrTo("")})
	assert.True(t, errors.Is(err, ErrInvalidInput), "an explicit empty value is not a left-out field: %v", err)
	stored, err := f.svc.GetDriveSettings(ctx, f.member, f.ws)
	require.NoError(t, err)
	assert.Equal(t, *DefaultDriveSettings(), *stored, "a rejected save stores nothing")

	_, err = f.svc.UpdateDriveSettings(f.as(f.other), f.other, f.ws, &DriveSettingsPatch{Sort: ptrTo("name")})
	assert.True(t, errors.Is(err, ErrAccessDenied), "got %v", err)
	_, err = f.svc.UpdateDriveSettings(f.as(f.other), f.other, f.ws, &DriveSettingsPatch{})
	assert.True(t, errors.Is(err, ErrAccessDenied), "an empty body is still checked: %v", err)
}
