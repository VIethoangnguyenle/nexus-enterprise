package domain_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/domain"
)

type recordingNotifier struct {
	mu  sync.Mutex
	got []domain.InvitationNotice
}

func (r *recordingNotifier) InvitationOffered(n domain.InvitationNotice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got = append(r.got, n)
}

func (r *recordingNotifier) notices(f *inviteFixture) []domain.InvitationNotice {
	f.svc.DrainMail(5 * time.Second)
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.InvitationNotice(nil), r.got...)
}

func TestInviteNotice_TellsTheAccountBehindAVerifiedAddress(t *testing.T) {
	f := newInviteFixture(t)
	n := &recordingNotifier{}
	f.svc = f.svc.WithInvitationNotifier(n)

	id := f.invite(t, inviter, domain.InviteInput{})

	got := n.notices(f)
	require.Len(t, got, 1)
	assert.Equal(t, newbieUser, got[0].UserID)
	assert.Equal(t, id, got[0].InvitationID)
	assert.NotEmpty(t, got[0].WorkspaceName)
}

func TestInviteNotice_NobodyIsToldForAnUnknownOrUnverifiedAddress(t *testing.T) {
	f := newInviteFixture(t)
	n := &recordingNotifier{}
	f.svc = f.svc.WithInvitationNotifier(n)
	// An address typed at signup but never proved is not in `emails` (the fake
	// keeps proved addresses only), the same as an address with no account.
	for _, addr := range []string{"nobody.at.all@example.org", "typed.not.proved@novapay.vn"} {
		require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: addr}), addr)
	}
	assert.Empty(t, n.notices(f))
	assert.Equal(t, 2, f.inv.count(), "the offers are stored all the same")
}

func TestInviteNotice_OnlyTheAddressedAccountIsNamed(t *testing.T) {
	f := newInviteFixture(t)
	n := &recordingNotifier{}
	f.svc = f.svc.WithInvitationNotifier(n)
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "lan@novapay.vn"}))
	got := n.notices(f)
	require.Len(t, got, 1)
	assert.Equal(t, "user-lan", got[0].UserID)
	assert.NotEqual(t, newbieUser, got[0].UserID)
}

func TestInviteNotice_AnInviteThatIsRefusedNotifiesNobody(t *testing.T) {
	f := newInviteFixture(t)
	n := &recordingNotifier{}
	f.svc = f.svc.WithInvitationNotifier(n)
	for _, caller := range []string{member, outsider, ""} {
		assert.Error(t, f.svc.InviteByEmail(ctx(), caller, ws1, domain.InviteInput{Email: newbieMail}), caller)
	}
	assert.Empty(t, n.notices(f))
}

func TestInviteNotice_InvitingAgainDoesNotNotifyTwice(t *testing.T) {
	f := newInviteFixture(t)
	n := &recordingNotifier{}
	f.svc = f.svc.WithInvitationNotifier(n)
	f.invite(t, inviter, domain.InviteInput{})
	f.invite(t, inviter, domain.InviteInput{})
	assert.Len(t, n.notices(f), 1, "a refreshed offer is the same offer")
}
