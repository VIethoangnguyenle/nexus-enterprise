package ngac

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles PostgreSQL persistence for the NGAC graph
type Store struct {
	db    *pgxpool.Pool
	graph *Graph
}

func NewStore(db *pgxpool.Pool, graph *Graph) *Store {
	return &Store{db: db, graph: graph}
}

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
	CREATE INDEX IF NOT EXISTS idx_assignments_child ON ngac_assignments(child_id);
	CREATE INDEX IF NOT EXISTS idx_assignments_parent ON ngac_assignments(parent_id);
	CREATE INDEX IF NOT EXISTS idx_associations_ua ON ngac_associations(ua_id);
	CREATE INDEX IF NOT EXISTS idx_associations_oa ON ngac_associations(oa_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_type ON ngac_nodes(node_type);
	CREATE INDEX IF NOT EXISTS idx_nodes_name ON ngac_nodes(name);
	`
	_, err := s.db.Exec(ctx, schema)
	return err
}

func (s *Store) LoadGraph(ctx context.Context) error {
	rows, err := s.db.Query(ctx, "SELECT id, name, node_type, properties, created_at FROM ngac_nodes")
	if err != nil {
		return fmt.Errorf("loading nodes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n NGACNode
		var props map[string]string
		if err := rows.Scan(&n.ID, &n.Name, &n.NodeType, &props, &n.CreatedAt); err != nil {
			return fmt.Errorf("scanning node: %w", err)
		}
		n.Properties = props
		s.graph.AddNode(&n)
	}

	rows, err = s.db.Query(ctx, "SELECT id, child_id, parent_id FROM ngac_assignments")
	if err != nil {
		return fmt.Errorf("loading assignments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.ChildID, &a.ParentID); err != nil {
			return fmt.Errorf("scanning assignment: %w", err)
		}
		if err := s.graph.AddAssignment(&a); err != nil {
			fmt.Printf("Warning: skipping assignment %s: %v\n", a.ID, err)
		}
	}

	rows, err = s.db.Query(ctx, "SELECT id, ua_id, oa_id, operations FROM ngac_associations")
	if err != nil {
		return fmt.Errorf("loading associations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a Association
		if err := rows.Scan(&a.ID, &a.UAID, &a.OAID, &a.Operations); err != nil {
			return fmt.Errorf("scanning association: %w", err)
		}
		if err := s.graph.AddAssociation(&a); err != nil {
			fmt.Printf("Warning: skipping association %s: %v\n", a.ID, err)
		}
	}
	return nil
}

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

func (s *Store) DeleteNode(ctx context.Context, nodeID string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM ngac_nodes WHERE id = $1", nodeID)
	if err != nil {
		return err
	}
	s.graph.RemoveNode(nodeID)
	return nil
}

func (s *Store) CreateAssignment(ctx context.Context, childID, parentID string) (*Assignment, error) {
	a := &Assignment{ID: uuid.New().String(), ChildID: childID, ParentID: parentID}
	if err := s.graph.AddAssignment(a); err != nil {
		return nil, err
	}
	s.graph.RemoveAssignment(childID, parentID)
	_, err := s.db.Exec(ctx,
		"INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3) ON CONFLICT (child_id, parent_id) DO NOTHING",
		a.ID, a.ChildID, a.ParentID)
	if err != nil {
		return nil, fmt.Errorf("inserting assignment: %w", err)
	}
	if err := s.graph.AddAssignment(a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Store) RemoveAssignment(ctx context.Context, childID, parentID string) error {
	_, err := s.db.Exec(ctx,
		"DELETE FROM ngac_assignments WHERE child_id = $1 AND parent_id = $2", childID, parentID)
	if err != nil {
		return err
	}
	s.graph.RemoveAssignment(childID, parentID)
	return nil
}

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

func (s *Store) GetGraph() *Graph { return s.graph }

func (s *Store) FindNodeByName(name, nodeType string) *NGACNode {
	return s.graph.FindNodeByName(name, nodeType)
}

func (s *Store) GetNodesByType(nodeType string) []*NGACNode {
	return s.graph.GetNodesByType(nodeType)
}

func (s *Store) GetNode(nodeID string) *NGACNode {
	return s.graph.GetNode(nodeID)
}

func (s *Store) IsAssigned(childID, parentID string) bool {
	return s.graph.IsAssigned(childID, parentID)
}

func (s *Store) HasSeedData(ctx context.Context) bool {
	var count int
	err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM ngac_nodes WHERE node_type = 'PC'").Scan(&count)
	return err == nil && count > 0
}

// GetUserByNGACNodeID retrieves a user by their NGAC node ID
func (s *Store) GetUserByNGACNodeID(ctx context.Context, ngacNodeID string) (userID, username string, err error) {
	err = s.db.QueryRow(ctx, "SELECT id, username FROM users WHERE ngac_node = $1", ngacNodeID).Scan(&userID, &username)
	if err == pgx.ErrNoRows {
		return "", "", fmt.Errorf("user not found for ngac node %s", ngacNodeID)
	}
	return
}
