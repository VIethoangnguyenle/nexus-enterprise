package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/domain"
)

// spyJoiner records who the workspace asked to be put into #general.
type spyJoiner struct {
	joined []string // workspace/user/node
	err    error
}

func (s *spyJoiner) JoinWorkspaceChannel(_ context.Context, workspaceID, userID, userNodeID string) error {
	s.joined = append(s.joined, workspaceID+"/"+userID+"/"+userNodeID)
	return s.err
}

// Accepting an invitation admits the person to the workspace and then to its
// #general: the admission writes the Members edge, which does not by itself open the
// workspace-wide channel.
func TestAcceptInvitation_PutsTheNewMemberInTheWorkspaceChannel(t *testing.T) {
	f := newInviteFixture(t)
	joiner := &spyJoiner{}
	f.svc = f.svc.WithChannelJoiner(joiner)
	id := f.invite(t, inviter, domain.InviteInput{})

	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)

	require.NoError(t, err)
	assert.Equal(t, []string{ws1 + "/" + newbieUser + "/" + newbie}, joiner.joined)
}

// When the admission itself fails nothing is joined; when only the channel join
// fails the person is still a member and the invitation stays accepted.
func TestAcceptInvitation_ChannelJoinFollowsAdmissionAndNeverBlocksIt(t *testing.T) {
	f := newInviteFixture(t)
	joiner := &spyJoiner{}
	f.svc = f.svc.WithChannelJoiner(joiner)
	f.dir.ensure = errors.New("tenant_users down")
	id := f.invite(t, inviter, domain.InviteInput{})

	_, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)
	require.Error(t, err)
	assert.Empty(t, joiner.joined, "a refused admission puts nobody in #general")

	f = newInviteFixture(t)
	joiner = &spyJoiner{err: errors.New("messaging down")}
	f.svc = f.svc.WithChannelJoiner(joiner)
	id = f.invite(t, inviter, domain.InviteInput{})

	res, err := f.svc.AcceptInvitation(ctx(), newbieUser, newbie, id)

	require.NoError(t, err, "the person is a member whether or not #general could be joined")
	assert.Equal(t, ws1, res.WorkspaceID)
	assert.Equal(t, []string{"CreateAssignment " + newbie + "->" + members1}, f.write.mutations)
}
