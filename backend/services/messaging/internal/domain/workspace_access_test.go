package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// withDocumentsOA gives the fixture's workspace a Documents OA row.
func (f *fixture) withDocumentsOA(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	id := fmt.Sprintf("test-docs-oa-%d", time.Now().UnixNano())
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		id, ngac.DocumentsOAName(ngac.WorkspaceID(f.wsID)), ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = $1 WHERE id = $2`, id, f.wsID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = NULL WHERE id = $1`, f.wsID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, id)
	})
	return id
}

func TestAuthorizeWorkspaceAccess(t *testing.T) {
	f := newFixture(t)
	oa := f.withDocumentsOA(t)
	ctx := context.Background()

	t.Run("read on the Documents OA allows", func(t *testing.T) {
		f.read.allow[grant{userNode, oa, ngac.OpRead}] = true
		require.NoError(t, f.svc.AuthorizeWorkspaceAccess(ctx, f.wsID, userNode, f.wsID, ngac.OpRead))
	})
	t.Run("a user without read is denied", func(t *testing.T) {
		err := f.svc.AuthorizeWorkspaceAccess(ctx, f.wsID, "ngac-user-stranger", f.wsID, ngac.OpRead)
		assert.ErrorIs(t, err, domain.ErrAccessDenied)
	})
	t.Run("a workspace outside the session tenant is denied before policy is asked", func(t *testing.T) {
		before := len(f.read.asked())
		err := f.svc.AuthorizeWorkspaceAccess(ctx, f.wsID, userNode, "some-other-tenant", ngac.OpRead)
		assert.ErrorIs(t, err, domain.ErrAccessDenied)
		assert.Len(t, f.read.asked(), before)
	})
	t.Run("an unknown workspace is denied", func(t *testing.T) {
		err := f.svc.AuthorizeWorkspaceAccess(ctx, "no-such-ws", userNode, "no-such-ws", ngac.OpRead)
		assert.ErrorIs(t, err, domain.ErrAccessDenied)
	})
	t.Run("a policy error is a refusal", func(t *testing.T) {
		f.read.err = fmt.Errorf("policy down")
		defer func() { f.read.err = nil }()
		err := f.svc.AuthorizeWorkspaceAccess(ctx, f.wsID, userNode, f.wsID, ngac.OpRead)
		assert.ErrorIs(t, err, domain.ErrAccessDenied)
	})
	t.Run("missing identity is invalid", func(t *testing.T) {
		assert.Error(t, f.svc.AuthorizeWorkspaceAccess(ctx, f.wsID, "", f.wsID, ngac.OpRead))
		assert.Error(t, f.svc.AuthorizeWorkspaceAccess(ctx, f.wsID, userNode, "", ngac.OpRead))
	})
}
