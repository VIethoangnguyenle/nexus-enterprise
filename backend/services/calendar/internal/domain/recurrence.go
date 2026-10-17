package domain

import (
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

const (
	// maxRecurrenceCount is the most occurrences a rule may ask for with COUNT.
	maxRecurrenceCount = 730
	// recurrenceHorizonYears cuts the expansion of a rule that has neither COUNT
	// nor UNTIL, so "forever" is bounded.
	recurrenceHorizonYears = 5
	// maxRecurrenceInterval keeps INTERVAL to something a person could mean.
	maxRecurrenceInterval = 1000
)

// untilLayout is how UNTIL is written back into a rule: UTC, RFC 5545 form.
const untilLayout = "20060102T150405Z"

// recurrence is a repeating rule anchored at a start, expanded in the event's own
// zone: the wall-clock time of every occurrence is that of the start, whatever
// the zone's offset is on that day.
type recurrence struct {
	rule   *rrule.RRule
	opt    rrule.ROption
	start  time.Time
	length time.Duration // timed events: end − start
	days   int           // all-day events: the number of days covered
	allDay bool
	loc    *time.Location
	// horizon is where expansion stops: UNTIL, the last COUNT occurrence, or five
	// years after the start.
	horizon time.Time
}

// span is one occurrence: [start, end).
type span struct{ start, end time.Time }

// loadZone resolves an IANA zone name. The zone of the server ("Local") and an
// empty name are refused: an event's time must mean the same everywhere.
func loadZone(tzid string) (*time.Location, error) {
	if tzid == "" || tzid == "Local" {
		return nil, invalid("tzid must be an IANA time zone such as Asia/Ho_Chi_Minh")
	}
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		return nil, invalid("unknown time zone %q", tzid)
	}
	return loc, nil
}

// normalizeRRule strips the optional "RRULE:" prefix and surrounding space.
func normalizeRRule(body string) string {
	body = strings.TrimSpace(body)
	if len(body) >= 6 && strings.EqualFold(body[:6], "RRULE:") {
		body = body[6:]
	}
	return strings.TrimSpace(body)
}

// newRecurrence parses and validates a rule for an event that starts at start
// and ends at end in loc. Only DAILY, WEEKLY, MONTHLY and YEARLY are accepted,
// COUNT is at most 730, and sub-day parts are refused.
func newRecurrence(body string, start, end time.Time, allDay bool, loc *time.Location) (*recurrence, error) {
	body = normalizeRRule(body)
	if body == "" || strings.ContainsAny(body, "\r\n") {
		return nil, invalid("the recurrence rule is not valid")
	}
	opt, err := rrule.StrToROptionInLocation(body, loc)
	if err != nil || !opt.Dtstart.IsZero() {
		return nil, invalid("the recurrence rule is not valid")
	}
	switch opt.Freq {
	case rrule.DAILY, rrule.WEEKLY, rrule.MONTHLY, rrule.YEARLY:
	default:
		return nil, invalid("a recurrence repeats daily, weekly, monthly or yearly")
	}
	if opt.Count < 0 || opt.Count > maxRecurrenceCount {
		return nil, invalid("a recurrence repeats at most %d times", maxRecurrenceCount)
	}
	if opt.Interval < 0 || opt.Interval > maxRecurrenceInterval {
		return nil, invalid("the recurrence interval is not valid")
	}
	if opt.Count > 0 && !opt.Until.IsZero() {
		return nil, invalid("a recurrence ends after a count or on a date, not both")
	}
	if len(opt.Byhour) > 0 || len(opt.Byminute) > 0 || len(opt.Bysecond) > 0 || len(opt.Byeaster) > 0 {
		return nil, invalid("the recurrence rule is not valid")
	}
	start = start.In(loc)
	if !opt.Until.IsZero() && !strings.Contains(untilValue(body), "T") {
		// A date-only UNTIL includes that whole day.
		y, m, d := opt.Until.In(loc).Date()
		opt.Until = time.Date(y, m, d, 23, 59, 59, 0, loc)
	}
	opt.Dtstart = start
	horizon := start.AddDate(recurrenceHorizonYears, 0, 0)
	if opt.Count == 0 && opt.Until.IsZero() {
		opt.Until = horizon
	}
	r, err := rrule.NewRRule(*opt)
	if err != nil {
		return nil, invalid("the recurrence rule is not valid")
	}
	ru := &recurrence{rule: r, opt: *opt, start: start, allDay: allDay, loc: loc}
	if allDay {
		ru.days = wholeDays(start, end.In(loc))
	} else {
		ru.length = end.Sub(start)
	}
	if opt.Count > 0 {
		all := r.All()
		if len(all) == 0 {
			return nil, invalid("the recurrence rule has no occurrence")
		}
		ru.horizon = all[len(all)-1]
	} else {
		ru.horizon = opt.Until
	}
	return ru, nil
}

// untilValue is the text after UNTIL= in a rule body, "" when there is none.
func untilValue(body string) string {
	for _, part := range strings.Split(body, ";") {
		if v, ok := strings.CutPrefix(part, "UNTIL="); ok {
			return v
		}
	}
	return ""
}

// wholeDays is the number of calendar days from a to b (both midnights in loc).
func wholeDays(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return int(time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC).Sub(time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

// endOf is the end of the occurrence that starts at t.
func (ru *recurrence) endOf(t time.Time) time.Time {
	if ru.allDay {
		return t.In(ru.loc).AddDate(0, 0, ru.days)
	}
	return t.Add(ru.length)
}

// lastEnd is where the series stops reaching: what recurrence_until stores.
func (ru *recurrence) lastEnd() time.Time { return ru.endOf(ru.horizon) }

// between returns the occurrences that overlap [from, to), in order.
func (ru *recurrence) between(from, to time.Time) []span {
	// An occurrence overlaps when it starts before `to` and ends after `from`.
	// Widen the search by the longest it can last, then filter exactly.
	reach := ru.length
	if ru.allDay {
		reach = time.Duration(ru.days+1) * 24 * time.Hour
	}
	var out []span
	for _, st := range ru.rule.Between(from.Add(-reach), to, true) {
		sp := span{start: st, end: ru.endOf(st)}
		if overlaps(sp.start, sp.end, from, to) {
			out = append(out, sp)
		}
	}
	return out
}

// has reports whether the rule generates an occurrence that starts exactly at t.
func (ru *recurrence) has(t time.Time) bool {
	return len(ru.rule.Between(t, t, true)) > 0
}

// countBefore is how many occurrences start before t.
func (ru *recurrence) countBefore(t time.Time) int {
	n := 0
	for _, st := range ru.rule.Between(ru.start, t, true) {
		if st.Before(t) {
			n++
		}
	}
	return n
}

// overlaps reports whether [s, e) touches [from, to); an event with no length
// counts when its instant is inside.
func overlaps(s, e, from, to time.Time) bool {
	if !s.Before(to) {
		return false
	}
	if e.After(s) {
		return e.After(from)
	}
	return !s.Before(from)
}

// ruleParts splits a rule body into its KEY=VALUE parts.
func ruleParts(body string) []string {
	var parts []string
	for _, p := range strings.Split(normalizeRRule(body), ";") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// withoutKeys is the rule body without the named parts.
func withoutKeys(body string, keys ...string) []string {
	var out []string
	for _, p := range ruleParts(body) {
		k, _, _ := strings.Cut(p, "=")
		drop := false
		for _, d := range keys {
			drop = drop || k == d
		}
		if !drop {
			out = append(out, p)
		}
	}
	return out
}

// endedBefore is the rule body ended just before t: UNTIL one second earlier, and
// no COUNT (the two never go together).
func endedBefore(body string, t time.Time) string {
	parts := append(withoutKeys(body, "COUNT", "UNTIL"), "UNTIL="+t.Add(-time.Second).UTC().Format(untilLayout))
	return strings.Join(parts, ";")
}

// withCount is the rule body asking for n occurrences from its start, in place of
// any COUNT it had.
func withCount(body string, n int) string {
	return strings.Join(append(withoutKeys(body, "COUNT"), "COUNT="+strconv.Itoa(n)), ";")
}

// shiftWeekdays moves every plain weekday of BYDAY by n days (a weekly rule whose
// series was dragged to another day of the week). Ordinal weekdays ("2WE") belong
// to monthly and yearly rules and are left as they are.
func shiftWeekdays(body string, n int) string {
	codes := []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"}
	parts := ruleParts(body)
	for i, p := range parts {
		v, ok := strings.CutPrefix(p, "BYDAY=")
		if !ok {
			continue
		}
		days := strings.Split(v, ",")
		for j, d := range days {
			for k, c := range codes {
				if d == c {
					days[j] = codes[((k+n)%7+7)%7]
				}
			}
		}
		parts[i] = "BYDAY=" + strings.Join(days, ",")
	}
	return strings.Join(parts, ";")
}
