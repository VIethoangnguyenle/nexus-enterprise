package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
)

// A group made by a user is private, whatever type the request names. The one
// workspace-wide channel is made by the auth service alone (see
// workspace_channel_test.go); a user-scoped call must not be able to produce a
// second one, nor any type the table does not allow.
func TestCreateChannel_UserScopedAlwaysStoresPrivate(t *testing.T) {
	for _, sent := range []string{"workspace", "group", "private", "", "dm", "not-a-channel-type"} {
		t.Run(fmt.Sprintf("sent %q", sent), func(t *testing.T) {
			f := newFixture(t)
			channelsOA := f.withChannelsOA()
			f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
			in := f.createInput(fmt.Sprintf("private-%d", time.Now().UnixNano()))
			in.ChannelType = sent

			ch, err := f.svc.CreateChannel(context.Background(), in)

			require.NoError(t, err)
			assert.Equal(t, "private", ch.ChannelType)
			var stored string
			require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT channel_type FROM channels WHERE id = $1`, ch.Id).Scan(&stored))
			assert.Equal(t, "private", stored)
		})
	}
}
