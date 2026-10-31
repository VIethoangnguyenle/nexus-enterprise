package store

import (
	"context"
	"fmt"

	"ngac-platform/pkg/httputil"
)

// errChannelGone is a write to a channel row that no longer exists (a second
// disband, a transfer just after one): not found, which the caller may hear.
var errChannelGone = fmt.Errorf("channel %w", httputil.ErrNotFound)

// UpdateChannelSettings updates the group management policies of a channel.
// Only the provided non-empty fields are changed.
func (s *Store) UpdateChannelSettings(ctx context.Context, channelID, postPolicy, invitePolicy, mentionAllPolicy string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE channels SET
			post_policy        = COALESCE(NULLIF($2, ''), post_policy),
			invite_policy      = COALESCE(NULLIF($3, ''), invite_policy),
			mention_all_policy = COALESCE(NULLIF($4, ''), mention_all_policy)
		 WHERE id = $1`,
		channelID, postPolicy, invitePolicy, mentionAllPolicy)
	if err != nil {
		return fmt.Errorf("update channel settings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errChannelGone
	}
	return nil
}

// TransferChannelOwnership updates created_by to the new owner.
func (s *Store) TransferChannelOwnership(ctx context.Context, channelID, newOwnerUserID string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE channels SET created_by = $2 WHERE id = $1`, channelID, newOwnerUserID)
	if err != nil {
		return fmt.Errorf("transfer channel ownership: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errChannelGone
	}
	return nil
}

// DeleteChannel deletes a channel and its members (cascade).
func (s *Store) DeleteChannel(ctx context.Context, channelID string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM channels WHERE id = $1`, channelID)
	if err != nil {
		return fmt.Errorf("delete channel: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return errChannelGone
	}
	return nil
}
