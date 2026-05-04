package ngac

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles PostgreSQL persistence for the NGAC graph.
//
// Responsibility mapping:
//   - PAP (Policy Administration): CreateNode, DeleteNode, CreateAssignment,
//     RemoveAssignment, CreateAssociation, RemoveAssociationByUAOA, InitSchema  (see pap_store.go)
//   - PIP (Policy Information): LoadGraph, GetGraph, FindNodeByName,
//     GetNodesByType, GetNode, IsAssigned, HasSeedData
type Store struct {
	db    *pgxpool.Pool
	graph *Graph
}

func NewStore(db *pgxpool.Pool, graph *Graph) *Store {
	return &Store{db: db, graph: graph}
}

// --- PIP: Data hydration ---

// LoadGraph hydrates the in-memory graph from database (PIP).
// Loads nodes (excluding O-type for memory optimization), assignments, and associations.
func (s *Store) LoadGraph(ctx context.Context) error {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, node_type, properties, created_at FROM ngac_nodes
		 WHERE node_type IN ('U', 'UA', 'OA', 'PC')`)
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

	rows, err = s.db.Query(ctx,
		`SELECT a.id, a.child_id, a.parent_id FROM ngac_assignments a
		 JOIN ngac_nodes c ON a.child_id = c.id
		 JOIN ngac_nodes p ON a.parent_id = p.id
		 WHERE c.node_type IN ('U', 'UA', 'OA', 'PC')
		   AND p.node_type IN ('U', 'UA', 'OA', 'PC')`)
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
			slog.Warn("skipping assignment during graph load", "id", a.ID, "error", err)
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
			slog.Warn("skipping association during graph load", "id", a.ID, "error", err)
		}
	}
	return nil
}

// --- PIP: Read-only data access ---

// GetGraph returns the in-memory graph reference (PIP).
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
