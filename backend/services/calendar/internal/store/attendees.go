package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Attendee returns one attendee of an event; a person who is not one is
// ErrNotFound.
func (s *Store) Attendee(ctx context.Context, eventID, userNodeID string) (*Attendee, error) {
	a := &Attendee{}
	err := s.pool.QueryRow(ctx,
		`SELECT user_node_id, response, comment, is_organizer FROM calendar_attendees
		  WHERE event_id = $1 AND user_node_id = $2`, eventID, userNodeID,
	).Scan(&a.UserNodeID, &a.Response, &a.Comment, &a.IsOrganizer)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("attendee: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("attendee: %w", err)
	}
	return a, nil
}

// AddAttendees adds guests to an event. A person who is already an attendee keeps
// their row and their answer.
func (s *Store) AddAttendees(ctx context.Context, eventID string, add []Attendee) error {
	if len(add) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("add attendees: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, a := range add {
		var responded *time.Time
		if a.Response != "needs_action" {
			now := time.Now()
			responded = &now
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO calendar_attendees (event_id, user_node_id, response, comment, is_organizer, responded_at)
			 VALUES ($1, $2, $3, $4, FALSE, $5) ON CONFLICT (event_id, user_node_id) DO NOTHING`,
			eventID, a.UserNodeID, a.Response, a.Comment, responded); err != nil {
			return fmt.Errorf("add attendee: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("add attendees: %w", err)
	}
	return nil
}

// RemoveAttendees removes guests with their answers. The organizer is never
// removed.
func (s *Store) RemoveAttendees(ctx context.Context, eventID string, nodes []string) error {
	if len(nodes) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("remove attendees: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`DELETE FROM calendar_attendee_occurrences WHERE event_id = $1 AND user_node_id = ANY($2)
		   AND user_node_id NOT IN (SELECT user_node_id FROM calendar_attendees WHERE event_id = $1 AND is_organizer)`,
		eventID, nodes); err != nil {
		return fmt.Errorf("remove answers: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM calendar_attendees WHERE event_id = $1 AND user_node_id = ANY($2) AND NOT is_organizer`,
		eventID, nodes); err != nil {
		return fmt.Errorf("remove attendees: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("remove attendees: %w", err)
	}
	return nil
}

// SetResponse records an attendee's answer for the whole event or series and
// forgets their answers to single occurrences. A person who is not an attendee is
// ErrNotFound.
func (s *Store) SetResponse(ctx context.Context, eventID, userNodeID, response, comment string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("set response: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`UPDATE calendar_attendees SET response = $3, comment = $4, responded_at = NOW()
		  WHERE event_id = $1 AND user_node_id = $2`, eventID, userNodeID, response, comment)
	if err != nil {
		return fmt.Errorf("set response: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("attendee: %w", ErrNotFound)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM calendar_attendee_occurrences WHERE event_id = $1 AND user_node_id = $2`, eventID, userNodeID); err != nil {
		return fmt.Errorf("forget occurrence answers: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("set response: %w", err)
	}
	return nil
}

// SetOccurrenceResponse records an attendee's answer for the occurrence of a
// series that stands at original in it.
func (s *Store) SetOccurrenceResponse(ctx context.Context, eventID, userNodeID string, original time.Time, response, comment string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO calendar_attendee_occurrences (event_id, user_node_id, original_start, response, comment)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (event_id, user_node_id, original_start)
		 DO UPDATE SET response = EXCLUDED.response, comment = EXCLUDED.comment, responded_at = NOW()`,
		eventID, userNodeID, original, response, comment)
	if err != nil {
		return fmt.Errorf("set occurrence response: %w", err)
	}
	return nil
}

// OccKey names one occurrence of one series: the instant is the occurrence's
// original start in Unix nanoseconds.
type OccKey struct {
	EventID string
	At      int64
}

// OccurrenceResponses returns the person's answers to single occurrences of the
// given series.
func (s *Store) OccurrenceResponses(ctx context.Context, userNodeID string, eventIDs []string) (map[OccKey]string, error) {
	out := map[OccKey]string{}
	if len(eventIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT event_id, original_start, response FROM calendar_attendee_occurrences
		  WHERE user_node_id = $1 AND event_id = ANY($2)`, userNodeID, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("occurrence responses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, resp string
		var at time.Time
		if err := rows.Scan(&id, &at, &resp); err != nil {
			return nil, fmt.Errorf("scan occurrence response: %w", err)
		}
		out[OccKey{EventID: id, At: at.UnixNano()}] = resp
	}
	return out, rows.Err()
}

// OccurrenceAnswers returns every attendee's answer to one occurrence of a
// series, keyed by person.
func (s *Store) OccurrenceAnswers(ctx context.Context, eventID string, original time.Time) (map[string]*Attendee, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_node_id, response, comment FROM calendar_attendee_occurrences
		  WHERE event_id = $1 AND original_start = $2`, eventID, original)
	if err != nil {
		return nil, fmt.Errorf("occurrence answers: %w", err)
	}
	defer rows.Close()
	out := map[string]*Attendee{}
	for rows.Next() {
		a := &Attendee{}
		if err := rows.Scan(&a.UserNodeID, &a.Response, &a.Comment); err != nil {
			return nil, fmt.Errorf("scan occurrence answer: %w", err)
		}
		out[a.UserNodeID] = a
	}
	return out, rows.Err()
}

// SetEventNodes records the graph nodes of an event that has guests or a call, or
// clears them (empty ids) once it has neither.
func (s *Store) SetEventNodes(ctx context.Context, eventID, oaID, uaID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE calendar_events SET ngac_oa_id = NULLIF($2, ''), guests_ua_id = NULLIF($3, '') WHERE id = $1`,
		eventID, oaID, uaID)
	if err != nil {
		return fmt.Errorf("set event nodes: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("event: %w", ErrNotFound)
	}
	return nil
}

// ChannelMembers lists the user nodes the conversation's member list holds. It is
// a cache of the graph: a caller that acts on it verifies each against the policy.
func (s *Store) ChannelMembers(ctx context.Context, channelID string) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT ngac_node_id FROM channel_members WHERE channel_id = $1 ORDER BY ngac_node_id`, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel members: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan channel member: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// OutOfOffice reports which of the people have an out-of-office event of their own
// personal calendar that overlaps [from, to).
func (s *Store) OutOfOffice(ctx context.Context, workspaceID string, nodes []string, from, to time.Time) (map[string]bool, error) {
	out := map[string]bool{}
	if len(nodes) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT DISTINCT c.owner_node_id
		   FROM calendar_events e JOIN calendars c ON c.id = e.calendar_id
		  WHERE c.workspace_id = $1 AND c.kind = 'personal' AND c.owner_node_id = ANY($2)
		    AND e.kind = 'ooo' AND e.recurring_event_id IS NULL AND e.rrule IS NULL
		    AND e.starts_at < $4 AND e.ends_at > $3`, workspaceID, nodes, from, to)
	if err != nil {
		return nil, fmt.Errorf("out of office: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("scan out of office: %w", err)
		}
		out[n] = true
	}
	return out, rows.Err()
}
