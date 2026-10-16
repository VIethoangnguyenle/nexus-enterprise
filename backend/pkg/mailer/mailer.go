// Package mailer sends email over SMTP for every service that needs to: the
// auth service's sign-in codes and the workspace service's invitations share
// one transport, one set of safety rules and one SMTP_* configuration.
//
// The transport is STARTTLS or implicit TLS only; there is no plaintext mode, so
// credentials and message contents never cross the network unencrypted. It uses
// only the standard library.
package mailer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Transport security modes.
const (
	// TLSStartTLS upgrades a plain connection with STARTTLS (port 587).
	TLSStartTLS = "starttls"
	// TLSImplicit wraps the connection in TLS from the first byte (port 465).
	TLSImplicit = "tls"
)

const (
	defaultDialTimeout = 10 * time.Second
	defaultSendTimeout = 30 * time.Second
	defaultIdleTimeout = 30 * time.Second
	maxAddressLen      = 254
)

// Config configures a Sender.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	// From is the sender, "Display Name <address>" or a bare address. For Gmail it
	// must be the account itself or a verified alias.
	From string
	// TLS is TLSStartTLS or TLSImplicit. Empty picks implicit TLS for port 465
	// and STARTTLS otherwise.
	TLS string
	// DialTimeout bounds connecting; SendTimeout bounds the whole exchange.
	// Zero values use 10s and 30s.
	DialTimeout time.Duration
	SendTimeout time.Duration
	// IdleTimeout is how long an authenticated connection is kept for the next
	// message. Zero uses 30s.
	IdleTimeout time.Duration
	// TLSConfig overrides the TLS settings (a private CA, in tests). ServerName
	// defaults to Host.
	TLSConfig *tls.Config
}

// Message is one email to one recipient: a plain-text part and an HTML part.
type Message struct {
	// To is a bare address (no display name, no list).
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers messages through one SMTP account.
type Sender struct {
	cfg  Config
	from mail.Address

	// mu serialises transactions and guards the kept session and its idle timer.
	mu   sync.Mutex
	sess *session
	idle *time.Timer
}

// New validates the configuration and returns a sender.
func New(cfg Config) (*Sender, error) {
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
		cfg.TLS = TLSStartTLS
		if cfg.Port == "465" {
			cfg.TLS = TLSImplicit
		}
	case TLSStartTLS, TLSImplicit:
	default:
		return nil, fmt.Errorf("smtp: tls mode %q must be %q or %q", cfg.TLS, TLSStartTLS, TLSImplicit)
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = defaultDialTimeout
	}
	if cfg.SendTimeout <= 0 {
		cfg.SendTimeout = defaultSendTimeout
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}
	if strings.ContainsAny(cfg.From, "\r\n") {
		return nil, errors.New("smtp: from address contains a line break")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("smtp: from address: %w", err)
	}
	return &Sender{cfg: cfg, from: *from}, nil
}

// Send delivers m. The error never contains the message body.
func (s *Sender) Send(ctx context.Context, m Message) error {
	rcpt, err := parseRecipient(m.To)
	if err != nil {
		return err
	}
	msg, err := s.build(rcpt, m, time.Now())
	if err != nil {
		return err
	}
	return s.deliver(ctx, rcpt, msg)
}

// parseRecipient accepts only a bare addr-spec. Anything that could add a
// header, a second recipient or a display name is refused, not cleaned.
func parseRecipient(s string) (string, error) {
	if s == "" || len(s) > maxAddressLen {
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

func (s *Sender) tlsConfig() *tls.Config {
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

// session is one authenticated SMTP connection, kept between sends.
type session struct {
	conn net.Conn
	c    *smtp.Client
}

func (ss *session) close(polite bool) {
	if polite {
		_ = ss.c.Quit()
	}
	_ = ss.c.Close()
	_ = ss.conn.Close()
}

// connect dials, secures and authenticates a fresh session under ctx.
func (s *Sender) connect(ctx context.Context) (*session, error) {
	addr := net.JoinHostPort(s.cfg.Host, s.cfg.Port)
	dialer := &net.Dialer{Timeout: s.cfg.DialTimeout}
	var (
		conn net.Conn
		err  error
	)
	if s.cfg.TLS == TLSImplicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: s.tlsConfig()}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("smtp: connect: %w", err)
	}
	// A server that stops answering must not hold the caller: every read and
	// write ends at the deadline.
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp: set deadline: %w", err)
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("smtp: greeting: %w", err)
	}
	ss := &session{conn: conn, c: c}
	if s.cfg.TLS == TLSStartTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			ss.close(false)
			// Refuse rather than send credentials in the clear.
			return nil, errors.New("smtp: server does not offer STARTTLS")
		}
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			ss.close(false)
			return nil, fmt.Errorf("smtp: starttls: %w", err)
		}
	}
	if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
		ss.close(false)
		return nil, fmt.Errorf("smtp: auth: %w", err)
	}
	return ss, nil
}

// transact runs one mail transaction on an open session.
func (ss *session) transact(from, rcpt string, msg []byte) error {
	c := ss.c
	if err := c.Mail(from); err != nil {
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
	return nil
}

// deliver runs one SMTP transaction under the send timeout. The authenticated
// connection stays open for IdleTimeout afterwards, so a burst of messages pays
// the TCP, TLS and login round trips once. A kept connection that no longer
// answers is replaced; a failed transaction drops the connection, so the next
// send starts from a clean state.
func (s *Sender) deliver(ctx context.Context, rcpt string, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.SendTimeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idle != nil {
		s.idle.Stop()
		s.idle = nil
	}

	ss := s.sess
	s.sess = nil
	if ss != nil {
		// The server may have closed an idle connection; one NOOP finds out.
		if err := ss.conn.SetDeadline(deadline); err != nil || ss.c.Noop() != nil {
			ss.close(false)
			ss = nil
		}
	}
	if ss == nil {
		var err error
		if ss, err = s.connect(ctx); err != nil {
			return err
		}
	}
	if err := ss.transact(s.from.Address, rcpt, msg); err != nil {
		ss.close(false)
		return err
	}
	s.sess = ss
	s.idle = time.AfterFunc(s.cfg.IdleTimeout, s.closeIdle)
	return nil
}

// closeIdle ends a kept connection nobody has used for IdleTimeout.
func (s *Sender) closeIdle() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sess != nil {
		_ = s.sess.conn.SetDeadline(time.Now().Add(2 * time.Second))
		s.sess.close(true)
		s.sess = nil
	}
	s.idle = nil
}

// Close ends the kept connection, if any. The sender stays usable: the next
// Send connects again.
func (s *Sender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idle != nil {
		s.idle.Stop()
		s.idle = nil
	}
	if s.sess != nil {
		_ = s.sess.conn.SetDeadline(time.Now().Add(2 * time.Second))
		s.sess.close(true)
		s.sess = nil
	}
	return nil
}

// headerText folds anything that could end a header line into a space, so text
// from outside (a person's name in a subject) cannot add headers.
func headerText(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == 0x85 || r == 0x2028 || r == 0x2029 {
			return ' '
		}
		return r
	}, s)
}

// build renders the multipart/alternative message (plain text first, HTML
// second) with CRLF line endings.
func (s *Sender) build(rcpt string, m Message, now time.Time) ([]byte, error) {
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
	header("Subject", mime.QEncoding.Encode("utf-8", headerText(m.Subject)))
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
	if err := part("text/plain", m.Text); err != nil {
		return nil, fmt.Errorf("smtp: encode text part: %w", err)
	}
	if err := part("text/html", m.HTML); err != nil {
		return nil, fmt.Errorf("smtp: encode html part: %w", err)
	}
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String()), nil
}

// MaskEmail hides most of an address for logs: "mo***@novapay.vn". Anything that
// is not an address with a local part longer than two characters becomes "***".
func MaskEmail(addr string) string {
	local, domain, ok := strings.Cut(addr, "@")
	if ok && len(local) > 2 {
		return local[:2] + "***@" + domain
	}
	return "***"
}
