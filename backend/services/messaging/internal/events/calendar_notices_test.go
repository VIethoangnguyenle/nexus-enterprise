package events

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/messaging/internal/domain"
)

// fakeGate answers who may read what, names people, and records the pushes.
type fakeGate struct {
	reads  map[string]bool // "node|oa"
	names  map[string]string
	pushed []domain.CalendarPush
	pushTo []string
}

func (g *fakeGate) CanRead(_ context.Context, node, oa string) bool     { return g.reads[node+"|"+oa] }
func (g *fakeGate) UserDisplayName(_ context.Context, id string) string { return g.names[id] }
func (g *fakeGate) PushCalendar(_ context.Context, user string, p domain.CalendarPush) {
	g.pushTo = append(g.pushTo, user)
	g.pushed = append(g.pushed, p)
}

func calendarConsumer() (*Consumer, *recordingNotifier, *fakeGate) {
	n := &recordingNotifier{}
	g := &fakeGate{
		reads: map[string]bool{"n-bob|oa1": true, "n-carol|oa1": true, "n-alice|oa1": true},
		names: map[string]string{"u-alice": "Alice Nguyen"},
	}
	return &Consumer{notifSv: n, calendar: g}, n, g
}

func calNotice(typ string, extra string) []byte {
	return []byte(fmt.Sprintf(`{"record":"notice","type":%q,"workspace_id":"w1","event_id":"e1","calendar_id":"c1","oa_id":"oa1",
		"actor_user_id":"u-alice","title":"Standup","start":"2026-11-02T02:00:00Z","end":"2026-11-02T03:00:00Z",
		"recipients":[{"node_id":"n-bob","user_id":"u-bob"},{"node_id":"n-carol","user_id":"u-carol"}]%s}`, typ, extra))
}

func bodyOf(t *testing.T, raw string) CalendarBody {
	t.Helper()
	var b CalendarBody
	require.NoError(t, json.Unmarshal([]byte(raw), &b))
	return b
}

func TestInvitedNotifiesEachGuestAndPushes(t *testing.T) {
	c, n, g := calendarConsumer()
	c.handleCalendarEvent(context.Background(), calNotice("event.invited", ""))

	require.Len(t, n.got, 2, "one notification for each guest")
	for i, user := range []string{"u-bob", "u-carol"} {
		got := n.got[i]
		assert.Equal(t, user, got.user)
		assert.Equal(t, "calendar_invitation", got.kind)
		assert.Equal(t, "Standup", got.title)
		assert.Equal(t, "calendar_event", got.entityType)
		assert.Equal(t, "e1", got.entityID)
		b := bodyOf(t, got.body)
		assert.Equal(t, "Alice Nguyen", b.Actor, "the host's name, never their id")
		assert.Equal(t, "2026-11-02T02:00:00Z", b.Start)
		assert.Equal(t, "w1", b.Workspace, "the link opens the event in its workspace")
		assert.NotContains(t, got.body, "u-alice")
		assert.NotContains(t, got.body, "e1")
	}
	assert.Equal(t, []string{"u-bob", "u-carol"}, g.pushTo, "an invitation also goes to the devices")
	assert.Equal(t, domain.CalendarPushInvitation, g.pushed[0].Kind)
	assert.Equal(t, "Alice Nguyen", g.pushed[0].ActorName)
}

func TestEachKindMakesItsOwnNotification(t *testing.T) {
	for typ, want := range map[string]string{
		"event.updated": "calendar_updated", "event.cancelled": "calendar_cancelled",
		"event.rsvp": "calendar_rsvp", "event.reminder": "calendar_reminder",
	} {
		c, n, g := calendarConsumer()
		extra := ""
		if typ == "event.rsvp" {
			extra = `,"response":"declined"`
		}
		c.handleCalendarEvent(context.Background(), calNotice(typ, extra))
		require.Len(t, n.got, 2, typ)
		assert.Equal(t, want, n.got[0].kind, typ)
		assert.Equal(t, "calendar_event", n.got[0].entityType, typ)
		pushes := 0
		if typ == "event.reminder" {
			pushes = 2
		}
		assert.Len(t, g.pushed, pushes, "%s: only an invitation and a reminder push", typ)
	}

	c, n, _ := calendarConsumer()
	c.handleCalendarEvent(context.Background(), calNotice("event.rsvp", `,"response":"declined"`))
	assert.Equal(t, "declined", bodyOf(t, n.got[0].body).Response)
}

func TestSharedLinksToTheCalendar(t *testing.T) {
	c, n, g := calendarConsumer()
	c.handleCalendarEvent(context.Background(), []byte(`{"record":"notice","type":"calendar.shared","calendar_id":"c1","oa_id":"oa1",
		"actor_user_id":"u-alice","recipients":[{"node_id":"n-bob","user_id":"u-bob"}]}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "calendar_shared", n.got[0].kind)
	assert.Equal(t, "calendar", n.got[0].entityType)
	assert.Equal(t, "c1", n.got[0].entityID)
	assert.Empty(t, g.pushed)
}

func TestNobodyWhoCannotReadTheEventIsTold(t *testing.T) {
	c, n, g := calendarConsumer()
	delete(g.reads, "n-carol|oa1") // removed from the event since the notice was made
	c.handleCalendarEvent(context.Background(), calNotice("event.reminder", ""))
	require.Len(t, n.got, 1)
	assert.Equal(t, "u-bob", n.got[0].user)
	assert.Equal(t, []string{"u-bob"}, g.pushTo, "nor is their phone pushed")

	// A cancellation is for people who can no longer read the event: that is why they hear of it.
	c, n, g = calendarConsumer()
	g.reads = map[string]bool{}
	c.handleCalendarEvent(context.Background(), calNotice("event.cancelled", ""))
	assert.Len(t, n.got, 2)

	// A notice that names no node to check denies.
	c, n, _ = calendarConsumer()
	c.handleCalendarEvent(context.Background(), []byte(`{"record":"notice","type":"event.invited","event_id":"e1","recipients":[{"node_id":"n-bob","user_id":"u-bob"}]}`))
	assert.Empty(t, n.got)
}

func TestARecordThatNamesNoOneOrNothingCreatesNothing(t *testing.T) {
	for name, body := range map[string]string{
		"no recipients":    `{"record":"notice","type":"event.invited","event_id":"e1","oa_id":"oa1"}`,
		"no event":         `{"record":"notice","type":"event.invited","oa_id":"oa1","recipients":[{"node_id":"n-bob","user_id":"u-bob"}]}`,
		"no account":       `{"record":"notice","type":"event.invited","event_id":"e1","oa_id":"oa1","recipients":[{"node_id":"n-bob","user_id":""}]}`,
		"unknown type":     `{"record":"notice","type":"event.created","event_id":"e1","oa_id":"oa1","recipients":[{"node_id":"n-bob","user_id":"u-bob"}]}`,
		"not json":         `{`,
		"a realtime frame": `{"domain":"calendar","kind":"updated","tenant_id":"t","workspace_id":"w1","ids":["e1"],"user_node_ids":["n-bob"]}`,
	} {
		c, n, g := calendarConsumer()
		c.handleCalendarEvent(context.Background(), []byte(body))
		assert.Empty(t, n.got, name)
		assert.Empty(t, g.pushed, name)
	}

	// Without a gate nothing is unchecked.
	c, n, _ := calendarConsumer()
	c.calendar = nil
	c.handleCalendarEvent(context.Background(), calNotice("event.invited", ""))
	assert.Empty(t, n.got)
}

func TestTheActorIsNotToldOfTheirOwnDoingButIsReminded(t *testing.T) {
	c, n, _ := calendarConsumer()
	c.handleCalendarEvent(context.Background(), []byte(`{"record":"notice","type":"event.updated","event_id":"e1","oa_id":"oa1","actor_user_id":"u-bob",
		"recipients":[{"node_id":"n-bob","user_id":"u-bob"},{"node_id":"n-carol","user_id":"u-carol"}]}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "u-carol", n.got[0].user)

	c, n, _ = calendarConsumer()
	c.handleCalendarEvent(context.Background(), []byte(`{"record":"notice","type":"event.reminder","event_id":"e1","oa_id":"oa1",
		"recipients":[{"node_id":"n-bob","user_id":"u-bob"}]}`))
	assert.Len(t, n.got, 1)
}

func TestAStaleReminderIsNotShown(t *testing.T) {
	c, n, _ := calendarConsumer()
	old := time.Now().Add(-time.Hour).UnixMilli()
	c.handleCalendarEvent(context.Background(), calNotice("event.reminder", fmt.Sprintf(`,"at":%d`, old)))
	assert.Empty(t, n.got)
	c.handleCalendarEvent(context.Background(), calNotice("event.reminder", fmt.Sprintf(`,"at":%d`, time.Now().UnixMilli())))
	assert.Len(t, n.got, 2)
}

// The realtime consumer must not turn a notice into a frame, nor lose a frame.
func TestTheRealtimeConsumerSkipsNoticesAndKeepsFrames(t *testing.T) {
	var pub capturePublisher
	rc := &RealtimeConsumer{pub: &pub, now: time.Now}
	rc.handle(CalendarTopic, calNotice("event.invited", ""))
	assert.Empty(t, pub.got, "a notice is not a frame")

	frame, err := json.Marshal(realtime.Event{Domain: realtime.DomainCalendar, Kind: realtime.KindUpdated, TenantID: "t", WorkspaceID: "w1", IDs: []string{"e1"}, ActorUserID: "u-alice"})
	require.NoError(t, err)
	assert.False(t, IsCalendarNotice(frame))
	rc.handle(CalendarTopic, frame)
	require.Len(t, pub.got, 1)
	assert.Equal(t, "u-alice", pub.got[0].ActorUserID)
}

type capturePublisher struct{ got []realtime.Event }

func (p *capturePublisher) PublishDomain(e realtime.Event) { p.got = append(p.got, e) }
