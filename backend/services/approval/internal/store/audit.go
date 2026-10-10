// Package store provides PostgreSQL data access for the approval service.
// Each public method executes a single query — no business logic here.
// All queries run on the tenant's schema via TenantConn.
package store

import (
	"context"
	"fmt"

	"ngac-platform/services/approval/internal/domain"
)

// InsertAuditEntry appends an audit record to the audit log.
func (s *Store) InsertAuditEntry(ctx context.Context, entry *domain.AuditEntry) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	// INET and JSONB columns reject empty strings — send nil for NULL
	var ipAddr interface{}
	if entry.IPAddress != "" {
		ipAddr = entry.IPAddress
	}
	var detail interface{}
	if entry.DetailJSON != "" {
		detail = entry.DetailJSON
	}

	_, err = c.Exec(ctx, `
		INSERT INTO approval_audit_log (id, request_id, action, actor_node_id, step_order, detail, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.ID, entry.RequestID, entry.Action, entry.ActorNodeID, entry.StepOrder, detail, ipAddr, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// ListAuditEntries retrieves all audit records for a request, ordered chronologically.
func (s *Store) ListAuditEntries(ctx context.Context, requestID string) ([]*domain.AuditEntry, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, action, actor_node_id, step_order, detail, ip_address, created_at
		FROM approval_audit_log WHERE request_id = $1 ORDER BY created_at ASC`, requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditEntry
	for rows.Next() {
		e := &domain.AuditEntry{}
		var detailJSON, ipAddress *string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.Action, &e.ActorNodeID, &e.StepOrder, &detailJSON, &ipAddress, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		if detailJSON != nil {
			e.DetailJSON = *detailJSON
		}
		if ipAddress != nil {
			e.IPAddress = *ipAddress
		}
		entries = append(entries, e)
	}
	return entries, nil
}
