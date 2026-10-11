// Package store provides PostgreSQL data access for the approval service.
// Each public method executes a single query — no business logic here.
// All queries run on the tenant's schema via TenantConn.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// dbConn is what a store method runs its statements on: a tenant-scoped
// connection of its own, or the transaction a caller opened with InTx.
type dbConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Release()
}

// txKey carries the open transaction on the context.
type txKey struct{}

// txConn runs a method's statements on the caller's transaction. A method that
// opens a transaction of its own gets a savepoint inside it, so it commits or
// rolls back with the caller; Release does nothing, the caller owns the connection.
type txConn struct{ pgx.Tx }

func (t txConn) BeginTx(ctx context.Context, _ pgx.TxOptions) (pgx.Tx, error) { return t.Tx.Begin(ctx) }

func (t txConn) Release() {}

// conn returns what a method should run on: the transaction on ctx if there is
// one, otherwise a tenant-scoped connection using the schema from context.
// Caller MUST call Release() when done.
func (s *Store) conn(ctx context.Context) (dbConn, error) {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return txConn{tx}, nil
	}
	return s.tenantConn(ctx)
}

func (s *Store) tenantConn(ctx context.Context) (*pgxpool.Conn, error) {
	schema := httputil.TenantSchemaFromCtx(ctx)
	if schema == "" {
		return nil, fmt.Errorf("tenant schema not set in context")
	}
	return httputil.TenantConn(ctx, s.db, schema)
}

// InTx runs fn with every store call made on the context it is given inside one
// transaction on the tenant's schema: all of them take effect or none does. fn
// returning an error rolls everything back. Called inside a transaction already,
// it joins it.
func (s *Store) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	c, err := s.tenantConn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()
	return pgx.BeginFunc(ctx, c, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// ProvisionSchema creates an isolated PostgreSQL schema for a tenant
// by calling the provision_tenant_schema() function in the public schema.
// Returns the schema name. This runs on the pool directly (no tenant scoping).
func (s *Store) ProvisionSchema(ctx context.Context, tenantID string) (string, error) {
	var schema string
	err := s.db.QueryRow(ctx, `SELECT provision_tenant_schema($1)`, tenantID).Scan(&schema)
	if err != nil {
		return "", fmt.Errorf("provision schema: %w", err)
	}
	return schema, nil
}

// EnsureSchema returns the tenant's schema, creating it only if the tenant has no
// registry row at all. A row that is not active (disabled, migrating, still
// provisioning) is someone else's state: it is left exactly as it is and
// domain.ErrSchemaNotActive comes back, where ProvisionSchema would set it active.
// The check and the creation run under a per-tenant lock, so two first calls at
// once both end with the one schema.
func (s *Store) EnsureSchema(ctx context.Context, tenantID string) (string, error) {
	var schema string
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "approval-schema:"+tenantID); err != nil {
			return fmt.Errorf("lock tenant schema: %w", err)
		}
		var status string
		err := tx.QueryRow(ctx, `SELECT schema_name, status FROM tenant_schemas WHERE tenant_id = $1`, tenantID).Scan(&schema, &status)
		switch {
		case err == nil && status == "active":
			return nil
		case err == nil:
			return domain.ErrSchemaNotActive
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("look up tenant schema: %w", err)
		}
		if err := tx.QueryRow(ctx, `SELECT provision_tenant_schema($1)`, tenantID).Scan(&schema); err != nil {
			return fmt.Errorf("provision schema: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return schema, nil
}
