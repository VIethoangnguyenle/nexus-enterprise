package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/messaging/internal/domain"
)

func (f *fixture) provisionInput(name string) domain.ProvisionWorkspaceChannelInput {
	return domain.ProvisionWorkspaceChannelInput{
		WorkspaceID: f.wsID, Name: name, CreatorUserID: f.userID, CreatorNodeID: userNode,
	}
}

func (f *fixture) workspaceChannelCount(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channels WHERE workspace_id = $1 AND channel_type = 'workspace'`, f.wsID).Scan(&n))
	return n
}

// The workspace-wide channel is made once per workspace, by its creator, who must
// hold create_channel like anyone starting a group. The refusal and the success
// run on the same fixture.
func TestProvisionWorkspaceChannel_MakesTheWorkspaceWideChannelOnce(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	name := fmt.Sprintf("wide-%d", time.Now().UnixNano())

	_, err := f.svc.ProvisionWorkspaceChannel(context.Background(), f.provisionInput(name))
	require.ErrorIs(t, err, domain.ErrAccessDenied, "the creator needs create_channel on Channels, as anyone starting a group")
	assert.Equal(t, 0, f.workspaceChannelCount(t))
	assert.Equal(t, 0, f.write.graphWrites(), "a refused provisioning writes nothing to the graph")

	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	ch, err := f.svc.ProvisionWorkspaceChannel(context.Background(), f.provisionInput(name))
	require.NoError(t, err)
	assert.Equal(t, "workspace", ch.ChannelType)
	assert.Equal(t, 1, f.workspaceChannelCount(t))
	assert.Equal(t, grpcauth.Caller{UserID: f.userID, NGACNodeID: userNode, TenantID: f.wsID}, f.read.callers[userNode],
		"the policy is asked on behalf of the creator, not of the service")

	writes := f.write.graphWrites()
	_, err = f.svc.ProvisionWorkspaceChannel(context.Background(), f.provisionInput(name+"-2"))
	require.ErrorIs(t, err, domain.ErrAlreadyExists, "a workspace has at most one workspace-wide channel")
	assert.Equal(t, 1, f.workspaceChannelCount(t))
	assert.Equal(t, writes, f.write.graphWrites(), "the refusal comes before any graph write")
}

func TestProvisionWorkspaceChannel_RejectsIncompleteInput(t *testing.T) {
	f := newFixture(t)
	f.read.allow[grant{userNode, f.withChannelsOA(), ngac.OpCreateChannel}] = true
	for name, mutate := range map[string]func(*domain.ProvisionWorkspaceChannelInput){
		"no workspace": func(in *domain.ProvisionWorkspaceChannelInput) { in.WorkspaceID = "" },
		"no name":      func(in *domain.ProvisionWorkspaceChannelInput) { in.Name = "" },
		"no creator":   func(in *domain.ProvisionWorkspaceChannelInput) { in.CreatorUserID = "" },
		"no node":      func(in *domain.ProvisionWorkspaceChannelInput) { in.CreatorNodeID = "" },
	} {
		in := f.provisionInput("wide-invalid")
		mutate(&in)
		_, err := f.svc.ProvisionWorkspaceChannel(context.Background(), in)
		assert.ErrorIsf(t, err, domain.ErrInvalidInput, name)
	}
	assert.Equal(t, 0, f.workspaceChannelCount(t))
	assert.Equal(t, 0, f.write.graphWrites())
}
