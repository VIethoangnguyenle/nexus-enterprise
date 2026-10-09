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
// Loads nodes (excluding O-type for memory optimization), assignments, associations
// and prohibitions.
//
// It adds to whatever the graph already holds; it does not remove nodes or
// edges that have since been deleted from the database. Use ReloadGraph to
// make the in-memory graph equal to the database.
func (s *Store) LoadGraph(ctx context.Context) error {
	return loadGraphInto(ctx, s.db, s.graph)
}

// ReloadGraph rebuilds the in-memory graph from the database and swaps it in
// atomically, so deletions are reflected as well as additions (PIP).
//
// The new graph is built off to the side; readers keep evaluating against the
// old one until the swap. On error the current graph is left untouched.
//
// This is how a read replica (policy-read) follows writes made by the policy
// writer: it has no local mutations to preserve, so replacing wholesale is
// exact. See ReplicaGraphRefresher.
func (s *Store) ReloadGraph(ctx context.Context) error {
	fresh := NewGraph()
	if err := loadGraphInto(ctx, s.db, fresh); err != nil {
		return err
	}
	s.graph.replaceWith(fresh)
	return nil
}

// loadGraphInto reads U, UA, OA and PC nodes plus their assignments and all
// associations from db into g.
//
// Every result set is checked with rows.Err(): a stream cut off midway must
// fail the load, not yield a silently partial graph (which ReloadGraph would
// then swap in).
func loadGraphInto(ctx context.Context, db *pgxpool.Pool, g *Graph) error {
	rows, err := db.Query(ctx,
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
		g.AddNode(&n)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("loading nodes: %w", err)
	}

	rows, err = db.Query(ctx,
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
		if err := g.AddAssignment(&a); err != nil {
			slog.Warn("skipping assignment during graph load", "id", a.ID, "error", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("loading assignments: %w", err)
	}

	rows, err = db.Query(ctx, "SELECT id, ua_id, oa_id, operations FROM ngac_associations")
	if err != nil {
		return fmt.Errorf("loading associations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var a Association
		if err := rows.Scan(&a.ID, &a.UAID, &a.OAID, &a.Operations); err != nil {
			return fmt.Errorf("scanning association: %w", err)
		}
		if err := g.AddAssociation(&a); err != nil {
			slog.Warn("skipping association during graph load", "id", a.ID, "error", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("loading associations: %w", err)
	}

	rows, err = db.Query(ctx,
		`SELECT id, name, subject_id, operations, target_oa_ids, intersection FROM ngac_prohibitions`)
	if err != nil {
		return fmt.Errorf("loading prohibitions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p Prohibition
		if err := rows.Scan(&p.ID, &p.Name, &p.SubjectID, &p.Operations, &p.TargetOAIDs, &p.Intersection); err != nil {
			return fmt.Errorf("scanning prohibition: %w", err)
		}
		if err := g.AddProhibition(&p); err != nil {
			// Skipping would silently drop a deny rule; failing keeps the
			// previous graph (ReloadGraph) or stops startup (LoadGraph).
			return fmt.Errorf("loading prohibition %q: %w", p.Name, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("loading prohibitions: %w", err)
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
