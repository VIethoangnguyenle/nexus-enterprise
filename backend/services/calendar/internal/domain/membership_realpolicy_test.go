//go:build realpolicy

package domain

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/calendar/internal/store"
	"ngac-platform/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

// Workspace membership in the calendar is "under the workspace's Members UA", not
// "the policy class is among my ancestors": an event's guest UA and a group's UA
// sit under the class too, and a person who is only in one of them must not get
// the workspace's calendar.
func TestRealPolicy_CalendarMembershipIsTheMembersUA(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	g := testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)

	guests := testutil.NewNode(t, write, ngac.CalendarEventGuestsUAName("rp-ev"), ngac.TypeUA, g.PC)
	group := testutil.NewNode(t, write, ngac.ChannelMembersUAName("rp-ch"), ngac.TypeUA, g.PC)

	st := newMemStore(&store.Workspace{ID: g.WorkspaceID, Name: "Acme", Type: "organization", PCNodeID: g.PC})
	person := func(name string, parents ...string) string {
		node := testutil.NewUser(t, write, parents...)
		st.people[node] = &store.Person{NodeID: node, DisplayName: name}
		return node
	}
	owner := person("Owner", g.Owners)
	member := person("Member", g.Members)
	viaGuests := person("Guest", guests)
	viaGroup := person("Grouped", group)
	foreign := person("Foreign", other.Members)

	svc := NewService(st, read, write, nil)
	ctxOf := func(node string) context.Context {
		return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "user-" + node, NGACNodeID: node, TenantID: g.WorkspaceID})
	}

	// Allow: owners and members, on the workspace calendar.
	for name, node := range map[string]string{"owner": owner, "member": member} {
		cals, err := svc.ListCalendars(ctxOf(node), g.WorkspaceID)
		require.NoError(t, err, name)
		assert.NotEmpty(t, cals, name)
	}

	// Deny: reaching the policy class some other way is not membership.
	for _, node := range map[string]string{"event guest only": viaGuests, "group member only": viaGroup, "another workspace's member": foreign} {
		_, err := svc.ListCalendars(ctxOf(node), g.WorkspaceID)
		requireKind(t, err, ErrAccessDenied)
	}

	// A named person must be a member too: the member can be named, the guest-list-only person cannot.
	_, err := svc.requirePerson(ctxOf(member), st.workspace, member)
	require.NoError(t, err)
	_, err = svc.requirePerson(ctxOf(member), st.workspace, viaGuests)
	requireKind(t, err, ErrAccessDenied)
	_, err = svc.requirePerson(ctxOf(member), st.workspace, viaGroup)
	requireKind(t, err, ErrAccessDenied)
}
