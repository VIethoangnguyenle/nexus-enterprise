package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ChannelMemberNodes returns a conversation's workspace and the NGAC nodes of
// its members, as the channel_members cache lists them: who is told of the
// conversation's activity (never of its content, which is read through the
// conversation and its access check). An unknown conversation has no workspace
// and no members.
func (s *Store) ChannelMemberNodes(ctx context.Context, channelID string) (string, []string, error) {
	var workspaceID string
	err := s.db.QueryRow(ctx, `SELECT workspace_id FROM channels WHERE id = $1`, channelID).Scan(&workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, fmt.Errorf("channel workspace: %w", err)
	}
	rows, err := s.db.Query(ctx, `SELECT ngac_node_id FROM channel_members WHERE channel_id = $1`, channelID)
	if err != nil {
		return "", nil, fmt.Errorf("channel members: %w", err)
	}
	nodes, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", nil, fmt.Errorf("channel members: %w", err)
	}
	return workspaceID, nodes, nil
}
