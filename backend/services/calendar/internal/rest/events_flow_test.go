package rest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Event endpoints end to end: real routes, JWT, domain and store against the test
// database, the policy service faked.

type occJSON struct {
	EventID            string `json:"eventId"`
	OccurrenceStart    string `json:"occurrenceStart"`
	Start              string `json:"start"`
	End                string `json:"end"`
	AllDay             bool   `json:"allDay"`
	CalendarID         string `json:"calendarId"`
	Title              string `json:"title"`
	Location           string `json:"location"`
	BusyOnly           bool   `json:"busyOnly"`
	MyResponse         string `json:"myResponse"`
	IsOrganizer        bool   `json:"isOrganizer"`
	IsRecurring        bool   `json:"isRecurring"`
	RecurrenceText     string `json:"recurrenceText"`
	CanEdit            bool   `json:"canEdit"`
	Version            int    `json:"version"`
	RRule              string `json:"rrule"`
	TZID               string `json:"tzid"`
	PreservedOverrides int    `json:"preservedOverrides"`
	Attendees          []struct {
		Person struct {
			DisplayName string `json:"displayName"`
			Hue         int    `json:"hue"`
		} `json:"person"`
		IsOrganizer bool `json:"isOrganizer"`
	} `json:"attendees"`
	Reminders []struct {
		MinutesBefore int `json:"minutesBefore"`
	} `json:"reminders"`
}

func (f *flow) personalCal(who string) string {
	f.t.Helper()
	for _, c := range f.list(who) {
		if c.Kind == "personal" && c.IsMine {
			return c.ID
		}
	}
	f.t.Fatal("no personal calendar")
	return ""
}

func (f *flow) occurrences(who, cal string) []occJSON {
	f.t.Helper()
	q := url.Values{"from": {"2026-11-01T00:00:00+07:00"}, "to": {"2026-12-01T00:00:00+07:00"}, "calendars": {cal}}
	code, body := f.do(http.MethodGet, f.base()+"/events?"+q.Encode(), who, "")
	require.Equal(f.t, http.StatusOK, code, string(body))
	var out struct {
		Occurrences []occJSON `json:"occurrences"`
	}
	require.NoError(f.t, json.Unmarshal(body, &out))
	return out.Occurrences
}

func (f *flow) createEvent(who, body string) (int, occJSON, []byte) {
	f.t.Helper()
	code, raw := f.do(http.MethodPost, f.base()+"/events", who, body)
	var o occJSON
	if code == http.StatusCreated {
		require.NoError(f.t, json.Unmarshal(raw, &o))
	}
	return code, o, raw
}

func (f *flow) patch(who, id, query, body string) (int, occJSON, []byte) {
	f.t.Helper()
	code, raw := f.do(http.MethodPatch, f.base()+"/events/"+id+query, who, body)
	var o occJSON
	if code == http.StatusOK {
		require.NoError(f.t, json.Unmarshal(raw, &o))
	}
	return code, o, raw
}

func eventBody(cal, extra string) string {
	return `{"calendarId":"` + cal + `","title":"Standup","location":"Room 1","start":"2026-11-02T09:00:00+07:00","end":"2026-11-02T10:00:00+07:00",` +
		`"allDay":false,"tzid":"Asia/Ho_Chi_Minh","kind":"event","transparency":"busy","visibility":"default","attendees":[],` +
		`"guestsCanInvite":false,"guestsCanSeeList":true,"hasCall":false` + extra + `}`
}

func scopeQ(scope, occurrence string) string {
	return "?" + url.Values{"scope": {scope}, "occurrence": {occurrence}}.Encode()
}

func TestEventScopesAndConflict(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")

	code, created, raw := f.createEvent("alice", eventBody(cal, `,"rrule":"FREQ=WEEKLY;BYDAY=MO"`))
	require.Equal(t, http.StatusCreated, code, string(raw))
	assert.Equal(t, "Hằng tuần vào thứ Hai", created.RecurrenceText)
	assert.Equal(t, 1, created.Version)
	assert.True(t, created.IsOrganizer)
	require.Len(t, created.Attendees, 1)
	assert.Equal(t, "Alice Nguyen", created.Attendees[0].Person.DisplayName)
	require.Len(t, created.Reminders, 1)
	assert.Equal(t, 10, created.Reminders[0].MinutesBefore)
	assert.NotContains(t, string(raw), "null", "absent optional fields are omitted, never null")
	assert.NotContains(t, string(raw), "node-"+f.sfx, "no graph id reaches a client")
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM ngac_nodes WHERE name LIKE 'CalEv%' AND id LIKE $1`, "node-"+f.sfx+"-%"), "a plain event has no node")

	occ := f.occurrences("alice", cal)
	require.Len(t, occ, 5)
	assert.Equal(t, "2026-11-09T09:00:00+07:00", occ[1].Start)
	assert.Equal(t, created.EventID, occ[1].EventID)
	assert.True(t, occ[1].CanEdit)
	assert.Equal(t, "accepted", occ[1].MyResponse)

	// This one only.
	code, got, raw := f.patch("alice", created.EventID, scopeQ("this", occ[1].OccurrenceStart),
		`{"version":1,"title":"Moved","start":"2026-11-09T10:00:00+07:00","end":"2026-11-09T11:00:00+07:00"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	assert.Equal(t, "Moved", got.Title)
	assert.Equal(t, created.EventID, got.EventID)
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM calendar_events WHERE calendar_id = $1`, cal), "the series and one exception")
	occ = f.occurrences("alice", cal)
	require.Len(t, occ, 5)
	assert.Equal(t, "Moved", occ[1].Title)
	assert.Equal(t, "2026-11-09T10:00:00+07:00", occ[1].Start)

	// All: the series is renamed, the exception is kept and counted.
	code, got, raw = f.patch("alice", created.EventID, scopeQ("all", occ[0].OccurrenceStart), `{"version":1,"title":"Sync"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	assert.Equal(t, "Sync", got.Title)
	assert.Equal(t, 1, got.PreservedOverrides)
	assert.Equal(t, 2, got.Version)
	occ = f.occurrences("alice", cal)
	assert.Equal(t, []string{"Sync", "Moved", "Sync", "Sync", "Sync"}, []string{occ[0].Title, occ[1].Title, occ[2].Title, occ[3].Title, occ[4].Title})

	// The same edit again from the old version: a conflict with a message to show.
	code, _, raw = f.patch("alice", created.EventID, scopeQ("all", occ[0].OccurrenceStart), `{"version":1,"title":"Stale"}`)
	assert.Equal(t, http.StatusConflict, code)
	assert.Contains(t, string(raw), "Sự kiện vừa được người khác sửa. Tải lại rồi thử lại.")

	// Following: from the 23rd on, in the afternoon, as a new event.
	code, tail, raw := f.patch("alice", created.EventID, scopeQ("following", occ[3].OccurrenceStart),
		`{"version":2,"title":"Afternoon","start":"2026-11-23T14:00:00+07:00","end":"2026-11-23T15:00:00+07:00"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	assert.NotEqual(t, created.EventID, tail.EventID)
	assert.Equal(t, "Afternoon", tail.Title)
	assert.Equal(t, "2026-11-23T14:00:00+07:00", tail.Start)
	require.Len(t, tail.Attendees, 1, "the guests are copied")
	assert.Equal(t, 3, f.count(`SELECT COUNT(*) FROM calendar_events WHERE calendar_id = $1`, cal), "old series, its exception and the tail")
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendar_reminders WHERE event_id = $1`, tail.EventID))
	occ = f.occurrences("alice", cal)
	assert.Equal(t, []string{"2026-11-02T09:00", "2026-11-09T10:00", "2026-11-16T09:00", "2026-11-23T14:00", "2026-11-30T14:00"},
		[]string{occ[0].Start[:16], occ[1].Start[:16], occ[2].Start[:16], occ[3].Start[:16], occ[4].Start[:16]})

	// Delete this occurrence, then it and what follows, then the rest.
	code, _ = f.do(http.MethodDelete, f.base()+"/events/"+tail.EventID+scopeQ("this", occ[3].OccurrenceStart), "alice", "")
	assert.Equal(t, http.StatusNoContent, code)
	assert.Len(t, f.occurrences("alice", cal), 4)
	code, _ = f.do(http.MethodDelete, f.base()+"/events/"+created.EventID+scopeQ("following", occ[2].OccurrenceStart), "alice", "")
	assert.Equal(t, http.StatusNoContent, code)
	occ = f.occurrences("alice", cal)
	require.Len(t, occ, 3, "two before the cut and the tail's 30 November")
	assert.Equal(t, []string{"Sync", "Moved", "Afternoon"}, []string{occ[0].Title, occ[1].Title, occ[2].Title})
	code, _ = f.do(http.MethodDelete, f.base()+"/events/"+created.EventID+scopeQ("all", ""), "alice", "")
	assert.Equal(t, http.StatusNoContent, code)
	code, _ = f.do(http.MethodDelete, f.base()+"/events/"+tail.EventID+scopeQ("all", ""), "alice", "")
	assert.Equal(t, http.StatusNoContent, code)
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_events WHERE calendar_id = $1`, cal), "exceptions, guests and reminders went with them")
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_attendees WHERE user_node_id = $1`, f.nodes["alice"]))
	code, _ = f.do(http.MethodGet, f.base()+"/events/"+created.EventID, "alice", "")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestEventAuthorizationOverREST(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	code, ev, raw := f.createEvent("alice", eventBody(cal, ""))
	require.Equal(t, http.StatusCreated, code, string(raw))
	code, priv, raw := f.createEvent("alice", strings.Replace(eventBody(cal, ""), `"visibility":"default"`, `"visibility":"private"`, 1))
	require.Equal(t, http.StatusCreated, code, string(raw))

	// A colleague with no share: busy blocks only, never details; no writes.
	occ := f.occurrences("carol", cal)
	require.Len(t, occ, 2)
	for _, o := range occ {
		assert.True(t, o.BusyOnly)
		assert.Equal(t, "Bận", o.Title)
		assert.Empty(t, o.Location)
		assert.False(t, o.CanEdit)
	}
	_, body := f.do(http.MethodGet, f.base()+"/events?from=2026-11-01T00:00:00%2B07:00&to=2026-12-01T00:00:00%2B07:00&calendars="+cal, "carol", "")
	assert.NotContains(t, string(body), "Standup")
	assert.NotContains(t, string(body), "Room 1")
	code, _ = f.do(http.MethodGet, f.base()+"/events/"+ev.EventID, "carol", "")
	assert.Equal(t, http.StatusForbidden, code, "a stranger reads no details")
	code, _, _ = f.createEvent("carol", eventBody(cal, ""))
	assert.Equal(t, http.StatusForbidden, code)
	code, _, _ = f.patch("carol", ev.EventID, "", `{"version":1,"title":"x"}`)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodDelete, f.base()+"/events/"+ev.EventID, "carol", "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodGet, f.base()+"/events/"+ev.EventID, "gus", "")
	assert.Equal(t, http.StatusForbidden, code, "nor does someone outside the workspace")

	// A view share reads the ordinary event, not the private one, and edits neither.
	code, _ = f.do(http.MethodPut, f.base()+"/calendars/"+cal+"/shares/"+f.nodes["bob"], "alice", `{"level":"view"}`)
	require.Equal(t, http.StatusOK, code)
	code, _, _ = f.patch("bob", ev.EventID, "", `{"version":1,"title":"x"}`)
	assert.Equal(t, http.StatusForbidden, code)
	code, body = f.do(http.MethodGet, f.base()+"/events/"+ev.EventID, "bob", "")
	require.Equal(t, http.StatusOK, code, string(body))
	assert.Contains(t, string(body), `"canEdit":false`)
	code, _ = f.do(http.MethodGet, f.base()+"/events/"+priv.EventID, "bob", "")
	assert.Equal(t, http.StatusForbidden, code, "a private event is hidden from a view share")
	byTitle := map[string]bool{}
	for _, o := range f.occurrences("bob", cal) {
		byTitle[o.Title] = o.BusyOnly
	}
	assert.Equal(t, map[string]bool{"Standup": false, "Bận": true}, byTitle)

	// An edit share changes the ordinary event.
	code, _ = f.do(http.MethodPut, f.base()+"/calendars/"+cal+"/shares/"+f.nodes["bob"], "alice", `{"level":"edit"}`)
	require.Equal(t, http.StatusOK, code)
	code, got, raw := f.patch("bob", ev.EventID, "", `{"version":1,"title":"By Bob"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	assert.Equal(t, "By Bob", got.Title)
	assert.True(t, got.CanEdit)

	// The workspace calendar is read-only to an ordinary member.
	var wsCal string
	for _, c := range f.list("alice") {
		if c.Kind == "workspace" {
			wsCal = c.ID
		}
	}
	code, _, _ = f.createEvent("alice", eventBody(wsCal, ""))
	assert.Equal(t, http.StatusForbidden, code)
}

func TestEventValidationOverREST(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	for name, body := range map[string]string{
		"no title":      strings.Replace(eventBody(cal, ""), `"title":"Standup"`, `"title":""`, 1),
		"ends early":    strings.Replace(eventBody(cal, ""), `10:00:00+07:00`, `08:00:00+07:00`, 1),
		"bad zone":      strings.Replace(eventBody(cal, ""), `Asia/Ho_Chi_Minh`, `Nowhere/Land`, 1),
		"hourly":        eventBody(cal, `,"rrule":"FREQ=HOURLY"`),
		"too many":      eventBody(cal, `,"rrule":"FREQ=DAILY;COUNT=731"`),
		"guests":        strings.Replace(eventBody(cal, ""), `"attendees":[]`, `"attendees":["x"]`, 1),
		"bad reminders": eventBody(cal, `,"reminders":[{"minutesBefore":-5}]`),
	} {
		code, _, raw := f.createEvent("alice", body)
		assert.Equal(t, http.StatusUnprocessableEntity, code, name+": "+string(raw))
	}
	code, _, _ := f.createEvent("alice", `not json`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _, _ = f.createEvent("alice", eventBody("missing", ""))
	assert.Equal(t, http.StatusNotFound, code)
	code, _, _ = f.createEvent("", eventBody(cal, ""))
	assert.Equal(t, http.StatusUnauthorized, code)

	code, ev, _ := f.createEvent("alice", eventBody(cal, `,"rrule":"FREQ=DAILY"`))
	require.Equal(t, http.StatusCreated, code)
	code, _, _ = f.patch("alice", ev.EventID, "", `{"version":1,"title":"x"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code, "a repeating event needs a scope")
	code, _, _ = f.patch("alice", ev.EventID, scopeQ("all", ""), `{"title":"x"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code, "and an edit says which version it started from")
	code, _, _ = f.patch("alice", ev.EventID, scopeQ("all", ""), `{"version":1,"rrule":null}`)
	assert.Equal(t, http.StatusOK, code, "null removes the repeat")
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_events WHERE recurring_event_id IS NOT NULL AND calendar_id = $1`, cal))
	code, body := f.do(http.MethodGet, f.base()+"/events/"+ev.EventID, "alice", "")
	require.Equal(t, http.StatusOK, code)
	assert.NotContains(t, string(body), `"rrule"`)

	code, _ = f.do(http.MethodGet, f.base()+"/events?from=x&to=y", "alice", "")
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = f.do(http.MethodGet, f.base()+"/events?from=2026-11-01T00:00:00Z&to=2026-12-01T00:00:00Z&calendars="+f.list("alice")[1].ID+",nope", "alice", "")
	assert.Equal(t, http.StatusNotFound, code)
}
