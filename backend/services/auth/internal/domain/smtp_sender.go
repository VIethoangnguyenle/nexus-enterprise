package domain

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTP transport security modes.
const (
	// SMTPTLSStartTLS upgrades a plain connection with STARTTLS (port 587).
	SMTPTLSStartTLS = "starttls"
	// SMTPTLSImplicit wraps the connection in TLS from the first byte (port 465).
	SMTPTLSImplicit = "tls"
)

const (
	defaultSMTPDialTimeout = 10 * time.Second
	defaultSMTPSendTimeout = 30 * time.Second
	maxSMTPAddressLen      = 254
)

// SMTPConfig configures SMTPSender.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	// From is the sender, "Display Name <address>" or a bare address. For Gmail it
	// must be the account itself or a verified alias.
	From string
	// TLS is SMTPTLSStartTLS or SMTPTLSImplicit. Empty picks implicit TLS for port
	// 465 and STARTTLS otherwise. There is no plaintext mode: credentials and
	// codes never cross the network unencrypted.
	TLS string
	// DialTimeout bounds connecting; SendTimeout bounds the whole exchange.
	// Zero values use 10s and 30s.
	DialTimeout time.Duration
	SendTimeout time.Duration
	// TLSConfig overrides the TLS settings (a private CA, in tests). ServerName
	// defaults to Host.
	TLSConfig *tls.Config
}

// SMTPSender delivers one-time codes by email over SMTP. It reaches the owner
// of the mailbox, so a code accepted after it proves the address.
type SMTPSender struct {
	cfg  SMTPConfig
	from mail.Address
	// validity is how long a code lives, as stated in the message.
	validity time.Duration
}

var (
	_ CodeSender      = (*SMTPSender)(nil)
	_ OwnershipProver = (*SMTPSender)(nil)
)

// NewSMTPSender validates the configuration and returns a sender.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Port = strings.TrimSpace(cfg.Port)
	cfg.TLS = strings.ToLower(strings.TrimSpace(cfg.TLS))
	if cfg.Host == "" || strings.ContainsAny(cfg.Host, " /\r\n") {
		return nil, errors.New("smtp: host is required and must be a bare host name")
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if n, err := strconv.Atoi(cfg.Port); err != nil || n < 1 || n > 65535 {
		return nil, fmt.Errorf("smtp: port %q is not a valid port", cfg.Port)
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, errors.New("smtp: username and password are required")
	}
	switch cfg.TLS {
	case "":
		cfg.TLS = SMTPTLSStartTLS
		if cfg.Port == "465" {
			cfg.TLS = SMTPTLSImplicit
		}
	case SMTPTLSStartTLS, SMTPTLSImplicit:
	default:
		return nil, fmt.Errorf("smtp: tls mode %q must be %q or %q", cfg.TLS, SMTPTLSStartTLS, SMTPTLSImplicit)
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaultSMTPDialTimeout
	}
	if cfg.SendTimeout <= 0 {
		cfg.SendTimeout = defaultSMTPSendTimeout
	}
	if strings.ContainsAny(cfg.From, "\r\n") {
		return nil, errors.New("smtp: from address contains a line break")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("smtp: from address: %w", err)
	}
	return &SMTPSender{cfg: cfg, from: *from, validity: otpTTL}, nil
}

// DeliversToOwner reports that codes sent here reach only the mailbox owner.
func (*SMTPSender) DeliversToOwner() bool { return true }

// SendCode emails the code to identifier. Only email identifiers are supported.
// The error never contains the code or the message body.
func (s *SMTPSender) SendCode(ctx context.Context, identifier, identType, code string) error {
	if identType != "email" {
		return fmt.Errorf("smtp: cannot deliver to a %q identifier", identType)
	}
	rcpt, err := parseRecipient(identifier)
	if err != nil {
		return err
	}
	msg, err := s.buildMessage(rcpt, code, time.Now())
	if err != nil {
		return err
	}
	if err := s.deliver(ctx, rcpt, msg); err != nil {
		return err
	}
	slog.Info("OTP email sent", "to", maskIdentifier(rcpt, "email"))
	return nil
}

// parseRecipient accepts only a bare addr-spec. Anything that could add a
// header, a second recipient or a display name is refused, not cleaned.
func parseRecipient(s string) (string, error) {
	if s == "" || len(s) > maxSMTPAddressLen {
		return "", errors.New("smtp: recipient is empty or too long")
	}
	for _, r := range s {
		if r <= 0x20 || r == 0x7f || strings.ContainsRune("<>,;:\"\\()[]", r) {
			return "", errors.New("smtp: recipient contains a character not allowed in an address")
		}
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || a.Name != "" {
		return "", errors.New("smtp: recipient is not a plain email address")
	}
	return s, nil
}

func (s *SMTPSender) tlsConfig() *tls.Config {
	var c *tls.Config
	if s.cfg.TLSConfig != nil {
		c = s.cfg.TLSConfig.Clone()
	} else {
		c = &tls.Config{}
	}
	if c.ServerName == "" {
		c.ServerName = s.cfg.Host
	}
	if c.MinVersion == 0 {
		c.MinVersion = tls.VersionTLS12
	}
	return c
}

// deliver runs one SMTP transaction under the send timeout.
func (s *SMTPSender) deliver(ctx context.Context, rcpt string, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.SendTimeout)
	defer cancel()

	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{Timeout: s.cfg.DialTimeout}
	var (
		conn net.Conn
		err  error
	)
	if s.cfg.TLS == SMTPTLSImplicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: s.tlsConfig()}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp: connect: %w", err)
	}
	defer conn.Close()
	// A server that stops answering must not hold the request: every read and
	// write ends at the deadline.
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp: set deadline: %w", err)
	}

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp: greeting: %w", err)
	}
	defer c.Close()

	if s.cfg.TLS == SMTPTLSStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			// Refuse rather than send credentials in the clear.
			return errors.New("smtp: server does not offer STARTTLS")
		}
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			return fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
		return fmt.Errorf("smtp: auth: %w", err)
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("smtp: mail from: %w", err)
	}
	if err := c.Rcpt(rcpt); err != nil {
		return fmt.Errorf("smtp: rcpt to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp: data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp: write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp: message not accepted: %w", err)
	}
	// The message is accepted; a failing QUIT changes nothing.
	_ = c.Quit()
	return nil
}

// buildMessage renders the multipart/alternative message (plain text first,
// HTML second) with CRLF line endings.
func (s *SMTPSender) buildMessage(rcpt, code string, now time.Time) ([]byte, error) {
	minutes := int(s.validity / time.Minute)
	subject := "Mã đăng nhập Nexus: " + code
	text := fmt.Sprintf("Mã đăng nhập Nexus của bạn là: %s\n\n"+
		"Mã có hiệu lực trong %d phút. Đừng chia sẻ mã này với bất kỳ ai.\n\n"+
		"Nếu bạn không yêu cầu mã này, hãy bỏ qua email này.\n", code, minutes)
	htmlBody := fmt.Sprintf("<!DOCTYPE html>\n<html lang=\"vi\"><body style=\"font-family:Arial,sans-serif;color:#1a1a1a\">\n"+
		"<p>Mã đăng nhập Nexus của bạn là:</p>\n"+
		"<p style=\"font-size:28px;font-weight:bold;letter-spacing:4px\">%s</p>\n"+
		"<p>Mã có hiệu lực trong %d phút. Đừng chia sẻ mã này với bất kỳ ai.</p>\n"+
		"<p>Nếu bạn không yêu cầu mã này, hãy bỏ qua email này.</p>\n</body></html>\n",
		html.EscapeString(code), minutes)

	idRand := make([]byte, 16)
	boundRand := make([]byte, 12)
	if _, err := rand.Read(idRand); err != nil {
		return nil, fmt.Errorf("smtp: random: %w", err)
	}
	if _, err := rand.Read(boundRand); err != nil {
		return nil, fmt.Errorf("smtp: random: %w", err)
	}
	domainPart := "localhost"
	if i := strings.LastIndex(s.from.Address, "@"); i >= 0 && i+1 < len(s.from.Address) {
		domainPart = s.from.Address[i+1:]
	}
	boundary := "nexus-" + hex.EncodeToString(boundRand)

	var b strings.Builder
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", s.from.String())
	header("To", rcpt)
	header("Subject", mime.QEncoding.Encode("utf-8", subject))
	header("Date", now.Format(time.RFC1123Z))
	header("Message-ID", "<"+hex.EncodeToString(idRand)+"@"+domainPart+">")
	header("MIME-Version", "1.0")
	header("Auto-Submitted", "auto-generated")
	header("Content-Type", "multipart/alternative; boundary=\""+boundary+"\"")
	b.WriteString("\r\n")

	part := func(contentType, body string) error {
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: " + contentType + "; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		w := quotedprintable.NewWriter(&b)
		if _, err := w.Write([]byte(body)); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		b.WriteString("\r\n")
		return nil
	}
	if err := part("text/plain", text); err != nil {
		return nil, fmt.Errorf("smtp: encode text part: %w", err)
	}
	if err := part("text/html", htmlBody); err != nil {
		return nil, fmt.Errorf("smtp: encode html part: %w", err)
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String()), nil
}
