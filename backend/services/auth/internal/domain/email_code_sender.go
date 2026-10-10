package domain

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"time"

	"ngac-platform/pkg/mailer"
)

// Mailer sends one email. *mailer.Sender is the production implementation.
type Mailer interface {
	Send(ctx context.Context, m mailer.Message) error
}

// EmailCodeSender delivers one-time codes by email. It reaches the owner of the
// mailbox, so a code accepted after it proves the address.
type EmailCodeSender struct {
	mail Mailer
	// validity is how long a code lives, as stated in the message.
	validity time.Duration
}

var (
	_ CodeSender      = (*EmailCodeSender)(nil)
	_ OwnershipProver = (*EmailCodeSender)(nil)
)

// NewEmailCodeSender sends codes through m.
func NewEmailCodeSender(m Mailer) *EmailCodeSender {
	return &EmailCodeSender{mail: m, validity: otpTTL}
}

// DeliversToOwner reports that codes sent here reach only the mailbox owner.
func (*EmailCodeSender) DeliversToOwner() bool { return true }

// SendCode emails the code to identifier. Only email identifiers are supported.
// The error never contains the code or the message body.
func (s *EmailCodeSender) SendCode(ctx context.Context, identifier, identType, code string) error {
	if identType != "email" {
		return fmt.Errorf("smtp: cannot deliver to a %q identifier", identType)
	}
	if err := s.mail.Send(ctx, otpMessage(identifier, code, s.validity)); err != nil {
		return err
	}
	slog.Info("OTP email sent", "to", maskIdentifier(identifier, "email"))
	return nil
}

// otpMessage is the sign-in code email: plain text and HTML, in Vietnamese.
func otpMessage(rcpt, code string, validity time.Duration) mailer.Message {
	minutes := int(validity / time.Minute)
	text := fmt.Sprintf("Mã đăng nhập Nexus của bạn là: %s\n\n"+
		"Mã có hiệu lực trong %d phút. Đừng chia sẻ mã này với bất kỳ ai.\n\n"+
		"Nếu bạn không yêu cầu mã này, hãy bỏ qua email này.\n", code, minutes)
	htmlBody := fmt.Sprintf("<!DOCTYPE html>\n<html lang=\"vi\"><body style=\"font-family:Arial,sans-serif;color:#1a1a1a\">\n"+
		"<p>Mã đăng nhập Nexus của bạn là:</p>\n"+
		"<p style=\"font-size:28px;font-weight:bold;letter-spacing:4px\">%s</p>\n"+
		"<p>Mã có hiệu lực trong %d phút. Đừng chia sẻ mã này với bất kỳ ai.</p>\n"+
		"<p>Nếu bạn không yêu cầu mã này, hãy bỏ qua email này.</p>\n</body></html>\n",
		html.EscapeString(code), minutes)
	return mailer.Message{To: rcpt, Subject: "Mã đăng nhập Nexus: " + code, Text: text, HTML: htmlBody}
}
