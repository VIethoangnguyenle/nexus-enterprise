package ngac_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A policy process starting while another transaction deletes a node must not wait
// for it. InitSchema used to run CREATE INDEX IF NOT EXISTS on an already complete
// schema, which still takes a ShareLock on the table and, held to the end of its one
// transaction, deadlocked against DELETE FROM ngac_nodes cascading into
// ngac_assignments.
func TestInitSchema_CompleteSchemaDoesNotBlockOnNodeDelete(t *testing.T) {
	s, pool := setupStore(t)
	ctx := context.Background()
	require.NoError(t, s.InitSchema(ctx), "first run creates anything missing")

	child, parent := "init-schema-c-"+time.Now().Format("150405.000000"), "init-schema-p-"+time.Now().Format("150405.000000")
	for _, id := range []string{child, parent} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'UA')`, id)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)`, child+"-a", child, parent)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, child, parent)
	})

	// Hold the delete open: row locks on ngac_nodes and, by cascade, ngac_assignments.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { tx.Rollback(ctx) })
	_, err = tx.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, parent)
	require.NoError(t, err)

	initCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	require.NoError(t, s.InitSchema(initCtx), "InitSchema waited on a concurrent node delete")
}
