package ngac

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OperationStore manages the dynamic operation registry.
type OperationStore struct {
	pool *pgxpool.Pool
}

// NewOperationStore creates a new OperationStore.
func NewOperationStore(pool *pgxpool.Pool) *OperationStore {
	return &OperationStore{pool: pool}
}

// RegisterResult holds the outcome of a bulk registration.
type RegisterResult struct {
	Registered   []string
	AlreadyExist []string
}

// Register persists new operations. Idempotent: existing ones appear in AlreadyExist.
func (s *OperationStore) Register(ctx context.Context, operations []string) (*RegisterResult, error) {
	if len(operations) == 0 {
		return &RegisterResult{}, nil
	}

	result := &RegisterResult{}

	for _, op := range operations {
		if op == "" {
			continue
		}
		tag, err := s.pool.Exec(ctx,
			`INSERT INTO ngac_operations (name) VALUES ($1) ON CONFLICT (name) DO NOTHING`, op)
		if err != nil {
			return nil, fmt.Errorf("registering operation %q: %w", op, err)
		}
		if tag.RowsAffected() > 0 {
			result.Registered = append(result.Registered, op)
			slog.Info("operation registered", "name", op)
		} else {
			result.AlreadyExist = append(result.AlreadyExist, op)
		}
	}
	return result, nil
}

// List returns all registered operations.
func (s *OperationStore) List(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT name FROM ngac_operations ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing operations: %w", err)
	}
	defer rows.Close()

	var ops []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning operation: %w", err)
		}
		ops = append(ops, name)
	}
	return ops, nil
}

// Exists checks if an operation is registered.
func (s *OperationStore) Exists(ctx context.Context, operation string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM ngac_operations WHERE name = $1)`, operation).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking operation %q: %w", operation, err)
	}
	return exists, nil
}

// ValidateOperations checks that all operations are registered.
// Returns the list of unregistered operations (empty if all valid).
func (s *OperationStore) ValidateOperations(ctx context.Context, operations []string) ([]string, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	registered, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	regSet := make(map[string]bool, len(registered))
	for _, r := range registered {
		regSet[r] = true
	}

	var invalid []string
	for _, op := range operations {
		if !regSet[op] {
			invalid = append(invalid, op)
		}
	}
	return invalid, nil
}
