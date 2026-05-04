package httputil

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantSchemaKey is the context key for the tenant schema name.
type tenantSchemaKey struct{}

// WithTenantSchema stores the tenant schema name in the context.
func WithTenantSchema(ctx context.Context, schema string) context.Context {
	return context.WithValue(ctx, tenantSchemaKey{}, schema)
}

// TenantSchemaFromCtx extracts the tenant schema name from context.
// Returns empty string if not set.
func TenantSchemaFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(tenantSchemaKey{}).(string)
	return v
}

// TenantSchemaResolver resolves tenant_id → schema_name using the tenant_schemas
// registry in the public schema. Caches in-memory to avoid per-request lookups.
type TenantSchemaResolver struct {
	db    *pgxpool.Pool
	mu    sync.RWMutex
	cache map[string]string // tenant_id → schema_name
}

// NewTenantSchemaResolver creates a resolver backed by the given DB pool.
func NewTenantSchemaResolver(db *pgxpool.Pool) *TenantSchemaResolver {
	return &TenantSchemaResolver{db: db, cache: make(map[string]string)}
}

// Resolve returns the schema name for a tenant, using cache when available.
func (r *TenantSchemaResolver) Resolve(ctx context.Context, tenantID string) (string, error) {
	r.mu.RLock()
	schema, ok := r.cache[tenantID]
	r.mu.RUnlock()
	if ok {
		return schema, nil
	}

	err := r.db.QueryRow(ctx,
		`SELECT schema_name FROM tenant_schemas WHERE tenant_id = $1 AND status = 'active'`,
		tenantID,
	).Scan(&schema)
	if err != nil {
		return "", fmt.Errorf("resolve tenant schema: %w", err)
	}

	r.mu.Lock()
	r.cache[tenantID] = schema
	r.mu.Unlock()

	return schema, nil
}

// Invalidate removes a cached entry (e.g., after schema migration).
func (r *TenantSchemaResolver) Invalidate(tenantID string) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()
}

// TenantConn acquires a connection from the pool and sets the search_path
// to the tenant's schema. The caller MUST call release() when done.
func TenantConn(ctx context.Context, pool *pgxpool.Pool, schema string) (*pgxpool.Conn, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	// pgx.Identifier.Sanitize() prevents SQL injection on schema names.
	_, err = conn.Exec(ctx, "SET search_path TO "+pgx.Identifier{schema}.Sanitize()+", public")
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("set search_path: %w", err)
	}

	return conn, nil
}
