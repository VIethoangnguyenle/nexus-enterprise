package store

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetUserSettings returns the workspace-scope settings one person has set in one
// workspace, by key. A person with none gets an empty map.
func (s *Store) GetUserSettings(ctx context.Context, userID, workspaceID string) (map[string]json.RawMessage, error) {
	rows, err := s.db.Query(ctx,
		`SELECT key, value FROM user_settings WHERE user_id = $1 AND workspace_id = $2`, userID, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("read user settings: %w", err)
	}
	defer rows.Close()
	out := map[string]json.RawMessage{}
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan user setting: %w", err)
		}
		out[key] = json.RawMessage(value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read user settings: %w", err)
	}
	return out, nil
}

// PutUserSettings writes the given keys for one person in one workspace, all or
// none, replacing a key already set.
func (s *Store) PutUserSettings(ctx context.Context, userID, workspaceID string, kv map[string]json.RawMessage) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin user settings write: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for key, value := range kv {
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_settings (user_id, workspace_id, key, value) VALUES ($1, $2, $3, $4)
			 ON CONFLICT (user_id, workspace_id, key) WHERE workspace_id IS NOT NULL
			 DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
			userID, workspaceID, key, []byte(value)); err != nil {
			return fmt.Errorf("write user setting %s: %w", key, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user settings write: %w", err)
	}
	return nil
}
