package ngac

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// --- PAP: Schema initialization ---

// schemaObjects are the tables and indexes InitSchema creates. When every one exists
// the DDL is skipped: even as a no-op, CREATE INDEX IF NOT EXISTS takes a ShareLock on
// its table before it looks for the index, and all of it runs in one implicit
// transaction. Two statements of that transaction therefore lock ngac_assignments
// and ngac_nodes in the order opposite to a DELETE FROM ngac_nodes cascading into
// ngac_assignments, so a policy process starting beside a node delete (a restart, a
// second replica) deadlocked with it and blocked every graph write meanwhile.
var schemaObjects = []string{
	"ngac_nodes", "ngac_assignments", "ngac_associations", "ngac_operations", "ngac_prohibitions",
	"idx_assignments_child", "idx_assignments_parent", "idx_associations_ua", "idx_associations_oa",
	"idx_nodes_type", "idx_nodes_name", "idx_prohibitions_subject",
}

func (s *Store) schemaPresent(ctx context.Context) (bool, error) {
	var missing int
	err := s.db.QueryRow(ctx,
		`SELECT count(*) FROM unnest($1::text[]) AS o(name) WHERE to_regclass(o.name) IS NULL`,
		schemaObjects).Scan(&missing)
	return err == nil && missing == 0, err
}

func (s *Store) InitSchema(ctx context.Context) error {
	if present, err := s.schemaPresent(ctx); err != nil {
		return err
	} else if present {
		return s.backfillOperations(ctx)
	}
	schema := `
	CREATE TABLE IF NOT EXISTS ngac_nodes (
		id         TEXT PRIMARY KEY,
		name       TEXT NOT NULL,
		node_type  TEXT NOT NULL CHECK (node_type IN ('U','UA','O','OA','PC')),
		properties JSONB DEFAULT '{}',
		created_at TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS ngac_assignments (
		id        TEXT PRIMARY KEY,
		child_id  TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		parent_id TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		UNIQUE(child_id, parent_id)
	);
	CREATE TABLE IF NOT EXISTS ngac_associations (
		id         TEXT PRIMARY KEY,
		ua_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		oa_id      TEXT NOT NULL REFERENCES ngac_nodes(id) ON DELETE CASCADE,
		operations TEXT[] NOT NULL,
		UNIQUE(ua_id, oa_id)
	);
	CREATE TABLE IF NOT EXISTS ngac_operations (
		name        TEXT PRIMARY KEY,
		description TEXT DEFAULT '',
		created_at  TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE TABLE IF NOT EXISTS ngac_prohibitions (
		id            TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
		name          TEXT UNIQUE NOT NULL,
		subject_id    TEXT NOT NULL,
		operations    TEXT[] NOT NULL,
		target_oa_ids TEXT[] NOT NULL,
		intersection  BOOLEAN DEFAULT FALSE,
		created_at    TIMESTAMPTZ DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_assignments_child ON ngac_assignments(child_id);
	CREATE INDEX IF NOT EXISTS idx_assignments_parent ON ngac_assignments(parent_id);
	CREATE INDEX IF NOT EXISTS idx_associations_ua ON ngac_associations(ua_id);
	CREATE INDEX IF NOT EXISTS idx_associations_oa ON ngac_associations(oa_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON ngac_nodes(node_type);
	CREATE INDEX IF NOT EXISTS idx_nodes_name ON ngac_nodes(name);
	CREATE INDEX IF NOT EXISTS idx_prohibitions_subject ON ngac_prohibitions(subject_id);
	`
	if _, err := s.db.Exec(ctx, schema); err != nil {
		return err
	}
	return s.backfillOperations(ctx)
}

// backfillOperations records every operation already used by an association
// (migration-safe). Row locks only, so it never contends with graph writes.
func (s *Store) backfillOperations(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
	INSERT INTO ngac_operations (name)
	SELECT DISTINCT unnest(operations) AS op
	FROM ngac_associations
	ON CONFLICT (name) DO NOTHING`)
	return err
}

// --- PAP: Policy mutations ---

// CreateNode creates a graph node and persists it to DB (PAP).
// Side effect: also updates in-memory graph (PIP).
func (s *Store) CreateNode(ctx context.Context, name, nodeType string, properties map[string]string) (*NGACNode, error) {
	if !IsValidNodeType(nodeType) {
		return nil, fmt.Errorf("invalid node type: %s", nodeType)
	}
	node := &NGACNode{
		ID: uuid.New().String(), Name: name, NodeType: nodeType,
		Properties: properties, CreatedAt: time.Now(),
	}
	if properties == nil {
		properties = map[string]string{}
	}
	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $2, $3, $4)",
		node.ID, node.Name, node.NodeType, properties)
	if err != nil {
		return nil, fmt.Errorf("inserting node: %w", err)
	}
	s.graph.AddNode(node)
	return node, nil
}

// DeleteNode removes a node from DB and in-memory graph (PAP).
func (s *Store) DeleteNode(ctx context.Context, nodeID string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", nodeID)
	if err != nil {
		return err
	}
	s.graph.RemoveNode(nodeID)
	return nil
}

// CreateAssignment creates a containment edge in DB and graph (PAP).
// Pattern: validate (read-only) → DB write → graph mutation.
func (s *Store) CreateAssignment(ctx context.Context, childID, parentID string) (*Assignment, error) {
	a := &Assignment{ID: uuid.New().String(), ChildID: childID, ParentID: parentID}

	// Validate without mutating graph state
	if err := s.graph.ValidateAssignment(a); err != nil {
		return nil, err
	}

	// Persist to DB first — graph stays clean on DB failure
	// An edge that exists keeps its row ID (the no-op update returns it), so the
	// graph never holds a second entry under an ID the database does not know.
	err := s.db.QueryRow(ctx,
		`INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)
		 ON CONFLICT (child_id, parent_id) DO UPDATE SET child_id = EXCLUDED.child_id RETURNING id`,
		a.ID, a.ChildID, a.ParentID).Scan(&a.ID)
	if err != nil {
		return nil, fmt.Errorf("inserting assignment: %w", err)
	}

	// DB succeeded → update in-memory graph
	if err := s.graph.AddAssignment(a); err != nil {
		return nil, err
	}
	return a, nil
}

// RemoveAssignment removes a containment edge from DB and graph (PAP).
func (s *Store) RemoveAssignment(ctx context.Context, childID, parentID string) error {
	_, err := s.db.Exec(ctx,
		"DELETE FROM ngac_assignments WHERE child_id = $1 AND parent_id = $2", childID, parentID)
	if err != nil {
		return err
	}
	s.graph.RemoveAssignment(childID, parentID)
	return nil
}

// CreateAssociation creates a permission edge in DB and graph (PAP).
// Pattern: validate (read-only) → DB write → graph mutation.
func (s *Store) CreateAssociation(ctx context.Context, uaID, oaID string, operations []string) (*Association, error) {
	s.assocMu.Lock()
	defer s.assocMu.Unlock()
	a := &Association{ID: uuid.New().String(), UAID: uaID, OAID: oaID, Operations: operations}

	// Validate node existence and types before the DB write. Checking only
	// after the insert left a row the graph refused, which the next LoadGraph
	// would then hit.
	if err := s.graph.ValidateAssociation(a); err != nil {
		return nil, err
	}
	// A second grant to the same UA and OA updates the row and keeps its ID;
	// the graph must carry that ID, or a reload would disagree with this process.
	err := s.db.QueryRow(ctx,
		"INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES ($1, $2, $3, $4) ON CONFLICT (ua_id, oa_id) DO UPDATE SET operations = $4 RETURNING id",
		a.ID, a.UAID, a.OAID, operations).Scan(&a.ID)
	if err != nil {
		return nil, fmt.Errorf("inserting association: %w", err)
	}
	if err := s.graph.AddAssociation(a); err != nil {
		return nil, err
	}
	return a, nil
}

// RemoveAssociationByUAOA removes a permission edge by UA+OA pair (PAP).
// Pattern: DB write → graph mutation, like every other mutation here. If the
// DB delete fails the in-memory graph must still match the DB; removing the
// edge from memory first would make this process deny what every other
// process (and this one after a restart) still allows.
func (s *Store) RemoveAssociationByUAOA(ctx context.Context, uaID, oaID string) error {
	s.assocMu.Lock()
	defer s.assocMu.Unlock()
	if _, err := s.db.Exec(ctx,
		"DELETE FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", uaID, oaID); err != nil {
		return fmt.Errorf("deleting association: %w", err)
	}

	// DB succeeded → update in-memory graph. (ua_id, oa_id) is unique, so there
	// is at most one edge to remove.
	for _, a := range s.graph.GetAssociationsFromUA(uaID) {
		if a.OAID == oaID {
			s.graph.RemoveAssociationByID(a.ID)
			break
		}
	}
	return nil
}
