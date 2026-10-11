package store_test

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/approval/internal/domain"
)

func TestWorkspaceType_ReadsTheWorkspacesType(t *testing.T) {
	ctx, st, pool := scratchTenant(t)
	tenant := grpcauth.CallerFrom(ctx).TenantID

	got, err := st.WorkspaceType(ctx, tenant)
	if err != nil || got != "organization" {
		t.Fatalf("WorkspaceType = %q, %v; want organization", got, err)
	}

	if _, err := pool.Exec(context.Background(), `UPDATE workspaces SET type = 'personal' WHERE id = $1`, tenant); err != nil {
		t.Fatal(err)
	}
	if got, err := st.WorkspaceType(ctx, tenant); err != nil || got != "personal" {
		t.Fatalf("after the change WorkspaceType = %q, %v; want personal", got, err)
	}
}

// A workspace that is not there is ErrNotFound, so the guard can refuse it
// instead of reading the absence as an organization.
func TestWorkspaceType_UnknownWorkspaceIsNotFound(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	for _, id := range []string{"no-such-workspace", ""} {
		if _, err := st.WorkspaceType(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("WorkspaceType(%q) error = %v; want ErrNotFound", id, err)
		}
	}
}

// The type is read from the shared table, so a workspace with no tenant schema
// (a personal one) is answered too.
func TestWorkspaceType_NeedsNoTenantSchema(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	bare := grpcauth.WithCaller(context.Background(), grpcauth.CallerFrom(ctx))
	if _, err := st.WorkspaceType(bare, grpcauth.CallerFrom(ctx).TenantID); err != nil {
		t.Fatalf("WorkspaceType without a tenant schema on the context: %v", err)
	}
}
