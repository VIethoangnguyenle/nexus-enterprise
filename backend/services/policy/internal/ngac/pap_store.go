package ngac

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// --- PAP: Schema initialization ---

func (s *Store) InitSchema(ctx context.Context) error {
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

	-- Auto-populate operations from existing associations (migration-safe)
	INSERT INTO ngac_operations (name)
	SELECT DISTINCT unnest(operations) AS op
	FROM ngac_associations
	ON CONFLICT (name) DO NOTHING;
	`
	_, err := s.db.Exec(ctx, schema)
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
	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3) ON CONFLICT (child_id, parent_id) DO NOTHING",
		a.ID, a.ChildID, a.ParentID)
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
func (s *Store) CreateAssociation(ctx context.Context, uaID, oaID string, operations []string) (*Association, error) {
	a := &Association{ID: uuid.New().String(), UAID: uaID, OAID: oaID, Operations: operations}
	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES ($1, $2, $3, $4) ON CONFLICT (ua_id, oa_id) DO UPDATE SET operations = $4",
		a.ID, a.UAID, a.OAID, operations)
	if err != nil {
		return nil, fmt.Errorf("inserting association: %w", err)
	}
	if err := s.graph.AddAssociation(a); err != nil {
		return nil, err
	}
	return a, nil
}

// RemoveAssociationByUAOA removes a permission edge by UA+OA pair (PAP).
func (s *Store) RemoveAssociationByUAOA(ctx context.Context, uaID, oaID string) error {
	assocs := s.graph.GetAssociationsFromUA(uaID)
	for _, a := range assocs {
		if a.OAID == oaID {
			s.graph.RemoveAssociationByID(a.ID)
			break
		}
	}
	_, err := s.db.Exec(ctx, "DELETE FROM ngac_associations WHERE ua_id = $1 AND oa_id = $2", uaID, oaID)
	return err
}
