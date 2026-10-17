package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrStale: the row was changed by someone else since the caller read it (its
// version is not the one the caller holds).
var ErrStale = errors.New("stale version")

// Event is one row of calendar_events: a single event, the master of a repeating
// series, or an exception that replaces (or cancels) one occurrence of a series.
type Event struct {
	ID              string
	WorkspaceID     string
	CalendarID      string
	OrganizerNodeID string
	Title           string
	Description     string
	Location        string
	StartsAt        time.Time
	EndsAt          time.Time
	// AllDay events keep their dates as midnights of TZID; EndsAt is exclusive.
	AllDay bool
	// TZID is the zone the event was made in; a repeating event expands in it.
	TZID string
	// RRule is the RFC 5545 rule body ("FREQ=WEEKLY;BYDAY=WE"), empty for none.
	RRule string
	// RecurrenceUntil bounds the last instance's end, so a range query can skip
	// a series that has finished.
	RecurrenceUntil *time.Time
	// RecurringEventID and OriginalStart name the series and the occurrence an
	// exception row replaces.
	RecurringEventID string
	OriginalStart    *time.Time
	Cancelled        bool
	Kind             string
	Transparency     string
	Visibility       string
	GuestsCanInvite  bool
	GuestsCanSeeList bool
	HasCall          bool
	// NgacOAID and GuestsUAID exist only once the event has guests or a call.
	NgacOAID   string
	GuestsUAID string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SeriesID is the row that owns the guests and reminders of this one: the series
// for an exception, the event itself otherwise.
func (e *Event) SeriesID() string {
	if e.RecurringEventID != "" {
		return e.RecurringEventID
	}
	return e.ID
}

const eventColumns = `e.id, e.workspace_id, e.calendar_id, e.organizer_node_id, e.title, e.description, e.location,
	e.starts_at, e.ends_at, e.all_day, e.tzid, COALESCE(e.rrule, ''), e.recurrence_until,
	COALESCE(e.recurring_event_id, ''), e.original_start, e.cancelled, e.kind, e.transparency, e.visibility,
	e.guests_can_invite, e.guests_can_see_list, e.has_call, COALESCE(e.ngac_oa_id, ''), COALESCE(e.guests_ua_id, ''),
	e.version, e.created_at, e.updated_at`

const eventSelect = `SELECT ` + eventColumns + ` FROM calendar_events e`

func scanEvent(row interface{ Scan(...any) error }) (*Event, error) {
	e := &Event{}
	err := row.Scan(&e.ID, &e.WorkspaceID, &e.CalendarID, &e.OrganizerNodeID, &e.Title, &e.Description, &e.Location,
		&e.StartsAt, &e.EndsAt, &e.AllDay, &e.TZID, &e.RRule, &e.RecurrenceUntil,
		&e.RecurringEventID, &e.OriginalStart, &e.Cancelled, &e.Kind, &e.Transparency, &e.Visibility,
		&e.GuestsCanInvite, &e.GuestsCanSeeList, &e.HasCall, &e.NgacOAID, &e.GuestsUAID,
		&e.Version, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func collectEvents(rows pgx.Rows) ([]*Event, error) {
	defer rows.Close()
	var out []*Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EventByID reads one event row. One that does not exist is ErrNotFound.
func (s *Store) EventByID(ctx context.Context, id string) (*Event, error) {
	e, err := scanEvent(s.pool.QueryRow(ctx, eventSelect+` WHERE e.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("event: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("event: %w", err)
	}
	return e, nil
}

// OverrideOf reads the exception row of a series at one occurrence; none is
// ErrNotFound.
func (s *Store) OverrideOf(ctx context.Context, seriesID string, original time.Time) (*Event, error) {
	e, err := scanEvent(s.pool.QueryRow(ctx,
		eventSelect+` WHERE e.recurring_event_id = $1 AND e.original_start = $2`, seriesID, original))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("override: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("override: %w", err)
	}
	return e, nil
}

// Overrides lists every exception row of a series, oldest occurrence first.
func (s *Store) Overrides(ctx context.Context, seriesID string) ([]*Event, error) {
	rows, err := s.pool.Query(ctx,
		eventSelect+` WHERE e.recurring_event_id = $1 ORDER BY e.original_start, e.id`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("overrides: %w", err)
	}
	return collectEvents(rows)
}

// RangeEvents returns what can show inside [from, to) on the calendars: the
// single events and series masters that may reach it, and the exception rows of
// those series, plus any exception that was moved into the range (with its
// series, whichever time that starts at).
func (s *Store) RangeEvents(ctx context.Context, calendarIDs []string, from, to time.Time) (masters, overrides []*Event, err error) {
	if len(calendarIDs) == 0 {
		return nil, nil, nil
	}
	rows, err := s.pool.Query(ctx,
		eventSelect+` WHERE e.calendar_id = ANY($1) AND e.recurring_event_id IS NULL AND e.starts_at < $3
		   AND ((e.rrule IS NULL AND e.ends_at >= $2)
		     OR (e.rrule IS NOT NULL AND (e.recurrence_until IS NULL OR e.recurrence_until >= $2)))
		 ORDER BY e.starts_at, e.id`, calendarIDs, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("range events: %w", err)
	}
	if masters, err = collectEvents(rows); err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	ids := make([]string, 0, len(masters))
	for _, m := range masters {
		known[m.ID] = true
		ids = append(ids, m.ID)
	}
	rows, err = s.pool.Query(ctx,
		eventSelect+` WHERE e.recurring_event_id IS NOT NULL AND (e.recurring_event_id = ANY($1)
		   OR (e.calendar_id = ANY($2) AND e.starts_at < $4 AND e.ends_at >= $3))
		 ORDER BY e.original_start, e.id`, ids, calendarIDs, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("range overrides: %w", err)
	}
	if overrides, err = collectEvents(rows); err != nil {
		return nil, nil, err
	}
	var missing []string
	for _, o := range overrides {
		if !known[o.RecurringEventID] {
			known[o.RecurringEventID] = true
			missing = append(missing, o.RecurringEventID)
		}
	}
	if len(missing) > 0 {
		rows, err = s.pool.Query(ctx, eventSelect+` WHERE e.id = ANY($1)`, missing)
		if err != nil {
			return nil, nil, fmt.Errorf("range masters: %w", err)
		}
		more, err := collectEvents(rows)
		if err != nil {
			return nil, nil, err
		}
		masters = append(masters, more...)
	}
	return masters, overrides, nil
}

// InsertEvent stores an event with its default reminders (minutes before). A
// single event or series master also gets its organizer as the first attendee. A
// second exception for the same occurrence is ErrStale: someone else made it
// first.
func (s *Store) InsertEvent(ctx context.Context, e *Event, reminders []int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := insertEventTx(ctx, tx, e, reminders, e.RecurringEventID == ""); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

func insertEventTx(ctx context.Context, tx pgx.Tx, e *Event, reminders []int, organizer bool) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO calendar_events (id, workspace_id, calendar_id, organizer_node_id, title, description, location,
		    starts_at, ends_at, all_day, tzid, rrule, recurrence_until, recurring_event_id, original_start, cancelled,
		    kind, transparency, visibility, guests_can_invite, guests_can_see_list, has_call)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,''),$13,NULLIF($14,''),$15,$16,$17,$18,$19,$20,$21,$22)
		 RETURNING version, created_at, updated_at`,
		e.ID, e.WorkspaceID, e.CalendarID, e.OrganizerNodeID, e.Title, e.Description, e.Location,
		e.StartsAt, e.EndsAt, e.AllDay, e.TZID, e.RRule, e.RecurrenceUntil, e.RecurringEventID, e.OriginalStart, e.Cancelled,
		e.Kind, e.Transparency, e.Visibility, e.GuestsCanInvite, e.GuestsCanSeeList, e.HasCall,
	).Scan(&e.Version, &e.CreatedAt, &e.UpdatedAt)
	if isUniqueViolation(err) {
		return fmt.Errorf("insert event: %w", ErrStale)
	}
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	if organizer {
		if _, err := tx.Exec(ctx,
			`INSERT INTO calendar_attendees (event_id, user_node_id, response, is_organizer, responded_at)
			 VALUES ($1, $2, 'accepted', TRUE, NOW())`, e.ID, e.OrganizerNodeID); err != nil {
			return fmt.Errorf("insert organizer: %w", err)
		}
	}
	return writeReminders(ctx, tx, e.ID, reminders)
}

func writeReminders(ctx context.Context, tx pgx.Tx, eventID string, minutes []int) error {
	for _, m := range minutes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO calendar_reminders (event_id, user_node_id, minutes_before) VALUES ($1, NULL, $2)
			 ON CONFLICT DO NOTHING`, eventID, m); err != nil {
			return fmt.Errorf("insert reminder: %w", err)
		}
	}
	return nil
}

// EventChange is what UpdateEvent writes besides the row itself.
type EventChange struct {
	// Reminders replaces the event's default reminders when not nil.
	Reminders *[]int
	// Originals moves the original_start of exception rows (by their ID), so a
	// series that was shifted keeps its cancellations and edits.
	Originals map[string]time.Time
	// DropOverrides deletes the exceptions of a series that stopped repeating.
	DropOverrides bool
}

// UpdateEvent writes the mutable columns of the row and bumps its version, only
// when the row is still at version. The row moves calendars together with its
// exceptions. A row changed since is ErrStale; a row that is gone is ErrNotFound.
func (s *Store) UpdateEvent(ctx context.Context, e *Event, version int, ch EventChange) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := updateEventTx(ctx, tx, e, version); err != nil {
		return err
	}
	if e.RecurringEventID == "" {
		if _, err := tx.Exec(ctx, `UPDATE calendar_events SET calendar_id = $2 WHERE recurring_event_id = $1 AND calendar_id <> $2`,
			e.ID, e.CalendarID); err != nil {
			return fmt.Errorf("move exceptions: %w", err)
		}
	}
	if ch.DropOverrides {
		if _, err := tx.Exec(ctx, `DELETE FROM calendar_events WHERE recurring_event_id = $1`, e.ID); err != nil {
			return fmt.Errorf("drop exceptions: %w", err)
		}
	}
	if len(ch.Originals) > 0 {
		// Two steps: the unique index would refuse a shift onto an occurrence that
		// has not moved yet.
		for id := range ch.Originals {
			if _, err := tx.Exec(ctx, `UPDATE calendar_events SET original_start = NULL WHERE id = $1`, id); err != nil {
				return fmt.Errorf("shift exceptions: %w", err)
			}
		}
		for id, at := range ch.Originals {
			if _, err := tx.Exec(ctx, `UPDATE calendar_events SET original_start = $2 WHERE id = $1`, id, at); err != nil {
				return fmt.Errorf("shift exceptions: %w", err)
			}
		}
	}
	if ch.Reminders != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM calendar_reminders WHERE event_id = $1 AND user_node_id IS NULL`, e.ID); err != nil {
			return fmt.Errorf("replace reminders: %w", err)
		}
		if err := writeReminders(ctx, tx, e.ID, *ch.Reminders); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

func updateEventTx(ctx context.Context, tx pgx.Tx, e *Event, version int) error {
	err := tx.QueryRow(ctx,
		`UPDATE calendar_events SET calendar_id = $3, title = $4, description = $5, location = $6, starts_at = $7,
		    ends_at = $8, all_day = $9, tzid = $10, rrule = NULLIF($11, ''), recurrence_until = $12, cancelled = $13,
		    kind = $14, transparency = $15, visibility = $16, guests_can_invite = $17, guests_can_see_list = $18,
		    has_call = $19, version = version + 1, updated_at = NOW()
		  WHERE id = $1 AND version = $2
		 RETURNING version, updated_at`,
		e.ID, version, e.CalendarID, e.Title, e.Description, e.Location, e.StartsAt,
		e.EndsAt, e.AllDay, e.TZID, e.RRule, e.RecurrenceUntil, e.Cancelled,
		e.Kind, e.Transparency, e.Visibility, e.GuestsCanInvite, e.GuestsCanSeeList,
		e.HasCall,
	).Scan(&e.Version, &e.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var one int
		if perr := tx.QueryRow(ctx, `SELECT 1 FROM calendar_events WHERE id = $1`, e.ID).Scan(&one); errors.Is(perr, pgx.ErrNoRows) {
			return fmt.Errorf("event: %w", ErrNotFound)
		}
		return fmt.Errorf("event: %w", ErrStale)
	}
	if err != nil {
		return fmt.Errorf("update event: %w", err)
	}
	return nil
}

// CutSeries ends a series before cutFrom: it writes the series row (whose rule the
// caller has already ended) at version, drops the exceptions from cutFrom on, and
// stores tail, when there is one, as a new event that takes over the guests and,
// unless tailReminders is set, the reminders.
func (s *Store) CutSeries(ctx context.Context, series *Event, version int, cutFrom time.Time, tail *Event, tailReminders *[]int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cut series: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := updateEventTx(ctx, tx, series, version); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM calendar_events WHERE recurring_event_id = $1 AND original_start >= $2`,
		series.ID, cutFrom); err != nil {
		return fmt.Errorf("drop later exceptions: %w", err)
	}
	if tail != nil {
		if err := insertEventTx(ctx, tx, tail, nil, false); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO calendar_attendees (event_id, user_node_id, response, comment, is_organizer, responded_at)
			 SELECT $2, user_node_id, response, comment, is_organizer, responded_at
			   FROM calendar_attendees WHERE event_id = $1`, series.ID, tail.ID); err != nil {
			return fmt.Errorf("copy guests: %w", err)
		}
		if tailReminders != nil {
			err = writeReminders(ctx, tx, tail.ID, *tailReminders)
		} else {
			_, err = tx.Exec(ctx,
				`INSERT INTO calendar_reminders (event_id, user_node_id, minutes_before)
				 SELECT $2, user_node_id, minutes_before FROM calendar_reminders WHERE event_id = $1`, series.ID, tail.ID)
		}
		if err != nil {
			return fmt.Errorf("copy reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cut series: %w", err)
	}
	return nil
}

// DeleteEvent removes the event with its exceptions, guests and reminders. A row
// that is already gone is not an error.
func (s *Store) DeleteEvent(ctx context.Context, id string) error {
	if _, err := s.pool.Exec(ctx, `DELETE FROM calendar_events WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return nil
}

// Attendee is one guest (or the organizer) of an event.
type Attendee struct {
	UserNodeID  string
	Response    string
	Comment     string
	IsOrganizer bool
}

// Attendees lists the guests of an event, the organizer first.
func (s *Store) Attendees(ctx context.Context, eventID string) ([]*Attendee, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT user_node_id, response, comment, is_organizer FROM calendar_attendees
		  WHERE event_id = $1 ORDER BY is_organizer DESC, user_node_id`, eventID)
	if err != nil {
		return nil, fmt.Errorf("attendees: %w", err)
	}
	defer rows.Close()
	var out []*Attendee
	for rows.Next() {
		a := &Attendee{}
		if err := rows.Scan(&a.UserNodeID, &a.Response, &a.Comment, &a.IsOrganizer); err != nil {
			return nil, fmt.Errorf("scan attendee: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// MyResponses returns the person's answer to each of the events they are an
// attendee of, keyed by event.
func (s *Store) MyResponses(ctx context.Context, userNodeID string, eventIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(eventIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT event_id, response FROM calendar_attendees WHERE user_node_id = $1 AND event_id = ANY($2)`,
		userNodeID, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("my responses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, resp string
		if err := rows.Scan(&id, &resp); err != nil {
			return nil, fmt.Errorf("scan response: %w", err)
		}
		out[id] = resp
	}
	return out, rows.Err()
}

// GuestCounts counts the guests of each event, the organizer excluded.
func (s *Store) GuestCounts(ctx context.Context, eventIDs []string) (map[string]int, error) {
	out := map[string]int{}
	if len(eventIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT event_id, COUNT(*) FROM calendar_attendees
		  WHERE event_id = ANY($1) AND NOT is_organizer GROUP BY event_id`, eventIDs)
	if err != nil {
		return nil, fmt.Errorf("guest counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("scan guest count: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// Reminders lists the event's default reminders (minutes before), longest first.
func (s *Store) Reminders(ctx context.Context, eventID string) ([]int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT minutes_before FROM calendar_reminders
		  WHERE event_id = $1 AND user_node_id IS NULL ORDER BY minutes_before DESC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("reminders: %w", err)
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var m int
		if err := rows.Scan(&m); err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DefaultReminder is the minutes before a new event the person asked to be
// reminded, 0 for none. Someone who never set it gets ten minutes, the column's
// own default.
func (s *Store) DefaultReminder(ctx context.Context, workspaceID, userNodeID string) (int, error) {
	var m int
	err := s.pool.QueryRow(ctx,
		`SELECT default_reminder FROM calendar_user_settings WHERE workspace_id = $1 AND user_node_id = $2`,
		workspaceID, userNodeID).Scan(&m)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultReminderMinutes, nil
	}
	if err != nil {
		return 0, fmt.Errorf("default reminder: %w", err)
	}
	return m, nil
}

// DefaultReminderMinutes is what calendar_user_settings.default_reminder starts at.
const DefaultReminderMinutes = 10
