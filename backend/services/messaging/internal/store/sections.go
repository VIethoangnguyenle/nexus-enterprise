// Package store — personal sections of the chat navigator.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Section is one of a user's navigator sections, with its conversations.
type Section struct {
	ID         string
	Name       string
	Position   int
	ChannelIDs []string
}

// ErrSectionNameTaken is returned when the user already has a section of that name.
var ErrSectionNameTaken = errors.New("section name taken")

// ListSections returns a user's sections in a workspace, in order, each with
// its conversations.
func (s *Store) ListSections(ctx context.Context, workspaceID, userID string) ([]*Section, error) {
	rows, err := s.db.Query(ctx,
		`SELECT cs.id, cs.name, cs.position,
		        COALESCE(ARRAY(SELECT i.channel_id FROM chat_section_items i WHERE i.section_id = cs.id ORDER BY i.channel_id), '{}')
		 FROM chat_sections cs
		 WHERE cs.workspace_id = $1 AND cs.user_id = $2
		 ORDER BY cs.position, cs.created_at`, workspaceID, userID)
	if err != nil {
		return nil, fmt.Errorf("list sections: %w", err)
	}
	defer rows.Close()
	var out []*Section
	for rows.Next() {
		sec := &Section{}
		if err := rows.Scan(&sec.ID, &sec.Name, &sec.Position, &sec.ChannelIDs); err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}

// GetSection returns a user's own section; nil when it is not theirs or not there.
func (s *Store) GetSection(ctx context.Context, workspaceID, userID, sectionID string) (*Section, error) {
	sec := &Section{}
	err := s.db.QueryRow(ctx,
		`SELECT cs.id, cs.name, cs.position,
		        COALESCE(ARRAY(SELECT i.channel_id FROM chat_section_items i WHERE i.section_id = cs.id ORDER BY i.channel_id), '{}')
		 FROM chat_sections cs WHERE cs.id = $1 AND cs.workspace_id = $2 AND cs.user_id = $3`,
		sectionID, workspaceID, userID).Scan(&sec.ID, &sec.Name, &sec.Position, &sec.ChannelIDs)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get section: %w", err)
	}
	return sec, nil
}

func nameTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_chat_sections_name"
}

// InsertSection adds a section at a position (nil: last), shifting the ones at
// and after it, and places the given conversations in it, in one transaction.
func (s *Store) InsertSection(ctx context.Context, workspaceID, userID string, sec *Section, position *int) error {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if position == nil {
			var last int
			if err := tx.QueryRow(ctx,
				`SELECT COALESCE(MAX(position) + 1, 0) FROM chat_sections WHERE workspace_id = $1 AND user_id = $2`,
				workspaceID, userID).Scan(&last); err != nil {
				return err
			}
			sec.Position = last
		} else {
			sec.Position = max(*position, 0)
			if _, err := tx.Exec(ctx,
				`UPDATE chat_sections SET position = position + 1
				 WHERE workspace_id = $1 AND user_id = $2 AND position >= $3`, workspaceID, userID, sec.Position); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO chat_sections (id, workspace_id, user_id, name, position) VALUES ($1, $2, $3, $4, $5)`,
			sec.ID, workspaceID, userID, sec.Name, sec.Position); err != nil {
			return err
		}
		for _, ch := range sec.ChannelIDs {
			if err := placeChannel(ctx, tx, userID, sec.ID, ch); err != nil {
				return err
			}
		}
		return nil
	})
	if nameTaken(err) {
		return ErrSectionNameTaken
	}
	if err != nil {
		return fmt.Errorf("insert section: %w", err)
	}
	return nil
}

// RenameSection renames a user's own section.
func (s *Store) RenameSection(ctx context.Context, userID, sectionID, name string) error {
	_, err := s.db.Exec(ctx, `UPDATE chat_sections SET name = $3 WHERE id = $1 AND user_id = $2`, sectionID, userID, name)
	if nameTaken(err) {
		return ErrSectionNameTaken
	}
	if err != nil {
		return fmt.Errorf("rename section: %w", err)
	}
	return nil
}

// SwapSections exchanges the positions of two of a user's sections.
func (s *Store) SwapSections(ctx context.Context, userID, a, b string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE chat_sections cs SET position = o.position
		 FROM chat_sections o
		 WHERE cs.user_id = $1 AND o.user_id = $1
		   AND ((cs.id = $2 AND o.id = $3) OR (cs.id = $3 AND o.id = $2))`, userID, a, b)
	if err != nil {
		return fmt.Errorf("reorder sections: %w", err)
	}
	return nil
}

// DeleteSection removes a user's section; its conversations return to the default lists.
func (s *Store) DeleteSection(ctx context.Context, userID, sectionID string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM chat_sections WHERE id = $1 AND user_id = $2`, sectionID, userID); err != nil {
		return fmt.Errorf("delete section: %w", err)
	}
	return nil
}

func placeChannel(ctx context.Context, x execer, userID, sectionID, channelID string) error {
	_, err := x.Exec(ctx,
		`INSERT INTO chat_section_items (section_id, channel_id, user_id) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, channel_id) DO UPDATE SET section_id = EXCLUDED.section_id`,
		sectionID, channelID, userID)
	return err
}

// PlaceChannel puts a conversation in a user's section, moving it out of any other.
func (s *Store) PlaceChannel(ctx context.Context, userID, sectionID, channelID string) error {
	if err := placeChannel(ctx, s.db, userID, sectionID, channelID); err != nil {
		return fmt.Errorf("place channel: %w", err)
	}
	return nil
}

// UnplaceChannel takes a conversation out of whichever of the user's sections holds it.
func (s *Store) UnplaceChannel(ctx context.Context, userID, channelID string) error {
	if _, err := s.db.Exec(ctx, `DELETE FROM chat_section_items WHERE user_id = $1 AND channel_id = $2`, userID, channelID); err != nil {
		return fmt.Errorf("unplace channel: %w", err)
	}
	return nil
}
