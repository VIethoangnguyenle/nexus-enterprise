package store

import "context"

// DisplayNames maps owner keys to the display name of the person behind them.
//
// A drive item records its owner either as a user id (files: the uploader) or
// as an NGAC node id (folders: the creator), so each key is matched against
// both. A person without a display name is shown by username. Keys that are
// not a person ("system" on the root folders) are left out of the result.
func (s *Store) DisplayNames(ctx context.Context, ownerKeys []string) (map[string]string, error) {
	names := make(map[string]string)
	if len(ownerKeys) == 0 {
		return names, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT id, COALESCE(ngac_node, ''), COALESCE(NULLIF(display_name, ''), username)
		   FROM users
		  WHERE id = ANY($1) OR ngac_node = ANY($1)`, ownerKeys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, node, name string
		if err := rows.Scan(&id, &node, &name); err != nil {
			return nil, err
		}
		names[id] = name
		if node != "" {
			names[node] = name
		}
	}
	return names, rows.Err()
}
