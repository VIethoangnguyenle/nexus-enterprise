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

// Guests, answers, free/busy, suggestions, search and settings end to end: real
// routes, JWT, domain and SQL against the test database, the policy service faked.

type guestDetail struct {
	EventID    string `json:"eventId"`
	Title      string `json:"title"`
	Version    int    `json:"version"`
	MyResponse string `json:"myResponse"`
	CanEdit    bool   `json:"canEdit"`
	GuestCount int    `json:"guestCount"`
	Attendees  []struct {
		Person struct {
			NodeID      string `json:"nodeId"`
			DisplayName string `json:"displayName"`
		} `json:"person"`
		Response    string `json:"response"`
		Comment     string `json:"comment"`
		IsOrganizer bool   `json:"isOrganizer"`
	} `json:"attendees"`
}

func (f *flow) detail(who, id, query string) (int, guestDetail) {
	f.t.Helper()
	code, raw := f.do(http.MethodGet, f.base()+"/events/"+id+query, who, "")
	var d guestDetail
	if code == http.StatusOK {
		require.NoError(f.t, json.Unmarshal(raw, &d))
	}
	return code, d
}

func (f *flow) invitedTo(who string) []occJSON {
	f.t.Helper()
	q := url.Values{"from": {"2026-11-01T00:00:00+07:00"}, "to": {"2026-12-01T00:00:00+07:00"}}
	code, body := f.do(http.MethodGet, f.base()+"/events?"+q.Encode(), who, "")
	require.Equal(f.t, http.StatusOK, code, string(body))
	var out struct {
		Occurrences []occJSON `json:"occurrences"`
	}
	require.NoError(f.t, json.Unmarshal(body, &out))
	return out.Occurrences
}

func quoted(nodes ...string) string {
	b, _ := json.Marshal(nodes)
	return string(b)
}

func TestGuestFlowOverREST(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	bob, carol, gus := f.nodes["bob"], f.nodes["carol"], f.nodes["gus"]

	// Alice invites Bob to a repeating event. Nobody outside the workspace can be invited.
	code, _, raw := f.createEvent("alice", strings.Replace(eventBody(cal, `,"rrule":"FREQ=WEEKLY;BYDAY=MO"`), `"attendees":[]`, `"attendees":`+quoted(gus), 1))
	assert.Equal(t, http.StatusForbidden, code, string(raw))
	code, _, raw = f.createEvent("alice", strings.Replace(eventBody(cal, ""), `"attendees":[]`, `"attendees":["a@b.co"]`, 1))
	assert.Equal(t, http.StatusUnprocessableEntity, code, string(raw))
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_events WHERE calendar_id = $1`, cal), "nothing stored for a refused invitation")

	code, ev, raw := f.createEvent("alice", strings.Replace(eventBody(cal, `,"rrule":"FREQ=WEEKLY;BYDAY=MO"`), `"attendees":[]`, `"attendees":`+quoted(bob), 1))
	require.Equal(t, http.StatusCreated, code, string(raw))
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM calendar_attendees WHERE event_id = $1`, ev.EventID))
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendar_events WHERE id = $1 AND ngac_oa_id IS NOT NULL AND guests_ua_id IS NOT NULL`, ev.EventID))
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM ngac_nodes WHERE name LIKE 'CalEv%' AND id LIKE $1`, "node-"+f.sfx+"-%"))

	// Bob sees it on his own listing though Alice's calendar is not on his list.
	got := f.invitedTo("bob")
	require.Len(t, got, 5)
	assert.False(t, got[0].BusyOnly)
	assert.Equal(t, "needs_action", got[0].MyResponse)
	assert.False(t, got[0].CanEdit)
	assert.Empty(t, f.invitedTo("carol"))

	// He answers one occurrence, then the whole series; the host sees each.
	code, raw = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/rsvp", "bob",
		`{"response":"declined","comment":"Away","scope":"this","occurrence":"`+got[1].OccurrenceStart+`"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	got = f.invitedTo("bob")
	assert.Equal(t, []string{"needs_action", "declined", "needs_action", "needs_action", "needs_action"},
		[]string{got[0].MyResponse, got[1].MyResponse, got[2].MyResponse, got[3].MyResponse, got[4].MyResponse})
	code, d := f.detail("alice", ev.EventID, "?occurrence="+url.QueryEscape(got[1].OccurrenceStart))
	require.Equal(t, http.StatusOK, code)
	require.Len(t, d.Attendees, 2)
	assert.Equal(t, "declined", d.Attendees[1].Response)
	assert.Equal(t, "Away", d.Attendees[1].Comment)
	assert.Equal(t, "Bob Tran", d.Attendees[1].Person.DisplayName)
	code, raw = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/rsvp", "bob", `{"response":"accepted","scope":"all"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_attendee_occurrences WHERE event_id = $1`, ev.EventID))
	assert.Equal(t, "accepted", f.invitedTo("bob")[1].MyResponse)

	// Deny: answering is for guests; outsiders; a repeating event needs a scope.
	for _, who := range []string{"carol", "gus"} {
		code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/rsvp", who, `{"response":"accepted","scope":"all"}`)
		assert.Equal(t, http.StatusForbidden, code, who)
	}
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/rsvp", "bob", `{"response":"accepted"}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/rsvp", "bob", `not json`)
	assert.Equal(t, http.StatusBadRequest, code)

	// Deny: a guest edits and deletes nothing, and invites only when allowed.
	code, _, _ = f.patch("bob", ev.EventID, scopeQ("all", ""), `{"version":1,"title":"x"}`)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodDelete, f.base()+"/events/"+ev.EventID+scopeQ("all", ""), "bob", "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/guests", "bob", `{"attendees":`+quoted(carol)+`}`)
	assert.Equal(t, http.StatusForbidden, code)

	// Allow: the host lets guests invite; Bob brings Carol, who may read and not edit.
	code, _, raw = f.patch("alice", ev.EventID, scopeQ("all", ""), `{"version":1,"guestsCanInvite":true}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	code, raw = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/guests", "bob", `{"attendees":`+quoted(carol)+`}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	var after guestDetail
	require.NoError(t, json.Unmarshal(raw, &after))
	assert.Equal(t, 2, after.GuestCount)
	code, d = f.detail("carol", ev.EventID, "")
	require.Equal(t, http.StatusOK, code)
	assert.False(t, d.CanEdit)
	code, _, _ = f.patch("carol", ev.EventID, scopeQ("all", ""), `{"version":2,"title":"x"}`)
	assert.Equal(t, http.StatusForbidden, code)
	// Bob still invites only people of the workspace.
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/guests", "bob", `{"attendees":`+quoted(gus)+`}`)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/guests", "bob", `{"attendees":["c@d.co"]}`)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	// A group: Alice is in the conversation, Bob and Carol are not.
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/guests", "bob", `{"inviteChannelId":"`+f.chat+`"}`)
	assert.Equal(t, http.StatusForbidden, code, "Bob cannot read the group")

	// The host removes Bob by sending the guests wanted; his next read is refused.
	code, _, raw = f.patch("alice", ev.EventID, scopeQ("all", ""), `{"version":2,"attendees":`+quoted(carol)+`}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	code, _ = f.detail("bob", ev.EventID, "")
	assert.Equal(t, http.StatusForbidden, code, "a removed guest reads nothing")
	assert.Empty(t, f.invitedTo("bob"))
	code, _ = f.do(http.MethodPost, f.base()+"/events/"+ev.EventID+"/rsvp", "bob", `{"response":"accepted","scope":"all"}`)
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_attendees WHERE event_id = $1 AND user_node_id = $2`, ev.EventID, bob))
	code, _ = f.detail("carol", ev.EventID, "")
	assert.Equal(t, http.StatusOK, code, "the others keep it")

	// "This and following" gives the tail its own nodes, and its guests read it.
	occ := f.occurrences("alice", cal)
	code, tail, raw := f.patch("alice", ev.EventID, scopeQ("following", occ[2].OccurrenceStart), `{"version":3,"title":"Afternoon"}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	require.NotEqual(t, ev.EventID, tail.EventID)
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendar_events WHERE id = $1 AND ngac_oa_id IS NOT NULL AND guests_ua_id IS NOT NULL`, tail.EventID))
	assert.Equal(t, 2, f.count(`SELECT COUNT(*) FROM calendar_attendees WHERE event_id = $1`, tail.EventID))
	code, _ = f.detail("carol", tail.EventID, "")
	assert.Equal(t, http.StatusOK, code)
	code, _ = f.detail("bob", tail.EventID, "")
	assert.Equal(t, http.StatusForbidden, code)

	// Deleting both removes every node they made.
	for _, id := range []string{tail.EventID, ev.EventID} {
		code, _ = f.do(http.MethodDelete, f.base()+"/events/"+id+scopeQ("all", ""), "alice", "")
		assert.Equal(t, http.StatusNoContent, code)
	}
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM ngac_nodes WHERE name LIKE 'CalEv%' AND id LIKE $1`, "node-"+f.sfx+"-%"))
	code, _ = f.detail("carol", ev.EventID, "")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestInvitingAGroupOverREST(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	// Bob is on the group's cached member list only; Alice is in the group.
	f.pool.Exec(t.Context(), `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, f.chat, f.nodes["bob"])

	code, _, raw := f.createEvent("alice", strings.Replace(eventBody(cal, ""), `"attendees":[]`, `"attendees":[],"inviteChannelId":"`+f.chat+`"`, 1))
	require.Equal(t, http.StatusCreated, code, string(raw))
	assert.Equal(t, 0, f.count(`SELECT COUNT(*) FROM calendar_attendees WHERE NOT is_organizer AND event_id IN (SELECT id FROM calendar_events WHERE calendar_id = $1)`, cal),
		"the cache lists Bob but the graph does not, so he is not invited")
	code, _, _ = f.createEvent("bob", strings.Replace(eventBody(f.personalCal("bob"), ""), `"attendees":[]`, `"attendees":[],"inviteChannelId":"`+f.chat+`"`, 1))
	assert.Equal(t, http.StatusForbidden, code, "someone who cannot read the group cannot invite it")
}

func TestFreeBusySuggestionsSearchAndSettingsOverREST(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	alice, bob, gus := f.nodes["alice"], f.nodes["bob"], f.nodes["gus"]
	code, _, raw := f.createEvent("alice", strings.Replace(eventBody(cal, ""), `"title":"Standup"`, `"title":"Budget review"`, 1))
	require.Equal(t, http.StatusCreated, code, string(raw))

	fb := func(who string, users ...string) (int, []byte) {
		q := url.Values{"users": {strings.Join(users, ",")}, "from": {"2026-11-02T00:00:00+07:00"}, "to": {"2026-11-03T00:00:00+07:00"}}
		return f.do(http.MethodGet, f.base()+"/freebusy?"+q.Encode(), who, "")
	}
	type busyBody struct {
		People []struct {
			Person    struct{ DisplayName string } `json:"person"`
			Busy      []map[string]string          `json:"busy"`
			WorkStart string                       `json:"workStart"`
			WorkEnd   string                       `json:"workEnd"`
		} `json:"people"`
	}

	// A colleague with no share: the range only, never the title.
	code, raw = fb("carol", alice, bob)
	require.Equal(t, http.StatusOK, code, string(raw))
	var seen busyBody
	require.NoError(t, json.Unmarshal(raw, &seen))
	require.Len(t, seen.People, 2)
	assert.Equal(t, "Alice Nguyen", seen.People[0].Person.DisplayName)
	require.Len(t, seen.People[0].Busy, 1)
	assert.Equal(t, "2026-11-02T09:00:00+07:00", seen.People[0].Busy[0]["start"])
	assert.NotContains(t, string(raw), "Budget")
	assert.Equal(t, "08:00", seen.People[0].WorkStart)
	assert.Equal(t, "17:30", seen.People[0].WorkEnd)
	assert.Equal(t, "[]", string(mustJSON(t, seen.People[1].Busy)), "an empty list")
	// The owner sees the title.
	code, raw = fb("alice", alice)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, string(raw), "Budget review")

	// Deny: another workspace's person, and a caller outside the workspace.
	code, _ = fb("alice", alice, gus)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = fb("gus", alice)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = fb("alice")
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	// Suggestions skip Alice's 09:00-10:00.
	sg := func(who, minutes string, users ...string) (int, []byte) {
		q := url.Values{"users": {strings.Join(users, ",")}, "durationMin": {minutes}, "day": {"2026-11-02"}}
		return f.do(http.MethodGet, f.base()+"/freebusy/suggest?"+q.Encode(), who, "")
	}
	code, raw = sg("alice", "60", alice, bob)
	require.Equal(t, http.StatusOK, code, string(raw))
	var slots struct {
		Slots []map[string]string `json:"slots"`
	}
	require.NoError(t, json.Unmarshal(raw, &slots))
	require.Len(t, slots.Slots, 3)
	assert.Equal(t, "2026-11-02T08:00:00+07:00", slots.Slots[0]["start"])
	assert.Equal(t, "2026-11-02T10:00:00+07:00", slots.Slots[1]["start"])
	code, _ = sg("alice", "60", alice, gus)
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = sg("alice", "x", alice)
	assert.Equal(t, http.StatusUnprocessableEntity, code)
	code, _ = sg("alice", "1", alice)
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	// Search: the title (any case), nothing for a stranger, 422 on an overlong term.
	search := func(who, q string) (int, []occJSON) {
		code, raw := f.do(http.MethodGet, f.base()+"/events/search?q="+url.QueryEscape(q), who, "")
		var out struct {
			Occurrences []occJSON `json:"occurrences"`
		}
		if code == http.StatusOK {
			require.NoError(t, json.Unmarshal(raw, &out))
		}
		return code, out.Occurrences
	}
	code, found := search("alice", "BUDGET")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, found, 1)
	assert.Equal(t, "Budget review", found[0].Title)
	code, found = search("alice", "100%_")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, found, "pattern characters are taken literally")
	code, found = search("carol", "budget")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, found)
	code, _ = search("gus", "budget")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = search("alice", strings.Repeat("x", 101))
	assert.Equal(t, http.StatusUnprocessableEntity, code)

	// Settings: defaults, then saved, then reflected in free/busy and in new events.
	code, raw = f.do(http.MethodGet, f.base()+"/settings", "alice", "")
	require.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, `{"tzid":"Asia/Ho_Chi_Minh","workStart":"08:00","workEnd":"17:30","defaultReminder":10}`, string(raw))
	code, raw = f.do(http.MethodPut, f.base()+"/settings", "alice", `{"tzid":"Asia/Bangkok","workStart":"09:30","workEnd":"18:00","defaultReminder":0}`)
	require.Equal(t, http.StatusOK, code, string(raw))
	code, raw = f.do(http.MethodGet, f.base()+"/settings", "alice", "")
	assert.JSONEq(t, `{"tzid":"Asia/Bangkok","workStart":"09:30","workEnd":"18:00","defaultReminder":0}`, string(raw))
	code, raw = fb("carol", alice)
	require.NoError(t, json.Unmarshal(raw, &seen))
	assert.Equal(t, "09:30", seen.People[0].WorkStart)
	for _, bad := range []string{
		`{"tzid":"Nowhere/Land","workStart":"09:00","workEnd":"18:00","defaultReminder":10}`,
		`{"tzid":"Asia/Bangkok","workStart":"9am","workEnd":"18:00","defaultReminder":10}`,
		`{"tzid":"Asia/Bangkok","workStart":"18:00","workEnd":"09:00","defaultReminder":10}`,
	} {
		code, _ = f.do(http.MethodPut, f.base()+"/settings", "alice", bad)
		assert.Equal(t, http.StatusUnprocessableEntity, code, bad)
	}
	code, _ = f.do(http.MethodPut, f.base()+"/settings", "alice", `x`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = f.do(http.MethodGet, f.base()+"/settings", "gus", "")
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = f.do(http.MethodGet, f.base()+"/settings", "", "")
	assert.Equal(t, http.StatusUnauthorized, code)
	code, _, raw = f.createEvent("alice", eventBody(cal, ""))
	require.Equal(t, http.StatusCreated, code, string(raw))
	assert.NotContains(t, string(raw), `"minutesBefore"`, "the saved default (none) applies")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
