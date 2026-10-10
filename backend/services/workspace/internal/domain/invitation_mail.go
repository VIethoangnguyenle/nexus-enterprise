package domain

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"
	"time"

	"ngac-platform/pkg/mailer"
	"ngac-platform/services/workspace/internal/store"
)

// How the invitation email is sent. It is never part of the invite request: the
// response must be the same, and take as long, whatever happens to the mail.
const (
	// DefaultInviteResendWindow is how long after an invitation was emailed that
	// inviting the same address again sends nothing.
	DefaultInviteResendWindow = 10 * time.Minute

	inviteMailTimeout  = 45 * time.Second
	inviteMailInFlight = 16
)

// vietnamTime is the zone dates in the email are written in. A fixed offset, so
// the container needs no tz database.
var vietnamTime = time.FixedZone("ICT", 7*60*60)

// InviteMailer sends one email. *mailer.Sender is the production implementation.
type InviteMailer interface {
	Send(ctx context.Context, m mailer.Message) error
}

// WithInviteMail turns on the email sent to an invited address. appBaseURL is
// the web app, where the invitee signs in and finds the invitation. Without it
// (no SMTP configured) invitations are recorded and nothing is sent.
func (s *Service) WithInviteMail(m InviteMailer, appBaseURL string) *Service {
	s.mail = m
	s.appBaseURL = strings.TrimRight(appBaseURL, "/")
	return s
}

// WithInviteResendWindow sets how soon the same invitation may be emailed again.
func (s *Service) WithInviteResendWindow(d time.Duration) *Service {
	s.mailResend = d
	return s
}

// DrainMail waits for the invitation emails still being sent, up to timeout.
// Call it on shutdown, and in tests before looking at what was sent.
func (s *Service) DrainMail(timeout time.Duration) {
	done := make(chan struct{})
	go func() { s.mailWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("invitation emails still sending at shutdown")
	}
}

// emailInvitation sends the invitation email in the background, after the
// invitation is stored. Nothing that happens here reaches the caller: not the
// outcome, not how long it took.
func (s *Service) emailInvitation(ctx context.Context, wsName string, inv *store.Invitation) {
	if s.mail == nil {
		slog.Info("invitation email skipped: SMTP is not configured")
		return
	}
	select {
	case s.mailSlots <- struct{}{}:
	default:
		slog.Warn("invitation email skipped: too many are being sent")
		return
	}
	// Keep the caller's identity for the name lookups, lose its cancellation:
	// the request ends the moment the 202 is written.
	bg := context.WithoutCancel(ctx)
	copied := *inv
	s.mailWG.Add(1)
	go func() {
		defer s.mailWG.Done()
		defer func() { <-s.mailSlots }()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("invitation email panicked", "panic", r)
			}
		}()
		s.deliverInvitation(bg, wsName, &copied)
	}()
}

func (s *Service) deliverInvitation(ctx context.Context, wsName string, inv *store.Invitation) {
	ctx, cancel := context.WithTimeout(ctx, inviteMailTimeout)
	defer cancel()
	masked := mailer.MaskEmail(inv.Email)

	now := s.clock()
	ok, err := s.invitations.ClaimInvitationEmail(ctx, inv.ID, now, now.Add(-s.mailResend))
	if err != nil {
		slog.Warn("invitation email not sent", "to", masked, "error", redact(err, inv.Email, masked))
		return
	}
	if !ok {
		slog.Info("invitation email skipped: sent recently", "to", masked)
		return
	}
	msg := invitationMessage(s.describe(ctx, inv, wsName), s.appBaseURL)
	if err := s.mail.Send(ctx, msg); err != nil {
		slog.Warn("invitation email not sent", "to", masked, "error", redact(err, inv.Email, masked))
		// Give the slot back: the next invite may try again.
		rel, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if rerr := s.invitations.ReleaseInvitationEmail(rel, inv.ID); rerr != nil {
			slog.Warn("could not release an invitation email slot", "error", rerr)
		}
		return
	}
	slog.Info("invitation email sent", "to", masked)
}

// redact keeps the full address out of a log line: a server's refusal often
// repeats the recipient.
func redact(err error, addr, masked string) string {
	return strings.ReplaceAll(err.Error(), addr, masked)
}

// oneLine makes a name safe to place in a sentence or a subject: whatever
// whitespace it holds, line breaks included, becomes single spaces.
func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// invitationMessage writes the email: who invited, to what, with which role and
// department, until when, and where to sign in. Names only, never an ID.
func invitationMessage(v *InvitationView, appURL string) mailer.Message {
	inviter := oneLine(v.InviterName)
	if inviter == "" {
		inviter = "Một quản trị viên"
	}
	ws := oneLine(v.WorkspaceName)
	role, dept := oneLine(v.RoleName), oneLine(v.DepartmentName)
	until := v.ExpiresAt.In(vietnamTime).Format("02/01/2006")

	var details [][2]string
	if role != "" {
		details = append(details, [2]string{"Vai trò", role})
	}
	if dept != "" {
		details = append(details, [2]string{"Phòng ban", dept})
	}
	const mustMatch = "Hãy đăng nhập bằng đúng địa chỉ email này: lời mời chỉ hiện với tài khoản đã xác minh chính email đó."

	var t strings.Builder
	fmt.Fprintf(&t, "Chào bạn,\n\n%s mời bạn tham gia workspace \"%s\" trên Nexus.\n", inviter, ws)
	for _, d := range details {
		fmt.Fprintf(&t, "%s: %s\n", d[0], d[1])
	}
	fmt.Fprintf(&t, "\nLời mời có hiệu lực đến hết ngày %s.\n\n", until)
	fmt.Fprintf(&t, "Đăng nhập để xem và chấp nhận lời mời: %s\n%s\n\n", appURL, mustMatch)
	t.WriteString("Nếu bạn không quen người này, hãy bỏ qua email này.\n")

	var h strings.Builder
	h.WriteString("<!DOCTYPE html>\n<html lang=\"vi\"><body style=\"font-family:Arial,sans-serif;color:#1a1a1a\">\n")
	h.WriteString("<p>Chào bạn,</p>\n")
	fmt.Fprintf(&h, "<p><strong>%s</strong> mời bạn tham gia workspace <strong>%s</strong> trên Nexus.</p>\n",
		html.EscapeString(inviter), html.EscapeString(ws))
	if len(details) > 0 {
		h.WriteString("<ul>\n")
		for _, d := range details {
			fmt.Fprintf(&h, "<li>%s: <strong>%s</strong></li>\n", d[0], html.EscapeString(d[1]))
		}
		h.WriteString("</ul>\n")
	}
	fmt.Fprintf(&h, "<p>Lời mời có hiệu lực đến hết ngày %s.</p>\n", until)
	fmt.Fprintf(&h, "<p><a href=\"%s\">Đăng nhập Nexus để xem lời mời</a></p>\n", html.EscapeString(appURL))
	fmt.Fprintf(&h, "<p>%s</p>\n", mustMatch)
	h.WriteString("<p>Nếu bạn không quen người này, hãy bỏ qua email này.</p>\n</body></html>\n")

	return mailer.Message{
		To:      v.Email,
		Subject: fmt.Sprintf("%s mời bạn vào %s trên Nexus", inviter, ws),
		Text:    t.String(),
		HTML:    h.String(),
	}
}
