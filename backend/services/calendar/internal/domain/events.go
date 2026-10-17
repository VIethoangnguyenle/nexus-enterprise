package domain

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/store"
)

// Limits on what an event carries.
const (
	maxTitleRunes       = 200
	maxDescriptionRunes = 10000
	maxLocationRunes    = 500
	maxReminders        = 10
	maxReminderMinutes  = 40320 // four weeks
	maxRangeDays        = 400
	maxCalendarsPerList = 50
	minYear, maxYear    = 1970, 2200
)

// conflict is the refusal of an edit that started from an older version of the
// event; a transport answers 409.
func conflict() error { return classify(ErrAlreadyExists, "%s", staleMessage) }

// storeErr maps the store's own refusals to ours.
func storeErr(what string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrStale):
		return conflict()
	case errors.Is(err, store.ErrNotFound):
		return notFound("event not found")
	}
	return internal(what, err)
}

// --- reading access ---------------------------------------------------------

// levelOn is what the caller holds on a calendar: manage, write, read,
// freebusy, or "" for nothing.
func (s *Service) levelOn(ctx context.Context, who grpcauth.Caller, cal *store.Calendar) (string, error) {
	held, err := s.accessLevels(ctx, who.NGACNodeID, []*store.Calendar{cal})
	if err != nil {
		return "", err
	}
	return accessTo(held, cal), nil
}

func canRead(level string) bool {
	return level == AccessRead || level == AccessWrite || level == AccessManage
}

func canWrite(level string) bool { return level == AccessWrite || level == AccessManage }

// showsDetail reports whether the caller may read an event's details given what
// they hold on its calendar. A private event shows them only to its organizer,
// its guests, the owner of a personal calendar and whoever manages the calendar.
func showsDetail(level string, ev *store.Event, cal *store.Calendar, who string, guest bool) bool {
	if !canRead(level) {
		return false
	}
	if ev.Visibility != VisibilityPrivate {
		return true
	}
	return guest || ev.OrganizerNodeID == who || level == AccessManage ||
		(cal.Kind == store.KindPersonal && cal.OwnerNodeID == who)
}

// viewData is what the views of several occurrences need besides the rows.
type viewData struct {
	who       string
	people    map[string]*store.Person
	responses map[string]string // the caller's answer, by event
	guests    map[string]int
}

// loadViewData reads, in one round trip per kind, the people, answers and guest
// counts the occurrences refer to.
func (s *Service) loadViewData(ctx context.Context, wsID, who string, insts []instance) (*viewData, error) {
	v := &viewData{who: who}
	seenNode, seenEvent := map[string]bool{}, map[string]bool{}
	var nodes, ids []string
	for _, in := range insts {
		if !seenNode[in.Row.OrganizerNodeID] {
			seenNode[in.Row.OrganizerNodeID] = true
			nodes = append(nodes, in.Row.OrganizerNodeID)
		}
		if !seenEvent[in.Master.ID] {
			seenEvent[in.Master.ID] = true
			ids = append(ids, in.Master.ID)
		}
	}
	var err error
	if v.people, err = s.store.People(ctx, wsID, nodes); err != nil {
		return nil, internal("name organizers", err)
	}
	if v.responses, err = s.store.MyResponses(ctx, who, ids); err != nil {
		return nil, internal("load answers", err)
	}
	if v.guests, err = s.store.GuestCounts(ctx, ids); err != nil {
		return nil, internal("count guests", err)
	}
	return v, nil
}

func (v *viewData) person(node string) *PersonRef {
	if p := refOf(v.people[node]); p != nil {
		return p
	}
	return formerMemberRef(node)
}

// zoneOf is the zone of a stored event. The zone was validated when the event
// was written, so a failure here falls back to UTC rather than hiding the event.
func zoneOf(tzid string) *time.Location {
	loc, err := time.LoadLocation(tzid)
	if err != nil || tzid == "" || tzid == "Local" {
		slog.Warn("event with an unknown time zone; showing it in UTC", "tzid", tzid)
		return time.UTC
	}
	return loc
}

// occurrenceView shapes one occurrence. detail says whether the caller may read
// its details: without it only the time is real.
func occurrenceView(in instance, level string, detail bool, v *viewData) *OccurrenceView {
	row, m := in.Row, in.Master
	loc := zoneOf(row.TZID)
	o := &OccurrenceView{
		EventID:         m.ID,
		OccurrenceStart: formatWhen(in.Original, loc, row.AllDay),
		Start:           formatWhen(in.Start, loc, row.AllDay),
		End:             formatWhen(in.End, loc, row.AllDay),
		AllDay:          row.AllDay,
		CalendarID:      m.CalendarID,
		Version:         m.Version,
	}
	if in.Row != in.Master {
		o.Version = row.Version
	}
	if !detail {
		o.Title, o.BusyOnly, o.Kind = busyTitle, true, KindEvent
		return o
	}
	o.Title, o.Location, o.Kind = row.Title, row.Location, row.Kind
	o.MyResponse = v.responses[m.ID]
	o.IsOrganizer = row.OrganizerNodeID == v.who
	o.HasCall = row.HasCall
	o.IsRecurring = m.RRule != ""
	if o.IsRecurring {
		o.RecurrenceText = RecurrenceText(m.RRule, m.StartsAt.In(zoneOf(m.TZID)))
	}
	o.CanEdit = canWrite(level)
	o.GuestCount = v.guests[m.ID]
	o.Organizer = v.person(row.OrganizerNodeID)
	return o
}

// --- listing ----------------------------------------------------------------

// ListEvents returns the occurrences inside [from, to) of the named calendars, or
// of every calendar the caller has switched on. A calendar the caller holds
// nothing on is refused; one they only see free/busy of yields "Bận" blocks.
func (s *Service) ListEvents(ctx context.Context, wsID string, q ListEventsQuery) ([]*OccurrenceView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	from, err := time.Parse(time.RFC3339, q.From)
	if err != nil {
		return nil, invalid("from must be an RFC 3339 time")
	}
	to, err := time.Parse(time.RFC3339, q.To)
	if err != nil {
		return nil, invalid("to must be an RFC 3339 time")
	}
	if !to.After(from) {
		return nil, invalid("to must be after from")
	}
	if to.Sub(from) > maxRangeDays*24*time.Hour {
		return nil, invalid("a range is at most %d days", maxRangeDays)
	}

	cals, levels, err := s.calendarsToList(ctx, who, ws, q.Calendars)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(cals))
	byID := map[string]*store.Calendar{}
	for i, c := range cals {
		ids[i], byID[c.ID] = c.ID, c
	}
	masters, overrides, err := s.store.RangeEvents(ctx, ids, from, to)
	if err != nil {
		return nil, internal("list events", err)
	}
	insts, err := expandEvents(masters, overrides, from, to)
	if err != nil {
		return nil, internal("expand events", err)
	}
	v, err := s.loadViewData(ctx, ws.ID, who.NGACNodeID, insts)
	if err != nil {
		return nil, err
	}
	out := make([]*OccurrenceView, 0, len(insts))
	for _, in := range insts {
		cal := byID[in.Master.CalendarID]
		level := levels[cal.ID]
		_, guest := v.responses[in.Master.ID]
		detail := showsDetail(level, in.Row, cal, who.NGACNodeID, guest)
		if !detail && in.Row.Transparency == TransparencyFree {
			continue // free time is not busy time
		}
		out = append(out, occurrenceView(in, level, detail, v))
	}
	return out, nil
}

// calendarsToList resolves the calendars a listing covers with the caller's
// access to each.
func (s *Service) calendarsToList(ctx context.Context, who grpcauth.Caller, ws *store.Workspace, named []string) ([]*store.Calendar, map[string]string, error) {
	levels := map[string]string{}
	if len(named) == 0 {
		cals, settings, held, err := s.callerCalendars(ctx, who, ws)
		if err != nil {
			return nil, nil, err
		}
		var out []*store.Calendar
		for _, c := range cals {
			level := accessTo(held, c)
			if level == "" || (settings[c.ID] != nil && !settings[c.ID].Visible) {
				continue
			}
			out = append(out, c)
			levels[c.ID] = level
		}
		return out, levels, nil
	}
	if len(named) > maxCalendarsPerList {
		return nil, nil, invalid("at most %d calendars at a time", maxCalendarsPerList)
	}
	var cals []*store.Calendar
	seen := map[string]bool{}
	for _, id := range named {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		c, err := s.loadCalendar(ctx, ws.ID, id)
		if err != nil {
			return nil, nil, err
		}
		cals = append(cals, c)
	}
	held, err := s.accessLevels(ctx, who.NGACNodeID, cals)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range cals {
		level := accessTo(held, c)
		if level == "" {
			return nil, nil, denied("no read access")
		}
		levels[c.ID] = level
	}
	return cals, levels, nil
}

// --- reading one event ------------------------------------------------------

// loadEvent reads a single event or series of the workspace together with its
// calendar. An exception row is never addressed on its own.
func (s *Service) loadEvent(ctx context.Context, wsID, id string) (*store.Event, *store.Calendar, error) {
	e, err := s.store.EventByID(ctx, id)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, nil, notFound("event not found")
	case err != nil:
		return nil, nil, internal("load event", err)
	case e.WorkspaceID != wsID || e.RecurringEventID != "":
		return nil, nil, notFound("event not found")
	}
	cal, err := s.loadCalendar(ctx, wsID, e.CalendarID)
	if err != nil {
		return nil, nil, err
	}
	return e, cal, nil
}

// isGuest reports whether the caller reads the event through its own OA: a guest
// holds read there and nowhere else on the host's calendar.
func (s *Service) isGuest(ctx context.Context, who string, e *store.Event) bool {
	if e.NgacOAID == "" {
		return false
	}
	ok, _ := policyclient.New(s.policyRead).Check(ctx, who, e.NgacOAID, ngac.OpRead)
	return ok
}

// GetEvent returns an event in full. For a repeating event, occurrence names the
// instance to show (the first when empty).
func (s *Service) GetEvent(ctx context.Context, wsID, id, occurrence string) (*EventDetailView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	e, cal, err := s.loadEvent(ctx, wsID, id)
	if err != nil {
		return nil, err
	}
	level, err := s.levelOn(ctx, who, cal)
	if err != nil {
		return nil, err
	}
	responses, err := s.store.MyResponses(ctx, who.NGACNodeID, []string{e.ID})
	if err != nil {
		return nil, internal("load answers", err)
	}
	_, answered := responses[e.ID]
	invited := s.isGuest(ctx, who.NGACNodeID, e)
	switch {
	case showsDetail(level, e, cal, who.NGACNodeID, answered || invited):
	case invited && !canRead(level):
		// A guest reads the event they were invited to, nothing else of the calendar.
	default:
		return nil, denied("no read access")
	}
	in, err := s.instanceAt(ctx, e, occurrence)
	if err != nil {
		return nil, err
	}
	return s.detailView(ctx, ws, who, in, level, true, 0)
}

// instanceAt resolves the occurrence of an event a request names: the event
// itself when single, otherwise the instance at that start (the first when none).
func (s *Service) instanceAt(ctx context.Context, e *store.Event, occurrence string) (instance, error) {
	ru, err := recurrenceOf(e)
	if err != nil {
		return instance{}, internal("read recurrence", err)
	}
	if ru == nil {
		return instance{Row: e, Master: e, Original: e.StartsAt, Start: e.StartsAt, End: e.EndsAt}, nil
	}
	orig := e.StartsAt
	if occurrence != "" {
		if orig, err = parseOccurrence(occurrence, ru.loc, e.AllDay); err != nil {
			return instance{}, err
		}
	}
	o, err := s.store.OverrideOf(ctx, e.ID, orig)
	switch {
	case err == nil:
		if o.Cancelled {
			return instance{}, notFound("that occurrence was cancelled")
		}
		return instance{Row: o, Master: e, Original: orig, Start: o.StartsAt, End: o.EndsAt}, nil
	case !errors.Is(err, store.ErrNotFound):
		return instance{}, internal("load exception", err)
	case !ru.has(orig):
		return instance{}, notFound("occurrence not found")
	}
	return instance{Row: e, Master: e, Original: orig, Start: orig, End: ru.endOf(orig)}, nil
}

// parseOccurrence reads an occurrenceStart the server issued.
func parseOccurrence(v string, loc *time.Location, allDay bool) (time.Time, error) {
	t, err := parseWhen(v, loc, allDay)
	if err != nil {
		return time.Time{}, invalid("occurrence is not one this service issued")
	}
	return t, nil
}

// detailView shapes the whole event behind an occurrence for the caller.
func (s *Service) detailView(ctx context.Context, ws *store.Workspace, who grpcauth.Caller, in instance, level string, detail bool, preserved int) (*EventDetailView, error) {
	v, err := s.loadViewData(ctx, ws.ID, who.NGACNodeID, []instance{in})
	if err != nil {
		return nil, err
	}
	m, row := in.Master, in.Row
	d := &EventDetailView{
		OccurrenceView: *occurrenceView(in, level, detail, v),
		TZID:           row.TZID, Transparency: row.Transparency, Visibility: row.Visibility,
		GuestsCanInvite: row.GuestsCanInvite, GuestsCanSeeList: row.GuestsCanSeeList,
		Attendees: []AttendeeView{}, Reminders: []ReminderInput{}, PreservedOverrides: preserved,
	}
	if !detail {
		return d, nil
	}
	d.Description = row.Description
	d.RRule = m.RRule
	rems, err := s.store.Reminders(ctx, m.ID)
	if err != nil {
		return nil, internal("load reminders", err)
	}
	for _, r := range rems {
		d.Reminders = append(d.Reminders, ReminderInput{MinutesBefore: r})
	}
	if canWrite(level) || row.OrganizerNodeID == who.NGACNodeID || row.GuestsCanSeeList {
		list, err := s.store.Attendees(ctx, m.ID)
		if err != nil {
			return nil, internal("load guests", err)
		}
		nodes := make([]string, len(list))
		for i, a := range list {
			nodes[i] = a.UserNodeID
		}
		people, err := s.store.People(ctx, ws.ID, nodes)
		if err != nil {
			return nil, internal("name guests", err)
		}
		v.people = people
		for _, a := range list {
			d.Attendees = append(d.Attendees, AttendeeView{Person: v.person(a.UserNodeID), Response: a.Response, Comment: a.Comment, IsOrganizer: a.IsOrganizer})
		}
	}
	return d, nil
}

// --- creating ---------------------------------------------------------------

// CreateEvent adds an event to a calendar the caller may write.
func (s *Service) CreateEvent(ctx context.Context, wsID string, in EventInput) (*EventDetailView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	cal, err := s.loadCalendar(ctx, wsID, in.CalendarID)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, who.NGACNodeID, cal, ngac.OpWrite); err != nil {
		return nil, err
	}
	if len(in.Attendees) > 0 || in.InviteChannelID != "" {
		return nil, invalid("guests cannot be added yet")
	}
	e := &store.Event{
		ID: uuid.NewString(), WorkspaceID: ws.ID, CalendarID: cal.ID, OrganizerNodeID: who.NGACNodeID,
		Kind: KindEvent, Transparency: TransparencyBusy, Visibility: VisibilityDefault, GuestsCanSeeList: true,
		Title: in.Title, Description: in.Description, Location: in.Location, TZID: in.TZID, AllDay: in.AllDay,
		GuestsCanInvite: in.GuestsCanInvite, HasCall: in.HasCall,
	}
	if in.Kind != "" {
		e.Kind = in.Kind
	}
	if in.Transparency != "" {
		e.Transparency = in.Transparency
	}
	if in.Visibility != "" {
		e.Visibility = in.Visibility
	}
	e.GuestsCanSeeList = in.GuestsCanSeeList
	loc, err := loadZone(in.TZID)
	if err != nil {
		return nil, err
	}
	if e.StartsAt, err = parseWhen(in.Start, loc, in.AllDay); err != nil {
		return nil, err
	}
	if e.EndsAt, err = parseWhen(in.End, loc, in.AllDay); err != nil {
		return nil, err
	}
	if in.RRule != nil {
		e.RRule = normalizeRRule(*in.RRule)
	}
	if err := finalizeEvent(e); err != nil {
		return nil, err
	}
	var minutes []int
	if in.Reminders != nil {
		if minutes, err = reminderMinutes(*in.Reminders); err != nil {
			return nil, err
		}
	} else {
		def, err := s.store.DefaultReminder(ctx, ws.ID, who.NGACNodeID)
		if err != nil {
			return nil, internal("load default reminder", err)
		}
		if def > 0 {
			minutes = []int{def}
		}
	}
	if err := s.store.InsertEvent(ctx, e, minutes); err != nil {
		return nil, internal("save event", err)
	}
	s.announce(ctx, ws, cal, who.NGACNodeID, realtime.KindCreated, e.ID)
	level, err := s.levelOn(ctx, who, cal)
	if err != nil {
		return nil, err
	}
	return s.detailView(ctx, ws, who, instance{Row: e, Master: e, Original: e.StartsAt, Start: e.StartsAt, End: e.EndsAt}, level, true, 0)
}

// finalizeEvent checks an event whole and fills in what follows from it: the
// bound a range query prunes a finished series by.
func finalizeEvent(e *store.Event) error {
	e.Title = strings.TrimSpace(e.Title)
	switch n := utf8.RuneCountInString(e.Title); {
	case n < 1 || n > maxTitleRunes:
		return invalid("the title is 1 to %d characters", maxTitleRunes)
	case utf8.RuneCountInString(e.Description) > maxDescriptionRunes:
		return invalid("the description is at most %d characters", maxDescriptionRunes)
	case utf8.RuneCountInString(e.Location) > maxLocationRunes:
		return invalid("the location is at most %d characters", maxLocationRunes)
	}
	switch e.Kind {
	case KindEvent, KindFocus, KindOOO:
	default:
		return invalid("kind must be event, focus or ooo")
	}
	switch e.Transparency {
	case TransparencyBusy, TransparencyFree:
	default:
		return invalid("transparency must be busy or free")
	}
	switch e.Visibility {
	case VisibilityDefault, VisibilityPrivate:
	default:
		return invalid("visibility must be default or private")
	}
	loc, err := loadZone(e.TZID)
	if err != nil {
		return err
	}
	if e.StartsAt.IsZero() || e.EndsAt.IsZero() {
		return invalid("start and end are required")
	}
	if !e.EndsAt.After(e.StartsAt) {
		return invalid("the event must end after it starts")
	}
	if y := e.StartsAt.In(loc).Year(); y < minYear || y > maxYear {
		return invalid("the start is out of range")
	}
	e.RecurrenceUntil = nil
	if e.RRule == "" {
		return nil
	}
	ru, err := newRecurrence(e.RRule, e.StartsAt, e.EndsAt, e.AllDay, loc)
	if err != nil {
		return err
	}
	last := ru.lastEnd()
	e.RecurrenceUntil = &last
	return nil
}

// reminderMinutes validates reminders and returns their minutes, longest first.
func reminderMinutes(in []ReminderInput) ([]int, error) {
	if len(in) > maxReminders {
		return nil, invalid("at most %d reminders", maxReminders)
	}
	seen := map[int]bool{}
	out := []int{}
	for _, r := range in {
		if r.MinutesBefore < 0 || r.MinutesBefore > maxReminderMinutes {
			return nil, invalid("a reminder is 0 to %d minutes before", maxReminderMinutes)
		}
		if !seen[r.MinutesBefore] {
			seen[r.MinutesBefore] = true
			out = append(out, r.MinutesBefore)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out, nil
}

// --- changing ---------------------------------------------------------------

// editing is what a change to an event starts from: who asks, and the event with
// its calendar and the caller's reach over it.
type editing struct {
	who   grpcauth.Caller
	ws    *store.Workspace
	e     *store.Event
	cal   *store.Calendar
	level string
}

// beginEdit admits the caller to change or delete an event: write on its
// calendar, and the right to read it (an organizer's private event is not
// changed blind by someone who cannot see it).
func (s *Service) beginEdit(ctx context.Context, wsID, id string) (*editing, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	e, cal, err := s.loadEvent(ctx, wsID, id)
	if err != nil {
		return nil, err
	}
	level, err := s.levelOn(ctx, who, cal)
	if err != nil {
		return nil, err
	}
	if !canWrite(level) {
		return nil, denied("no write access")
	}
	responses, err := s.store.MyResponses(ctx, who.NGACNodeID, []string{e.ID})
	if err != nil {
		return nil, internal("load answers", err)
	}
	_, guest := responses[e.ID]
	if !showsDetail(level, e, cal, who.NGACNodeID, guest) {
		return nil, denied("no write access")
	}
	return &editing{who: who, ws: ws, e: e, cal: cal, level: level}, nil
}

// versionOK reports whether the version the client holds is current for the
// instance it edits: the series' own, or the exception's when the instance has one.
func versionOK(held int, series *store.Event, override *store.Event) bool {
	return held == series.Version || (override != nil && held == override.Version)
}

// scopeOf validates the scope of a change to a repeating event. A single event
// takes none, and any it is given means "all".
func scopeOf(q ScopeQuery, recurring bool) (string, error) {
	if !recurring {
		return ScopeAll, nil
	}
	switch q.Scope {
	case ScopeThis, ScopeFollowing:
		if q.Occurrence == "" {
			return "", invalid("occurrence is required for scope %s", q.Scope)
		}
		return q.Scope, nil
	case ScopeAll:
		return ScopeAll, nil
	case "":
		return "", invalid("scope is required for a repeating event")
	}
	return "", invalid("scope must be this, following or all")
}

// UpdateEvent changes an event, an occurrence of it, or it and the occurrences
// after it, as the scope says. The patch's version must be the one the edit
// started from, else the answer is a 409.
func (s *Service) UpdateEvent(ctx context.Context, wsID, id string, q ScopeQuery, p EventPatch) (*EventDetailView, error) {
	ed, err := s.beginEdit(ctx, wsID, id)
	if err != nil {
		return nil, err
	}
	if p.Version <= 0 {
		return nil, invalid("version is required")
	}
	if (p.Attendees != nil && len(*p.Attendees) > 0) || (p.InviteChannelID != nil && *p.InviteChannelID != "") {
		return nil, invalid("guests cannot be changed yet")
	}
	scope, err := scopeOf(q, ed.e.RRule != "")
	if err != nil {
		return nil, err
	}
	var target *store.Calendar
	if p.CalendarID != nil && *p.CalendarID != ed.e.CalendarID {
		if scope != ScopeAll {
			return nil, invalid("an event moves to another calendar as a whole")
		}
		if ed.e.NgacOAID != "" {
			return nil, invalid("an event with guests cannot change calendar")
		}
		if target, err = s.loadCalendar(ctx, ed.ws.ID, *p.CalendarID); err != nil {
			return nil, err
		}
		if err := s.authorize(ctx, ed.who.NGACNodeID, target, ngac.OpWrite); err != nil {
			return nil, err
		}
	}
	minutes, err := optionalReminders(p.Reminders)
	if err != nil {
		return nil, err
	}

	var in instance
	var preserved int
	switch {
	case ed.e.RRule == "":
		in, err = s.updateSingle(ctx, ed, p, target, minutes)
	case scope == ScopeThis:
		in, err = s.updateOccurrence(ctx, ed, q.Occurrence, p)
	default:
		in, preserved, err = s.updateSeries(ctx, ed, scope, q.Occurrence, p, target, minutes)
	}
	if err != nil {
		return nil, err
	}
	s.announce(ctx, ed.ws, ed.cal, ed.who.NGACNodeID, realtime.KindUpdated, distinct(ed.e.ID, in.Master.ID)...)
	if in.Master.CalendarID != ed.cal.ID {
		moved, err := s.loadCalendar(ctx, ed.ws.ID, in.Master.CalendarID)
		if err == nil {
			s.announce(ctx, ed.ws, moved, ed.who.NGACNodeID, realtime.KindUpdated, ed.e.ID)
		}
	}
	return s.detailView(ctx, ed.ws, ed.who, in, ed.level, true, preserved)
}

func optionalReminders(in *[]ReminderInput) (*[]int, error) {
	if in == nil {
		return nil, nil
	}
	m, err := reminderMinutes(*in)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// applyContent copies the fields a patch sets that do not depend on time or scope.
func applyContent(e *store.Event, p *EventPatch) {
	if p.Title != nil {
		e.Title = *p.Title
	}
	if p.Description != nil {
		e.Description = *p.Description
	}
	if p.Location != nil {
		e.Location = *p.Location
	}
	if p.Kind != nil {
		e.Kind = *p.Kind
	}
	if p.Transparency != nil {
		e.Transparency = *p.Transparency
	}
	if p.Visibility != nil {
		e.Visibility = *p.Visibility
	}
	if p.GuestsCanInvite != nil {
		e.GuestsCanInvite = *p.GuestsCanInvite
	}
	if p.GuestsCanSeeList != nil {
		e.GuestsCanSeeList = *p.GuestsCanSeeList
	}
	if p.HasCall != nil {
		e.HasCall = *p.HasCall
	}
}

// retime resolves a patch's zone and all-day flag over an event's.
func retime(e *store.Event, p *EventPatch) (loc *time.Location, allDay bool, err error) {
	tz := e.TZID
	if p.TZID != nil {
		tz = *p.TZID
	}
	allDay = e.AllDay
	if p.AllDay != nil {
		allDay = *p.AllDay
	}
	loc, err = loadZone(tz)
	return loc, allDay, err
}

// reposition is where an instance that stands at [curStart, curEnd) goes given the
// patch: its start and end when sent, else the instance's own, the length kept
// when only the start moves.
func reposition(p *EventPatch, loc *time.Location, allDay bool, curStart, curEnd time.Time) (start, end time.Time, err error) {
	start, end = curStart, curEnd
	if p.Start != nil {
		if start, err = parseWhen(*p.Start, loc, allDay); err != nil {
			return
		}
		end = fromWall(wall(start, loc).Add(wall(curEnd, loc).Sub(wall(curStart, loc))), loc)
	}
	if p.End != nil {
		if end, err = parseWhen(*p.End, loc, allDay); err != nil {
			return
		}
	}
	return start, end, nil
}

// updateSingle changes an event that does not repeat (or makes it repeat).
func (s *Service) updateSingle(ctx context.Context, ed *editing, p EventPatch, target *store.Calendar, minutes *[]int) (instance, error) {
	if !versionOK(p.Version, ed.e, nil) {
		return instance{}, conflict()
	}
	n := *ed.e
	applyContent(&n, &p)
	loc, allDay, err := retime(&n, &p)
	if err != nil {
		return instance{}, err
	}
	zoneChanged := loc.String() != n.TZID || allDay != n.AllDay
	if zoneChanged && (p.Start == nil || p.End == nil) {
		return instance{}, invalid("changing the time zone or all-day setting needs the new start and end")
	}
	if p.Start != nil || p.End != nil {
		if n.StartsAt, n.EndsAt, err = reposition(&p, loc, allDay, n.StartsAt, n.EndsAt); err != nil {
			return instance{}, err
		}
	}
	n.TZID, n.AllDay = loc.String(), allDay
	if p.RRule.Set {
		n.RRule = ""
		if p.RRule.Value != nil {
			n.RRule = normalizeRRule(*p.RRule.Value)
		}
	}
	if target != nil {
		n.CalendarID = target.ID
	}
	if err := finalizeEvent(&n); err != nil {
		return instance{}, err
	}
	if err := s.store.UpdateEvent(ctx, &n, ed.e.Version, store.EventChange{Reminders: minutes}); err != nil {
		return instance{}, storeErr("save event", err)
	}
	return instance{Row: &n, Master: &n, Original: n.StartsAt, Start: n.StartsAt, End: n.EndsAt}, nil
}

// exceptionOf is a new exception row for the occurrence at orig of a series,
// starting as a copy of the series' fields.
func exceptionOf(m *store.Event, orig, start, end time.Time) *store.Event {
	o := *m
	o.ID = uuid.NewString()
	o.RecurringEventID, o.OriginalStart = m.ID, &orig
	o.RRule, o.RecurrenceUntil = "", nil
	o.NgacOAID, o.GuestsUAID = "", ""
	o.StartsAt, o.EndsAt = start, end
	o.Cancelled, o.Version = false, 1
	return &o
}

// occurrenceFor finds what the occurrence at orig of a series is now: its
// exception if it has one, else the span the rule gives it (override is nil then).
func (s *Service) occurrenceFor(ctx context.Context, m *store.Event, occurrence string) (orig time.Time, override *store.Event, sp span, err error) {
	ru, err := recurrenceOf(m)
	if err != nil {
		return orig, nil, sp, internal("read recurrence", err)
	}
	if orig, err = parseOccurrence(occurrence, ru.loc, m.AllDay); err != nil {
		return orig, nil, sp, err
	}
	override, err = s.store.OverrideOf(ctx, m.ID, orig)
	switch {
	case err == nil:
		if override.Cancelled {
			return orig, nil, sp, notFound("that occurrence was cancelled")
		}
		return orig, override, span{override.StartsAt, override.EndsAt}, nil
	case !errors.Is(err, store.ErrNotFound):
		return orig, nil, sp, internal("load exception", err)
	case !ru.has(orig):
		return orig, nil, sp, notFound("occurrence not found")
	}
	return orig, nil, span{orig, ru.endOf(orig)}, nil
}

// updateOccurrence changes one occurrence of a series: it writes (or rewrites) the
// exception that replaces it.
func (s *Service) updateOccurrence(ctx context.Context, ed *editing, occurrence string, p EventPatch) (instance, error) {
	if p.RRule.Set || p.TZID != nil || p.AllDay != nil {
		return instance{}, invalid("the repeat rule, time zone and all-day setting belong to the whole series")
	}
	orig, override, sp, err := s.occurrenceFor(ctx, ed.e, occurrence)
	if err != nil {
		return instance{}, err
	}
	// The token of an exception is its own; of an occurrence that has none, the series'.
	if (override != nil && p.Version != override.Version) || (override == nil && p.Version != ed.e.Version) {
		return instance{}, conflict()
	}
	var row *store.Event
	if override != nil {
		c := *override
		row = &c
	} else {
		row = exceptionOf(ed.e, orig, sp.start, sp.end)
	}
	applyContent(row, &p)
	loc := zoneOf(row.TZID)
	if row.StartsAt, row.EndsAt, err = reposition(&p, loc, row.AllDay, sp.start, sp.end); err != nil {
		return instance{}, err
	}
	if err := finalizeEvent(row); err != nil {
		return instance{}, err
	}
	if override != nil {
		err = s.store.UpdateEvent(ctx, row, override.Version, store.EventChange{})
	} else {
		err = s.store.InsertEvent(ctx, row, nil)
	}
	if err != nil {
		return instance{}, storeErr("save exception", err)
	}
	return instance{Row: row, Master: ed.e, Original: orig, Start: row.StartsAt, End: row.EndsAt}, nil
}

// updateSeries changes a series for all its occurrences or for the named one and
// those after it. A change that starts at the first occurrence is a change of the
// whole series either way.
func (s *Service) updateSeries(ctx context.Context, ed *editing, scope, occurrence string, p EventPatch, target *store.Calendar, minutes *[]int) (instance, int, error) {
	m := ed.e
	loc, allDay, err := retime(m, &p)
	if err != nil {
		return instance{}, 0, err
	}
	cur := span{m.StartsAt, m.EndsAt}
	orig := m.StartsAt
	var override *store.Event
	if occurrence != "" {
		if orig, override, cur, err = s.occurrenceFor(ctx, m, occurrence); err != nil {
			return instance{}, 0, err
		}
	}
	if !versionOK(p.Version, m, override) {
		return instance{}, 0, conflict()
	}
	zoneChanged := loc.String() != m.TZID || allDay != m.AllDay
	if zoneChanged && (p.Start == nil || p.End == nil || !orig.Equal(m.StartsAt)) {
		return instance{}, 0, invalid("changing the time zone or all-day setting needs the first occurrence's new start and end")
	}
	newStart, newEnd, err := reposition(&p, loc, allDay, cur.start, cur.end)
	if err != nil {
		return instance{}, 0, err
	}
	moved := !newStart.Equal(cur.start) || !newEnd.Equal(cur.end)
	if !newEnd.After(newStart) {
		return instance{}, 0, invalid("the event must end after it starts")
	}

	if scope == ScopeFollowing && !orig.Equal(m.StartsAt) {
		return s.splitSeries(ctx, ed, orig, override, cur, span{newStart, newEnd}, p, loc, minutes)
	}

	n := *m
	applyContent(&n, &p)
	n.TZID, n.AllDay = loc.String(), allDay
	change := store.EventChange{Reminders: minutes}
	all, err := s.store.Overrides(ctx, m.ID)
	if err != nil {
		return instance{}, 0, internal("load exceptions", err)
	}
	if moved {
		n.StartsAt = newStart
		if !zoneChanged {
			n.StartsAt = shiftBy(m.StartsAt, cur.start, newStart, loc)
		}
		if allDay {
			n.EndsAt = n.StartsAt.AddDate(0, 0, wholeDays(newStart, newEnd))
		} else {
			n.EndsAt = fromWall(wall(n.StartsAt, loc).Add(wall(newEnd, loc).Sub(wall(newStart, loc))), loc)
		}
		if !p.RRule.Set {
			if d := wall(newStart, loc).Weekday() - wall(cur.start, loc).Weekday(); d != 0 {
				n.RRule = shiftWeekdays(n.RRule, int(d))
			}
		}
		change.Originals = map[string]time.Time{}
		for _, o := range all {
			if o.OriginalStart != nil {
				change.Originals[o.ID] = shiftBy(*o.OriginalStart, cur.start, newStart, loc)
			}
		}
	}
	if p.RRule.Set {
		n.RRule = ""
		if p.RRule.Value != nil {
			n.RRule = normalizeRRule(*p.RRule.Value)
		}
		if n.RRule == "" {
			change.DropOverrides, change.Originals, all = true, nil, nil // no series, no exceptions
		}
	}
	if target != nil {
		n.CalendarID = target.ID
	}
	if err := finalizeEvent(&n); err != nil {
		return instance{}, 0, err
	}
	if err := s.store.UpdateEvent(ctx, &n, m.Version, change); err != nil {
		return instance{}, 0, storeErr("save series", err)
	}
	preserved := 0
	for _, o := range all {
		if !o.Cancelled {
			preserved++
		}
	}
	return s.firstOrNamed(&n, orig, moved, cur.start, newStart, loc, change.Originals), preserved, nil
}

// firstOrNamed is the instance an edit of the series answers with: the one the
// client named, carried to where it now stands, else the series' first.
func (s *Service) firstOrNamed(n *store.Event, orig time.Time, moved bool, curStart, newStart time.Time, loc *time.Location, originals map[string]time.Time) instance {
	first := instance{Row: n, Master: n, Original: n.StartsAt, Start: n.StartsAt, End: n.EndsAt}
	ru, err := recurrenceOf(n)
	if err != nil || ru == nil {
		return first
	}
	at := orig
	if moved {
		at = shiftBy(orig, curStart, newStart, loc)
	}
	if ru.has(at) {
		return instance{Row: n, Master: n, Original: at, Start: at, End: ru.endOf(at)}
	}
	return first
}

// splitSeries ends the series just before the named occurrence and starts a new
// event there with the change applied; guests and reminders are copied over, and
// the exceptions from that occurrence on are dropped with the old tail.
func (s *Service) splitSeries(ctx context.Context, ed *editing, orig time.Time, override *store.Event, cur, pos span, p EventPatch, loc *time.Location, minutes *[]int) (instance, int, error) {
	m := ed.e
	ru, err := recurrenceOf(m)
	if err != nil {
		return instance{}, 0, internal("read recurrence", err)
	}
	remaining := 0
	if ru.opt.Count > 0 {
		remaining = ru.opt.Count - ru.countBefore(orig)
	}

	tail := *m
	if override != nil {
		tail = *override
		tail.RRule, tail.RecurringEventID, tail.OriginalStart, tail.Cancelled = m.RRule, "", nil, false
		tail.OrganizerNodeID, tail.CalendarID = m.OrganizerNodeID, m.CalendarID
	}
	tail.ID, tail.Version = uuid.NewString(), 1
	tail.NgacOAID, tail.GuestsUAID = "", ""
	applyContent(&tail, &p)
	tail.TZID, tail.StartsAt, tail.EndsAt = loc.String(), pos.start, pos.end
	body := m.RRule
	switch {
	case p.RRule.Set:
		body = ""
		if p.RRule.Value != nil {
			body = normalizeRRule(*p.RRule.Value)
		}
	default:
		if remaining > 0 {
			body = withCount(body, remaining)
		}
		if d := wall(pos.start, loc).Weekday() - wall(cur.start, loc).Weekday(); d != 0 {
			body = shiftWeekdays(body, int(d))
		}
	}
	tail.RRule = body
	if err := finalizeEvent(&tail); err != nil {
		return instance{}, 0, err
	}

	old := *m
	old.RRule = endedBefore(m.RRule, orig)
	if err := finalizeEvent(&old); err != nil {
		return instance{}, 0, internal("end the series", err)
	}
	if err := s.store.CutSeries(ctx, &old, m.Version, orig, &tail, minutes); err != nil {
		return instance{}, 0, storeErr("split series", err)
	}
	return instance{Row: &tail, Master: &tail, Original: tail.StartsAt, Start: tail.StartsAt, End: tail.EndsAt}, 0, nil
}

// --- deleting ---------------------------------------------------------------

// DeleteEvent removes an event, one occurrence of it, or it and every later
// occurrence. version, when not zero, is the version the caller's view had.
func (s *Service) DeleteEvent(ctx context.Context, wsID, id string, q ScopeQuery, version int) error {
	ed, err := s.beginEdit(ctx, wsID, id)
	if err != nil {
		return err
	}
	scope, err := scopeOf(q, ed.e.RRule != "")
	if err != nil {
		return err
	}
	m := ed.e
	var orig time.Time
	var override *store.Event
	var sp span
	if scope != ScopeAll {
		if orig, override, sp, err = s.occurrenceFor(ctx, m, q.Occurrence); err != nil {
			return err
		}
	}
	if version > 0 && !versionOK(version, m, override) {
		return conflict()
	}

	switch {
	case scope == ScopeAll || (scope == ScopeFollowing && orig.Equal(m.StartsAt)):
		if err := s.deleteWhole(ctx, m); err != nil {
			return err
		}
	case scope == ScopeFollowing:
		old := *m
		old.RRule = endedBefore(m.RRule, orig)
		if err := finalizeEvent(&old); err != nil {
			return internal("end the series", err)
		}
		if err := s.store.CutSeries(ctx, &old, m.Version, orig, nil, nil); err != nil {
			return storeErr("end series", err)
		}
	default: // this occurrence only: it stays as a cancelled exception
		if override != nil {
			c := *override
			c.Cancelled = true
			err = s.store.UpdateEvent(ctx, &c, override.Version, store.EventChange{})
		} else {
			c := exceptionOf(m, orig, sp.start, sp.end)
			c.Cancelled = true
			err = s.store.InsertEvent(ctx, c, nil)
		}
		if err != nil {
			return storeErr("cancel occurrence", err)
		}
	}
	s.announce(ctx, ed.ws, ed.cal, ed.who.NGACNodeID, realtime.KindDeleted, m.ID)
	return nil
}

// deleteWhole removes an event for good, with the graph nodes it made. The nodes
// go first: a failure leaves the event whole, so the delete can be retried.
func (s *Service) deleteWhole(ctx context.Context, e *store.Event) error {
	for _, node := range []string{e.GuestsUAID, e.NgacOAID} {
		if node == "" {
			continue
		}
		if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: node}); err != nil {
			return internal("delete the event's access nodes", err)
		}
	}
	return internal("delete event", s.store.DeleteEvent(ctx, e.ID))
}

// --- announcing -------------------------------------------------------------

// announce tells the screens that can see a calendar that an event changed. The
// audience follows the calendar: everyone in the workspace for the workspace
// calendar, the conversation for a group's, the owner and the people it is shared
// with for a personal one. The frame carries ids only.
func (s *Service) announce(ctx context.Context, ws *store.Workspace, cal *store.Calendar, actor, kind string, eventIDs ...string) {
	e := realtime.For(ctx, realtime.DomainCalendar, kind, ws.ID, eventIDs...)
	e.ParentID = cal.ID
	switch cal.Kind {
	case store.KindChannel:
		e.ChannelID = cal.ChannelID
	case store.KindPersonal:
		nodes := []string{cal.OwnerNodeID, actor}
		shares, err := s.store.Shares(ctx, cal.ID)
		if err != nil {
			slog.Warn("could not list shares to address a calendar event", "calendar", cal.ID, "error", err)
		}
		for _, sh := range shares {
			nodes = append(nodes, sh.GranteeNodeID)
		}
		e.UserNodeIDs = distinct(nodes...)
	}
	s.emit(e)
}
