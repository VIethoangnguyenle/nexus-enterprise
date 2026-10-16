package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrNoSuchChannel means the channel an avatar write names does not exist.
var ErrNoSuchChannel = errors.New("no such channel")

// AvatarImages is one group picture in the three sizes it is served in.
type AvatarImages struct {
	Small  []byte // 64×64
	Medium []byte // 192×192
	Large  []byte // 1024×1024
}

// avatarColumns maps a served size to its column. The column name in a query
// only ever comes from here, never from the request.
var avatarColumns = map[int]string{64: "small", 192: "medium", 1024: "large"}

// SetChannelAvatar stores the channel's picture under token and points the
// channel at url, replacing any picture it had. It returns ErrNoSuchChannel
// when the channel does not exist.
//
// The channels row is written first, in both SetChannelAvatar and
// ClearChannelAvatar, so the two always lock in the same order and cannot
// deadlock against each other.
func (s *Store) SetChannelAvatar(ctx context.Context, channelID, token string, img AvatarImages, url string) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE channels SET avatar_url = $2 WHERE id = $1`, channelID, url)
		if err != nil {
			return fmt.Errorf("set channel avatar url: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNoSuchChannel
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO channel_avatars (channel_id, token, small, medium, large)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (channel_id) DO UPDATE SET
			     token = EXCLUDED.token, small = EXCLUDED.small, medium = EXCLUDED.medium,
			     large = EXCLUDED.large, created_at = NOW()`,
			channelID, token, img.Small, img.Medium, img.Large); err != nil {
			return fmt.Errorf("store channel avatar: %w", err)
		}
		return nil
	})
}

// ClearChannelAvatar removes the channel's picture. Having none is not an
// error; a channel that does not exist is ErrNoSuchChannel.
func (s *Store) ClearChannelAvatar(ctx context.Context, channelID string) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE channels SET avatar_url = '' WHERE id = $1`, channelID)
		if err != nil {
			return fmt.Errorf("clear channel avatar url: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNoSuchChannel
		}
		if _, err := tx.Exec(ctx, `DELETE FROM channel_avatars WHERE channel_id = $1`, channelID); err != nil {
			return fmt.Errorf("delete channel avatar: %w", err)
		}
		return nil
	})
}

// GetChannelAvatar returns one size of the picture stored under token, or nil
// when there is no such picture or size.
func (s *Store) GetChannelAvatar(ctx context.Context, token string, size int) ([]byte, error) {
	col, ok := avatarColumns[size]
	if !ok {
		return nil, nil
	}
	var b []byte
	err := s.db.QueryRow(ctx, `SELECT `+col+` FROM channel_avatars WHERE token = $1`, token).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get channel avatar: %w", err)
	}
	return b, nil
}
