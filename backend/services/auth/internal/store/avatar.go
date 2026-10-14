package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AvatarImages is one profile picture in the three sizes it is served in.
type AvatarImages struct {
	Small  []byte // 64×64
	Medium []byte // 192×192
	Large  []byte // 1024×1024
}

// avatarColumns maps a served size to its column. The column name in a query
// only ever comes from here, never from the request.
var avatarColumns = map[int]string{64: "small", 192: "medium", 1024: "large"}

// SetAvatar stores the person's picture under token and points their profile
// at url, replacing any picture they had. It returns ErrNoSuchUser when the
// account does not exist.
//
// The users row is written first, in both SetAvatar and ClearAvatar, so the
// two always lock in the same order and cannot deadlock against each other.
func (s *Store) SetAvatar(ctx context.Context, userID, token string, img AvatarImages, url string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET avatar_url = $2 WHERE id = $1`, userID, url)
		if err != nil {
			return fmt.Errorf("set avatar url: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNoSuchUser
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_avatars (user_id, token, small, medium, large)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (user_id) DO UPDATE SET
			     token = EXCLUDED.token, small = EXCLUDED.small, medium = EXCLUDED.medium,
			     large = EXCLUDED.large, created_at = NOW()`,
			userID, token, img.Small, img.Medium, img.Large); err != nil {
			return fmt.Errorf("store avatar: %w", err)
		}
		return nil
	})
}

// ClearAvatar removes the person's picture. Having none is not an error; an
// account that does not exist is ErrNoSuchUser.
func (s *Store) ClearAvatar(ctx context.Context, userID string) error {
	return s.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE users SET avatar_url = '' WHERE id = $1`, userID)
		if err != nil {
			return fmt.Errorf("clear avatar url: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNoSuchUser
		}
		if _, err := tx.Exec(ctx, `DELETE FROM user_avatars WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("delete avatar: %w", err)
		}
		return nil
	})
}

// GetAvatar returns one size of the picture stored under token, or nil when
// there is no such picture or size.
func (s *Store) GetAvatar(ctx context.Context, token string, size int) ([]byte, error) {
	col, ok := avatarColumns[size]
	if !ok {
		return nil, nil
	}
	var b []byte
	err := s.db.QueryRow(ctx, `SELECT `+col+` FROM user_avatars WHERE token = $1`, token).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get avatar: %w", err)
	}
	return b, nil
}

// inTx runs fn in a transaction, committing when it returns nil.
func (s *Store) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
