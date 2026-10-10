package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/workspace/internal/domain"
)

// Every invitation is announced to messaging, whatever the address: whether an
// account exists is for messaging to decide, so the inviter's answer cannot
// reveal it. The announcement names the invitation, never the address.
func TestInviteByEmail_AnnouncesEveryInvitationAlike(t *testing.T) {
	f := newInviteFixture(t)
	p := withProbe(f.adminFixture)

	for _, addr := range []string{newbieMail, "lan@novapay.vn", "nobody.at.all@example.org"} {
		require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: addr}), addr)
	}

	got := kinds(p)
	require.Len(t, got, 3, "one announcement per invitation, for existing and unknown addresses alike")
	for _, a := range got {
		assert.Equal(t, realtime.DomainWorkspace, a.domain)
		assert.Equal(t, realtime.KindInvitationCreated, a.kind)
		require.Len(t, a.ids, 1)
		assert.NotContains(t, a.ids[0], "@", "the address is not in the event")
	}
}

func TestInviteByEmail_RefusedAnnouncesNothing(t *testing.T) {
	f := newInviteFixture(t)
	p := withProbe(f.adminFixture)
	assert.Error(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "not an address"}))
	assert.ErrorIs(t, f.svc.InviteByEmail(ctx(), member, ws1, domain.InviteInput{Email: newbieMail}), domain.ErrAccessDenied)
	assert.Empty(t, p.Events())
}
