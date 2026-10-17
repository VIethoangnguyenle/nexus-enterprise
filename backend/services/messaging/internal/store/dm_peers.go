package store

import (
	"context"
	"fmt"
)

// DMPeerNodes returns the NGAC nodes of a conversation's members other than the
// person acting as userNodeID, from the channel_members cache a direct message is
// looked up by.
//
// A person is recorded there by one node (their workspace node, or their own) and
// may act as another, so excluding only userNodeID would count the caller's own
// stored node as somebody else. Every node that belongs to the same person is
// left out.
func (s *Store) DMPeerNodes(ctx context.Context, channelID, userNodeID string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`WITH me AS (
		   SELECT id AS user_id FROM users WHERE ngac_node = $2
		   UNION
		   SELECT user_id FROM tenant_users WHERE ngac_node_id = $2
		 )
		 SELECT m.ngac_node_id FROM channel_members m
		 WHERE m.channel_id = $1 AND m.ngac_node_id <> $2
		   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.ngac_node = m.ngac_node_id AND u.id IN (SELECT user_id FROM me))
		   AND NOT EXISTS (SELECT 1 FROM tenant_users t WHERE t.ngac_node_id = m.ngac_node_id AND t.user_id IN (SELECT user_id FROM me))`,
		channelID, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("dm peers: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("dm peers: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
