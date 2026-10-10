package domain_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
)

// fakeInvites keeps invitations in memory with the rules the table enforces:
// one pending offer per address per workspace, answers only while pending and
// unexpired, and only for the address it was made to.
type fakeInvites struct {
	mu     sync.Mutex
	byID   map[string]*store.Invitation
	emails map[string]string // user ID -> verified address on the account
	byNode map[string]string // user node -> verified address
	seq    int
	gets   int
}

func newFakeInvites() *fakeInvites {
	return &fakeInvites{byID: map[string]*store.Invitation{}, emails: map[string]string{}, byNode: map[string]string{}}
}

func (f *fakeInvites) UpsertInvitation(_ context.Context, inv *store.Invitation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.byID {
		if e.WorkspaceID == inv.WorkspaceID && e.Email == inv.Email && e.Status == store.InvitationPending {
			e.RoleID, e.DepartmentID, e.InvitedBy, e.ExpiresAt = inv.RoleID, inv.DepartmentID, inv.InvitedBy, inv.ExpiresAt
			return nil
		}
	}
	f.seq++
	c := *inv
	c.ID = fmt.Sprintf("inv-%d", f.seq)
	c.Status = store.InvitationPending
	f.byID[c.ID] = &c
	return nil
}

func (f *fakeInvites) GetInvitation(_ context.Context, id string) (*store.Invitation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	if i, ok := f.byID[id]; ok {
		c := *i
		return &c, nil
	}
	return nil, nil
}

func (f *fakeInvites) list(match func(*store.Invitation) bool, now time.Time) []*store.Invitation {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*store.Invitation
	for _, i := range f.byID {
		if i.Status == store.InvitationPending && i.ExpiresAt.After(now) && match(i) {
			c := *i
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}

func (f *fakeInvites) ListPendingForWorkspace(_ context.Context, ws string, now time.Time) ([]*store.Invitation, error) {
	return f.list(func(i *store.Invitation) bool { return i.WorkspaceID == ws }, now), nil
}

func (f *fakeInvites) ListPendingForEmail(_ context.Context, email string, now time.Time) ([]*store.Invitation, error) {
	return f.list(func(i *store.Invitation) bool { return i.Email == email }, now), nil
}

func (f *fakeInvites) RespondInvitation(_ context.Context, id, email, status string, now time.Time) (*store.Invitation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i, ok := f.byID[id]
	if !ok || i.Email != email || i.Status != store.InvitationPending || !i.ExpiresAt.After(now) {
		return nil, nil
	}
	i.Status = status
	c := *i
	return &c, nil
}

func (f *fakeInvites) ReopenInvitation(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i, ok := f.byID[id]; ok && i.Status == store.InvitationAccepted {
		i.Status = store.InvitationPending
	}
	return nil
}

func (f *fakeInvites) RevokeInvitation(_ context.Context, ws, id string, _ time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if i, ok := f.byID[id]; ok && i.WorkspaceID == ws && i.Status == store.InvitationPending {
		i.Status = store.InvitationRevoked
		return true, nil
	}
	return false, nil
}

func (f *fakeInvites) UserEmail(_ context.Context, userID string) (string, error) {
	return f.emails[userID], nil
}

func (f *fakeInvites) VerifiedEmailByNode(_ context.Context, node string) (string, error) {
	return f.byNode[node], nil
}

func (f *fakeInvites) RevokePendingForEmail(_ context.Context, ws, email string, _ time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, i := range f.byID {
		if i.WorkspaceID == ws && i.Email == email && i.Status == store.InvitationPending {
			i.Status = store.InvitationRevoked
			n++
		}
	}
	return n, nil
}

func (f *fakeInvites) status(email string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, i := range f.byID {
		if i.Email == email {
			out = append(out, i.Status)
		}
	}
	sort.Strings(out)
	return out
}

func (f *fakeInvites) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byID)
}

const (
	newbieUser = "user-newbie"
	newbieMail = "moi@novapay.vn"
)

type inviteFixture struct {
	*adminFixture
	inv *fakeInvites
	now *time.Time
}

func newInviteFixture(t *testing.T) *inviteFixture {
	t.Helper()
	f := newAdminFixture(t)
	inv := newFakeInvites()
	inv.emails[newbieUser] = newbieMail
	inv.emails["user-lan"] = "lan@novapay.vn"
	clock := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	f.svc = f.svc.WithInvitations(inv).WithClock(func() time.Time { return clock })
	// delegator may invite (and, as a manager, assign what it holds); inviter may only invite.
	f.read.grants[grant{delegator, mgmt1, ngac.OpInvite}] = true
	f.read.nodes[roleRead] = &policypb.NGACNode{Id: roleRead, Name: ngac.RoleUAName("r-read"), NodeType: ngac.TypeUA,
		Properties: map[string]string{ngac.PropType: ngac.PropTypeRole, ngac.PropDisplayName: "Người đọc"}}
	return &inviteFixture{adminFixture: f, inv: inv, now: &clock}
}

func (f *inviteFixture) advance(d time.Duration) {
	*f.now = f.now.Add(d)
	n := *f.now
	f.svc = f.svc.WithClock(func() time.Time { return n })
}

// inviteAndFind invites newbieMail as inviter and returns the invitation's ID.
func (f *inviteFixture) invite(t *testing.T, by string, in domain.InviteInput) string {
	t.Helper()
	if in.Email == "" {
		in.Email = newbieMail
	}
	require.NoError(t, f.svc.InviteByEmail(ctx(), by, ws1, in))
	f.inv.mu.Lock()
	defer f.inv.mu.Unlock()
	for id, i := range f.inv.byID {
		if i.Email == in.Email && i.Status == store.InvitationPending {
			return id
		}
	}
	t.Fatal("no pending invitation was recorded")
	return ""
}

// ---------------------------------------------------------------------------
// Inviting: a standing offer, never an assignment, never an oracle
// ---------------------------------------------------------------------------

func TestInviteByEmail_NeedsInviteAndRecordsNothingOtherwise(t *testing.T) {
	for _, caller := range []string{member, manager, outsider, otherOwner, ""} {
		f := newInviteFixture(t)
		err := f.svc.InviteByEmail(ctx(), caller, ws1, domain.InviteInput{Email: newbieMail})
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
		assert.Zero(t, f.inv.count(), caller)
		assert.Empty(t, f.write.mutations, caller)
	}
}

func TestInviteByEmail_NeverAssignsAndNeverConsultsAccounts(t *testing.T) {
	f := newInviteFixture(t)
	f.invite(t, inviter, domain.InviteInput{})
	assert.Empty(t, f.write.mutations, "an invitation changes nothing in the graph")
	assert.Empty(t, f.dir.listed, "and lists nobody in the workspace")
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))
}

// Whatever the address is (an account that exists, one that does not, a person
// already in the workspace) the answer and what is stored are the same.
func TestInviteByEmail_AnswersTheSameForEveryWellFormedAddress(t *testing.T) {
	f := newInviteFixture(t)
	for _, addr := range []string{newbieMail, "lan@novapay.vn" /* a member */, "nobody.at.all@example.org"} {
		require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: addr}), addr)
	}
	assert.Equal(t, 3, f.inv.count())
	assert.Empty(t, f.write.mutations)
}

func TestInviteByEmail_RejectsWhatIsNotAnAddress(t *testing.T) {
	for _, in := range []string{"", "   ", "no-at-sign", "a@", "@b.vn", "a b@c.vn"} {
		f := newInviteFixture(t)
		assert.ErrorIs(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: in}), domain.ErrInvalidInput, in)
		assert.Zero(t, f.inv.count(), in)
	}
}

func TestInviteByEmail_AgainRefreshesTheOneOpenOffer(t *testing.T) {
	f := newInviteFixture(t)
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "  Moi@NovaPay.vn "}))
	first := f.invite(t, inviter, domain.InviteInput{})
	f.advance(24 * time.Hour)
	second := f.invite(t, inviter, domain.InviteInput{Email: "moi@novapay.vn"})
	assert.Equal(t, first, second)
	assert.Equal(t, 1, f.inv.count())
	assert.Equal(t, f.now.Add(domain.InvitationTTL), f.inv.byID[first].ExpiresAt, "the clock restarts")
}

func TestInviteByEmail_RoleAndDepartmentAreCheckedForTheInviter(t *testing.T) {
	// inviter may invite but not manage: attaching a role is refused, and nothing is stored.
	f := newInviteFixture(t)
	err := f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: newbieMail, RoleID: roleRead})
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	err = f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: newbieMail, DepartmentID: "dept-root-1"})
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Zero(t, f.inv.count())

	// delegator manages and holds read on Documents: may attach roleRead, not roleEdit (write).
	require.NoError(t, f.svc.InviteByEmail(ctx(), delegator, ws1, domain.InviteInput{Email: newbieMail, RoleID: roleRead}))
	err = f.svc.InviteByEmail(ctx(), delegator, ws1, domain.InviteInput{Email: "other@novapay.vn", RoleID: roleEdit})
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a role that confers more than the inviter holds")
	assert.Equal(t, 1, f.inv.count())
}

func TestInviteByEmail_RoleAndDepartmentMustBelongToTheWorkspace(t *testing.T) {
	f := newInviteFixture(t)
	for name, in := range map[string]domain.InviteInput{
		"foreign role":       {Email: newbieMail, RoleID: owners2},
		"the Owners UA":      {Email: newbieMail, RoleID: owners1},
		"foreign department": {Email: newbieMail, DepartmentID: "dept-ws2"},
		"unknown role":       {Email: newbieMail, RoleID: "nope"},
	} {
		assert.ErrorIs(t, f.svc.InviteByEmail(ctx(), owner, ws1, in), domain.ErrNotFound, name)
	}
	assert.Zero(t, f.inv.count())
}

func TestInviteByEmail_IsLimitedPerCaller(t *testing.T) {
	f := newInviteFixture(t)
	f.svc = f.svc.WithInviteLimit(3, time.Hour)
	for i := 0; i < 3; i++ {
		require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: fmt.Sprintf("p%d@novapay.vn", i)}))
	}
	err := f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "p9@novapay.vn"})
	assert.ErrorIs(t, err, domain.ErrRateLimited)
	assert.Equal(t, 3, f.inv.count())

	// Another inviter has a budget of their own, and a caller who may not invite spends none.
	require.NoError(t, f.svc.InviteByEmail(ctx(), owner, ws1, domain.InviteInput{Email: "p9@novapay.vn"}))
	for i := 0; i < 5; i++ {
		assert.ErrorIs(t, f.svc.InviteByEmail(ctx(), member, ws1, domain.InviteInput{Email: "x@novapay.vn"}), domain.ErrAccessDenied)
	}

	// The window ends.
	f.advance(61 * time.Minute)
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "p10@novapay.vn"}))
}

// ---------------------------------------------------------------------------
// The workspace's side: who is waiting, withdrawing
// ---------------------------------------------------------------------------

func TestListInvitations_NeedsInviteAndNamesPeopleNotIDs(t *testing.T) {
	for _, caller := range []string{member, manager, outsider, otherOwner, ""} {
		f := newInviteFixture(t)
		_, err := f.svc.ListInvitations(ctx(), caller, ws1)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, caller)
	}
	f := newInviteFixture(t)
	f.dir.profiles[delegator] = &store.Profile{UserID: "user-del", NodeID: delegator, DisplayName: "Đỗ Văn Khải"}
	f.invite(t, delegator, domain.InviteInput{RoleID: roleRead, DepartmentID: "dept-root-1"})
	list, err := f.svc.ListInvitations(ctx(), inviter, ws1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, newbieMail, list[0].Email)
	assert.Equal(t, "Đỗ Văn Khải", list[0].InviterName)
	assert.Equal(t, "Người đọc", list[0].RoleName)
	assert.Equal(t, "Root", list[0].DepartmentName)
}

func TestListInvitations_LeavesOutExpiredAndAnswered(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	f.invite(t, inviter, domain.InviteInput{Email: "b@novapay.vn"})
	require.NoError(t, f.svc.RevokeInvitation(ctx(), inviter, ws1, id))
	list, _ := f.svc.ListInvitations(ctx(), inviter, ws1)
	assert.Len(t, list, 1)
	f.advance(domain.InvitationTTL + time.Minute)
	list, _ = f.svc.ListInvitations(ctx(), inviter, ws1)
	assert.Empty(t, list)
}

func TestRevokeInvitation_NeedsInviteAndStaysInsideTheWorkspace(t *testing.T) {
	for _, caller := range []string{member, manager, outsider, otherOwner, ""} {
		f := newInviteFixture(t)
		id := f.invite(t, inviter, domain.InviteInput{})
		assert.ErrorIs(t, f.svc.RevokeInvitation(ctx(), caller, ws1, id), domain.ErrAccessDenied, caller)
		assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail), caller)
	}
	// An owner of ws-2 reaching for ws-1's invitation through ws-2's own route.
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	assert.ErrorIs(t, f.svc.RevokeInvitation(ctx(), otherOwner, ws2, id), domain.ErrNotFound)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))
	assert.ErrorIs(t, f.svc.RevokeInvitation(ctx(), owner, ws1, "inv-none"), domain.ErrNotFound)
	require.NoError(t, f.svc.RevokeInvitation(ctx(), inviter, ws1, id))
	assert.Equal(t, []string{store.InvitationRevoked}, f.inv.status(newbieMail))
}

// ---------------------------------------------------------------------------
// The invitee's side
// ---------------------------------------------------------------------------

func TestListMyInvitations_OnlyOpenOffersToMyAddress(t *testing.T) {
	f := newInviteFixture(t)
	f.invite(t, inviter, domain.InviteInput{})
	f.invite(t, inviter, domain.InviteInput{Email: "someone.else@novapay.vn"})
	list, err := f.svc.ListMyInvitations(ctx(), newbieUser)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Acme", list[0].WorkspaceName)

	// A person with no address on the account sees nothing; a stranger's user ID is no one.
	for _, u := range []string{"user-no-mail", "user-nobody"} {
		got, err := f.svc.ListMyInvitations(ctx(), u)
		require.NoError(t, err)
		assert.Empty(t, got, u)
	}
	_, err = f.svc.ListMyInvitations(ctx(), "")
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	f.advance(domain.InvitationTTL + time.Minute)
	got, _ := f.svc.ListMyInvitations(ctx(), newbieUser)
	assert.Empty(t, got, "expired offers are not listed")
}

func TestAcceptInvitation_AddsTheMemberThroughMembersAndListsThem(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	res, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err)
	assert.Equal(t, ws1, res.WorkspaceID)
	assert.Equal(t, "Acme", res.WorkspaceName)
	assert.False(t, res.RoleApplied)
	assert.Equal(t, []string{"CreateAssignment " + newbie + "->" + members1}, f.write.mutations)
	assert.Equal(t, []string{ws1 + "/" + newbie}, f.dir.listed)
	assert.Equal(t, []string{store.InvitationAccepted}, f.inv.status(newbieMail))
}

func TestAcceptInvitation_RejectsWhatIsNotYoursOrNotOpen(t *testing.T) {
	// Someone else's invitation, an unknown one, and a person with no address: all "not found".
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	for name, user := range map[string]string{"another user": "user-lan", "a user with no address": "user-no-mail", "no such user": "user-nobody"} {
		_, err := f.svc.AcceptInvitation(ctx(), user, leaver, id)
		assert.ErrorIs(t, err, domain.ErrNotFound, name)
	}
	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, "inv-none")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = f.svc.AcceptInvitation(ctx(), "", newbie, id)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, "", id)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations, "nothing was written for any of those")
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))
}

func TestAcceptInvitation_ExpiredDeclinedRevokedAndRepeated(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	f.advance(domain.InvitationTTL + time.Minute)
	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "expired")
	assert.Empty(t, f.write.mutations)

	f = newInviteFixture(t)
	id = f.invite(t, inviter, domain.InviteInput{})
	require.NoError(t, f.svc.DeclineInvitation(ctx(), newbieUser, id))
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists, "declined")

	f = newInviteFixture(t)
	id = f.invite(t, inviter, domain.InviteInput{})
	require.NoError(t, f.svc.RevokeInvitation(ctx(), inviter, ws1, id))
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists, "revoked")

	f = newInviteFixture(t)
	id = f.invite(t, inviter, domain.InviteInput{})
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err)
	n := len(f.write.mutations)
	_, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists, "twice")
	assert.Len(t, f.write.mutations, n, "the second accept adds nothing")
}

func TestAcceptInvitation_TwoAtOnceAddOnce(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func() { defer wg.Done(); _, errs[i] = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id) }()
	}
	wg.Wait()
	ok := 0
	for _, e := range errs {
		if e == nil {
			ok++
		}
	}
	assert.Equal(t, 1, ok)
	assert.Len(t, f.write.mutations, 1)
}

func TestAcceptInvitation_RefusedWhenTheInviterCanNoLongerInvite(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	delete(f.read.grants, grant{inviter, mgmt1, ngac.OpInvite})
	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, f.write.mutations)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail), "the offer is not used up")
}

func TestAcceptInvitation_RoleAndDepartmentAreRecheckedAgainstTheInviterNow(t *testing.T) {
	in := domain.InviteInput{RoleID: roleRead, DepartmentID: "dept-root-1"}

	// Still allowed: both are given.
	f := newInviteFixture(t)
	id := f.invite(t, delegator, in)
	res, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err)
	assert.True(t, res.RoleApplied)
	assert.True(t, res.DepartmentApplied)
	assert.Contains(t, f.write.mutations, "CreateAssignment "+newbie+"->"+roleRead)
	assert.Contains(t, f.write.mutations, "CreateAssignment "+newbie+"->ua-dept-root-1")

	// The inviter has lost read on Documents since: the role confers more than they hold now.
	f = newInviteFixture(t)
	id = f.invite(t, delegator, in)
	delete(f.read.grants, grant{delegator, docs1, ngac.OpRead})
	res, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err, "the person still joins")
	assert.False(t, res.RoleApplied)
	assert.NotContains(t, f.write.mutations, "CreateAssignment "+newbie+"->"+roleRead)

	// The inviter has lost manage: neither extra is given.
	f = newInviteFixture(t)
	id = f.invite(t, delegator, in)
	delete(f.read.grants, grant{delegator, mgmt1, ngac.OpManage})
	res, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err)
	assert.False(t, res.RoleApplied)
	assert.False(t, res.DepartmentApplied)
	assert.Equal(t, []string{"CreateAssignment " + newbie + "->" + members1}, f.write.mutations)

	// The role was deleted in between.
	f = newInviteFixture(t)
	id = f.invite(t, delegator, in)
	kept := []*policypb.NGACNode{}
	for _, n := range f.read.descendants[pc1] {
		if n.Id != roleRead {
			kept = append(kept, n)
		}
	}
	f.read.descendants[pc1] = kept
	res, err = f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.NoError(t, err)
	assert.False(t, res.RoleApplied)
}

func TestAcceptInvitation_AlreadyAMemberJustAnswers(t *testing.T) {
	f := newInviteFixture(t)
	f.inv.emails["user-lan"] = "lan@novapay.vn"
	id := f.invite(t, inviter, domain.InviteInput{Email: "lan@novapay.vn"})
	_, err := f.svc.AcceptInvitation(ctx(), "user-lan", target, id)
	require.NoError(t, err)
	assert.Empty(t, f.write.mutations, "target already reaches the workspace")
	assert.Empty(t, f.dir.listed)
}

func TestAcceptInvitation_ListingFailureUndoesAndReopens(t *testing.T) {
	f := newInviteFixture(t)
	f.dir.ensure = fmt.Errorf("db down")
	id := f.invite(t, inviter, domain.InviteInput{})
	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.Error(t, err)
	assert.Equal(t, []string{
		"CreateAssignment " + newbie + "->" + members1,
		"RemoveAssignment " + newbie + "->" + members1,
	}, f.write.mutations)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail), "it can be accepted again")
}

func TestDeclineInvitation_OnlyMineAndOnlyWhileOpen(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	assert.ErrorIs(t, f.svc.DeclineInvitation(ctx(), "user-lan", id), domain.ErrNotFound)
	assert.ErrorIs(t, f.svc.DeclineInvitation(ctx(), "", id), domain.ErrAccessDenied)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))
	require.NoError(t, f.svc.DeclineInvitation(ctx(), newbieUser, id))
	assert.Equal(t, []string{store.InvitationDeclined}, f.inv.status(newbieMail))
	assert.ErrorIs(t, f.svc.DeclineInvitation(ctx(), newbieUser, id), domain.ErrAlreadyExists)
	assert.Empty(t, f.write.mutations)
}

// ---------------------------------------------------------------------------
// Removing a person withdraws what was still offered to them
// ---------------------------------------------------------------------------

func TestRemoveMember_RevokesPendingInvitationsForTheirVerifiedAddress(t *testing.T) {
	f := newInviteFixture(t)
	f.inv.byNode[leaver] = "leaver@novapay.vn"
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "leaver@novapay.vn"}))
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "someone.else@novapay.vn"}))

	require.NoError(t, f.svc.RemoveMember(ctx(), inviter, ws1, leaver))

	assert.Equal(t, []string{store.InvitationRevoked}, f.inv.status("leaver@novapay.vn"), "they cannot walk back in on an old offer")
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status("someone.else@novapay.vn"), "nobody else's offer is touched")
}

func TestRemoveMember_ADeniedRemovalRevokesNothing(t *testing.T) {
	f := newInviteFixture(t)
	f.inv.byNode[leaver] = "leaver@novapay.vn"
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "leaver@novapay.vn"}))
	require.ErrorIs(t, f.svc.RemoveMember(ctx(), member, ws1, leaver), domain.ErrAccessDenied)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status("leaver@novapay.vn"))
}

func TestRemoveMember_AnUnverifiedAddressHasNoOffersToRevoke(t *testing.T) {
	f := newInviteFixture(t)
	require.NoError(t, f.svc.InviteByEmail(ctx(), inviter, ws1, domain.InviteInput{Email: "leaver@novapay.vn"}))
	require.NoError(t, f.svc.RemoveMember(ctx(), inviter, ws1, leaver))
	// leaver's account has no proved address, so nobody could have accepted it as them; it stays for its real owner.
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status("leaver@novapay.vn"))
}

// An address typed at signup is not proved, so an account that merely claims an
// invited address is shown nothing and can accept nothing.
func TestInvitations_ANonVerifiedAccountSeesAndAcceptsNothing(t *testing.T) {
	f := newInviteFixture(t)
	id := f.invite(t, inviter, domain.InviteInput{})
	// "user-claimant" signed up with newbieMail but never proved it: the store reports no address for it.
	got, err := f.svc.ListMyInvitations(ctx(), "user-claimant")
	require.NoError(t, err)
	assert.Empty(t, got)
	_, err = f.svc.AcceptInvitation(ctx(), "user-claimant", "u-claimant", id)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.ErrorIs(t, f.svc.DeclineInvitation(ctx(), "user-claimant", id), domain.ErrNotFound)
	assert.Empty(t, f.write.mutations)
	assert.Equal(t, []string{store.InvitationPending}, f.inv.status(newbieMail))
}
