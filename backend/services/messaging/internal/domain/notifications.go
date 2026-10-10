package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// NotificationStore is the persistence the notification service needs.
// *store.Store implements it.
type NotificationStore interface {
	InsertNotification(ctx context.Context, n *store.Notification) error
	ListNotifications(ctx context.Context, userID, workspaceID string, limit, offset int) ([]*store.Notification, error)
	NotificationCounts(ctx context.Context, userID, workspaceID string) (total, unread int, err error)
	MarkNotificationRead(ctx context.Context, id, userID, workspaceID string) error
	MarkAllNotificationsRead(ctx context.Context, userID, workspaceID string) error
	FindMember(ctx context.Context, workspaceID, key string) (store.Member, bool, error)
	InviteesOf(ctx context.Context, invitationID string, now time.Time) ([]store.Invitee, error)
	DeleteNotificationsAbout(ctx context.Context, userID, notifType, subjectID string) error
	MarkNotificationsAboutRead(ctx context.Context, userID, workspaceID, subjectType, subjectID string) error
}

// NotificationPusher delivers a new notification to the recipient's open
// connections in the notification's workspace, and to no other session of theirs.
// An empty workspace means a personal notification: every session of the user.
// The WebSocket hub implements it.
type NotificationPusher interface {
	SendNotification(workspaceID, userID string, n *pb.Notification)
}

// Notification page bounds.
const (
	DefaultNotificationPage = 25
	MaxNotificationPage     = 50
)

// maxParamRunes bounds one param value (a rejection reason, say) so a notification
// stays a notification.
const maxParamRunes = 500

// ErrNotAMember reports that the person a notification is for does not belong to
// the workspace it was raised in; nothing is stored or pushed for them.
var ErrNotAMember = errors.New("recipient is not a member of the workspace")

// NotificationService owns a user's notifications: listing, reading, creating.
// Both the gRPC and the REST edge call it; neither runs SQL. Every read and every
// mark is for one user in one workspace.
type NotificationService struct {
	store NotificationStore
	push  NotificationPusher
	now   func() time.Time
}

// NewNotificationService returns the service. push may be nil: notifications are
// then stored without being pushed live.
func NewNotificationService(st NotificationStore, push NotificationPusher) *NotificationService {
	return &NotificationService{store: st, push: push, now: time.Now}
}

// NotificationPage is one page of a user's notifications and their totals.
type NotificationPage struct {
	Items  []*store.Notification
	Total  int
	Unread int
}

// requireScope refuses a call that does not say who is asking and in which
// workspace: without both there is nothing to scope the answer to.
func requireScope(userID, workspaceID string) error {
	if userID == "" || workspaceID == "" {
		return fmt.Errorf("%w: a signed-in user in a workspace is required", ErrAccessDenied)
	}
	return nil
}

// List returns one page of the user's notifications in the workspace, newest
// first. A limit outside 1..MaxNotificationPage becomes DefaultNotificationPage.
func (s *NotificationService) List(ctx context.Context, userID, workspaceID string, limit, offset int) (*NotificationPage, error) {
	if err := requireScope(userID, workspaceID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > MaxNotificationPage {
		limit = DefaultNotificationPage
	}
	if offset < 0 {
		offset = 0
	}
	total, unread, err := s.store.NotificationCounts(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListNotifications(ctx, userID, workspaceID, limit, offset)
	if err != nil {
		return nil, err
	}
	return &NotificationPage{Items: items, Total: total, Unread: unread}, nil
}

// UnreadCount returns how many notifications the user has not read in the workspace.
func (s *NotificationService) UnreadCount(ctx context.Context, userID, workspaceID string) (int, error) {
	if err := requireScope(userID, workspaceID); err != nil {
		return 0, err
	}
	_, unread, err := s.store.NotificationCounts(ctx, userID, workspaceID)
	return unread, err
}

// MarkRead marks one of the user's notifications in the workspace read. A
// notification that is not theirs, or is in another workspace, is left alone.
func (s *NotificationService) MarkRead(ctx context.Context, userID, workspaceID, notificationID string) error {
	if err := requireScope(userID, workspaceID); err != nil {
		return err
	}
	return s.store.MarkNotificationRead(ctx, notificationID, userID, workspaceID)
}

// MarkAllRead marks all of the user's notifications in the workspace read.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID, workspaceID string) error {
	if err := requireScope(userID, workspaceID); err != nil {
		return err
	}
	return s.store.MarkAllNotificationsRead(ctx, userID, workspaceID)
}

// MarkAboutRead marks the user's notifications about one subject read. It is how
// answering an invitation reads its notice. Scoped like every other mark.
func (s *NotificationService) MarkAboutRead(ctx context.Context, userID, workspaceID, subjectType, subjectID string) error {
	if err := requireScope(userID, workspaceID); err != nil {
		return err
	}
	if subjectType == "" || subjectID == "" {
		return fmt.Errorf("%w: a type and an id are required", ErrInvalidInput)
	}
	return s.store.MarkNotificationsAboutRead(ctx, userID, workspaceID, subjectType, subjectID)
}

// NotifyInvitation tells the accounts an invitation reaches that they were
// invited. It reads the stored invitation, so whether an account exists is
// decided here and never by (or reported to) the inviter's request: an address
// with no verified account produces nothing. The notice is personal (the invitee
// is not yet a member) and is pushed to every open session of theirs. Inviting
// again replaces the earlier notice.
func (s *NotificationService) NotifyInvitation(ctx context.Context, invitationID string) error {
	if invitationID == "" {
		return fmt.Errorf("%w: an invitation is required", ErrInvalidInput)
	}
	invitees, err := s.store.InviteesOf(ctx, invitationID, s.now())
	if err != nil {
		return err
	}
	for _, v := range invitees {
		actor, _, err := s.store.FindMember(ctx, v.WorkspaceID, v.InviterNode)
		if err != nil {
			return err
		}
		if err := s.store.DeleteNotificationsAbout(ctx, v.UserID, store.TypeWorkspaceInvitation, invitationID); err != nil {
			return err
		}
		n := &store.Notification{
			ID: uuid.New().String(), UserID: v.UserID, WorkspaceID: v.WorkspaceID, Type: store.TypeWorkspaceInvitation,
			ActorUserID: actor.UserID, ActorName: actor.Name,
			TargetType: store.TypeWorkspaceInvitation, TargetID: invitationID, TargetName: v.WorkspaceName,
			Params: map[string]string{}, CreatedAt: s.now(),
		}
		if err := s.store.InsertNotification(ctx, n); err != nil {
			return err
		}
		if s.push != nil {
			// No workspace: the invitee's sessions are in whichever one they have open.
			s.push.SendNotification("", n.UserID, NotificationToProto(n))
		}
	}
	return nil
}

// NewNotification is a notification to raise, as an event describes it.
//
// Recipient and Actor are a user id or an NGAC user node id (approvals name
// people by node); the service resolves them within WorkspaceID. Names are never
// supplied by the caller: they are read from the workspace's own membership.
type NewNotification struct {
	WorkspaceID string
	Type        string
	Recipient   string
	// Actor is who did it; empty or unresolvable means the notification names nobody.
	Actor      string
	TargetType string
	TargetID   string
	// TargetName is what the subject is called; empty when the event does not say.
	TargetName string
	Params     map[string]string
}

// CreateNotification stores a notification for its recipient and pushes it to
// their open connections in the same workspace. The Kafka consumer calls it;
// there is no caller to authorize. The push happens only after the row is stored.
//
// A person is never notified of their own act: when the actor and the recipient
// are the same person nothing is stored. A recipient who is not a member of the
// workspace gets ErrNotAMember.
func (s *NotificationService) CreateNotification(ctx context.Context, in NewNotification) error {
	if in.WorkspaceID == "" || in.Type == "" || in.Recipient == "" {
		return fmt.Errorf("%w: a notification needs a workspace, a type and a recipient", ErrInvalidInput)
	}
	to, ok, err := s.store.FindMember(ctx, in.WorkspaceID, in.Recipient)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAMember
	}
	var actor store.Member
	if in.Actor != "" {
		if actor, _, err = s.store.FindMember(ctx, in.WorkspaceID, in.Actor); err != nil {
			return err
		}
	}
	if actor.UserID != "" && actor.UserID == to.UserID {
		return nil
	}

	n := &store.Notification{
		ID: uuid.New().String(), UserID: to.UserID, WorkspaceID: in.WorkspaceID, Type: in.Type,
		ActorUserID: actor.UserID, ActorName: actor.Name,
		TargetType: in.TargetType, TargetID: in.TargetID, TargetName: in.TargetName,
		Params: clipParams(in.Params), CreatedAt: s.now(),
	}
	if err := s.store.InsertNotification(ctx, n); err != nil {
		return err
	}
	if s.push != nil {
		s.push.SendNotification(n.WorkspaceID, n.UserID, NotificationToProto(n))
	}
	return nil
}

// NotificationToProto is the wire form of a stored notification.
func NotificationToProto(n *store.Notification) *pb.Notification {
	return &pb.Notification{
		Id: n.ID, UserId: n.UserID, WorkspaceId: n.WorkspaceID, Type: n.Type,
		ActorUserId: n.ActorUserID, ActorName: n.ActorName,
		TargetType: n.TargetType, TargetId: n.TargetID, TargetName: n.TargetName,
		Params: n.Params, Read: n.Read, CreatedAt: timestamppb.New(n.CreatedAt),
	}
}

func clipParams(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if v == "" {
			continue
		}
		if utf8.RuneCountInString(v) > maxParamRunes {
			v = string([]rune(v)[:maxParamRunes])
		}
		out[k] = v
	}
	return out
}
