package domain

import (
	"sort"
	"time"

	"ngac-platform/services/calendar/internal/store"
)

// instance is one occurrence of an event inside a range: what a grid draws.
type instance struct {
	// Row is the row whose fields the occurrence shows: the exception row for a
	// changed occurrence, else the event itself.
	Row *store.Event
	// Master is the single event or the series the occurrence belongs to.
	Master *store.Event
	// Original is the occurrence's place in its series (its identity); for an
	// exception it is where the series had it.
	Original time.Time
	Start    time.Time
	End      time.Time
}

// recurrenceOf builds the expansion rule of a repeating event, nil for a single
// one. A stored rule that no longer parses is an error of ours.
func recurrenceOf(m *store.Event) (*recurrence, error) {
	if m.RRule == "" {
		return nil, nil
	}
	loc, err := loadZone(m.TZID)
	if err != nil {
		return nil, err
	}
	return newRecurrence(m.RRule, m.StartsAt, m.EndsAt, m.AllDay, loc)
}

// expandEvents lists the occurrences that overlap [from, to): single events as
// they are, series expanded in their own zone, each occurrence an exception
// replaces swapped for the exception, and cancelled ones dropped. An exception
// that was moved shows where it now is, whatever the series says.
func expandEvents(masters, overrides []*store.Event, from, to time.Time) ([]instance, error) {
	byMaster := map[string][]*store.Event{}
	for _, o := range overrides {
		byMaster[o.RecurringEventID] = append(byMaster[o.RecurringEventID], o)
	}
	var out []instance
	for _, m := range masters {
		ru, err := recurrenceOf(m)
		if err != nil {
			return nil, err
		}
		if ru == nil {
			if overlaps(m.StartsAt, m.EndsAt, from, to) {
				out = append(out, instance{Row: m, Master: m, Original: m.StartsAt, Start: m.StartsAt, End: m.EndsAt})
			}
			continue
		}
		replaced := map[int64]bool{}
		for _, o := range byMaster[m.ID] {
			if o.OriginalStart != nil {
				replaced[o.OriginalStart.UnixNano()] = true
			}
		}
		for _, sp := range ru.between(from, to) {
			if !replaced[sp.start.UnixNano()] {
				out = append(out, instance{Row: m, Master: m, Original: sp.start, Start: sp.start, End: sp.end})
			}
		}
		for _, o := range byMaster[m.ID] {
			if o.Cancelled || o.OriginalStart == nil || !overlaps(o.StartsAt, o.EndsAt, from, to) {
				continue
			}
			out = append(out, instance{Row: o, Master: m, Original: *o.OriginalStart, Start: o.StartsAt, End: o.EndsAt})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case !a.Start.Equal(b.Start):
			return a.Start.Before(b.Start)
		case !a.End.Equal(b.End):
			return a.End.Before(b.End)
		case a.Master.ID != b.Master.ID:
			return a.Master.ID < b.Master.ID
		}
		return a.Original.Before(b.Original)
	})
	return out, nil
}

// formatWhen writes an instant the way the clients read it: an RFC 3339 string in
// the event's own zone, or the calendar date for an all-day event.
func formatWhen(t time.Time, loc *time.Location, allDay bool) string {
	if allDay {
		return t.In(loc).Format("2006-01-02")
	}
	return t.In(loc).Format(time.RFC3339)
}

// parseWhen reads a start or end the client sent. An all-day event takes a date
// (or the date part of an instant, in its zone) and means that day's midnight; a
// timed one an RFC 3339 instant.
func parseWhen(v string, loc *time.Location, allDay bool) (time.Time, error) {
	if allDay {
		if d, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			return d, nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			y, m, d := t.In(loc).Date()
			return time.Date(y, m, d, 0, 0, 0, 0, loc), nil
		}
		return time.Time{}, invalid("an all-day event takes dates as yyyy-MM-dd")
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, invalid("times are RFC 3339, such as 2026-10-14T09:00:00+07:00")
	}
	return t, nil
}

// wall is a time's wall clock in loc read as if it were UTC, so two of them
// subtract to the wall-clock difference whatever the zone's offsets were.
func wall(t time.Time, loc *time.Location) time.Time {
	y, mo, d := t.In(loc).Date()
	h, mi, s := t.In(loc).Clock()
	return time.Date(y, mo, d, h, mi, s, t.Nanosecond(), time.UTC)
}

// fromWall is the inverse of wall: the instant whose wall clock in loc is w's.
func fromWall(w time.Time, loc *time.Location) time.Time {
	return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), w.Nanosecond(), loc)
}

// shiftBy moves t by the wall-clock distance from `from` to `to`, in loc: a series
// dragged from 09:00 to 10:00 stays at 10:00 through a clock change.
func shiftBy(t, from, to time.Time, loc *time.Location) time.Time {
	return fromWall(wall(t, loc).Add(wall(to, loc).Sub(wall(from, loc))), loc)
}
