package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/calendar/internal/store"
)

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

func at(loc *time.Location, y int, m time.Month, d, h, mi int) time.Time {
	return time.Date(y, m, d, h, mi, 0, 0, loc)
}

// series builds a repeating event row.
func series(id, rule string, start, end time.Time, allDay bool, tz string) *store.Event {
	return &store.Event{ID: id, CalendarID: "cal", Title: id, StartsAt: start, EndsAt: end, AllDay: allDay, TZID: tz, RRule: rule, Version: 1,
		Kind: "event", Transparency: "busy", Visibility: "default"}
}

func starts(ins []instance, loc *time.Location) []string {
	var out []string
	for _, i := range ins {
		out = append(out, i.Start.In(loc).Format("2006-01-02 15:04"))
	}
	return out
}

func expand(t *testing.T, masters []*store.Event, overrides []*store.Event, from, to time.Time) []instance {
	t.Helper()
	out, err := expandEvents(masters, overrides, from, to)
	require.NoError(t, err)
	return out
}

func TestWeeklyKeepsWallClockAcrossDaylightSaving(t *testing.T) {
	ny := zone(t, "America/New_York")
	m := series("w", "FREQ=WEEKLY;BYDAY=MO", at(ny, 2026, 10, 26, 9, 0), at(ny, 2026, 10, 26, 10, 0), false, "America/New_York")
	got := expand(t, []*store.Event{m}, nil, at(ny, 2026, 10, 25, 0, 0), at(ny, 2026, 11, 17, 0, 0))
	require.Equal(t, []string{"2026-10-26 09:00", "2026-11-02 09:00", "2026-11-09 09:00", "2026-11-16 09:00"}, starts(got, ny))
	// The clocks went back on 1 November: the instant moves by an hour, the wall clock does not.
	assert.Equal(t, 13, got[0].Start.UTC().Hour())
	assert.Equal(t, 14, got[1].Start.UTC().Hour())
	assert.Equal(t, time.Hour, got[1].End.Sub(got[1].Start))
}

func TestMonthlySecondWednesday(t *testing.T) {
	loc := zone(t, "Asia/Ho_Chi_Minh")
	m := series("m", "FREQ=MONTHLY;BYDAY=2WE", at(loc, 2026, 10, 14, 14, 0), at(loc, 2026, 10, 14, 15, 0), false, "Asia/Ho_Chi_Minh")
	got := expand(t, []*store.Event{m}, nil, at(loc, 2026, 10, 1, 0, 0), at(loc, 2027, 1, 1, 0, 0))
	assert.Equal(t, []string{"2026-10-14 14:00", "2026-11-11 14:00", "2026-12-09 14:00"}, starts(got, loc))
}

func TestOverrideReplacesItsOccurrenceAndMoves(t *testing.T) {
	loc := zone(t, "Asia/Ho_Chi_Minh")
	m := series("d", "FREQ=DAILY", at(loc, 2026, 10, 14, 9, 0), at(loc, 2026, 10, 14, 10, 0), false, "Asia/Ho_Chi_Minh")
	orig := at(loc, 2026, 10, 15, 9, 0)
	moved := &store.Event{ID: "o", RecurringEventID: "d", OriginalStart: &orig, Title: "Moved",
		StartsAt: at(loc, 2026, 10, 15, 16, 0), EndsAt: at(loc, 2026, 10, 15, 17, 0), TZID: m.TZID}
	got := expand(t, []*store.Event{m}, []*store.Event{moved}, at(loc, 2026, 10, 14, 0, 0), at(loc, 2026, 10, 17, 0, 0))
	assert.Equal(t, []string{"2026-10-14 09:00", "2026-10-15 16:00", "2026-10-16 09:00"}, starts(got, loc))
	assert.Equal(t, "Moved", got[1].Row.Title)
	assert.Equal(t, orig.UnixNano(), got[1].Original.UnixNano(), "an exception keeps the identity of the occurrence it replaces")
	assert.Same(t, m, got[1].Master)
}

func TestCancelledOccurrenceIsDropped(t *testing.T) {
	loc := zone(t, "Asia/Ho_Chi_Minh")
	m := series("d", "FREQ=DAILY", at(loc, 2026, 10, 14, 9, 0), at(loc, 2026, 10, 14, 10, 0), false, "Asia/Ho_Chi_Minh")
	orig := at(loc, 2026, 10, 15, 9, 0)
	gone := &store.Event{ID: "o", RecurringEventID: "d", OriginalStart: &orig, Cancelled: true, StartsAt: orig, EndsAt: orig.Add(time.Hour)}
	got := expand(t, []*store.Event{m}, []*store.Event{gone}, at(loc, 2026, 10, 14, 0, 0), at(loc, 2026, 10, 17, 0, 0))
	assert.Equal(t, []string{"2026-10-14 09:00", "2026-10-16 09:00"}, starts(got, loc))
}

func TestFollowingSplitLeavesEarlierOccurrencesAlone(t *testing.T) {
	loc := zone(t, "Asia/Ho_Chi_Minh")
	start, end := at(loc, 2026, 10, 5, 9, 0), at(loc, 2026, 10, 5, 10, 0)
	m := series("w", "FREQ=WEEKLY;BYDAY=MO", start, end, false, "Asia/Ho_Chi_Minh")
	cut := at(loc, 2026, 10, 19, 9, 0)
	m.RRule = endedBefore(m.RRule, cut)
	tail := series("t", "FREQ=WEEKLY;BYDAY=MO", at(loc, 2026, 10, 19, 11, 0), at(loc, 2026, 10, 19, 12, 0), false, "Asia/Ho_Chi_Minh")
	got := expand(t, []*store.Event{m, tail}, nil, at(loc, 2026, 10, 1, 0, 0), at(loc, 2026, 11, 10, 0, 0))
	assert.Equal(t, []string{"2026-10-05 09:00", "2026-10-12 09:00", "2026-10-19 11:00", "2026-10-26 11:00", "2026-11-02 11:00", "2026-11-09 11:00"}, starts(got, loc))
}

func TestCountEndsTheSeries(t *testing.T) {
	loc := zone(t, "UTC")
	m := series("c", "FREQ=DAILY;COUNT=3", at(loc, 2026, 10, 14, 9, 0), at(loc, 2026, 10, 14, 10, 0), false, "UTC")
	got := expand(t, []*store.Event{m}, nil, at(loc, 2026, 10, 1, 0, 0), at(loc, 2027, 1, 1, 0, 0))
	assert.Equal(t, []string{"2026-10-14 09:00", "2026-10-15 09:00", "2026-10-16 09:00"}, starts(got, loc))
}

func TestUntilIncludesItsDay(t *testing.T) {
	ny := zone(t, "America/New_York")
	m := series("u", "FREQ=DAILY;UNTIL=20261016", at(ny, 2026, 10, 14, 20, 0), at(ny, 2026, 10, 14, 21, 0), false, "America/New_York")
	got := expand(t, []*store.Event{m}, nil, at(ny, 2026, 10, 1, 0, 0), at(ny, 2026, 11, 1, 0, 0))
	assert.Equal(t, []string{"2026-10-14 20:00", "2026-10-15 20:00", "2026-10-16 20:00"}, starts(got, ny))

	z := series("z", "FREQ=DAILY;UNTIL=20261016T235959Z", at(ny, 2026, 10, 14, 20, 0), at(ny, 2026, 10, 14, 21, 0), false, "America/New_York")
	assert.Len(t, expand(t, []*store.Event{z}, nil, at(ny, 2026, 10, 1, 0, 0), at(ny, 2026, 11, 1, 0, 0)), 2, "16 Oct 20:00 EDT is 17 Oct 00:00Z, past the UNTIL")
}

func TestAllDayKeepsDatesAndSpan(t *testing.T) {
	loc := zone(t, "Asia/Ho_Chi_Minh")
	// A two-day event every week: Wed 14 and Thu 15 Oct.
	m := series("a", "FREQ=WEEKLY", at(loc, 2026, 10, 14, 0, 0), at(loc, 2026, 10, 16, 0, 0), true, "Asia/Ho_Chi_Minh")
	got := expand(t, []*store.Event{m}, nil, at(loc, 2026, 10, 19, 0, 0), at(loc, 2026, 10, 30, 0, 0))
	require.Len(t, got, 2)
	assert.Equal(t, "2026-10-21", formatWhen(got[0].Start, loc, true))
	assert.Equal(t, "2026-10-23", formatWhen(got[0].End, loc, true), "the end of an all-day span is exclusive")
	assert.Equal(t, "2026-10-28", formatWhen(got[1].Start, loc, true))
	// An occurrence that began before the range but still covers its first day is in.
	got = expand(t, []*store.Event{m}, nil, at(loc, 2026, 10, 22, 0, 0), at(loc, 2026, 10, 23, 0, 0))
	require.Len(t, got, 1)
	assert.Equal(t, "2026-10-21", formatWhen(got[0].Start, loc, true))
}

func TestIntervalSkipsPeriods(t *testing.T) {
	loc := zone(t, "UTC")
	m := series("i", "FREQ=WEEKLY;INTERVAL=2;BYDAY=TU", at(loc, 2026, 10, 13, 8, 0), at(loc, 2026, 10, 13, 9, 0), false, "UTC")
	got := expand(t, []*store.Event{m}, nil, at(loc, 2026, 10, 1, 0, 0), at(loc, 2026, 11, 20, 0, 0))
	assert.Equal(t, []string{"2026-10-13 08:00", "2026-10-27 08:00", "2026-11-10 08:00"}, starts(got, loc))
}

func TestExpansionStopsFiveYearsOutWithoutAnEnd(t *testing.T) {
	loc := zone(t, "UTC")
	m := series("f", "FREQ=DAILY", at(loc, 2026, 1, 1, 9, 0), at(loc, 2026, 1, 1, 10, 0), false, "UTC")
	assert.NotEmpty(t, expand(t, []*store.Event{m}, nil, at(loc, 2030, 12, 1, 0, 0), at(loc, 2030, 12, 5, 0, 0)))
	assert.Empty(t, expand(t, []*store.Event{m}, nil, at(loc, 2031, 2, 1, 0, 0), at(loc, 2031, 2, 5, 0, 0)))
}

func TestRuleValidation(t *testing.T) {
	loc := zone(t, "UTC")
	s, e := at(loc, 2026, 10, 14, 9, 0), at(loc, 2026, 10, 14, 10, 0)
	for rule, ok := range map[string]bool{
		"FREQ=DAILY":                          true,
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE":       true,
		"FREQ=MONTHLY;COUNT=730":              true,
		"FREQ=YEARLY;INTERVAL=2":              true,
		"FREQ=MONTHLY;COUNT=731":              false,
		"FREQ=HOURLY":                         false,
		"FREQ=SECONDLY;COUNT=5":               false,
		"FREQ=DAILY;COUNT=3;UNTIL=20271001":   false,
		"FREQ=DAILY;BYHOUR=9":                 false,
		"FREQ=DAILY;INTERVAL=-1":              false,
		"FREQ=DAILY;INTERVAL=100000":          false,
		"FREQ=NEVER":                          false,
		"FREQ=DAILY\nDTSTART:20261014T090000": false,
		"BYDAY=MO":                            false,
		"":                                    false,
		"not a rule":                          false,
	} {
		_, err := newRecurrence(rule, s, e, false, loc)
		if ok {
			assert.NoError(t, err, rule)
		} else {
			requireKind(t, err, ErrInvalidInput)
		}
	}
}

func TestEndedBeforeAndCountBefore(t *testing.T) {
	loc := zone(t, "UTC")
	body := endedBefore("FREQ=WEEKLY;COUNT=10;BYDAY=MO", at(loc, 2026, 10, 19, 9, 0))
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO;UNTIL=20261019T085959Z", body)
	assert.False(t, strings.Contains(body, "COUNT"))

	ru, err := newRecurrence("FREQ=WEEKLY;COUNT=10;BYDAY=MO", at(loc, 2026, 10, 5, 9, 0), at(loc, 2026, 10, 5, 10, 0), false, loc)
	require.NoError(t, err)
	assert.Equal(t, 2, ru.countBefore(at(loc, 2026, 10, 19, 9, 0)))
	assert.True(t, ru.has(at(loc, 2026, 10, 19, 9, 0)))
	assert.False(t, ru.has(at(loc, 2026, 10, 20, 9, 0)))
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO;COUNT=8", withCount("FREQ=WEEKLY;COUNT=10;BYDAY=MO", 8))
	assert.Equal(t, at(loc, 2026, 12, 7, 10, 0), ru.lastEnd(), "the tenth Monday ends at 10:00")
}

func TestShiftWeekdaysAndWallDelta(t *testing.T) {
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=TH,SA", shiftWeekdays("FREQ=WEEKLY;BYDAY=WE,FR", 1))
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=SU,MO", shiftWeekdays("FREQ=WEEKLY;BYDAY=MO,TU", -1))
	assert.Equal(t, "FREQ=MONTHLY;BYDAY=2WE", shiftWeekdays("FREQ=MONTHLY;BYDAY=2WE", 1), "ordinal weekdays are the editor's to change")

	ny := zone(t, "America/New_York")
	// A series that began in summer, moved one hour later from a winter occurrence.
	got := shiftBy(at(ny, 2026, 10, 5, 9, 0), at(ny, 2026, 11, 2, 9, 0), at(ny, 2026, 11, 2, 10, 0), ny)
	assert.Equal(t, at(ny, 2026, 10, 5, 10, 0), got)
}
