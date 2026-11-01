package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"ngac-platform/services/approval/internal/domain"
)

// GroupMembers returns the people (user nodes) at or below a role's or
// department's UA, read from the graph tables, which are the source of truth.
func (s *Store) GroupMembers(ctx context.Context, uaNodeID string) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		WITH RECURSIVE down(id) AS (
			SELECT $1::text
			UNION
			SELECT a.child_id FROM ngac_assignments a JOIN down ON a.parent_id = down.id
		)
		SELECT n.id FROM down JOIN ngac_nodes n ON n.id = down.id
		 WHERE n.node_type = 'U' ORDER BY n.id`, uaNodeID)
	if err != nil {
		return nil, fmt.Errorf("group members: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("group members: %w", err)
	}
	return ids, nil
}

// ListOrganizationSchemas returns the active, existing approval schemas of
// organization workspaces.
func (s *Store) ListOrganizationSchemas(ctx context.Context) ([]domain.TenantSchema, error) {
	rows, err := s.db.Query(ctx, `
		SELECT ts.tenant_id, ts.schema_name FROM tenant_schemas ts
		  JOIN workspaces w ON w.id = ts.tenant_id AND w.type = 'organization'
		 WHERE ts.status = 'active' AND ts.schema_name IN (SELECT nspname FROM pg_namespace)
		 ORDER BY ts.tenant_id`)
	if err != nil {
		return nil, fmt.Errorf("list organization schemas: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (domain.TenantSchema, error) {
		var t domain.TenantSchema
		err := r.Scan(&t.TenantID, &t.Schema)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("list organization schemas: %w", err)
	}
	return out, nil
}

// ListPendingRequestIDs returns the ids of the tenant's pending requests.
func (s *Store) ListPendingRequestIDs(ctx context.Context) ([]string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()
	rows, err := c.Query(ctx, `SELECT id FROM approval_requests WHERE status = 'pending' ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list pending request ids: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("list pending request ids: %w", err)
	}
	return ids, nil
}
