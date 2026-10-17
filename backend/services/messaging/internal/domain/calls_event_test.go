package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// eventWorld is a calendar event with a video call, a guest who reads it, a second
// guest, and a person who has no grant on it.
type eventWorld struct {
	calOA    string
	f        *fixture
	media    *fakeMedia
	eventID  string
	oaID     string
	guest    string
	guestN   string
	guest2   string
	guest2N  string
	stranger string
	strangeN string
}

func newEventWorld(t *testing.T) *eventWorld {
	t.Helper()
	f := newFixture(t)
	ctx := context.Background()
	w := &eventWorld{f: f, media: newFakeMedia()}

	f.svc.SetCallMedia(w.media)

	n := time.Now().UnixNano()
	calOA, calID := fmt.Sprintf("test-cal-oa-%d", n), fmt.Sprintf("test-cal-%d", n)
	w.calOA = calOA
	w.oaID, w.eventID = fmt.Sprintf("test-ev-oa-%d", n), fmt.Sprintf("test-ev-%d", n)
	for id, name := range map[string]string{calOA: ngac.CalendarOAName(ngac.CalendarID(calID)), w.oaID: ngac.CalendarEventOAName(ngac.CalendarEventID(w.eventID))} {
		_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, 'OA')`, id, name)
		require.NoError(t, err)
	}
	_, err := f.pool.Exec(ctx,
		`INSERT INTO calendars (id, workspace_id, kind, owner_node_id, ngac_oa_id) VALUES ($1, $2, 'personal', $3, $4)`,
		calID, f.wsID, userNode, calOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO calendar_events (id, workspace_id, calendar_id, organizer_node_id, title, starts_at, ends_at, has_call, ngac_oa_id)
		 VALUES ($1, $2, $3, $4, 'Standup', now(), now() + interval '1 hour', TRUE, $5)`,
		w.eventID, f.wsID, calID, userNode, w.oaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM calls WHERE calendar_event_id = $1`, w.eventID)
		f.pool.Exec(ctx, `DELETE FROM calendar_events WHERE id = $1`, w.eventID)
		f.pool.Exec(ctx, `DELETE FROM calendars WHERE id = $1`, calID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, calOA, w.oaID)
	})

	w.guest, w.guestN = testutil.CreateUser(t, f.pool)
	w.guest2, w.guest2N = testutil.CreateUser(t, f.pool)
	w.stranger, w.strangeN = testutil.CreateUser(t, f.pool)
	w.readers(w.guestN, w.guest2N)
	// Everyone here belongs to the workspace, the stranger included: what keeps the
	// stranger out is the grant on the event, which these tests are about.
	f.read.addMember(f.wsID, w.guestN, w.guest2N, w.strangeN)
	return w
}

// tenantCtx is a request made in the event's workspace.
func (w *eventWorld) tenantCtx() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{TenantID: w.f.wsID})
}

// attend puts a person on the event's guest list.
func (w *eventWorld) attend(t *testing.T, node string) {
	t.Helper()
	_, err := w.f.pool.Exec(context.Background(),
		`INSERT INTO calendar_attendees (event_id, user_node_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, w.eventID, node)
	require.NoError(t, err)
}

// calendarReader gives a person read on the calendar's OA and, through the
// assignment beneath it, on the event's own: a view share, a group member.
func (w *eventWorld) calendarReader(node string) {
	w.f.read.mu.Lock()
	defer w.f.read.mu.Unlock()
	w.f.read.allow[grant{node, w.calOA, ngac.OpRead}] = true
	w.f.read.allow[grant{node, w.oaID, ngac.OpRead}] = true
}

func (w *eventWorld) readers(nodes ...string) {
	w.f.read.mu.Lock()
	defer w.f.read.mu.Unlock()
	for _, n := range nodes {
		w.f.read.allow[grant{n, w.oaID, ngac.OpRead}] = true
	}
}

// remove takes a guest's access to the event away, as removing them does.
func (w *eventWorld) remove(node string) {
	w.f.read.mu.Lock()
	defer w.f.read.mu.Unlock()
	delete(w.f.read.allow, grant{node, w.oaID, ngac.OpRead})
}

func (w *eventWorld) start(user, node string) (callID string, created bool, err error) {
	join, created, err := w.f.svc.StartEventCall(w.tenantCtx(), domain.StartEventCallInput{EventID: w.eventID, UserID: user, UserNodeID: node})
	if err != nil {
		return "", false, err
	}
	return join.Call.Id, created, nil
}

func TestEventCall_AGuestStartsItAndOthersJoinTheSameOne(t *testing.T) {
	w := newEventWorld(t)
	ctx := w.tenantCtx()
	join, created, err := w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{EventID: w.eventID, UserID: w.guest, UserNodeID: w.guestN})
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, "active", join.Call.Status)
	assert.Equal(t, "video", join.Call.Kind)
	assert.Empty(t, join.Call.ChannelId, "an event's call is in no conversation")
	assert.Equal(t, w.eventID, join.Call.CalendarEventId, "the screen names the call after its event")
	assert.Equal(t, "token-for-"+w.guest, join.Token)
	assert.Equal(t, "wss://calls.test/livekit", join.Url)
	assert.True(t, w.media.hasRoom(join.Call.Id), "the room is opened ahead of the first person")

	// One open call per event: the second guest is handed a ticket into it.
	again, created, err := w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{EventID: w.eventID, UserID: w.guest2, UserNodeID: w.guest2N})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, join.Call.Id, again.Call.Id)
	assert.Equal(t, "token-for-"+w.guest2, again.Token)
	var open int
	require.NoError(t, w.f.pool.QueryRow(ctx, `SELECT count(*) FROM calls WHERE calendar_event_id = $1`, w.eventID).Scan(&open))
	assert.Equal(t, 1, open)

	// A ticket into the call, and the call itself, to a guest who asks.
	tok, err := w.f.svc.CallToken(ctx, join.Call.Id, w.guest2, w.guest2N)
	require.NoError(t, err)
	assert.Equal(t, "token-for-"+w.guest2, tok.Token)
	got, err := w.f.svc.GetCall(ctx, join.Call.Id, w.guestN)
	require.NoError(t, err)
	assert.Equal(t, join.Call.Id, got.Id)
}

func TestEventCall_Deny(t *testing.T) {
	w := newEventWorld(t)
	ctx := w.tenantCtx()
	callID, _, err := w.start(w.guest, w.guestN)
	require.NoError(t, err)
	before := len(w.media.tokens)

	// A person who does not read the event neither starts, joins, gets a ticket nor sees the call.
	_, _, err = w.start(w.stranger, w.strangeN)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = w.f.svc.CallToken(ctx, callID, w.stranger, w.strangeN)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = w.f.svc.GetCall(ctx, callID, w.strangeN)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Len(t, w.media.tokens, before, "no ticket was signed")

	// A guest taken off the event loses all of it at once.
	w.remove(w.guest2N)
	_, _, err = w.start(w.guest2, w.guest2N)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = w.f.svc.CallToken(ctx, callID, w.guest2, w.guest2N)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = w.f.svc.GetCall(ctx, callID, w.guest2N)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)

	// An event call is not a conversation's: nobody is invited into it from a chat.
	assert.ErrorIs(t, w.f.svc.InviteToCall(ctx, callID, w.guest, w.guestN, []string{w.guest2}), domain.ErrInvalidInput)

	// Decline and cancel are for direct calls.
	_, err = w.f.svc.DeclineCall(ctx, callID, w.guest2, w.guest2N)
	assert.Error(t, err)

	// An event that has no call, or does not exist, gives none.
	_, err = w.f.pool.Exec(ctx, `UPDATE calendar_events SET has_call = FALSE WHERE id = $1`, w.eventID)
	require.NoError(t, err)
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventRoomFinished, Room: callID}))
	_, _, err = w.start(w.guest, w.guestN)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, _, err = w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{EventID: "no-such-event", UserID: w.guest, UserNodeID: w.guestN})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, _, err = w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{UserID: w.guest, UserNodeID: w.guestN})
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestEventCall_AnEventWithNoNodeOfItsOwnReadsItsCalendar(t *testing.T) {
	w := newEventWorld(t)
	ctx := w.tenantCtx()
	// The event keeps no OA (no guests): the owner of its calendar reads through the calendar's.
	var calOA string
	require.NoError(t, w.f.pool.QueryRow(ctx,
		`SELECT c.ngac_oa_id FROM calendar_events e JOIN calendars c ON c.id = e.calendar_id WHERE e.id = $1`, w.eventID).Scan(&calOA))
	_, err := w.f.pool.Exec(ctx, `UPDATE calendar_events SET ngac_oa_id = NULL WHERE id = $1`, w.eventID)
	require.NoError(t, err)
	w.f.read.mu.Lock()
	w.f.read.allow[grant{w.guestN, calOA, ngac.OpRead}] = true
	w.f.read.mu.Unlock()

	_, created, err := w.start(w.guest, w.guestN)
	require.NoError(t, err)
	assert.True(t, created)
	_, _, err = w.start(w.stranger, w.strangeN)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestEventCall_ASeriesCallBelongsToTheSeries(t *testing.T) {
	w := newEventWorld(t)
	ctx := w.tenantCtx()
	exception := w.eventID + "-x"
	_, err := w.f.pool.Exec(ctx,
		`INSERT INTO calendar_events (id, workspace_id, calendar_id, organizer_node_id, title, starts_at, ends_at, recurring_event_id, original_start)
		 SELECT $1, workspace_id, calendar_id, organizer_node_id, title, starts_at, ends_at, id, starts_at FROM calendar_events WHERE id = $2`,
		exception, w.eventID)
	require.NoError(t, err)
	t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM calendar_events WHERE id = $1`, exception) })

	first, _, err := w.start(w.guest, w.guestN)
	require.NoError(t, err)
	join, created, err := w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{EventID: exception, UserID: w.guest2, UserNodeID: w.guest2N})
	require.NoError(t, err)
	assert.False(t, created, "an occurrence's call is the series' call")
	assert.Equal(t, first, join.Call.Id)
}

func TestEventCall_APersonWhoLostTheEventIsPutOutOfTheRoom(t *testing.T) {
	w := newEventWorld(t)
	// Tickets are handed to people of the workspace, which is what the sweep finds them by.
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: w.guest, NGACNodeID: w.guestN, TenantID: w.f.wsID})
	join, _, err := w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{EventID: w.eventID, UserID: w.guest, UserNodeID: w.guestN})
	require.NoError(t, err)
	callID := join.Call.Id
	_, err = w.f.svc.CallToken(ctx, callID, w.guest2, w.guest2N)
	require.NoError(t, err)

	// Their ticket was signed before they were taken off the event.
	w.remove(w.guest2N)
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: callID, Identity: w.guest2, Participant: "sid-2"}))
	assert.Contains(t, w.media.removed, callID+"/"+w.guest2)
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: callID, Identity: w.guest, Participant: "sid-1"}))
	got, err := w.f.svc.GetCall(ctx, callID, w.guestN)
	require.NoError(t, err)
	require.Len(t, got.Participants, 1, "only the guest who still reads it is in the call")
	assert.Equal(t, w.guest, got.Participants[0].UserId)

	// The sweep puts out a guest who was already inside when the right was withdrawn.
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: callID, Identity: w.guest2, Participant: "sid-3"}))
	w.readers(w.guest2N)
	require.NoError(t, w.f.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: callID, Identity: w.guest2, Participant: "sid-4"}))
	w.remove(w.guest2N)
	require.NoError(t, w.f.svc.EvictLostCallParticipants(ctx, "", w.f.wsID))
	got, err = w.f.svc.GetCall(ctx, callID, w.guestN)
	require.NoError(t, err)
	for _, p := range got.Participants {
		assert.NotEqual(t, w.guest2, p.UserId)
	}
}

func TestEventCall_APrivateEventIsForItsAttendeesOnly(t *testing.T) {
	w := newEventWorld(t)
	ctx := w.tenantCtx()
	_, err := w.f.pool.Exec(ctx, `UPDATE calendar_events SET visibility = 'private' WHERE id = $1`, w.eventID)
	require.NoError(t, err)
	w.attend(t, w.guestN)
	// The event's OA sits under its calendar's, so a reader of the calendar reads
	// it too; guest2 holds read on the OA but is not on the guest list.
	w.calendarReader(w.strangeN)

	// Allow: a guest who is on the list starts the call.
	callID, created, err := w.start(w.guest, w.guestN)
	require.NoError(t, err)
	assert.True(t, created)
	before := len(w.media.tokens)

	// Deny: neither a reader of the calendar nor someone without a row starts,
	// joins, gets a ticket for or sees it.
	for _, p := range []struct{ user, node string }{{w.stranger, w.strangeN}, {w.guest2, w.guest2N}} {
		_, _, err = w.start(p.user, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, p.node)
		_, err = w.f.svc.CallToken(ctx, callID, p.user, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, p.node)
		_, err = w.f.svc.GetCall(ctx, callID, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, p.node)
	}
	assert.Len(t, w.media.tokens, before, "no ticket was signed")

	// An attendee row alone is not enough: the access must still be there.
	w.attend(t, w.guest2N)
	w.remove(w.guest2N)
	_, err = w.f.svc.CallToken(ctx, callID, w.guest2, w.guest2N)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)

	// Allow: the attendee gets a ticket; an event that is not private is open to
	// whoever reads it, a reader of the calendar included.
	_, err = w.f.svc.CallToken(ctx, callID, w.guest, w.guestN)
	require.NoError(t, err)
	_, err = w.f.pool.Exec(ctx, `UPDATE calendar_events SET visibility = 'default' WHERE id = $1`, w.eventID)
	require.NoError(t, err)
	_, err = w.f.svc.CallToken(ctx, callID, w.stranger, w.strangeN)
	require.NoError(t, err)
}

func TestEventCall_OnlyTheEventsOwnWorkspaceMayJoin(t *testing.T) {
	w := newEventWorld(t)
	callID, _, err := w.start(w.guest, w.guestN)
	require.NoError(t, err)
	before := len(w.media.tokens)

	elsewhere := grpcauth.WithCaller(context.Background(), grpcauth.Caller{TenantID: w.f.wsID + "-elsewhere"})
	nowhere := context.Background()
	for name, ctx := range map[string]context.Context{"another workspace": elsewhere, "no workspace": nowhere} {
		_, _, err = w.f.svc.StartEventCall(ctx, domain.StartEventCallInput{EventID: w.eventID, UserID: w.guest, UserNodeID: w.guestN})
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		_, err = w.f.svc.CallToken(ctx, callID, w.guest, w.guestN)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
		_, err = w.f.svc.GetCall(ctx, callID, w.guestN)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
	}
	assert.Len(t, w.media.tokens, before, "no ticket was signed")
}
