// Package store is the calendar service's database access. It makes no
// authorization decision: who may see or change a calendar is the policy
// service's, asked by the domain.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Refusals the store decides itself, so a caller can tell them from a failure of
// the database.
var (
	// ErrNotFound: the row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists: a unique index refused the insert. For a calendar it
	// means another request created it first; the caller reads that one.
	ErrAlreadyExists = errors.New("already exists")
)

// Store provides database access for the calendar domain.
type Store struct {
	pool *pgxpool.Pool
}

// New creates a Store backed by the given connection pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// uniqueViolation is the Postgres error code for a unique index refusing a row.
const uniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
