package domain

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/calendar/internal/store"
)

const (
	// maxFreeBusyPeople bounds the people one free/busy or suggestion request covers.
	maxFreeBusyPeople = 20
	// maxFreeBusyDays bounds the range of one free/busy request.
	maxFreeBusyDays = 62
	// slotStep is the grid suggested times fall on, and maxSlots how many are offered.
	slotStep = 30 * time.Minute
	maxSlots = 3

	minDurationMinutes = 5
	maxDurationMinutes = 720
)

// busyBlock is a stretch of someone's time that an event takes.
type busyBlock struct {
	start, end time.Time
	in         instance
	title      string
}

// freeBusyPeople admits the people a request names: each an active member of the
// caller's workspace by the database and by the graph. A person of another
// workspace is refused (403), and nobody is answered about.
func (s *Service) freeBusyPeople(ctx context.Context, ws *store.Workspace, users []string) ([]*store.Person, error) {
	var nodes []string
	seen := map[string]bool{}
	for _, u := range users {
		if u = strings.TrimSpace(u); u != "" && !seen[u] {
			seen[u] = true
			nodes = append(nodes, u)
		}
	}
	switch {
	case len(nodes) == 0:
		return nil, invalid("name the people to look at")
	case len(nodes) > maxFreeBusyPeople:
		return nil, invalid("at most %d people at a time", maxFreeBusyPeople)
	}
	out := make([]*store.Person, 0, len(nodes))
	for _, n := range nodes {
		p, err := s.requirePerson(ctx, ws, n)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func nodesOf(people []*store.Person) []string {
	out := make([]string, len(people))
	for i, p := range people {
		out[i] = p.NodeID
	}
	return out
}

// FreeBusy answers, for each person, when they are busy inside [from, to) and what
// their working hours are. Busy is an event they are on (or that sits on their
// personal calendar) that is marked busy and that they have not declined,
// repeating events expanded. A title is shown only for an event the caller may
// read: they read the calendar it is on, or they are a guest of it.
func (s *Service) FreeBusy(ctx context.Context, wsID string, users []string, fromS, toS string) ([]*PersonFreeBusy, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	from, err := time.Parse(time.RFC3339, fromS)
	if err != nil {
		return nil, invalid("from must be an RFC 3339 time")
	}
	to, err := time.Parse(time.RFC3339, toS)
	if err != nil {
		return nil, invalid("to must be an RFC 3339 time")
	}
	if !to.After(from) {
		return nil, invalid("to must be after from")
	}
	if to.Sub(from) > maxFreeBusyDays*24*time.Hour {
		return nil, invalid("a range is at most %d days", maxFreeBusyDays)
	}
	people, err := s.freeBusyPeople(ctx, ws, users)
	if err != nil {
		return nil, err
	}
	nodes := nodesOf(people)
	settings, err := s.store.Settings(ctx, ws.ID, nodes)
	if err != nil {
		return nil, internal("load working hours", err)
	}
	blocks, err := s.busyBlocks(ctx, ws, who, nodes, from, to, true)
	if err != nil {
		return nil, err
	}
	out := make([]*PersonFreeBusy, 0, len(people))
	for _, p := range people {
		st := settings[p.NodeID]
		fb := &PersonFreeBusy{Person: refOf(p), Busy: []BusySpan{}, WorkStart: st.WorkStart, WorkEnd: st.WorkEnd}
		for _, b := range blocks[p.NodeID] {
			loc := zoneOf(b.in.Row.TZID)
			fb.Busy = append(fb.Busy, BusySpan{Start: formatWhen(b.start, loc, false), End: formatWhen(b.end, loc, false), Title: b.title})
		}
		out = append(out, fb)
	}
	return out, nil
}

// busyBlocks lists, per person, the stretches of [from, to) their events take, by
// start. With titles, a block carries the event's title when the caller may read it.
func (s *Service) busyBlocks(ctx context.Context, ws *store.Workspace, caller grpcauth.Caller, nodes []string, from, to time.Time, titles bool) (map[string][]busyBlock, error) {
	out := map[string][]busyBlock{}
	var all []*busyBlock
	for _, node := range nodes {
		masters, overrides, err := s.eventsOf(ctx, ws, node, from, to)
		if err != nil {
			return nil, err
		}
		insts, err := expandEvents(masters, overrides, from, to)
		if err != nil {
			return nil, internal("expand events", err)
		}
		ids := make([]string, 0, len(masters))
		for _, m := range masters {
			ids = append(ids, m.ID)
		}
		answers, err := s.store.MyResponses(ctx, node, ids)
		if err != nil {
			return nil, internal("load answers", err)
		}
		perOccurrence, err := s.store.OccurrenceResponses(ctx, node, ids)
		if err != nil {
			return nil, internal("load answers", err)
		}
		v := &viewData{responses: answers, occurrences: perOccurrence}
		for _, in := range insts {
			if in.Row.Transparency != TransparencyBusy || !in.End.After(in.Start) {
				continue
			}
			if v.responseFor(in.Master.ID, in.Original) == ResponseDeclined {
				continue
			}
			out[node] = append(out[node], busyBlock{start: in.Start, end: in.End, in: in})
		}
		sort.SliceStable(out[node], func(i, j int) bool { return out[node][i].start.Before(out[node][j].start) })
		for i := range out[node] {
			all = append(all, &out[node][i])
		}
	}
	if titles {
		if err := s.nameBlocks(ctx, ws, caller, all); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// eventsOf gathers what can take a person's time in a range: the events of their
// personal calendar (whoever made them) and the events they are invited to
// elsewhere. A calendar they never opened does not exist yet and holds nothing.
func (s *Service) eventsOf(ctx context.Context, ws *store.Workspace, node string, from, to time.Time) (masters, overrides []*store.Event, err error) {
	cal, err := s.store.CalendarFor(ctx, ws.ID, store.KindPersonal, node, "")
	switch {
	case err == nil:
		if masters, overrides, err = s.store.RangeEvents(ctx, []string{cal.ID}, from, to); err != nil {
			return nil, nil, internal("list events", err)
		}
	case !isNotFound(err):
		return nil, nil, internal("find calendar", err)
	}
	am, ao, err := s.store.RangeAttended(ctx, node, from, to)
	if err != nil {
		return nil, nil, internal("list invitations", err)
	}
	have := map[string]bool{}
	for _, m := range masters {
		have[m.ID] = true
	}
	haveOv := map[string]bool{}
	for _, o := range overrides {
		haveOv[o.ID] = true
	}
	for _, m := range am {
		if !have[m.ID] {
			have[m.ID] = true
			masters = append(masters, m)
		}
	}
	for _, o := range ao {
		if !haveOv[o.ID] {
			haveOv[o.ID] = true
			overrides = append(overrides, o)
		}
	}
	return masters, overrides, nil
}

// nameBlocks gives each block the title of its event when the caller may read it.
func (s *Service) nameBlocks(ctx context.Context, ws *store.Workspace, caller grpcauth.Caller, blocks []*busyBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	cals := map[string]*store.Calendar{}
	var list []*store.Calendar
	seen := map[string]bool{}
	var ids []string
	for _, b := range blocks {
		if id := b.in.Master.ID; !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
		cid := b.in.Master.CalendarID
		if _, done := cals[cid]; done {
			continue
		}
		c, err := s.store.CalendarByID(ctx, cid)
		switch {
		case err == nil && c.WorkspaceID == ws.ID:
			cals[cid] = c
			list = append(list, c)
		case err == nil || isNotFound(err):
			cals[cid] = nil
		default:
			return internal("load calendar", err)
		}
	}
	held, err := s.accessLevels(ctx, caller.NGACNodeID, list)
	if err != nil {
		return err
	}
	mine, err := s.store.MyResponses(ctx, caller.NGACNodeID, ids)
	if err != nil {
		return internal("load answers", err)
	}
	for _, b := range blocks {
		cal := cals[b.in.Master.CalendarID]
		if cal == nil {
			continue
		}
		_, guest := mine[b.in.Master.ID]
		if guest || showsDetail(accessTo(held, cal), b.in.Row, cal, caller.NGACNodeID, false) {
			b.title = b.in.Row.Title
		}
	}
	return nil
}

// SuggestSlots offers up to three of the earliest times on a day when everyone is
// free for the duration, inside everyone's working hours. The day is a date in the
// caller's zone; each person's hours are theirs on that date in their own zone, and
// times fall on a half-hour grid of the caller's clock.
func (s *Service) SuggestSlots(ctx context.Context, wsID string, users []string, durationMin int, day string) ([]TimeSlot, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if durationMin < minDurationMinutes || durationMin > maxDurationMinutes {
		return nil, invalid("duration is %d to %d minutes", minDurationMinutes, maxDurationMinutes)
	}
	people, err := s.freeBusyPeople(ctx, ws, users)
	if err != nil {
		return nil, err
	}
	nodes := nodesOf(people)
	settings, err := s.store.Settings(ctx, ws.ID, append([]string{who.NGACNodeID}, nodes...))
	if err != nil {
		return nil, internal("load working hours", err)
	}
	callerLoc := zoneOf(settings[who.NGACNodeID].TZID)
	d, err := time.ParseInLocation("2006-01-02", day, callerLoc)
	if err != nil {
		return nil, invalid("day must be a date such as 2026-11-02")
	}
	y, mo, dd := d.Date()

	var open, shut time.Time
	for i, p := range people {
		st := settings[p.NodeID]
		loc := zoneOf(st.TZID)
		a, errA := clockOn(st.WorkStart, y, mo, dd, loc)
		b, errB := clockOn(st.WorkEnd, y, mo, dd, loc)
		if errA != nil || errB != nil {
			return nil, internal("read working hours", errors.Join(errA, errB))
		}
		if i == 0 || a.After(open) {
			open = a
		}
		if i == 0 || b.Before(shut) {
			shut = b
		}
	}
	slots := []TimeSlot{}
	length := time.Duration(durationMin) * time.Minute
	if !shut.After(open) || shut.Sub(open) < length {
		return slots, nil
	}
	blocks, err := s.busyBlocks(ctx, ws, who, nodes, open, shut, false)
	if err != nil {
		return nil, err
	}
	for t := onGrid(open, callerLoc); !t.Add(length).After(shut) && len(slots) < maxSlots; t = t.Add(slotStep) {
		free := true
		for _, bs := range blocks {
			for _, b := range bs {
				if overlaps(b.start, b.end, t, t.Add(length)) {
					free = false
				}
			}
		}
		if free {
			slots = append(slots, TimeSlot{Start: t.In(callerLoc).Format(time.RFC3339), End: t.Add(length).In(callerLoc).Format(time.RFC3339)})
		}
	}
	return slots, nil
}

// clockOn is the instant at which a wall clock "HH:mm" shows on a date in a zone.
func clockOn(hhmm string, y int, mo time.Month, d int, loc *time.Location) (time.Time, error) {
	c, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(y, mo, d, c.Hour(), c.Minute(), 0, 0, loc), nil
}

// onGrid rounds an instant up to the next half hour of the wall clock in loc.
func onGrid(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc).Truncate(time.Second)
	rem := time.Duration(t.Minute()%30)*time.Minute + time.Duration(t.Second())*time.Second
	if rem == 0 {
		return t
	}
	return t.Add(slotStep - rem)
}

func isNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
