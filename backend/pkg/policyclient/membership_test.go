package policyclient_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
)

// fakeGraph is the slice of the policy graph the membership helper reads:
// UA nodes by name, and each user's ancestor set.
type fakeGraph struct {
	byName    map[string]*policypb.NGACNode // keyed by name
	ancestors map[string][]*policypb.NGACNode
	err       error
	finds     int
	byID      map[string]*policypb.NGACNode
	assigned  map[[2]string]bool // {child id, parent id}
}

func newFakeGraph() *fakeGraph {
	return &fakeGraph{byName: map[string]*policypb.NGACNode{}, ancestors: map[string][]*policypb.NGACNode{},
		byID: map[string]*policypb.NGACNode{}, assigned: map[[2]string]bool{}}
}

func (g *fakeGraph) ua(id, name string) *policypb.NGACNode {
	n := &policypb.NGACNode{Id: id, Name: name, NodeType: ngac.TypeUA}
	g.byName[name] = n
	g.byID[id] = n
	return n
}

// user registers id as a U node, as every real caller of a membership check is.
func (g *fakeGraph) user(id string) {
	g.byID[id] = &policypb.NGACNode{Id: id, Name: id, NodeType: ngac.TypeU}
}

func (g *fakeGraph) GetNode(_ context.Context, in *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if g.err != nil {
		return nil, g.err
	}
	if n, ok := g.byID[in.GetNodeId()]; ok {
		return n, nil
	}
	// Whoever has an ancestor list in a test is a person.
	if _, ok := g.ancestors[in.GetNodeId()]; ok {
		return &policypb.NGACNode{Id: in.GetNodeId(), Name: in.GetNodeId(), NodeType: ngac.TypeU}, nil
	}
	return nil, status.Error(codes.NotFound, "no such node")
}

func (g *fakeGraph) IsAssigned(_ context.Context, in *policypb.IsAssignedRequest, _ ...grpc.CallOption) (*policypb.BoolResponse, error) {
	if g.err != nil {
		return nil, g.err
	}
	return &policypb.BoolResponse{Value: g.assigned[[2]string{in.GetChildId(), in.GetParentId()}]}, nil
}

func (g *fakeGraph) FindNodeByName(_ context.Context, in *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	g.finds++
	if g.err != nil {
		return nil, g.err
	}
	if n, ok := g.byName[in.GetName()]; ok && n.GetNodeType() == in.GetNodeType() {
		return n, nil
	}
	return nil, status.Error(codes.NotFound, "no such node")
}

func (g *fakeGraph) GetAncestors(_ context.Context, in *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	if g.err != nil {
		return nil, g.err
	}
	return &policypb.NodeList{Nodes: g.ancestors[in.GetNodeId()]}, nil
}

// Workspace ids are unique per test: the Members UA id is cached for the life of
// the process.
func workspaceFixture(t *testing.T, g *fakeGraph, ws ngac.WorkspaceID) (members, owners, tenantMember *policypb.NGACNode) {
	t.Helper()
	members = g.ua("m-"+string(ws), ngac.MembersUAName(ws))
	pc := &policypb.NGACNode{Id: "pc-" + string(ws), Name: ngac.PCName(ws), NodeType: ngac.TypePC}
	g.byName[pc.Name] = pc
	g.assigned[[2]string{members.Id, pc.Id}] = true
	owners = g.ua("o-"+string(ws), ngac.OwnersUAName(ws))
	tenantMember = g.ua("tm-"+string(ws), ngac.TenantMemberUAName(ws))
	return members, owners, tenantMember
}

func TestIsWorkspaceMember(t *testing.T) {
	ctx := context.Background()
	ws := ngac.WorkspaceID("ws-ismember")
	other := ngac.WorkspaceID("ws-ismember-other")
	g := newFakeGraph()
	members, owners, tenantMember := workspaceFixture(t, g, ws)
	otherMembers, _, _ := workspaceFixture(t, g, other)
	channelUA := g.ua("ch1", ngac.ChannelMembersUAName("c1"))
	guestUA := g.ua("guests1", "CalEvGuests_e1")
	roleUA := g.ua("role1", ngac.RoleUAName("r1"))
	deptUA := g.ua("dept1", ngac.DeptUAName("d1"))

	// Allow fixtures: each of these is under Members (Owners and TenantMember are
	// assigned beneath it), so the same graph answers the deny cases below.
	g.ancestors["u-member"] = []*policypb.NGACNode{members}
	g.ancestors["u-owner"] = []*policypb.NGACNode{owners, members}
	g.ancestors["u-tenant"] = []*policypb.NGACNode{tenantMember, members}
	// Deny fixtures: reach other UAs but never this workspace's Members UA.
	g.ancestors["u-channel-only"] = []*policypb.NGACNode{channelUA}
	g.ancestors["u-other-ws"] = []*policypb.NGACNode{otherMembers}
	g.ancestors["u-guest"] = []*policypb.NGACNode{guestUA}
	g.ancestors["u-role-dept"] = []*policypb.NGACNode{roleUA, deptUA}
	g.ancestors["u-nobody"] = nil

	cases := []struct {
		user string
		want bool
	}{
		{"u-member", true},
		{"u-owner", true},
		{"u-tenant", true},
		{"u-channel-only", false},
		{"u-other-ws", false},
		{"u-guest", false},
		{"u-role-dept", false},
		{"u-nobody", false},
	}
	for _, tc := range cases {
		t.Run(tc.user, func(t *testing.T) {
			got, err := policyclient.IsWorkspaceMember(ctx, g, tc.user, ws)
			if err != nil || got != tc.want {
				t.Fatalf("IsWorkspaceMember(%s) = (%v, %v), want (%v, nil)", tc.user, got, err, tc.want)
			}
		})
	}
}

// Only a person (a U node) is a member. A UA that sits under Members, such as
// Owners or a department, reaches the Members UA through its own ancestors and
// would pass an ancestor test; it must not.
func TestOnlyAUserNodeIsAMember(t *testing.T) {
	ctx := context.Background()
	ws := ngac.WorkspaceID("ws-onlyusers")
	g := newFakeGraph()
	members, owners, _ := workspaceFixture(t, g, ws)
	dept := g.ua("dept-ou", ngac.DeptUAName("d-ou"))
	g.ancestors["u-real"] = []*policypb.NGACNode{members}
	g.ancestors[owners.Id] = []*policypb.NGACNode{members}
	g.ancestors[dept.Id] = []*policypb.NGACNode{members}

	for _, tc := range []struct {
		id   string
		want bool
	}{
		{"u-real", true},
		{owners.Id, false},
		{dept.Id, false},
		{"no-such-node", false},
	} {
		got, err := policyclient.IsWorkspaceMember(ctx, g, tc.id, ws)
		if err != nil || got != tc.want {
			t.Errorf("IsWorkspaceMember(%s) = (%v, %v), want (%v, nil)", tc.id, got, err, tc.want)
		}
	}
	if got, err := policyclient.MemberWorkspaces(ctx, g, owners.Id); err != nil || len(got) != 0 {
		t.Errorf("MemberWorkspaces(owners UA) = (%v, %v), want none", got, err)
	}
	if got, err := policyclient.MemberWorkspaces(ctx, g, "u-real"); err != nil || len(got) != 1 {
		t.Errorf("MemberWorkspaces(user) = (%v, %v), want the workspace", got, err)
	}
}

// A UA named "<ws>_Members" that is not assigned under the workspace's policy
// class is not its Members UA: it is neither cached nor believed.
func TestMembersUAMustSitUnderThePolicyClass(t *testing.T) {
	ctx := context.Background()
	ws := ngac.WorkspaceID("ws-notunderpc")
	g := newFakeGraph()
	members, _, _ := workspaceFixture(t, g, ws)
	g.ancestors["u"] = []*policypb.NGACNode{members}

	delete(g.assigned, [2]string{members.Id, "pc-" + string(ws)})
	if id, err := policyclient.MembersUA(ctx, g, ws); err == nil {
		t.Fatalf("accepted %q although it is not under the policy class", id)
	}
	if ok, err := policyclient.IsWorkspaceMember(ctx, g, "u", ws); ok || err == nil {
		t.Fatalf("= (%v, %v), want (false, error)", ok, err)
	}

	// Same fixture, once it is assigned: accepted (so the denial above is the check, not the fixture).
	g.assigned[[2]string{members.Id, "pc-" + string(ws)}] = true
	if ok, err := policyclient.IsWorkspaceMember(ctx, g, "u", ws); err != nil || !ok {
		t.Fatalf("= (%v, %v), want (true, nil)", ok, err)
	}
}

func TestIsWorkspaceMemberFailsClosed(t *testing.T) {
	ctx := context.Background()

	t.Run("reader error", func(t *testing.T) {
		ws := ngac.WorkspaceID("ws-err")
		g := newFakeGraph()
		members, _, _ := workspaceFixture(t, g, ws)
		g.ancestors["u"] = []*policypb.NGACNode{members}
		// Same fixture answers yes while the reader works...
		if ok, err := policyclient.IsWorkspaceMember(ctx, g, "u", ws); err != nil || !ok {
			t.Fatalf("baseline = (%v, %v), want (true, nil)", ok, err)
		}
		// ...and must not once it fails.
		g.err = errors.New("policy unavailable")
		if ok, err := policyclient.IsWorkspaceMember(ctx, g, "u", ws); err == nil || ok {
			t.Fatalf("after outage = (%v, %v), want (false, error)", ok, err)
		}
	})

	t.Run("members UA missing", func(t *testing.T) {
		g := newFakeGraph()
		g.ancestors["u"] = []*policypb.NGACNode{{Id: "x", Name: "x", NodeType: ngac.TypeUA}}
		ok, err := policyclient.IsWorkspaceMember(ctx, g, "u", "ws-no-members")
		if ok || err == nil {
			t.Fatalf("= (%v, %v), want (false, error)", ok, err)
		}
	})

	t.Run("errors are not cached", func(t *testing.T) {
		ws := ngac.WorkspaceID("ws-nocache")
		g := newFakeGraph()
		workspaceFixture(t, g, ws)
		g.err = errors.New("down")
		if _, err := policyclient.MembersUA(ctx, g, ws); err == nil {
			t.Fatal("expected the outage to surface")
		}
		g.err = nil
		if id, err := policyclient.MembersUA(ctx, g, ws); err != nil || id != "m-"+string(ws) {
			t.Fatalf("after recovery = (%q, %v)", id, err)
		}
	})
}

func TestMembersUAIsCached(t *testing.T) {
	ctx := context.Background()
	ws := ngac.WorkspaceID("ws-cache")
	g := newFakeGraph()
	workspaceFixture(t, g, ws)
	for i := 0; i < 3; i++ {
		if _, err := policyclient.MembersUA(ctx, g, ws); err != nil {
			t.Fatal(err)
		}
	}
	if g.finds != 2 { // the Members UA and its policy class, once
		t.Fatalf("FindNodeByName called %d times, want 2", g.finds)
	}
}

func TestMemberWorkspacesAndShared(t *testing.T) {
	ctx := context.Background()
	wsA, wsB, wsC := ngac.WorkspaceID("ws-mw-a"), ngac.WorkspaceID("ws-mw-b"), ngac.WorkspaceID("ws-mw-c")
	g := newFakeGraph()
	mA, _, _ := workspaceFixture(t, g, wsA)
	mB, _, _ := workspaceFixture(t, g, wsB)
	mC, _, _ := workspaceFixture(t, g, wsC)
	channelUA := g.ua("ch-mw", ngac.ChannelMembersUAName("c-mw"))
	// A UA somebody named like a workspace's Members UA, but which is not the
	// node the workspace owns.
	spoof := &policypb.NGACNode{Id: "spoof", Name: ngac.MembersUAName(wsC), NodeType: ngac.TypeUA}

	g.ancestors["alice"] = []*policypb.NGACNode{mA, mB, channelUA}
	g.ancestors["bob"] = []*policypb.NGACNode{mB, mC}
	g.ancestors["mallory"] = []*policypb.NGACNode{spoof, channelUA}

	names := func(ws []ngac.WorkspaceID) []string {
		out := make([]string, len(ws))
		for i, w := range ws {
			out[i] = string(w)
		}
		sort.Strings(out)
		return out
	}

	got, err := policyclient.MemberWorkspaces(ctx, g, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if n := names(got); len(n) != 2 || n[0] != "ws-mw-a" || n[1] != "ws-mw-b" {
		t.Fatalf("alice = %v, want [ws-mw-a ws-mw-b]", n)
	}

	got, err = policyclient.MemberWorkspaces(ctx, g, "mallory")
	if err != nil || len(got) != 0 {
		t.Fatalf("mallory (spoofed _Members name) = (%v, %v), want none", got, err)
	}

	shared, err := policyclient.SharedWorkspaces(ctx, g, "alice", "bob")
	if err != nil || len(shared) != 1 || shared[0] != wsB {
		t.Fatalf("alice/bob shared = (%v, %v), want [ws-mw-b]", shared, err)
	}
	shared, err = policyclient.SharedWorkspaces(ctx, g, "alice", "mallory")
	if err != nil || len(shared) != 0 {
		t.Fatalf("alice/mallory shared = (%v, %v), want none", shared, err)
	}

	g.err = errors.New("down")
	if _, err := policyclient.SharedWorkspaces(ctx, g, "alice", "bob"); err == nil {
		t.Fatal("SharedWorkspaces must report an outage, not an empty intersection")
	}
}
