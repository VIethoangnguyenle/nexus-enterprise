package store

import (
	"context"
	"fmt"
	"time"
)

// ReminderSpec is one reminder of an event: the event's own (UserNodeID empty) or
// a person's override of it.
type ReminderSpec struct {
	UserNodeID string
	Minutes    int
}

// ReminderEvents lists, across every workspace, the events that carry reminders
// and may have an occurrence overlapping [from, to), with the exceptions of their
// series, as RangeEvents does.
func (s *Store) ReminderEvents(ctx context.Context, from, to time.Time) (masters, overrides []*Event, err error) {
	// $1 is unused by the choice of events (all of them); it is a typed zero so the
	// statement's parameters are all known to the server.
	return s.rangeEvents(ctx,
		`$1::int = 0 AND EXISTS (SELECT 1 FROM calendar_reminders r WHERE r.event_id = e.id)`,
		`$1::int = 0`, 0, from, to)
}

// ReminderSpecs reads the reminders of some events (their series), by event.
func (s *Store) ReminderSpecs(ctx context.Context, eventIDs []string) (map[string][]ReminderSpec, error) {
	out := map[string][]ReminderSpec{}
	if len(eventIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT event_id, COALESCE(user_node_id, ''), minutes_before FROM calendar_reminders
		  WHERE event_id = ANY($1) ORDER BY event_id, minutes_before DESC`, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("reminder specs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var r ReminderSpec
		if err := rows.Scan(&id, &r.UserNodeID, &r.Minutes); err != nil {
			return nil, fmt.Errorf("scan reminder spec: %w", err)
		}
		out[id] = append(out[id], r)
	}
	return out, rows.Err()
}

// ClaimReminder records that a reminder is being delivered to a person for one
// occurrence. It is true for exactly one caller however many services run it at
// once: the one whose insert took the row.
func (s *Store) ClaimReminder(ctx context.Context, eventID, userNodeID string, occurrence time.Time, minutes int) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`INSERT INTO calendar_reminder_deliveries (event_id, user_node_id, occurrence_start, minutes_before)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, eventID, userNodeID, occurrence, minutes)
	if err != nil {
		return false, fmt.Errorf("claim reminder: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
