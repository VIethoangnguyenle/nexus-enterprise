package grpc

import (
	"log/slog"
	"sort"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/policy/internal/ngac"
)

// maxUsersPerPermissionEvent bounds one event's audience; a change reaching
// more users is announced in several events.
const maxUsersPerPermissionEvent = 200

// SetRealtime installs the emitter that tells connected sessions their
// permissions changed. Without one the writer announces nothing.
func (s *WriteServer) SetRealtime(e realtime.Emitter) { s.realtime = e }

// announcePermissionChange emits one permission event per workspace for the
// given users, after the change is written and its caches are invalidated.
// Workspaces are resolved from the policy classes above the changed nodes; a
// change that sits under none has no tenant to tell and is not announced.
func (s *WriteServer) announcePermissionChange(users, workspaces []string) {
	if s.realtime == nil || len(users) == 0 {
		return
	}
	if len(workspaces) == 0 {
		slog.Debug("permission change not announced: workspace unresolved", "users", len(users))
		return
	}
	for _, ws := range workspaces {
		for _, batch := range batchUsers(users, maxUsersPerPermissionEvent) {
			s.realtime.Emit(realtime.Event{
				Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
				TenantID: ws, WorkspaceID: ws, UserNodeIDs: batch,
			})
		}
	}
}

// announceUsersOf announces a change to the permissions of the users who sit
// beneath subject: the one node whose own standing the write changed. anchors
// are further nodes of the same change (the parent of an edge, the object
// attribute of an association) used only to find the workspace; their own
// users are never part of the audience, since a write on an edge does not
// change what the other holders of the parent can do.
func (s *WriteServer) announceUsersOf(subject string, anchors ...string) {
	if s.realtime == nil || s.store == nil {
		return
	}
	g := s.store.GetGraph()
	if g == nil {
		return
	}
	ids := append([]string{subject}, anchors...)
	s.announcePermissionChange(affectedUsers(g, subject), ngac.AffectedWorkspaces(g, ids...))
}

// affectedUsers lists the user nodes whose effective permissions change when
// the edges or grants of a node change:
//
//   - a user: itself;
//   - a user attribute or a policy class: the users beneath it.
//
// An object attribute has no audience here. Objects reach people through the
// resource's own events (a folder created or moved, a channel created), and
// deriving "everyone who holds rights above it" meant scanning every user
// attribute in the graph on every write and told every member of the workspace
// each time anyone made a folder.
func affectedUsers(g ngac.GraphReader, nodeIDs ...string) []string {
	users := map[string]bool{}
	for _, id := range nodeIDs {
		n := g.GetNode(id)
		if n == nil {
			continue
		}
		switch n.NodeType {
		case ngac.NodeTypeUser:
			users[id] = true
		case ngac.NodeTypeUserAttribute, ngac.NodeTypePolicyClass:
			for did, d := range g.GetDescendants(id) {
				if d.NodeType == ngac.NodeTypeUser {
					users[did] = true
				}
			}
		}
	}
	out := make([]string, 0, len(users))
	for id := range users {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func batchUsers(ids []string, size int) [][]string {
	var out [][]string
	for len(ids) > 0 {
		n := size
		if len(ids) < n {
			n = len(ids)
		}
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}
