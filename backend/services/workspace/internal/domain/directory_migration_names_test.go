package domain_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
)

// Migration 064 builds the directory in SQL for the workspaces and departments that
// predate the code, and the node ids are found by name: a name that drifts from the
// ngac helpers would be a node the service never finds, or a second one. Applying a
// helper to the literal "%s" yields the format string, which the migration must carry
// verbatim, together with the mark and the one operation the service writes.
func TestMigration064UsesTheNamesAndMarkOfTheNgacHelpers(t *testing.T) {
	raw, err := os.ReadFile("../../../../../data/migrations/064_directory_visibility.sql")
	require.NoError(t, err)
	sql := string(raw)
	const verb = "%s"
	id, dept := ngac.WorkspaceID(verb), ngac.DeptID(verb)

	for _, name := range []string{
		ngac.DirectoryOAName(id), ngac.DirectoryOpenOAName(id), ngac.DirectoryNoDeptOAName(id),
		ngac.DirectoryOwnersOAName(id), ngac.DirectoryLimitedUAName(id), ngac.DirectoryLimitedProhibitionName(id),
		ngac.DeptDirOAName(dept), ngac.DeptPeerOAName(dept), ngac.DeptPeersUAName(dept),
	} {
		assert.Contains(t, sql, fmt.Sprintf("format('%s',", name), "migration 064 builds the name %q", name)
	}
	assert.Contains(t, sql, fmt.Sprintf("jsonb_build_object('%s', '%s', 'workspace_id', ws)", ngac.PropType, ngac.PropTypeDirectory),
		"the mark the service accepts a directory node by")
	assert.Contains(t, sql, fmt.Sprintf("ARRAY['%s']", ngac.OpRead), "the only operation the directory grants and prohibits")
	assert.Contains(t, sql, fmt.Sprintf("properties->>'%s' = '%s'", ngac.PropType, ngac.PropTypeDirectory))
}
