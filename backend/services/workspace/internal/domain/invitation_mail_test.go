package domain_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/mailer"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

const appURL = "https://nexus.example.test"

// mailFixture is the invite fixture with a real mailer pointed at a fake SMTP
// server, so what is asserted is what would reach the mailbox.
type mailFixture struct {
	*inviteFixture
	smtp *testutil.FakeSMTP
}

func newMailFixture(t *testing.T, mutate func(*testutil.FakeSMTP)) *mailFixture {
	t.Helper()
	f := newInviteFixture(t)
	srv := testutil.NewFakeSMTP(t, false, mutate)
	m, err := mailer.New(mailer.Config{
		Host: srv.Host, Port: srv.Port, Username: "nexus@mail.test", Password: "app-password-1234",
		From: "Nexus <nexus@mail.test>", TLS: "starttls",
		TLSConfig: &tls.Config{RootCAs: srv.Pool}, SendTimeout: 2 * time.Second,
	})
	require.NoError(t, err)
	f.svc = f.svc.WithInviteMail(m, appURL+"/")
	return &mailFixture{inviteFixture: f, smtp: srv}
}

// sent waits for the background sends and returns the messages accepted.
func (f *mailFixture) sent(t *testing.T) []*sentMail {
	f.svc.DrainMail(5 * time.Second)
	var out []*sentMail
	for _, raw := range f.smtp.Messages() {
		out = append(out, parseSent(t, raw))
	}
	return out
}

type sentMail struct {
	to, subject, text, html string
	header                  mail.Header
}

func parseSent(t *testing.T, raw string) *sentMail {
	t.Helper()
	m, err := mail.ReadMessage(strings.NewReader(raw))
	require.NoError(t, err)
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	require.NoError(t, err)
	out := &sentMail{to: m.Header.Get("To"), subject: subject, header: m.Header}
	_, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
	mr := multipart.NewReader(m.Body, params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		body, _ := io.ReadAll(quotedprintable.NewReader(p))
		if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") {
			out.html = string(body)
		} else {
			out.text = string(body)
		}
	}
	return out
}

func TestInviteMail_SendsOneEmailWithTheRightNames(t *testing.T) {
	f := newMailFixture(t, nil)
	f.dir.profiles[delegator] = &store.Profile{UserID: "user-del", NodeID: delegator, DisplayName: "Đỗ Văn Khải"}
	f.invite(t, delegator, domain.InviteInput{RoleID: roleRead, DepartmentID: "dept-root-1"})

	got := f.sent(t)
	require.Len(t, got, 1)
	m := got[0]
	assert.Equal(t, newbieMail, m.to)
	assert.Equal(t, "Đỗ Văn Khải mời bạn vào Acme trên Nexus", m.subject)
	assert.Contains(t, m.header.Get("From"), "nexus@mail.test")
	for _, body := range []string{m.text, m.html} {
		assert.Contains(t, body, "Đỗ Văn Khải")
		assert.Contains(t, body, "Acme")
		assert.Contains(t, body, "Người đọc", "the role, by name")
		assert.Contains(t, body, "Root", "the department, by name")
		assert.Contains(t, body, "17/10/2026", "expires seven days after 2026-10-10")
		assert.Contains(t, body, "đúng địa chỉ email này")
		assert.Contains(t, body, appURL)
	}
	assert.NotContains(t, m.text, appURL+"/", "the trailing slash of the base URL is dropped")
	assert.Contains(t, m.html, `<a href="`+appURL+`">`)
}

func TestInviteMail_OmitsRoleAndDepartmentWhenNoneAttached(t *testing.T) {
	f := newMailFixture(t, nil)
	f.invite(t, inviter, domain.InviteInput{})
	got := f.sent(t)
	require.Len(t, got, 1)
	assert.NotContains(t, got[0].text, "Vai trò")
	assert.NotContains(t, got[0].text, "Phòng ban")
	assert.NotContains(t, got[0].html, "<ul>")
}

func TestInviteMail_ShowsNoIdentifier(t *testing.T) {
	f := newMailFixture(t, nil)
	f.dir.profiles[delegator] = &store.Profile{UserID: "user-del", NodeID: delegator, DisplayName: "Đỗ Văn Khải"}
	f.invite(t, delegator, domain.InviteInput{RoleID: roleRead, DepartmentID: "dept-root-1"})
	got := f.sent(t)
	require.Len(t, got, 1)
	for _, body := range []string{got[0].subject, got[0].text, got[0].html} {
		assert.NotRegexp(t, uuidRE, body)
		for _, id := range []string{ws1, delegator, "user-del", roleRead, "dept-root-1", "ua-dept-root-1", ngac.RoleUAName("r-read")} {
			assert.NotContains(t, body, id)
		}
	}
}

func TestInviteMail_SMTPFailureStillAnswersAndKeepsTheInvitation(t *testing.T) {
	f := newMailFixture(t, func(s *testutil.FakeSMTP) { s.RejectData = true })
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: newbieMail}))
	f.svc.DrainMail(5 * time.Second)
	assert.Empty(t, f.smtp.Messages())
	assert.Equal(t, 1, f.smtp.Conns(), "it did try")
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))

	// The slot was given back: a failed send does not block the next attempt.
	f.inv.mu.Lock()
	assert.Empty(t, f.inv.emailed)
	f.inv.mu.Unlock()
}

func TestInviteMail_UnreachableServerDoesNotHoldTheAnswer(t *testing.T) {
	f := newMailFixture(t, func(s *testutil.FakeSMTP) { s.Hang = true })
	start := time.Now()
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: newbieMail}))
	assert.Less(t, time.Since(start), time.Second, "the answer does not wait for the mail")
	f.svc.DrainMail(10 * time.Second)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))
}

// An address with an account and one without get the same answer, the same
// stored effect and the same email: nothing in the path looks accounts up.
func TestInviteMail_SameForAnAccountAndAStranger(t *testing.T) {
	f := newMailFixture(t, nil)
	var sentTo []string
	for _, addr := range []string{newbieMail /* has an account */, "nobody.at.all@example.org"} {
		require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: addr}), addr)
	}
	got := f.sent(t)
	require.Len(t, got, 2)
	for _, m := range got {
		sentTo = append(sentTo, m.to)
		assert.Equal(t, got[0].subject, m.subject)
		assert.Equal(t, got[0].text, m.text)
	}
	assert.ElementsMatch(t, []string{newbieMail, "nobody.at.all@example.org"}, sentTo)
	assert.Zero(t, f.inv.gets)
}

func TestInviteMail_ResendWindowIsHonoured(t *testing.T) {
	f := newMailFixture(t, nil)
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(5 * time.Second)
	require.Len(t, f.smtp.Messages(), 1)

	f.advance(5 * time.Minute)
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(5 * time.Second)
	assert.Len(t, f.smtp.Messages(), 1, "inside the window: refreshed, not mailed again")

	f.advance(6 * time.Minute) // 11 minutes since the first
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(5 * time.Second)
	assert.Len(t, f.smtp.Messages(), 2, "past the window: mailed again")
	assert.Equal(t, 1, f.inv.count(), "all of it one invitation")
}

func TestInviteMail_AFailedSendDoesNotUseUpTheWindow(t *testing.T) {
	var mu sync.Mutex
	fail := true
	rec := &recordingMailer{fail: func() bool { mu.Lock(); defer mu.Unlock(); return fail }}
	f := newInviteFixture(t)
	f.svc = f.svc.WithInviteMail(rec, appURL)
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(time.Second)
	mu.Lock()
	fail = false
	mu.Unlock()
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(time.Second)
	assert.Equal(t, 2, rec.attempts())
	assert.Len(t, rec.messages(), 1)
}

func TestInviteMail_OnlyAnOpenInvitationIsMailedAgain(t *testing.T) {
	f := newMailFixture(t, nil)
	id := f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(5 * time.Second)
	require.NoError(t, f.svc.RevokeInvitation(ctx(), inviter, ws1, id))
	f.advance(time.Minute)
	f.invite(t, inviter, domain.InviteInput{}) // a new offer, never emailed
	f.svc.DrainMail(5 * time.Second)
	assert.Len(t, f.smtp.Messages(), 2)
}

func TestInviteMail_NamesCannotInjectHeaders(t *testing.T) {
	f := newMailFixture(t, nil)
	f.wsStore.ws[ws1].Name = "Acme\r\nBcc: evil@x.test"
	f.dir.profiles[delegator] = &store.Profile{UserID: "user-del", NodeID: delegator,
		DisplayName: "Khải\r\nTo: evil@x.test\nX-Evil: 1"}
	f.invite(t, delegator, domain.InviteInput{})

	f.svc.DrainMail(5 * time.Second)
	msgs := f.smtp.Messages()
	require.Len(t, msgs, 1)
	head, _, _ := strings.Cut(msgs[0], "\r\n\r\n")
	for _, bad := range []string{"\nBcc:", "\nX-Evil:", "\nTo: evil"} {
		assert.NotContains(t, head, bad)
	}
	m, err := mail.ReadMessage(strings.NewReader(msgs[0]))
	require.NoError(t, err)
	assert.Empty(t, m.Header.Get("Bcc"))
	assert.Empty(t, m.Header.Get("X-Evil"))
	assert.Equal(t, newbieMail, m.Header.Get("To"))
	assert.Len(t, m.Header["To"], 1)
	assert.Equal(t, []string{"<" + newbieMail + ">"}, f.smtp.Rcpts())

	// The subject reads as one line, names intact.
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	require.NoError(t, err)
	assert.NotContains(t, subject, "\n")
	assert.Contains(t, subject, "mời bạn vào Acme Bcc: evil@x.test trên Nexus")

	// A name with markup is text in the HTML, not markup.
	f.wsStore.ws[ws1].Name = `<script>alert(1)</script>`
	f.advance(11 * time.Minute)
	f.invite(t, delegator, domain.InviteInput{})
	got := f.sent(t)
	require.Len(t, got, 2)
	assert.NotContains(t, got[1].html, "<script>")
	assert.Contains(t, got[1].html, "&lt;script&gt;")
}

func TestInviteMail_FailuresAreLoggedMaskedAndNeverWithTheBody(t *testing.T) {
	logs := captureDomainLogs(t)
	f := newMailFixture(t, func(s *testutil.FakeSMTP) { s.RejectData = true })
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(5 * time.Second)
	out := logs.String()
	assert.Contains(t, out, "invitation email not sent")
	assert.Contains(t, out, "mo***@novapay.vn")
	assert.NotContains(t, out, newbieMail, "never the full address")
	assert.NotContains(t, out, "app-password-1234")
	assert.NotContains(t, out, "đúng địa chỉ email này", "never the body")
}

func TestInviteMail_ServerRefusalEchoingTheAddressIsRedacted(t *testing.T) {
	logs := captureDomainLogs(t)
	f := newInviteFixture(t)
	rec := &recordingMailer{failWith: fmt.Errorf("smtp: rcpt to: 550 <%s> no such user", newbieMail)}
	f.svc = f.svc.WithInviteMail(rec, appURL)
	f.invite(t, inviter, domain.InviteInput{})
	f.svc.DrainMail(time.Second)
	assert.NotContains(t, logs.String(), newbieMail)
	assert.Contains(t, logs.String(), "no such user")
}

func TestInviteMail_WithoutSMTPRecordsAndSkipsQuietly(t *testing.T) {
	logs := captureDomainLogs(t)
	f := newInviteFixture(t) // no mailer
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: newbieMail}))
	assert.Equal(t, 1, f.inv.count())
	assert.Contains(t, logs.String(), "invitation email skipped: SMTP is not configured")
	assert.NotContains(t, logs.String(), newbieMail)
	assert.Contains(t, logs.String(), "level=INFO")
	assert.NotContains(t, logs.String(), "level=WARN")
	assert.NotContains(t, logs.String(), "level=ERROR")
}

func TestInviteMail_NothingIsSentWhenTheInviteIsRefused(t *testing.T) {
	f := newMailFixture(t, nil)
	assert.ErrorIs(t, f.svc.InviteByEmail(ctx(), member, ws1, domain.InviteInput{Email: newbieMail}), domain.ErrAccessDenied)
	assert.Error(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "not an address"}))
	f.svc = f.svc.WithInviteLimit(1, time.Hour)
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "a@novapay.vn"}))
	assert.ErrorIs(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "b@novapay.vn"}), domain.ErrRateLimited)
	got := f.sent(t)
	require.Len(t, got, 1)
	assert.Equal(t, "a@novapay.vn", got[0].to)
}

// recordingMailer is an InviteMailer that keeps what it was asked to send.
type recordingMailer struct {
	mu       sync.Mutex
	sent     []mailer.Message
	tries    int
	fail     func() bool
	failWith error
}

func (r *recordingMailer) Send(_ context.Context, m mailer.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tries++
	if r.failWith != nil {
		return r.failWith
	}
	if r.fail != nil && r.fail() {
		return fmt.Errorf("smtp: down")
	}
	r.sent = append(r.sent, m)
	return nil
}

func (r *recordingMailer) attempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.tries
}

func (r *recordingMailer) messages() []mailer.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]mailer.Message(nil), r.sent...)
}
