package store

import (
	"context"
	"fmt"
	"strings"
)

// Settings are a person's calendar preferences in a workspace.
type Settings struct {
	TZID string
	// WorkStart and WorkEnd are HH:mm in TZID.
	WorkStart       string
	WorkEnd         string
	DefaultReminder int
}

// DefaultSettings are what a person who never saved any has.
func DefaultSettings() Settings {
	return Settings{TZID: "Asia/Ho_Chi_Minh", WorkStart: "08:00", WorkEnd: "17:30", DefaultReminder: DefaultReminderMinutes}
}

// Settings reads the settings of the people, keyed by node; someone who never saved
// any has DefaultSettings.
func (s *Store) Settings(ctx context.Context, workspaceID string, nodes []string) (map[string]Settings, error) {
	out := map[string]Settings{}
	for _, n := range nodes {
		out[n] = DefaultSettings()
	}
	if len(nodes) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT user_node_id, tzid, to_char(work_start, 'HH24:MI'), to_char(work_end, 'HH24:MI'), default_reminder
		   FROM calendar_user_settings WHERE workspace_id = $1 AND user_node_id = ANY($2)`, workspaceID, nodes)
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var node string
		var st Settings
		if err := rows.Scan(&node, &st.TZID, &st.WorkStart, &st.WorkEnd, &st.DefaultReminder); err != nil {
			return nil, fmt.Errorf("scan settings: %w", err)
		}
		out[node] = st
	}
	return out, rows.Err()
}

// SaveSettings stores a person's settings for a workspace.
func (s *Store) SaveSettings(ctx context.Context, workspaceID, userNodeID string, st Settings) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO calendar_user_settings (user_node_id, workspace_id, tzid, work_start, work_end, default_reminder)
		 VALUES ($1, $2, $3, $4::time, $5::time, $6)
		 ON CONFLICT (user_node_id, workspace_id) DO UPDATE
		   SET tzid = EXCLUDED.tzid, work_start = EXCLUDED.work_start, work_end = EXCLUDED.work_end,
		       default_reminder = EXCLUDED.default_reminder`,
		userNodeID, workspaceID, st.TZID, st.WorkStart, st.WorkEnd, st.DefaultReminder)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// likePattern turns a search term into an ILIKE pattern that matches it anywhere,
// with the pattern characters in the term taken literally.
func likePattern(term string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(term) + "%"
}

// SearchEvents finds single events and series whose title, place, description or
// guests' names contain the term, among those kept on the calendars or attended by
// the person, newest first. It makes no access decision: the caller keeps only
// what the person may read.
func (s *Store) SearchEvents(ctx context.Context, calendarIDs []string, userNodeID, term string, limit int) ([]*Event, error) {
	rows, err := s.pool.Query(ctx,
		eventSelect+` WHERE e.recurring_event_id IS NULL
		   AND (e.calendar_id = ANY($1)
		        OR e.id IN (SELECT event_id FROM calendar_attendees WHERE user_node_id = $2))
		   AND (e.title ILIKE $3 ESCAPE '\' OR e.location ILIKE $3 ESCAPE '\' OR e.description ILIKE $3 ESCAPE '\'
		        OR EXISTS (SELECT 1 FROM calendar_attendees a
		                     JOIN tenant_users tu ON tu.ngac_node_id = a.user_node_id AND tu.tenant_id = e.workspace_id
		                     JOIN users u ON u.id = tu.user_id
		                    WHERE a.event_id = e.id
		                      AND (u.display_name ILIKE $3 ESCAPE '\' OR u.username ILIKE $3 ESCAPE '\')))
		 ORDER BY e.starts_at DESC, e.id LIMIT $4`, calendarIDs, userNodeID, likePattern(term), limit)
	if err != nil {
		return nil, fmt.Errorf("search events: %w", err)
	}
	return collectEvents(rows)
}
