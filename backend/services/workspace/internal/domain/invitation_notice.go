package domain

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"ngac-platform/pkg/mailer"
	"ngac-platform/services/workspace/internal/store"
)

const inviteNoticeTimeout = 15 * time.Second

// InvitationNotice is a new offer, addressed to one account. Names only for the
// people reading it; the IDs are for the notification service to file it.
type InvitationNotice struct {
	InvitationID  string
	UserID        string // users.id of the account the offer is for
	WorkspaceName string
	InviterName   string
}

// InvitationNotifier tells the notification service that an account has been
// offered a workspace, so it appears in that person's Thông báo. Delivery is
// best effort: a lost notice costs the nudge, never the invitation.
type InvitationNotifier interface {
	InvitationOffered(n InvitationNotice)
}

// WithInvitationNotifier turns on the in-app notification for an invitee.
func (s *Service) WithInvitationNotifier(n InvitationNotifier) *Service {
	s.notifier = n
	return s
}

// notifyInvitee tells the account behind the address, when there is one with a
// PROVED address, that a new offer is waiting. It runs after the invitation is
// stored and off the request, so the answer to the inviter is the same whether
// or not an account exists. A refreshed offer (the address was invited again)
// notifies nobody twice. Unknown and unverified addresses notify nobody: only a
// proved address can see offers at all.
func (s *Service) notifyInvitee(ctx context.Context, wsName string, inv *store.Invitation) {
	if s.notifier == nil || !inv.Created {
		return
	}
	bg := context.WithoutCancel(ctx)
	copied := *inv
	s.mailWG.Add(1)
	go func() {
		defer s.mailWG.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("invitation notice panicked", "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(bg, inviteNoticeTimeout)
		defer cancel()
		ids, err := s.invitations.VerifiedUserIDsByEmail(ctx, copied.Email)
		if err != nil {
			slog.Warn("invitation notice not sent", "to", mailer.MaskEmail(copied.Email), "error", redact(err, copied.Email, mailer.MaskEmail(copied.Email)))
			return
		}
		if len(ids) == 0 {
			return
		}
		view := s.describe(ctx, &copied, wsName)
		for _, id := range ids {
			s.notifier.InvitationOffered(InvitationNotice{
				InvitationID: copied.ID, UserID: id,
				WorkspaceName: oneLine(view.WorkspaceName), InviterName: strings.TrimSpace(oneLine(view.InviterName)),
			})
		}
	}()
}
