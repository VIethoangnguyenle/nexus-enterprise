// Package domain — the video call of a calendar event.
//
// An event's call belongs to the event, not to a conversation. Like everything of
// an event it is authorized on the event's OA (its calendar's, for an event that
// has no OA of its own): a guest, the owner of the calendar and anyone it is shared
// with hold read there, and read is the right to be in the call — except for a
// private event, whose call is for its guest list alone. Nobody else is handed a
// ticket, and nobody of another workspace. There is one open call per event; asking again joins it.
package domain

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// StartEventCallInput is a request to start the call of an event, or to join the
// one already open.
type StartEventCallInput struct {
	EventID    string
	UserID     string
	UserNodeID string
}

// authorizeEvent loads the event a call belongs to and verifies the user may be
// in its call, now. The event must be of the caller's own workspace. The user
// holds read on the event's OA (its calendar's, for an event with none of its
// own) and, when the event is private, is also on its guest list: the OA sits
// under the calendar's, so read there is held by everyone who reads the calendar,
// which a private event does not open to. It is fail-closed: an event with no
// node to check, no workspace on the request, or a policy error is a refusal.
func (s *Service) authorizeEvent(ctx context.Context, eventID, userNodeID string) (*store.EventCallRef, error) {
	if eventID == "" {
		return nil, ErrInvalidInput
	}
	ref, err := s.store.EventCallRef(ctx, eventID, userNodeID)
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, fmt.Errorf("%w: event not found", ErrNotFound)
	}
	if tenant := grpcauth.CallerFrom(ctx).TenantID; tenant == "" || tenant != ref.WorkspaceID {
		// Another workspace's event is not there for this one.
		return nil, fmt.Errorf("%w: event not found", ErrNotFound)
	}
	if ref.OAID == "" || (ref.Private && !ref.Attending) {
		return nil, fmt.Errorf("%w: %s on event", ErrAccessDenied, ngac.OpRead)
	}
	if err := s.checkAccess(ctx, userNodeID, ref.OAID, ngac.OpRead); err != nil {
		return nil, err
	}
	return ref, nil
}

// StartEventCall starts the call of a calendar event, or hands the caller a ticket
// into the one already open. created is true when this request opened it. The
// event must carry a call, and the caller must read it.
func (s *Service) StartEventCall(ctx context.Context, in StartEventCallInput) (join *pb.CallJoin, created bool, err error) {
	if s.media == nil {
		return nil, false, ErrCallsUnavailable
	}
	ref, err := s.authorizeEvent(ctx, in.EventID, in.UserNodeID)
	if err != nil {
		return nil, false, err
	}
	if !ref.HasCall {
		return nil, false, fmt.Errorf("%w: this event has no video call", ErrNotFound)
	}
	if open, err := s.liveOpenEventCall(ctx, ref.ID); err != nil || open != nil {
		if err != nil {
			return nil, false, err
		}
		join, err := s.ticket(ctx, open, in.UserID, in.UserNodeID)
		return join, false, err
	}

	now := time.Now()
	call := &store.Call{
		ID: uuid.New().String(), CalendarEventID: ref.ID, Kind: "video", Status: callActive,
		StartedBy: in.UserID, CreatedAt: now, StartedAt: &now,
	}
	// The room is opened first: a call row whose room cannot be opened would be a
	// call nobody can join.
	if err := s.media.CreateRoom(ctx, call.ID, MaxCallParticipants, callRoomEmptyTimeout); err != nil {
		return nil, false, err
	}
	if err := s.store.InsertEventCall(ctx, call); err != nil {
		s.closeRoom(call.ID)
		if !errors.Is(err, store.ErrEventCallOpen) {
			return nil, false, err
		}
		// Someone else started it in the same instant: join theirs.
		open, err := s.store.OpenEventCall(ctx, ref.ID)
		if err != nil || open == nil {
			return nil, false, fmt.Errorf("start event call: %w", err)
		}
		join, err := s.ticket(ctx, open, in.UserID, in.UserNodeID)
		return join, false, err
	}
	join, err = s.ticket(ctx, call, in.UserID, in.UserNodeID)
	return join, err == nil, err
}

// liveOpenEventCall returns the event's open call, unless its room has been empty
// long enough that the call is over and LiveKit's word of it was lost.
func (s *Service) liveOpenEventCall(ctx context.Context, eventID string) (*store.Call, error) {
	open, err := s.store.OpenEventCall(ctx, eventID)
	if err != nil || open == nil {
		return nil, err
	}
	open, err = s.reconcile(ctx, open)
	if err != nil || !slices.Contains(openCallStatuses, open.Status) {
		return nil, err
	}
	return open, nil
}
