// Package domain — voice and video calls in a conversation.
//
// A call is not a graph node. Like a message it belongs to its channel, and
// every decision lands on the channel's content OA: starting a call or taking a
// ticket into its room needs write, seeing it needs read. The media runs in
// LiveKit (CallMedia); this file keeps the lifecycle, the card in the stream,
// and the rule that someone who loses the right to be in the conversation does
// not stay in its call.
package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

const (
	// MaxCallParticipants is how many people one call holds.
	MaxCallParticipants = 20
	// callRingFor is how long a direct call rings before it counts as missed.
	callRingFor = 45 * time.Second
	// callTicketFor is how long a ticket into a room stays valid. It only has to
	// last until the browser connects; a reconnect asks for a new one.
	callTicketFor = 10 * time.Minute
	// callRoomEmptyTimeout is how long LiveKit keeps a room nobody is in.
	callRoomEmptyTimeout = 30 * time.Second
	// callStaleAfter is how long an open call's room may stand empty (nobody
	// arriving or leaving) before the call is taken as ended even though
	// LiveKit's room_finished never arrived.
	callStaleAfter = callRoomEmptyTimeout + time.Minute
)

const linkedCall = "call"

// Call statuses.
const (
	callRinging   = "ringing"
	callActive    = "active"
	callEnded     = "ended"
	callMissed    = "missed"
	callDeclined  = "declined"
	callCancelled = "cancelled"
)

var openCallStatuses = []string{callRinging, callActive}

// Refusals particular to calls. They are conflicts: the request is well formed
// but the call is not in a state to take it.
var (
	ErrCallFull         = fmt.Errorf("%w: the call is full", httputil.ErrAlreadyExists)
	ErrCallEnded        = fmt.Errorf("%w: the call has ended", httputil.ErrAlreadyExists)
	ErrCallsUnavailable = errors.New("calls are not configured")
)

// CallMedia is the media server rooms live on. *livekit.Client implements it.
type CallMedia interface {
	PublicURL() string
	CreateRoom(ctx context.Context, name string, maxParticipants uint32, emptyTimeout time.Duration) error
	DeleteRoom(ctx context.Context, name string) error
	RemoveParticipant(ctx context.Context, room, identity string) error
	// Participants lists who is connected; a room that does not exist has nobody.
	Participants(ctx context.Context, room string) ([]string, error)
	JoinToken(room, identity, name string, ttl time.Duration) (string, error)
}

// SetCallMedia wires the media server. Without one, calls are unavailable.
func (s *Service) SetCallMedia(m CallMedia) { s.media = m }

// The kinds of call change announced to live sessions.
const (
	CallChangeRing     = "ring"     // a direct call starts ringing
	CallChangeCancel   = "cancel"   // the caller hung up before an answer
	CallChangeAnswered = "answered" // the peer joined: other devices stop ringing
	CallChangeDeclined = "declined"
	CallChangeMissed   = "missed"
	CallChangeState    = "state" // who is in it, or it ended
	CallChangeInvite   = "invited" // someone asked the people named to join
)

// CallChange is a committed change of a call. Users, when set, are the people
// every one of whose devices must hear it (ringing starts or stops); everyone
// following the conversation hears every change. Card is the call's card as it
// now reads, when its text changed.
type CallChange struct {
	Type  string
	Call  *pb.Call
	Card  *pb.Message
	Users []string
}

// CallObserver is told of every committed call change. The hub implements it.
type CallObserver interface {
	CallChanged(ctx context.Context, c CallChange)
}

// SetCallObserver wires the announcer of call changes. A nil observer is valid.
func (s *Service) SetCallObserver(o CallObserver) { s.callObserver = o }

func (s *Service) announceCall(ctx context.Context, c CallChange) {
	if s.callObserver != nil {
		s.callObserver.CallChanged(ctx, c)
	}
	s.pushCallChange(ctx, c)
}

// CallCardText is what a call's card says, which the conversation previews and
// search read: the kind of call, and once it is over how it ended or how long
// it lasted.
func CallCardText(kind, status string, startedAt, endedAt *time.Time) string {
	icon, name := "📞", "Cuộc gọi thoại"
	if kind == "video" {
		icon, name = "📹", "Cuộc gọi video"
	}
	switch status {
	case callMissed:
		return icon + " Cuộc gọi nhỡ"
	case callDeclined:
		return icon + " Cuộc gọi bị từ chối"
	case callCancelled:
		return icon + " Cuộc gọi đã huỷ"
	case callEnded:
		if startedAt != nil && endedAt != nil {
			return icon + " " + name + " · " + callLength(endedAt.Sub(*startedAt))
		}
	}
	return icon + " " + name
}

// callLength reads a duration the way the card shows it.
func callLength(d time.Duration) string {
	minutes := int(d / time.Minute)
	switch {
	case minutes < 1:
		return "dưới 1 phút"
	case minutes < 60:
		return fmt.Sprintf("%d phút", minutes)
	case minutes%60 == 0:
		return fmt.Sprintf("%d giờ", minutes/60)
	}
	return fmt.Sprintf("%d giờ %d phút", minutes/60, minutes%60)
}

// StartCallInput is a request to start a call, or join the one already open.
type StartCallInput struct {
	ChannelID  string
	UserID     string
	UserNodeID string
	Kind       string // audio | video
}

// StartCall starts a call in a conversation, with write on it. A direct message
// rings the other person for 45 seconds; a group call is open at once. When the
// conversation already has a call ringing or running, the caller is handed a
// ticket into that one and no card is posted. The card (when one was posted) is
// returned for the live stream.
func (s *Service) StartCall(ctx context.Context, in StartCallInput) (*pb.CallJoin, *pb.Message, error) {
	if s.media == nil {
		return nil, nil, ErrCallsUnavailable
	}
	ch, err := s.authorizeChannel(ctx, in.ChannelID, in.UserNodeID, ngac.OpWrite)
	if err != nil {
		return nil, nil, err
	}
	if in.Kind != "audio" && in.Kind != "video" {
		return nil, nil, fmt.Errorf("%w: a call is audio or video", ErrInvalidInput)
	}
	if open, err := s.liveOpenCall(ctx, ch.ID); err != nil || open != nil {
		if err != nil {
			return nil, nil, err
		}
		join, err := s.ticket(ctx, open, in.UserID, in.UserNodeID)
		return join, nil, err
	}

	now := time.Now()
	call := &store.Call{
		ID: uuid.New().String(), ChannelID: ch.ID, Kind: in.Kind, Status: callActive,
		StartedBy: in.UserID, CreatedAt: now, StartedAt: &now,
	}
	var ring []string
	if ch.ChannelType == "dm" {
		if ring, err = s.store.DMOthers(ctx, ch.ID, in.UserID); err != nil {
			return nil, nil, err
		}
		expires := now.Add(callRingFor)
		call.Status, call.StartedAt, call.RingExpiresAt = callRinging, nil, &expires
	}
	call.MessageID = uuid.New().String()
	msg := &store.Message{
		ID: call.MessageID, ChannelID: ch.ID, SenderID: in.UserID,
		Content:       CallCardText(call.Kind, call.Status, nil, nil),
		ContentFormat: "plain", MessageType: "system",
		LinkedEntityType: linkedCall, LinkedEntityID: call.ID, CreatedAt: now,
	}

	// The room is opened first: a call row whose room cannot be opened would be
	// a card nobody can join.
	if err := s.media.CreateRoom(ctx, call.ID, MaxCallParticipants, callRoomEmptyTimeout); err != nil {
		return nil, nil, err
	}
	if err := s.store.InsertCallWithMessage(ctx, msg, call); err != nil {
		s.closeRoom(call.ID)
		if !errors.Is(err, store.ErrCallOpen) {
			return nil, nil, err
		}
		// Someone else started one in the same instant: join theirs.
		open, err := s.store.OpenCall(ctx, ch.ID)
		if err != nil || open == nil {
			return nil, nil, fmt.Errorf("start call: %w", err)
		}
		join, err := s.ticket(ctx, open, in.UserID, in.UserNodeID)
		return join, nil, err
	}

	join, err := s.ticket(ctx, call, in.UserID, in.UserNodeID)
	if err != nil {
		return nil, nil, err
	}
	card := s.announcementProto(ctx, msg)
	change := CallChange{Type: CallChangeState, Call: join.Call}
	if call.Status == callRinging {
		change.Type, change.Users = CallChangeRing, ring
	}
	s.announceCall(ctx, change)
	return join, card, nil
}

// CallToken hands a ticket into a call's room to someone with write on its
// conversation, checked again now: a ticket is never granted on the strength
// of an earlier check. A call that is over, or full, gives none.
func (s *Service) CallToken(ctx context.Context, callID, userID, userNodeID string) (*pb.CallJoin, error) {
	if s.media == nil {
		return nil, ErrCallsUnavailable
	}
	call, err := s.authorizeCall(ctx, callID, userNodeID, ngac.OpWrite)
	if err != nil {
		return nil, err
	}
	if slices.Contains(openCallStatuses, call.Status) {
		if call, err = s.reconcile(ctx, call); err != nil {
			return nil, err
		}
	}
	if !slices.Contains(openCallStatuses, call.Status) {
		return nil, ErrCallEnded
	}
	return s.ticket(ctx, call, userID, userNodeID)
}

// GetCall returns a call as a reader of its conversation sees it: its state
// and who is in it now.
func (s *Service) GetCall(ctx context.Context, callID, userNodeID string) (*pb.Call, error) {
	call, err := s.authorizeCall(ctx, callID, userNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	return s.callProto(ctx, call)
}

// OpenCall returns the conversation's ringing or running call as a reader of it
// sees it, or nil when there is none.
func (s *Service) OpenCall(ctx context.Context, channelID, userNodeID string) (*pb.Call, error) {
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	call, err := s.store.OpenCall(ctx, channelID)
	if err != nil || call == nil {
		return nil, err
	}
	return s.callProto(ctx, call)
}

// DeclineCall is the other person turning down a ringing direct call.
func (s *Service) DeclineCall(ctx context.Context, callID, userID, userNodeID string) (*pb.Call, error) {
	call, err := s.authorizeCall(ctx, callID, userNodeID, ngac.OpWrite)
	if err != nil {
		return nil, err
	}
	if call.StartedBy == userID {
		return nil, fmt.Errorf("%w: the caller cancels, not declines", ErrAccessDenied)
	}
	// The person rung is the direct message's other member, nobody else.
	rung, err := s.store.DMOthers(ctx, call.ChannelID, call.StartedBy)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(rung, userID) {
		return nil, fmt.Errorf("%w: only the person called declines", ErrAccessDenied)
	}
	return s.endRinging(ctx, call, callDeclined, CallChangeDeclined)
}

// CancelCall is the caller hanging up a direct call before it was answered.
func (s *Service) CancelCall(ctx context.Context, callID, userID, userNodeID string) (*pb.Call, error) {
	call, err := s.authorizeCall(ctx, callID, userNodeID, ngac.OpWrite)
	if err != nil {
		return nil, err
	}
	if call.StartedBy != userID {
		return nil, fmt.Errorf("%w: only the caller cancels a call", ErrAccessDenied)
	}
	return s.endRinging(ctx, call, callCancelled, CallChangeCancel)
}

// endRinging closes a call that is still ringing, its room with it, and tells
// both people's devices.
func (s *Service) endRinging(ctx context.Context, call *store.Call, status, change string) (*pb.Call, error) {
	moved, err := s.store.TransitionCall(ctx, call.ID, []string{callRinging}, status)
	if err != nil {
		return nil, err
	}
	if moved == nil {
		return nil, ErrCallEnded
	}
	s.closeRoom(moved.ID)
	out, err := s.callChanged(ctx, moved, change, true)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ExpireRingingCalls marks direct calls that rang unanswered past their time as
// missed, closes their rooms, and returns their ids. Safe to run from every
// instance at once: each call is expired by exactly one.
func (s *Service) ExpireRingingCalls(ctx context.Context, now time.Time) ([]string, error) {
	calls, err := s.store.ExpireRinging(ctx, now)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(calls))
	for i := range calls {
		c := &calls[i]
		ids = append(ids, c.ID)
		s.closeRoom(c.ID)
		if _, err := s.callChanged(ctx, c, CallChangeMissed, true); err != nil {
			slog.Warn("missed call not announced", "call", c.ID, "error", err)
		}
	}
	return ids, nil
}

// The LiveKit webhook events the lifecycle reads.
const (
	CallEventJoined       = "participant_joined"
	CallEventLeft         = "participant_left"
	CallEventRoomFinished = "room_finished"
)

// CallEvent is a verified webhook from the media server.
type CallEvent struct {
	Kind     string
	Room     string // the call id
	Identity string // the user id, for participant events
	// Participant is the connection (LiveKit participant sid), for participant
	// events: one person may come through more than one in turn.
	Participant string
}

// HandleCallEvent applies a webhook. Someone who enters a room without write
// on its conversation (a ticket signed before they were removed) is put out
// and not recorded. Events for an unknown call are ignored.
func (s *Service) HandleCallEvent(ctx context.Context, ev CallEvent) error {
	call, err := s.store.GetCall(ctx, ev.Room)
	if err != nil {
		return err
	}
	if call == nil {
		return nil
	}
	switch ev.Kind {
	case CallEventJoined:
		return s.participantJoined(ctx, call, ev.Identity, ev.Participant)
	case CallEventLeft:
		if err := s.store.LeaveCall(ctx, call.ID, ev.Identity, ev.Participant); err != nil {
			return err
		}
		if err := s.endDirectCallLeftAlone(ctx, call); err != nil {
			return err
		}
		_, err := s.callChanged(ctx, call, CallChangeState, false)
		return err
	case CallEventRoomFinished:
		return s.roomFinished(ctx, call)
	}
	return nil
}

func (s *Service) participantJoined(ctx context.Context, call *store.Call, userID, sid string) error {
	if !slices.Contains(openCallStatuses, call.Status) || !s.mayStayInCall(ctx, call, userID) {
		if s.media != nil {
			if err := s.media.RemoveParticipant(ctx, call.ID, userID); err != nil {
				slog.Warn("could not remove a participant without the right to be in the call",
					"call", call.ID, "error", err)
			}
		}
		return nil
	}
	if err := s.store.JoinCall(ctx, call.ID, userID, sid); err != nil {
		return err
	}
	if call.Status == callRinging && userID != call.StartedBy {
		moved, err := s.store.TransitionCall(ctx, call.ID, []string{callRinging}, callActive)
		if err != nil {
			return err
		}
		if moved != nil {
			_, err = s.callChanged(ctx, moved, CallChangeAnswered, true)
			return err
		}
	}
	_, err := s.callChanged(ctx, call, CallChangeState, false)
	return err
}

// endDirectCallLeftAlone closes the room of an answered direct call once fewer
// than two people are in it for longer than a short grace: hanging up a call
// between two people ends it for both. LiveKit's room_finished then marks it ended. A ringing call is left to
// ring out or be cancelled; a group call goes on while anyone is in it.
func (s *Service) endDirectCallLeftAlone(ctx context.Context, call *store.Call) error {
	if call.Status != callActive {
		return nil
	}
	ch, err := s.loadChannel(ctx, call.ChannelID)
	if err != nil || ch.ChannelType != "dm" {
		return err
	}
	present, err := s.store.CallPresent(ctx, call.ID)
	if err != nil || len(present) >= 2 {
		return err
	}
	// A reload or a reconnect also leaves before it joins again: decide once
	// the grace has passed, on the call as it is then.
	time.AfterFunc(s.hangUpGrace(), func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		now, err := s.store.GetCall(ctx, call.ID)
		if err != nil || now == nil || now.Status != callActive {
			return
		}
		if present, err := s.store.CallPresent(ctx, call.ID); err == nil && len(present) < 2 {
			s.closeRoom(call.ID)
		}
	})
	return nil
}

// callHangUpGrace is how long a direct call waits, after one of its two people
// left, for them to come back before it ends for both.
const callHangUpGrace = 15 * time.Second

func (s *Service) hangUpGrace() time.Duration {
	if s.callHangUpGrace > 0 {
		return s.callHangUpGrace
	}
	return callHangUpGrace
}

// SetCallHangUpGraceForTest shortens the wait before a direct call left with
// one person ends.
func SetCallHangUpGraceForTest(s *Service, d time.Duration) { s.callHangUpGrace = d }

// roomFinished closes a call whose room LiveKit closed: a call that was
// running ended; one still ringing was not answered.
func (s *Service) roomFinished(ctx context.Context, call *store.Call) error {
	if err := s.store.CloseCallStays(ctx, call.ID); err != nil {
		return err
	}
	status, change, ring := callEnded, CallChangeState, false
	if call.Status == callRinging {
		status, change, ring = callMissed, CallChangeMissed, true
	}
	moved, err := s.store.TransitionCall(ctx, call.ID, openCallStatuses, status)
	if err != nil || moved == nil {
		return err
	}
	_, err = s.callChanged(ctx, moved, change, ring)
	return err
}

// EvictLostCallParticipants puts out of the open calls of a conversation, of a
// workspace, or — naming neither — of every conversation, each person in them
// who no longer holds write on the conversation. It runs when a membership is
// withdrawn, after a permission change, and periodically as a safety net.
func (s *Service) EvictLostCallParticipants(ctx context.Context, channelID, workspaceID string) error {
	if s.media == nil {
		return nil
	}
	calls, err := s.store.OpenCallsIn(ctx, channelID, workspaceID)
	if err != nil {
		return err
	}
	for i := range calls {
		call := &calls[i]
		present, err := s.store.CallPresent(ctx, call.ID)
		if err != nil {
			return err
		}
		for _, p := range present {
			if s.mayStayInCall(ctx, call, p.UserID) {
				continue
			}
			if err := s.media.RemoveParticipant(ctx, call.ID, p.UserID); err != nil {
				slog.Warn("could not remove a participant who lost access", "call", call.ID, "error", err)
				continue
			}
			if err := s.store.LeaveCall(ctx, call.ID, p.UserID, ""); err != nil {
				return err
			}
		}
	}
	return nil
}

// mayStayInCall reports whether a user still holds write on the call's
// conversation, as the node they were handed their ticket as (a person has one
// node per workspace). It fails closed: someone never handed a ticket, or a
// policy error, is a no.
func (s *Service) mayStayInCall(ctx context.Context, call *store.Call, userID string) bool {
	node, tenantID, err := s.store.CallAccessNode(ctx, call.ID, userID)
	if err != nil || node == "" {
		return false
	}
	// The question is the participant's own standing, so it is asked as them:
	// the policy service answers only on behalf of a person.
	ctx = grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: userID, NGACNodeID: node, TenantID: tenantID})
	_, err = s.authorizeChannel(ctx, call.ChannelID, node, ngac.OpWrite)
	return err == nil
}

// authorizeCall loads a call and checks op on its conversation's OA.
func (s *Service) authorizeCall(ctx context.Context, callID, userNodeID, op string) (*store.Call, error) {
	if callID == "" {
		return nil, ErrInvalidInput
	}
	call, err := s.store.GetCall(ctx, callID)
	if err != nil {
		return nil, err
	}
	if call == nil {
		return nil, fmt.Errorf("%w: call not found", ErrNotFound)
	}
	if _, err := s.authorizeChannel(ctx, call.ChannelID, userNodeID, op); err != nil {
		return nil, err
	}
	return call, nil
}

// liveOpenCall returns the conversation's open call, unless its room has been
// empty long enough that the call is over and LiveKit's word of it was lost.
func (s *Service) liveOpenCall(ctx context.Context, channelID string) (*store.Call, error) {
	open, err := s.store.OpenCall(ctx, channelID)
	if err != nil || open == nil {
		return nil, err
	}
	open, err = s.reconcile(ctx, open)
	if err != nil || !slices.Contains(openCallStatuses, open.Status) {
		return nil, err
	}
	return open, nil
}

// reconcile ends a running call whose room has stood empty — nobody arriving
// or leaving — past the time LiveKit would have closed it, and that LiveKit
// confirms is empty: the room_finished webhook never came.
func (s *Service) reconcile(ctx context.Context, call *store.Call) (*store.Call, error) {
	if call.Status != callActive {
		return call, nil
	}
	present, err := s.store.CallPresent(ctx, call.ID)
	if err != nil || len(present) > 0 {
		return call, err
	}
	lastSeen, err := s.store.CallLastSeen(ctx, call.ID)
	if err != nil || time.Since(lastSeen) < callStaleAfter {
		return call, err
	}
	inRoom, err := s.media.Participants(ctx, call.ID)
	if err != nil || len(inRoom) > 0 {
		return call, nil // unknown or occupied: leave it as it is
	}
	if err := s.roomFinished(ctx, call); err != nil {
		return nil, err
	}
	fresh, err := s.store.GetCall(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// ticket signs a ticket into an open call for a user who may join it. A full
// call has no room for someone not already in it.
func (s *Service) ticket(ctx context.Context, call *store.Call, userID, userNodeID string) (*pb.CallJoin, error) {
	present, err := s.store.CallPresent(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	inside := slices.ContainsFunc(present, func(p store.CallPerson) bool { return p.UserID == userID })
	if !inside && len(present) >= MaxCallParticipants {
		return nil, ErrCallFull
	}
	name, err := s.store.DisplayName(ctx, userID)
	if err != nil {
		return nil, err
	}
	// The node just checked is the one the room checks again later.
	if err := s.store.GrantCallAccess(ctx, call.ID, userID, userNodeID, grpcauth.CallerFrom(ctx).TenantID); err != nil {
		return nil, err
	}
	token, err := s.media.JoinToken(call.ID, userID, name, callTicketFor)
	if err != nil {
		return nil, err
	}
	out, err := s.callProtoWith(ctx, call, present)
	if err != nil {
		return nil, err
	}
	return &pb.CallJoin{Call: out, Token: token, Url: s.media.PublicURL()}, nil
}

// callChanged rewrites the card when the call's state shows on it, and
// announces the change. With ring, both people of a direct call hear it on
// every device.
func (s *Service) callChanged(ctx context.Context, call *store.Call, kind string, ring bool) (*pb.Call, error) {
	out, err := s.callProto(ctx, call)
	if err != nil {
		return nil, err
	}
	change := CallChange{Type: kind, Call: out}
	if !slices.Contains(openCallStatuses, call.Status) && call.MessageID != "" {
		change.Card = s.rewriteCard(ctx, call)
	}
	if ring {
		others, err := s.store.DMOthers(ctx, call.ChannelID, call.StartedBy)
		if err != nil {
			return nil, err
		}
		change.Users = append(others, call.StartedBy)
	}
	s.announceCall(ctx, change)
	return out, nil
}

// rewriteCard sets the card's text to how the call ended and returns the card
// as it now reads; nil when it could not be rewritten (the call still is).
func (s *Service) rewriteCard(ctx context.Context, call *store.Call) *pb.Message {
	text := CallCardText(call.Kind, call.Status, call.StartedAt, call.EndedAt)
	if err := s.store.SetCardText(ctx, call.MessageID, text); err != nil {
		slog.Warn("call card not updated", "call", call.ID, "error", err)
		return nil
	}
	s.bumpChannelID(ctx, call.ChannelID)
	msgs, err := s.store.GetThread(ctx, call.MessageID)
	if err != nil || len(msgs) == 0 {
		return nil
	}
	if err := s.EnrichMessagesWithMetadata(ctx, msgs[:1], call.ChannelID); err != nil {
		return nil
	}
	return messageToProto(msgs[0])
}

// closeRoom closes a room, after the request that asked for it is gone.
func (s *Service) closeRoom(room string) {
	if s.media == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.media.DeleteRoom(ctx, room); err != nil {
		slog.Warn("call room not closed", "call", room, "error", err)
	}
}

func (s *Service) callProto(ctx context.Context, call *store.Call) (*pb.Call, error) {
	present, err := s.store.CallPresent(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	return s.callProtoWith(ctx, call, present)
}

func (s *Service) callProtoWith(ctx context.Context, call *store.Call, present []store.CallPerson) (*pb.Call, error) {
	takers, err := s.store.CallTakers(ctx, call.ID)
	if err != nil {
		return nil, err
	}
	starter, err := s.store.CallPeople(ctx, []string{call.StartedBy})
	if err != nil {
		return nil, err
	}
	out := &pb.Call{
		Id: call.ID, ChannelId: call.ChannelID, Kind: call.Kind, Status: call.Status,
		StartedBy: call.StartedBy, MessageId: call.MessageID,
		CreatedAt:     timestamppb.New(call.CreatedAt),
		StartedAt:     optionalTimestamp(call.StartedAt),
		EndedAt:       optionalTimestamp(call.EndedAt),
		RingExpiresAt: optionalTimestamp(call.RingExpiresAt),
		TakerCount:    takers,
		Participants:  make([]*pb.CallPerson, 0, len(present)),
	}
	if len(starter) > 0 {
		out.StartedByName, out.StartedByAvatarUrl = starter[0].DisplayName, starter[0].AvatarURL
	}
	for _, p := range present {
		out.Participants = append(out.Participants, &pb.CallPerson{
			UserId: p.UserID, DisplayName: p.DisplayName, AvatarUrl: p.AvatarURL,
			JoinedAt: timestamppb.New(p.JoinedAt),
		})
	}
	return out, nil
}

func optionalTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
