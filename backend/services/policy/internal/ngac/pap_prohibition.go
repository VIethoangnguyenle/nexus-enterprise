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

// ProhibitionStore manages prohibition CRUD in the database.
type ProhibitionStore struct {
	pool *pgxpool.Pool
}

// NewProhibitionStore creates a new ProhibitionStore.
func NewProhibitionStore(pool *pgxpool.Pool) *ProhibitionStore {
	return &ProhibitionStore{pool: pool}
}

// Create inserts a new prohibition. Returns error if name already exists.
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
	slog.Info("prohibition created", "name", p.Name, "subject", p.SubjectID, "ops", p.Operations)
	return p, nil
}

// Remove deletes a prohibition by name. Returns error if not found.
func (s *ProhibitionStore) Remove(ctx context.Context, name string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM ngac_prohibitions WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("removing prohibition %q: %w", name, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("prohibition %q not found", name)
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

// FindForSubjects returns all prohibitions that apply to ANY of the given subject IDs
// (user + user's UA ancestors) and match the given operation.
func (s *ProhibitionStore) FindForSubjects(ctx context.Context, subjectIDs []string, operation string) ([]*Prohibition, error) {
	if len(subjectIDs) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, name, subject_id, operations, target_oa_ids, intersection
		 FROM ngac_prohibitions
		 WHERE subject_id = ANY($1) AND $2 = ANY(operations)`,
		subjectIDs, operation)
	if err != nil {
		return nil, fmt.Errorf("finding prohibitions for subjects: %w", err)
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

