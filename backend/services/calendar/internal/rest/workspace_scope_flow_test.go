package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
)

// joinSecondWorkspace makes Bob a member of a second workspace too, with the same
// user node, and returns its id.
func (f *flow) joinSecondWorkspace(who string) string {
	f.t.Helper()
	ws := "cal-wsb-" + f.sfx
	pc, members := "pcb-"+f.sfx, "membersb-"+f.sfx
	require.NoError(f.t, f.insertNode(pc, ngac.PCName(ngac.WorkspaceID(ws)), ngac.TypePC))
	require.NoError(f.t, f.insertNode(members, ngac.MembersUAName(ngac.WorkspaceID(ws)), ngac.TypeUA))
	f.w.Seed(pc, ngac.PCName(ngac.WorkspaceID(ws)), ngac.TypePC, nil)
	f.w.Seed(members, ngac.MembersUAName(ngac.WorkspaceID(ws)), ngac.TypeUA, nil)
	owners := "ownersb-" + f.sfx
	require.NoError(f.t, f.insertNode(owners, ngac.OwnersUAName(ngac.WorkspaceID(ws)), ngac.TypeUA))
	f.w.Seed(owners, ngac.OwnersUAName(ngac.WorkspaceID(ws)), ngac.TypeUA, nil)
	f.w.Assign(owners, pc)
	f.w.Assign(members, pc)
	f.w.Assign(f.nodes[who], members)
	exec := func(q string, args ...any) {
		_, err := f.pool.Exec(context.Background(), q, args...)
		require.NoError(f.t, err, q)
	}
	exec(`INSERT INTO workspaces (id, name, owner_id, type, ngac_pc_id) VALUES ($1, 'Second', $2, 'organization', $3)`, ws, f.users[who], pc)
	exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`, ws, f.users[who], f.nodes[who])
	f.t.Cleanup(func() {
		c := context.Background()
		f.pool.Exec(c, `DELETE FROM calendars WHERE workspace_id = $1`, ws)
		f.pool.Exec(c, `DELETE FROM tenant_users WHERE tenant_id = $1`, ws)
		f.pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, ws)
	})
	return ws
}

func TestAGuestOfAnotherWorkspacesEventSeesNothingOfItHere(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	bob := f.nodes["bob"]
	body := strings.Replace(eventBody(cal, ""), `"attendees":[]`, `"attendees":`+quoted(bob), 1)
	body = strings.Replace(body, `"title":"Standup"`, `"title":"Budget sync"`, 1)
	code, ev, raw := f.createEvent("alice", body)
	require.Equal(t, http.StatusCreated, code, string(raw))
	other := f.joinSecondWorkspace("bob")

	window := url.Values{"from": {"2026-11-01T00:00:00+07:00"}, "to": {"2026-12-01T00:00:00+07:00"}}
	get := func(ws, path string, q url.Values) []occJSON {
		code, raw := f.doIn(http.MethodGet, "/api/workspaces/"+ws+"/calendar"+path+"?"+q.Encode(), "bob", ws, "")
		require.Equal(t, http.StatusOK, code, string(raw))
		var out struct {
			Occurrences []occJSON `json:"occurrences"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		return out.Occurrences
	}
	has := func(list []occJSON) bool {
		for _, o := range list {
			if o.EventID == ev.EventID {
				return true
			}
		}
		return false
	}

	// Allow: in the event's own workspace the guest lists and finds it.
	assert.True(t, has(get(f.ws, "/events", window)))
	assert.True(t, has(get(f.ws, "/events/search", url.Values{"q": {"budget"}})))

	// Deny: in the other workspace, 200 and none of it.
	assert.False(t, has(get(other, "/events", window)), "an invitation of another workspace is not listed here")
	assert.False(t, has(get(other, "/events/search", url.Values{"q": {"budget"}})), "nor found")

	// Nor does it make Bob busy there.
	fb := url.Values{"users": {bob}, "from": {"2026-11-02T00:00:00+07:00"}, "to": {"2026-11-03T00:00:00+07:00"}}
	code, raw = f.doIn(http.MethodGet, "/api/workspaces/"+other+"/calendar/freebusy?"+fb.Encode(), "bob", other, "")
	require.Equal(t, http.StatusOK, code, string(raw))
	var busy struct {
		People []struct {
			Busy []json.RawMessage `json:"busy"`
		} `json:"people"`
	}
	require.NoError(t, json.Unmarshal(raw, &busy))
	for _, p := range busy.People {
		assert.Empty(t, p.Busy)
	}
	code, raw = f.doIn(http.MethodGet, "/api/workspaces/"+f.ws+"/calendar/freebusy?"+fb.Encode(), "bob", f.ws, "")
	require.Equal(t, http.StatusOK, code, string(raw))
	require.NoError(t, json.Unmarshal(raw, &busy))
	require.NotEmpty(t, busy.People)
	assert.NotEmpty(t, busy.People[0].Busy, "in the event's workspace the same query shows him busy")
}
