package ngac

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Prohibition represents a deny override in the NGAC graph.
// When a prohibition matches, it overrides an ALLOW decision from associations.
type Prohibition struct {
	ID           string
	Name         string
	SubjectID    string   // U or UA node being denied
	Operations   []string // operations to deny
	TargetOAIDs  []string // OA nodes being denied access to
	Intersection bool     // true=ALL targets must match, false=ANY target
}

// ProhibitionStore manages prohibition CRUD in the database and keeps the
// in-memory graph the PDP evaluates against in step with it.
//
// Writes go to the database first and to memory only after the database
// accepted them, so a failed write never leaves this process enforcing (or
// ignoring) a rule the rest of the system does not see. Publishing the EPP
// graph-mutation event and invalidating caches stays with the caller.
type ProhibitionStore struct {
	pool  *pgxpool.Pool
	graph *Graph // may be nil: database-only use (tools, tests)
}

// NewProhibitionStore creates a ProhibitionStore. graph is the in-memory graph
// to update on Create/Remove; pass the one the decision engine reads.
func NewProhibitionStore(pool *pgxpool.Pool, graph *Graph) *ProhibitionStore {
	return &ProhibitionStore{pool: pool, graph: graph}
}

// Create inserts a new prohibition, then loads it into memory.
// Returns error if name already exists.
func (s *ProhibitionStore) Create(ctx context.Context, p *Prohibition) (*Prohibition, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("prohibition name is required")
	}
	if p.SubjectID == "" {
		return nil, fmt.Errorf("subject_id is required")
	}
	if len(p.Operations) == 0 {
		return nil, fmt.Errorf("at least one operation is required")
	}
	if len(p.TargetOAIDs) == 0 {
		return nil, fmt.Errorf("at least one target_oa_id is required")
	}

	var id string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO ngac_prohibitions (name, subject_id, operations, target_oa_ids, intersection)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		p.Name, p.SubjectID, p.Operations, p.TargetOAIDs, p.Intersection).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("creating prohibition %q: %w", p.Name, err)
	}

	p.ID = id
	if s.graph != nil {
		if err := s.graph.AddProhibition(p); err != nil {
			// Unreachable after the validation above; if it ever happens the
			// database holds a rule memory lacks, so say so loudly.
			return nil, fmt.Errorf("loading prohibition %q into memory (stored in database): %w", p.Name, err)
		}
	}
	slog.Info("prohibition created", "name", p.Name, "subject", p.SubjectID, "ops", p.Operations)
	return p, nil
}

// Remove deletes a prohibition by name from the database, then from memory.
// Returns error if not found.
func (s *ProhibitionStore) Remove(ctx context.Context, name string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM ngac_prohibitions WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("removing prohibition %q: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("prohibition %q not found", name)
	}
	if s.graph != nil {
		s.graph.RemoveProhibition(name)
	}
	slog.Info("prohibition removed", "name", name)
	return nil
}

// GetByName retrieves a prohibition by name.
func (s *ProhibitionStore) GetByName(ctx context.Context, name string) (*Prohibition, error) {
	p := &Prohibition{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, subject_id, operations, target_oa_ids, intersection
		 FROM ngac_prohibitions WHERE name = $1`, name).
		Scan(&p.ID, &p.Name, &p.SubjectID, &p.Operations, &p.TargetOAIDs, &p.Intersection)
	if err != nil {
		return nil, fmt.Errorf("getting prohibition %q: %w", name, err)
	}
	return p, nil
}

// List returns all prohibitions, optionally filtered by subject_id.
func (s *ProhibitionStore) List(ctx context.Context, subjectID string) ([]*Prohibition, error) {
	var query string
	var args []any

	if subjectID != "" {
		query = `SELECT id, name, subject_id, operations, target_oa_ids, intersection
		         FROM ngac_prohibitions WHERE subject_id = $1 ORDER BY name`
		args = []any{subjectID}
	} else {
		query = `SELECT id, name, subject_id, operations, target_oa_ids, intersection
		         FROM ngac_prohibitions ORDER BY name`
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing prohibitions: %w", err)
	}
	defer rows.Close()

	var result []*Prohibition
	for rows.Next() {
		p := &Prohibition{}
		if err := rows.Scan(&p.ID, &p.Name, &p.SubjectID, &p.Operations, &p.TargetOAIDs, &p.Intersection); err != nil {
			return nil, fmt.Errorf("scanning prohibition: %w", err)
		}
		result = append(result, p)
	}
	return result, nil
}
