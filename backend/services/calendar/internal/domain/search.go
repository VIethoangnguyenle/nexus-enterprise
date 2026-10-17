package domain

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/services/calendar/internal/store"
)

const (
	// searchLimit is the most events a search answers.
	searchLimit = 50
	// searchCandidates is how many rows the database returns before the access
	// rules drop the ones the caller may not see.
	searchCandidates = 4 * searchLimit
	maxSearchRunes   = 100
)

// SearchEvents finds events whose title, place, description or guests' names
// contain the text (case-insensitively), among the events of the calendars the
// caller reads and the ones they are invited to. A repeating event answers with
// its next occurrence (its last, when it is over). An event the caller may not read
// in full never matches, and a guest's name is searched only where the caller sees
// the guest list.
func (s *Service) SearchEvents(ctx context.Context, wsID, q string) ([]*OccurrenceView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	term := strings.TrimSpace(q)
	if term == "" {
		return []*OccurrenceView{}, nil
	}
	if utf8.RuneCountInString(term) > maxSearchRunes {
		return nil, invalid("a search is at most %d characters", maxSearchRunes)
	}

	cals, _, held, err := s.callerCalendars(ctx, who, ws)
	if err != nil {
		return nil, err
	}
	byID := map[string]*store.Calendar{}
	levels := map[string]string{}
	var readable []string
	for _, c := range cals {
		byID[c.ID] = c
		levels[c.ID] = accessTo(held, c)
		if canRead(levels[c.ID]) {
			readable = append(readable, c.ID)
		}
	}
	rows, err := s.store.SearchEvents(ctx, readable, who.NGACNodeID, term, searchCandidates)
	if err != nil {
		return nil, internal("search events", err)
	}

	// Rows on calendars the caller does not list are there because they are a guest.
	ids := make([]string, len(rows))
	var strangers []*store.Calendar
	var oas []string
	for i, r := range rows {
		ids[i] = r.ID
		if byID[r.CalendarID] == nil {
			c, err := s.loadCalendar(ctx, ws.ID, r.CalendarID)
			if err != nil {
				return nil, err
			}
			byID[c.ID] = c
			strangers = append(strangers, c)
		}
		if r.NgacOAID != "" {
			oas = append(oas, r.NgacOAID)
		}
	}
	extra, err := s.accessLevels(ctx, who.NGACNodeID, strangers)
	if err != nil {
		return nil, err
	}
	for _, c := range strangers {
		levels[c.ID] = accessTo(extra, c)
	}
	perms, err := policyclient.New(s.policyRead).BatchCheck(ctx, who.NGACNodeID, oas, []string{ngac.OpRead})
	if err != nil {
		return nil, internal("check invitations", err)
	}
	mine, err := s.store.MyResponses(ctx, who.NGACNodeID, ids)
	if err != nil {
		return nil, internal("load answers", err)
	}

	needle := strings.ToLower(term)
	insts := make([]instance, 0, searchLimit)
	for _, r := range rows {
		cal, level := byID[r.CalendarID], levels[r.CalendarID]
		_, attending := mine[r.ID]
		invited := attending && r.NgacOAID != "" && perms.Has(r.NgacOAID, ngac.OpRead)
		if !showsDetail(level, r, cal, who.NGACNodeID, attending) && !invited {
			continue
		}
		if !mentions(r, needle) && !(canWrite(level) || r.OrganizerNodeID == who.NGACNodeID || r.GuestsCanSeeList) {
			continue // matched only through a guest list the caller does not see
		}
		in, ok, err := s.shownOccurrence(ctx, r)
		if err != nil {
			return nil, err
		}
		if ok {
			insts = append(insts, in)
		}
	}
	now := s.now()
	sort.SliceStable(insts, func(i, j int) bool {
		a, b := insts[i], insts[j]
		aUp, bUp := a.End.After(now), b.End.After(now)
		switch {
		case aUp != bUp:
			return aUp
		case aUp:
			return a.Start.Before(b.Start)
		}
		return a.Start.After(b.Start)
	})
	if len(insts) > searchLimit {
		insts = insts[:searchLimit]
	}
	v, err := s.loadViewData(ctx, ws.ID, who.NGACNodeID, insts)
	if err != nil {
		return nil, err
	}
	out := make([]*OccurrenceView, 0, len(insts))
	for _, in := range insts {
		out = append(out, occurrenceView(in, levels[in.Master.CalendarID], true, v))
	}
	return out, nil
}

// mentions reports whether the event's own text holds the (lower-cased) needle.
func mentions(e *store.Event, needle string) bool {
	for _, field := range []string{e.Title, e.Location, e.Description} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

// shownOccurrence picks the occurrence a search answers with: the event itself
// when single, else the next occurrence from now, else the last one. ok is false
// for a series with nothing to show.
func (s *Service) shownOccurrence(ctx context.Context, m *store.Event) (instance, bool, error) {
	if m.RRule == "" {
		return instance{Row: m, Master: m, Original: m.StartsAt, Start: m.StartsAt, End: m.EndsAt}, true, nil
	}
	overrides, err := s.store.Overrides(ctx, m.ID)
	if err != nil {
		return instance{}, false, internal("load exceptions", err)
	}
	now := s.now()
	for _, window := range [][2]time.Time{{now, now.AddDate(0, 0, maxRangeDays)}, {m.StartsAt.Add(-time.Hour), now}} {
		got, err := expandEvents([]*store.Event{m}, overrides, window[0], window[1])
		if err != nil {
			return instance{}, false, internal("expand events", err)
		}
		if len(got) == 0 {
			continue
		}
		if window[0].Equal(now) {
			return got[0], true, nil
		}
		return got[len(got)-1], true, nil
	}
	return instance{}, false, nil
}
