package ngac

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A personal UA hangs under no policy class, so the descent from the tenant PC
// never reaches it. A shard must still carry it, or a share made to a person
// grants nothing in any check that evaluates on the workspace shard.
func TestLoadShard_CarriesPersonalUAsOfLoadedUsersOnly(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test DB not available: %v", err)
	}

	sfx := uuid.NewString()
	ws := "shard-ws-" + sfx
	ids := map[string]string{}
	var created []string
	node := func(key, typ, props string) {
		id := key + "-" + sfx
		ids[key] = id
		created = append(created, id)
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1,$1,$2,$3::jsonb)`, id, typ, props)
		require.NoError(t, err, key)
	}
	assign := func(child, parent string) {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1,$2,$3)`,
			uuid.NewString(), ids[child], ids[parent])
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		for _, id := range created {
			pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, id)
		}
	})

	node("pc", "PC", `{"workspace_id":"`+ws+`","scope":"tenant","tenant_id":"t"}`)
	node("members", "UA", `{}`)
	node("alice", "U", `{}`)
	node("bob", "U", `{}`)
	node("alice-ua", "UA", `{"type":"personal_ua","user_node_id":"alice-`+sfx+`"}`)
	// A UA claiming to be Bob's personal attribute, with Alice assigned to it.
	node("bob-ua", "UA", `{"type":"personal_ua","user_node_id":"bob-`+sfx+`"}`)
	// A role that merely carries a personal-looking name.
	node("squat", "UA", `{}`)
	assign("members", "pc")
	assign("alice", "members")
	assign("bob", "members")
	assign("alice", "alice-ua")
	assign("alice", "bob-ua") // Alice is in a UA that belongs to Bob: not hers
	assign("alice", "squat")

	sm := &shardManager{db: pool}
	g, err := sm.loadShard(ctx, ws)
	require.NoError(t, err)

	assert.Contains(t, g.Nodes, ids["alice-ua"], "Alice's own personal UA must be in the shard")
	assert.Contains(t, g.Nodes, ids["members"])
	assert.NotContains(t, g.Nodes, ids["bob-ua"], "another user's personal UA must not be loaded through Alice")
	assert.NotContains(t, g.Nodes, ids["squat"], "an ordinary UA outside the policy class is not part of the shard")
	ancestors := g.GetAncestors(ids["alice"])
	assert.Contains(t, ancestors, ids["alice-ua"])
}
