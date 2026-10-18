package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUndefinedTable is SQLSTATE 42P01: the tenant has no approval_user_settings
// table yet, because nobody in it ever opened their approval settings.
const pgUndefinedTable = "42P01"

// NotifyOffByNode looks the given user nodes up in the tenant's
// approval_user_settings (keyed by user ID, resolved through the shared users
// table) and reports who turned notify_pending or notify_result off. Nodes with
// no row, and a tenant with no table yet, have both on.
func (s *Store) NotifyOffByNode(ctx context.Context, nodeIDs []string) (pendingOff, resultOff map[string]bool, err error) {
	pendingOff, resultOff = map[string]bool{}, map[string]bool{}
	var nodes []string
	for _, n := range nodeIDs {
		if n != "" {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		return pendingOff, resultOff, nil
	}
	c, err := s.conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT u.ngac_node, s.notify_pending, s.notify_result
		FROM approval_user_settings s JOIN users u ON u.id = s.user_id
		WHERE u.ngac_node = ANY($1)`, nodes)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUndefinedTable {
			return pendingOff, resultOff, nil
		}
		return nil, nil, fmt.Errorf("notify settings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var node string
		var pending, result bool
		if err := rows.Scan(&node, &pending, &result); err != nil {
			return nil, nil, fmt.Errorf("notify settings: %w", err)
		}
		pendingOff[node] = !pending
		resultOff[node] = !result
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("notify settings: %w", err)
	}
	return pendingOff, resultOff, nil
}
