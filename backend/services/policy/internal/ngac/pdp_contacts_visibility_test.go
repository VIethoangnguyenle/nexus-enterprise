package ngac_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vocab "ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for contacts visibility (docs/specs/contacts-visibility).
//
// The model decides who may see whom in Danh bạ, with no PDP change: reading a
// directory OA is the question, the answer comes from associations, and the one
// rule ("limited to your own department") is a prohibition on read. This is the
// first production use of a prohibition, so the vectors run it on every path the
// PDP has: single, batch, the CTE fallback and the decision cache.
//
// Workspace w (every id is "w:<label>"; departments are keyed by id, not name):
//
//	PC ── Owners ─┐        Members ── (every member, Owners beneath it)
//	   ├─ Mgmt ── Directory ─┬─ DirectoryOpen ◄─ Members {read}
//	   │                     │     ├─ DirectoryNoDept
//	   │                     │     └─ Dir_root ─ Dir_a ─ Dir_a1 ─ Dir_a1x
//	   │                     │                     └──── Dir_a2
//	   │                     ├─ DirectoryOwners ◄─ Members {read}      (outside Open)
//	   │                     ├─ Dir_h ◄─ Dept_h {read}  ─ Dir_h1       (hidden: outside Open)
//	   │                     └─ DirPeer_root ─ DirPeer_a ─ …           (peer tree, outside Open)
//	   ├─ DirectoryLimited   (no association; the prohibition's subject)
//	   └─ DeptPeers_<d>      (flat; ◄ users directly in d; {read} on DirPeer_<d>)
//
// Departments: root → {a → {a1 → {a1x}, a2}, h(hidden) → {h1}}.
//
// Every deny sits beside an allow on the same fixture, so an unknown node cannot
// fake the deny.

// dirFlaws are deliberate mistakes in the model. They exist so the vectors can be
// shown to catch them (TestContactsVisibility_VectorsCatchABrokenModel): a vector
// that stays green on a broken model proves nothing.
type dirFlaws struct {
	noRule            bool // the "own department only" prohibition is missing
	ownersUnderOpen   bool // DirectoryOwners placed under DirectoryOpen
	peersUnderMembers bool // DeptPeers UAs placed under Members instead of the policy class
	peerTreeUnderOpen bool // the peer tree placed under DirectoryOpen
	chainedPeers      bool // DeptPeers UAs chained like departments instead of flat
	hiddenUnderOpen   bool // a hidden department's Dir OA left under DirectoryOpen
	peerUnderParent   bool // a hidden department's DirPeer OA left under its parent's DirPeer
}

type dirDept struct {
	id, parent string
	hidden     bool
}

var dirDepts = []dirDept{
	{"root", "", false},
	{"a", "root", false},
	{"a1", "a", false},
	{"a1x", "a1", false},
	{"a1y", "a1", true},
	{"a2", "a", false},
	{"h", "root", true},
	{"h1", "h", false},
}

// dirWorld is one graph holding any number of workspaces.
type dirWorld struct {
	t *testing.T
	g *ngac.Graph
	n int
}

func newDirWorld(t *testing.T) *dirWorld { return &dirWorld{t: t, g: ngac.NewGraph()} }

func (w *dirWorld) node(id, name, nodeType string) {
	w.t.Helper()
	w.g.AddNode(&ngac.NGACNode{ID: id, Name: name, NodeType: nodeType})
}

func (w *dirWorld) assign(child, parent string) {
	w.t.Helper()
	w.n++
	require.NoError(w.t, w.g.AddAssignment(&ngac.Assignment{ID: fmt.Sprintf("as%d", w.n), ChildID: child, ParentID: parent}), "%s -> %s", child, parent)
}

func (w *dirWorld) associate(ua, oa string, ops []string) {
	w.t.Helper()
	w.n++
	require.NoError(w.t, w.g.AddAssociation(&ngac.Association{ID: fmt.Sprintf("ac%d", w.n), UAID: ua, OAID: oa, Operations: ops}), "%s -> %s", ua, oa)
}

// wid and vid name a node of workspace w or v.
func wid(label string) string { return "w:" + label }
func vid(label string) string { return "v:" + label }

// addWorkspace builds one workspace the way CreateWorkspace and the directory
// upkeep (phase 03) build it, plus the people of the scenario.
func (w *dirWorld) addWorkspace(ws string, flaws dirFlaws) {
	w.t.Helper()
	id := func(label string) string { return ws + ":" + label }
	wsID := vocab.WorkspaceID(ws)
	deptID := func(d string) vocab.DeptID { return vocab.DeptID(ws + "-" + d) }

	// Workspace bootstrap, as CreateWorkspace builds it.
	w.node(id("pc"), vocab.PCName(wsID), "PC")
	w.node(id("owners"), vocab.OwnersUAName(wsID), "UA")
	w.node(id("members"), vocab.MembersUAName(wsID), "UA")
	w.node(id("mgmt"), vocab.MgmtOAName(wsID), "OA")
	for _, l := range []string{"owners", "members", "mgmt"} {
		w.assign(id(l), id("pc"))
	}
	w.assign(id("owners"), id("members"))
	w.associate(id("owners"), id("mgmt"), vocab.AllOwnerOps())

	// A role that confers manage on Mgmt (a directory administrator), and one that
	// is limited to its own department.
	w.node(id("role_admin"), "Role_admin_"+ws, "UA")
	w.node(id("role_limited"), "Role_limited_"+ws, "UA")
	w.assign(id("role_admin"), id("pc"))
	w.assign(id("role_limited"), id("pc"))
	w.associate(id("role_admin"), id("mgmt"), vocab.AreaOps(vocab.AreaManagement))

	// Directory nodes.
	w.node(id("directory"), vocab.DirectoryOAName(wsID), "OA")
	w.node(id("open"), vocab.DirectoryOpenOAName(wsID), "OA")
	w.node(id("nodept"), vocab.DirectoryNoDeptOAName(wsID), "OA")
	w.node(id("dirowners"), vocab.DirectoryOwnersOAName(wsID), "OA")
	w.node(id("limited"), vocab.DirectoryLimitedUAName(wsID), "UA")
	w.assign(id("directory"), id("mgmt"))
	w.assign(id("open"), id("directory"))
	w.assign(id("nodept"), id("open"))
	if flaws.ownersUnderOpen {
		w.assign(id("dirowners"), id("open"))
	} else {
		w.assign(id("dirowners"), id("directory"))
	}
	w.assign(id("limited"), id("pc"))
	w.assign(id("role_limited"), id("limited"))
	w.associate(id("members"), id("open"), []string{vocab.OpRead})
	w.associate(id("members"), id("dirowners"), []string{vocab.OpRead})

	// Per-department nodes, all created before any edge is drawn.
	for _, d := range dirDepts {
		w.node(id("dept_"+d.id), vocab.DeptUAName(deptID(d.id)), "UA")
		w.node(id("dir_"+d.id), vocab.DeptDirOAName(deptID(d.id)), "OA")
		w.node(id("peer_"+d.id), vocab.DeptPeerOAName(deptID(d.id)), "OA")
		w.node(id("peers_"+d.id), vocab.DeptPeersUAName(deptID(d.id)), "UA")
	}
	for _, d := range dirDepts {
		// The department UA chains upward, as CreateDepartment builds it.
		if d.parent == "" {
			w.assign(id("dept_"+d.id), id("pc"))
		} else {
			w.assign(id("dept_"+d.id), id("dept_"+d.parent))
		}

		// The directory OA mirrors the tree; a hidden department's is moved out of Open.
		switch {
		case d.hidden && !flaws.hiddenUnderOpen:
			w.assign(id("dir_"+d.id), id("directory"))
			w.associate(id("dept_"+d.id), id("dir_"+d.id), []string{vocab.OpRead})
		case d.parent == "":
			w.assign(id("dir_"+d.id), id("open"))
		default:
			w.assign(id("dir_"+d.id), id("dir_"+d.parent))
		}
		if d.hidden && flaws.hiddenUnderOpen {
			w.assign(id("dir_"+d.id), id("dir_"+d.parent))
			w.associate(id("dept_"+d.id), id("dir_"+d.id), []string{vocab.OpRead})
		}

		// The peer tree mirrors it too, reached through the flat DeptPeers UA.
		switch {
		case d.hidden && !flaws.peerUnderParent:
			// Hiding moves the peer OA too (with its subtree), or the parent's people read it.
			w.assign(id("peer_"+d.id), id("directory"))
		case d.parent != "":
			w.assign(id("peer_"+d.id), id("peer_"+d.parent))
		case flaws.peerTreeUnderOpen:
			w.assign(id("peer_"+d.id), id("open"))
		default:
			w.assign(id("peer_"+d.id), id("directory"))
		}
		switch {
		case flaws.peersUnderMembers:
			w.assign(id("peers_"+d.id), id("members"))
		case flaws.chainedPeers && d.parent != "":
			w.assign(id("peers_"+d.id), id("peers_"+d.parent))
		default:
			w.assign(id("peers_"+d.id), id("pc"))
		}
		w.associate(id("peers_"+d.id), id("peer_"+d.id), []string{vocab.OpRead})
	}

	if !flaws.noRule {
		require.NoError(w.t, w.g.AddProhibition(dirRule(ws)))
	}
}

// dirRule is the workspace's one directory prohibition.
func dirRule(ws string) *ngac.Prohibition {
	return &ngac.Prohibition{
		Name:        vocab.DirectoryLimitedProhibitionName(vocab.WorkspaceID(ws)),
		SubjectID:   ws + ":limited",
		Operations:  []string{vocab.OpRead},
		TargetOAIDs: []string{ws + ":open"},
	}
}

// person adds a user of workspace ws. member puts them under Members; dept puts
// them in a department (and, as the directory upkeep does, in its DeptPeers UA);
// parents are further UAs (a role, Owners).
func (w *dirWorld) person(ws, label string, member bool, dept string, parents ...string) string {
	w.t.Helper()
	uid := ws + ":" + label
	w.node(uid, "User_"+uid, "U")
	if member {
		w.assign(uid, ws+":members")
	}
	if dept != "" {
		w.assign(uid, ws+":dept_"+dept)
		w.assign(uid, ws+":peers_"+dept)
	}
	for _, p := range parents {
		w.assign(uid, ws+":"+p)
	}
	return uid
}

// limitDept makes a department limited: its UA gets DirectoryLimited as an extra parent.
func (w *dirWorld) limitDept(ws, dept string) { w.assign(ws+":dept_"+dept, ws+":limited") }

// buildDirectory returns workspace w with its people, and a second workspace v.
//
//	owner   owner, in a2           m1  member in a1        m2  member in a2
//	lim     limited role, in a1    h1m member in h1        a1xm member in a1x
//	admin   mgmt role (manage)     exm member in a1 (removed by some vectors)
//	peerOnly  only under DeptPeers_a1      roleOnly  only in the limited role
//	rootm   member directly in root          a1ym  member in a1y (hidden, inside a1)
//	limNoDept  limited role, no department
//	vm      member of workspace v, in v's a1
func buildDirectory(t *testing.T, flaws dirFlaws) *dirWorld {
	t.Helper()
	w := newDirWorld(t)
	w.addWorkspace("w", flaws)
	w.addWorkspace("v", dirFlaws{})

	w.person("w", "owner", true, "a2", "owners")
	w.person("w", "m1", true, "a1")
	w.person("w", "m2", true, "a2")
	w.person("w", "lim", true, "a1", "role_limited")
	w.person("w", "h1m", true, "h1")
	w.person("w", "a1xm", true, "a1x")
	w.person("w", "admin", true, "", "role_admin")
	w.person("w", "exm", true, "a1")
	w.person("w", "rootm", true, "root")
	w.person("w", "a1ym", true, "a1y")
	w.person("w", "limNoDept", true, "", "role_limited")
	w.person("w", "peerOnly", false, "")
	w.assign(wid("peerOnly"), wid("peers_a1"))
	w.person("w", "roleOnly", false, "", "role_limited")
	w.person("v", "vm", true, "a1")
	return w
}

// dirCase is one decision. byRule marks a DENY that only the prohibition causes:
// the graph alone would ALLOW it.
type dirCase struct {
	why        string
	user, oa   string
	op         string
	want       bool
	byRule     bool
	skipSingle bool
}

func dirAllow(why, user, oa, op string) dirCase {
	return dirCase{why: why, user: user, oa: oa, op: op, want: true}
}
func dirDeny(why, user, oa, op string) dirCase { return dirCase{why: why, user: user, oa: oa, op: op} }
func dirDenyByRule(why, user, oa string) dirCase {
	return dirCase{why: why, user: user, oa: oa, op: vocab.OpRead, byRule: true}
}

// dirVectors is the decision matrix. DENY vectors come first; each is paired with
// an ALLOW on the same fixture further down.
func dirVectors() []dirCase {
	read, manage := vocab.OpRead, vocab.OpManage
	return []dirCase{
		// --- DENY: another policy class. ---
		dirDeny("another workspace's member reads Dir_a", vid("vm"), wid("dir_a"), read),
		dirDeny("another workspace's member reads the owners OA", vid("vm"), wid("dirowners"), read),
		dirDeny("another workspace's member reads a peer OA", vid("vm"), wid("peer_a1"), read),
		dirDeny("another workspace's member manages Directory", vid("vm"), wid("directory"), manage),

		// --- DENY: one hop short. ---
		dirDeny("a member reads a hidden department's OA", wid("m1"), wid("dir_h"), read),
		dirDeny("a member reads a hidden sub-department's OA", wid("m1"), wid("dir_h1"), read),
		dirDeny("a member reads a sibling's peer OA", wid("m1"), wid("peer_a2"), read),
		dirDeny("a member reads the Directory root (only Open is theirs)", wid("m1"), wid("directory"), read),
		dirDeny("a member manages Directory", wid("m1"), wid("directory"), manage),
		dirDeny("a member writes a directory OA", wid("m1"), wid("dir_a2"), vocab.OpWrite),
		dirDeny("an a1 member reads the peers of the sibling a2", wid("m1"), wid("peer_a2"), read),
		dirDeny("a hidden department's member does not read the a1 peers", wid("h1m"), wid("peer_a1"), read),

		// --- DENY: the prohibition overrides an otherwise-ALLOW from Members. ---
		dirDenyByRule("limited reads a sibling department", wid("lim"), wid("dir_a2")),
		dirDenyByRule("limited reads their own department through Open", wid("lim"), wid("dir_a1")),
		dirDenyByRule("limited reads the people with no department", wid("lim"), wid("nodept")),
		dirDenyByRule("limited reads the root department", wid("lim"), wid("dir_root")),
		dirDenyByRule("limited reads their own sub-department through Open", wid("lim"), wid("dir_a1x")),

		// --- DENY: a limited person's own tree is flat, one hop short of a parent or sibling. ---
		dirDeny("limited reads the parent department's peers", wid("lim"), wid("peer_a"), read),
		dirDeny("limited reads a sibling department's peers", wid("lim"), wid("peer_a2"), read),
		dirDeny("limited manages Directory", wid("lim"), wid("directory"), manage),

		// --- DENY: peers and roles never imply membership. ---
		dirDeny("someone only under DeptPeers reads Open", wid("peerOnly"), wid("dir_a2"), read),
		dirDeny("someone only under DeptPeers reads the owners OA", wid("peerOnly"), wid("dirowners"), read),
		dirDeny("someone only in the limited role reads Open", wid("roleOnly"), wid("dir_a2"), read),
		dirDeny("someone only in the limited role reads the owners OA", wid("roleOnly"), wid("dirowners"), read),

		// --- DENY: a hidden department leaks through the peer tree unless its peer OA moves too. ---
		dirDeny("a person directly in the parent reads a hidden department's peers", wid("rootm"), wid("peer_h"), read),
		dirDeny("a person directly in the parent reads a hidden sub-department's peers", wid("rootm"), wid("peer_h1"), read),
		dirDeny("a person directly in a department reads its hidden sub-department's peers", wid("m1"), wid("peer_a1y"), read),
		dirDeny("a person directly in a department reads its hidden sub-department", wid("m1"), wid("dir_a1y"), read),
		dirDeny("limited reads the peers of a hidden sub-department of their own department", wid("lim"), wid("peer_a1y"), read),
		dirDeny("limited reads a hidden sub-department of their own department", wid("lim"), wid("dir_a1y"), read),

		// --- DENY: a limited person with no department sees the owners and nothing else. ---
		dirDenyByRule("limited without a department reads the open tree", wid("limNoDept"), wid("dir_a2")),
		dirDenyByRule("limited without a department reads the people with no department", wid("limNoDept"), wid("nodept")),
		dirDeny("limited without a department reads any peers", wid("limNoDept"), wid("peer_a1"), read),

		// --- ALLOW, on the same fixture. ---
		dirAllow("a hidden department's own people read its peers", wid("h1m"), wid("peer_h1"), read),
		dirAllow("a hidden sub-department's people read it", wid("a1ym"), wid("peer_a1y"), read),
		dirAllow("a hidden sub-department's people read its directory OA", wid("a1ym"), wid("dir_a1y"), read),
		dirAllow("the parent's people read their own peers", wid("rootm"), wid("peer_root"), read),
		dirAllow("limited without a department reads the owners", wid("limNoDept"), wid("dirowners"), read),
		dirAllow("a member reads a sibling department", wid("m1"), wid("dir_a2"), read),
		dirAllow("a member reads the people with no department", wid("m1"), wid("nodept"), read),
		dirAllow("a member reads the owners OA", wid("m1"), wid("dirowners"), read),
		dirAllow("a member reads the root department", wid("m1"), wid("dir_root"), read),
		dirAllow("a member reads their own peers", wid("m1"), wid("peer_a1"), read),
		dirAllow("a hidden department's member reads it", wid("h1m"), wid("dir_h"), read),
		dirAllow("a hidden sub-department's member reads it", wid("h1m"), wid("dir_h1"), read),
		dirAllow("a hidden department's member reads an open department", wid("h1m"), wid("dir_a2"), read),
		dirAllow("limited reads their own peers", wid("lim"), wid("peer_a1"), read),
		dirAllow("limited reads their own sub-department's peers", wid("lim"), wid("peer_a1x"), read),
		dirAllow("limited still reads the owners OA", wid("lim"), wid("dirowners"), read),
		dirAllow("someone under DeptPeers reads exactly their peers", wid("peerOnly"), wid("peer_a1"), read),
		dirAllow("the owner reads a hidden department", wid("owner"), wid("dir_h"), read),
		dirAllow("the owner reads the owners OA", wid("owner"), wid("dirowners"), read),
		dirAllow("the owner manages Directory", wid("owner"), wid("directory"), manage),
		dirAllow("a management role manages Directory", wid("admin"), wid("directory"), manage),
		dirAllow("another workspace's member reads their own owners OA", vid("vm"), vid("dirowners"), read),
		dirAllow("another workspace's member reads their own sibling department", vid("vm"), vid("dir_a2"), read),
	}
}

func dirReq(user, oa, op string) ngac.AccessRequest {
	return ngac.AccessRequest{UserNodeID: user, ObjectNodeID: oa, Operation: op}
}

// dirViolations runs every case on the single and the batch path and returns what
// disagrees with the model. Empty means the model behaves.
func dirViolations(t *testing.T, g *ngac.Graph, cases []dirCase) []string {
	t.Helper()
	ctx := context.Background()
	e := ngac.NewDecisionEngine(g, nil)
	var bad []string
	for _, c := range cases {
		d := e.Decide(ctx, dirReq(c.user, c.oa, c.op))
		if got := d.Decision == ngac.DecisionAllow; got != c.want {
			bad = append(bad, fmt.Sprintf("single path: %s: got allow=%v, want %v", c.why, got, c.want))
		}
		batch := e.(batchDecider).DecideBatch(ctx, ngac.BatchAccessRequest{
			UserNodeID: c.user, ObjectNodeIDs: []string{c.oa}, Operations: []string{c.op},
		})
		if got := batch[c.oa][c.op]; got != c.want {
			bad = append(bad, fmt.Sprintf("batch path: %s: got allow=%v, want %v", c.why, got, c.want))
		}

		denial := d.Explanation.ProhibitionDenied
		switch {
		case c.byRule:
			if raw := g.CheckAccess(c.user, c.oa, c.op); raw.Decision != ngac.DecisionAllow {
				bad = append(bad, fmt.Sprintf("%s: the graph alone must allow (so only the prohibition denies), got %s", c.why, raw.Decision))
			}
			if denial == nil {
				bad = append(bad, fmt.Sprintf("%s: no prohibition is named in the denial", c.why))
			} else if want := vocab.DirectoryLimitedProhibitionName("w"); denial.ProhibitionName != want {
				bad = append(bad, fmt.Sprintf("%s: denied by %q, want %q", c.why, denial.ProhibitionName, want))
			}
		case !c.want && denial != nil:
			bad = append(bad, fmt.Sprintf("%s: denied by prohibition %q, but the graph itself must deny", c.why, denial.ProhibitionName))
		}
	}
	return bad
}

// graphReader answers the membership helper from an in-memory graph, so
// IsWorkspaceMember runs its real logic over the real fixture.
type graphReader struct{ g *ngac.Graph }

func pbNode(n *ngac.NGACNode) *policypb.NGACNode {
	return &policypb.NGACNode{Id: n.ID, Name: n.Name, NodeType: n.NodeType}
}

func (r graphReader) FindNodeByName(_ context.Context, in *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if n := r.g.FindNodeByName(in.GetName(), in.GetNodeType()); n != nil {
		return pbNode(n), nil
	}
	return nil, status.Error(codes.NotFound, "no such node")
}

func (r graphReader) GetNode(_ context.Context, in *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if n := r.g.GetNode(in.GetNodeId()); n != nil {
		return pbNode(n), nil
	}
	return nil, status.Error(codes.NotFound, "no such node")
}

func (r graphReader) IsAssigned(_ context.Context, in *policypb.IsAssignedRequest, _ ...grpc.CallOption) (*policypb.BoolResponse, error) {
	return &policypb.BoolResponse{Value: r.g.IsAssigned(in.GetChildId(), in.GetParentId())}, nil
}

func (r graphReader) GetAncestors(_ context.Context, in *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	out := &policypb.NodeList{}
	for _, n := range r.g.GetAncestors(in.GetNodeId()) {
		out.Nodes = append(out.Nodes, pbNode(n))
	}
	return out, nil
}

// dirMembership is who is a member of workspace w (v for vm): exactly the people
// under w_Members. A department, a peers UA and the limited role are not.
var dirMembership = map[string]bool{
	wid("owner"): true, wid("m1"): true, wid("m2"): true, wid("lim"): true, wid("h1m"): true,
	wid("a1xm"): true, wid("admin"): true, wid("exm"): true,
	wid("rootm"): true, wid("a1ym"): true, wid("limNoDept"): true,
	wid("peerOnly"): false, wid("roleOnly"): false, vid("vm"): false,
}

func membershipViolations(t *testing.T, g *ngac.Graph) []string {
	t.Helper()
	var bad []string
	for user, want := range dirMembership {
		got, err := policyclient.IsWorkspaceMember(context.Background(), graphReader{g}, user, "w")
		if err != nil {
			bad = append(bad, fmt.Sprintf("membership of %s: %v", user, err))
			continue
		}
		if got != want {
			bad = append(bad, fmt.Sprintf("membership of %s in w: got %v, want %v", user, got, want))
		}
	}
	// The other workspace's member is a member of v, and of v only.
	if ok, err := policyclient.IsWorkspaceMember(context.Background(), graphReader{g}, vid("vm"), "v"); err != nil || !ok {
		bad = append(bad, fmt.Sprintf("vm must be a member of v: %v, %v", ok, err))
	}
	return bad
}

func TestContactsVisibility_Vectors(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	for _, v := range dirViolations(t, w.g, dirVectors()) {
		t.Error(v)
	}
}

// Peers and roles are not membership: a person who reaches the policy class
// through DeptPeers or the limited role is still not a member of the workspace.
func TestContactsVisibility_DirectoryNodesNeverImplyMembership(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	for _, v := range membershipViolations(t, w.g) {
		t.Error(v)
	}
	for _, label := range []string{
		"directory", "open", "nodept", "dirowners", "limited", "peers_a1", "peers_h1", "role_limited",
	} {
		assert.Falsef(t, w.g.IsAssigned(wid(label), wid("members")), "%s must not sit directly under Members", label)
		assert.NotContainsf(t, w.g.GetAncestors(wid(label)), wid("members"), "%s must not be beneath Members at all", label)
	}
	for _, d := range dirDepts {
		for _, prefix := range []string{"peers_", "dir_", "peer_"} {
			assert.NotContainsf(t, w.g.GetAncestors(wid(prefix+d.id)), wid("members"), "%s%s must not be beneath Members", prefix, d.id)
		}
	}
}

// The model must catch its own mistakes: each flaw below turns at least one
// vector red, so a green run means the safeguards are actually in the graph.
func TestContactsVisibility_VectorsCatchABrokenModel(t *testing.T) {
	for name, flaws := range map[string]dirFlaws{
		"the prohibition is dropped":                               {noRule: true},
		"the owners OA is put under Open":                          {ownersUnderOpen: true},
		"DeptPeers UAs are put under Members":                      {peersUnderMembers: true},
		"the peer tree is put under Open":                          {peerTreeUnderOpen: true},
		"DeptPeers UAs are chained like departments":               {chainedPeers: true},
		"a hidden department's OA is left under Open":              {hiddenUnderOpen: true},
		"a hidden department's peer OA is left under its parent's": {peerUnderParent: true},
	} {
		t.Run(name, func(t *testing.T) {
			w := buildDirectory(t, flaws)
			bad := append(dirViolations(t, w.g, dirVectors()), membershipViolations(t, w.g)...)
			assert.NotEmpty(t, bad, "a broken model went unnoticed")
		})
	}
}

// A person who leaves the workspace loses the directory with the membership: the
// peer edge alone, and the department alone, are not enough to keep reading.
func TestContactsVisibility_ExMemberLosesTheDirectory(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	e := ngac.NewDecisionEngine(w.g, nil)
	read := func(oa string) bool {
		return e.Decide(context.Background(), dirReq(wid("exm"), oa, vocab.OpRead)).Decision == ngac.DecisionAllow
	}
	// Before: a member in a1.
	for _, oa := range []string{"peer_a1", "dir_a2", "dirowners"} {
		require.Truef(t, read(wid(oa)), "precondition: a member reads %s", oa)
	}

	w.g.RemoveAssignment(wid("exm"), wid("peers_a1"))
	w.g.RemoveAssignment(wid("exm"), wid("members"))
	w.g.RemoveAssignment(wid("exm"), wid("dept_a1"))

	for _, oa := range []string{"peer_a1", "dir_a2", "dirowners", "dir_root", "nodept"} {
		assert.Falsef(t, read(wid(oa)), "a person who left reads %s", oa)
	}
	assert.True(t, e.Decide(context.Background(), dirReq(wid("m1"), wid("dirowners"), vocab.OpRead)).Decision == ngac.DecisionAllow,
		"the people who stayed keep reading")
}

// Limiting a department limits everyone under it, the owner included; and an
// owner stays an administrator, because the shortcut is manage and the rule is read.
func TestContactsVisibility_LimitedDepartment(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	w.limitDept("w", "a2")
	e := ngac.NewDecisionEngine(w.g, nil)
	ask := func(user, oa, op string) *ngac.AccessDecision {
		return e.Decide(context.Background(), dirReq(wid(user), wid(oa), op))
	}
	read := vocab.OpRead

	// Deny: people of the limited department read nothing outside it.
	for _, user := range []string{"m2", "owner"} {
		d := ask(user, "dir_a1", read)
		assert.Equalf(t, ngac.DecisionDeny, d.Decision, "%s (in the limited department) reads a sibling", user)
		require.NotNilf(t, d.Explanation.ProhibitionDenied, "%s: denied by the rule", user)
		assert.Equalf(t, ngac.DecisionDeny, ask(user, "nodept", read).Decision, "%s reads the people with no department", user)
	}
	assert.Equal(t, ngac.DecisionDeny, ask("m2", "peer_a1", read).Decision, "a limited member reads a sibling's peers")
	// Allow, same fixture: their own tree, the owners, and people outside the limit.
	assert.Equal(t, ngac.DecisionAllow, ask("m2", "peer_a2", read).Decision, "the limited reads their own department")
	assert.Equal(t, ngac.DecisionAllow, ask("m2", "dirowners", read).Decision, "the limited still reads the owners")
	assert.Equal(t, ngac.DecisionAllow, ask("m1", "dir_a2", read).Decision, "a member of another department is not limited")
	assert.Equal(t, ngac.DecisionAllow, ask("m1", "dir_a1", read).Decision)

	// An owner in a limited department is still an administrator.
	assert.Equal(t, ngac.DecisionAllow, ask("owner", "directory", vocab.OpManage).Decision, "the owner still manages the directory")
	assert.Equal(t, ngac.DecisionDeny, ask("m2", "directory", vocab.OpManage).Decision, "a limited member does not")
	assert.Equal(t, ngac.DecisionAllow, ask("owner", "peer_a2", read).Decision, "owners reach every peer OA through Mgmt")

	// A person who moves to another department is limited by that one, with no
	// per-person upkeep: the limit follows the department UA, not the person.
	w.g.RemoveAssignment(wid("m2"), wid("dept_a2"))
	w.g.RemoveAssignment(wid("m2"), wid("peers_a2"))
	w.assign(wid("m2"), wid("dept_a1"))
	w.assign(wid("m2"), wid("peers_a1"))
	assert.Equal(t, ngac.DecisionAllow, ask("m2", "dir_a1", read).Decision, "after the move the rule no longer applies")
	assert.Equal(t, ngac.DecisionAllow, ask("m2", "dir_a2", read).Decision)
}

// A limited person inside a hidden department keeps that department's subtree
// (it sits outside Open, which is all the rule covers) and loses the open tree.
func TestContactsVisibility_LimitedInsideAHiddenDepartmentKeepsTheirSubtree(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	w.limitDept("w", "h1")
	e := ngac.NewDecisionEngine(w.g, nil)
	ask := func(oa string) *ngac.AccessDecision {
		return e.Decide(context.Background(), dirReq(wid("h1m"), wid(oa), vocab.OpRead))
	}

	d := ask("dir_a2")
	assert.Equal(t, ngac.DecisionDeny, d.Decision, "the open tree is closed to a limited person")
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, ngac.DecisionDeny, ask("nodept").Decision)
	assert.Equal(t, ngac.DecisionAllow, ask("dir_h1").Decision, "their own hidden subtree stays visible")
	assert.Equal(t, ngac.DecisionAllow, ask("dir_h").Decision)
	assert.Equal(t, ngac.DecisionAllow, ask("peer_h1").Decision)
	assert.Equal(t, ngac.DecisionAllow, ask("dirowners").Decision)
}

// Owners are visible to everyone, even when they sit in a hidden department: the
// owners OA is outside both Open and every department's tree.
func TestContactsVisibility_OwnersAreVisibleWhateverTheirDepartment(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	w.person("w", "ownerInH1", true, "h1", "owners")
	e := ngac.NewDecisionEngine(w.g, nil)
	ask := func(user, oa string) string {
		return e.Decide(context.Background(), dirReq(wid(user), wid(oa), vocab.OpRead)).Decision
	}
	// The department tree is closed to an ordinary member, the owners OA is not.
	assert.Equal(t, ngac.DecisionDeny, ask("m1", "dir_h1"))
	assert.Equal(t, ngac.DecisionAllow, ask("m1", "dirowners"))
	assert.Equal(t, ngac.DecisionAllow, ask("lim", "dirowners"))
}

// A prohibition is created and removed by the workspace; removing it gives the
// view back, and the rule never touches another workspace.
func TestContactsVisibility_RuleIsPerWorkspaceAndRemovable(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	w.person("v", "vlim", true, "a1", "role_limited")
	e := ngac.NewDecisionEngine(w.g, nil)
	ask := func(user, oa string) string {
		return e.Decide(context.Background(), dirReq(user, oa, vocab.OpRead)).Decision
	}

	assert.Equal(t, ngac.DecisionDeny, ask(vid("vlim"), vid("dir_a2")), "limited in v is limited in v")
	assert.Equal(t, ngac.DecisionDeny, ask(wid("lim"), wid("dir_a2")), "limited in w is limited in w")
	assert.Equal(t, ngac.DecisionAllow, ask(vid("vm"), vid("dir_a2")), "an unlimited member of v is not")

	w.g.RemoveProhibition(vocab.DirectoryLimitedProhibitionName("w"))
	assert.Equal(t, ngac.DecisionAllow, ask(wid("lim"), wid("dir_a2")), "removing w's rule gives w's limited people the open tree back")
	assert.Equal(t, ngac.DecisionDeny, ask(vid("vlim"), vid("dir_a2")), "and leaves v's rule alone")
}

// A misconfigured association from another workspace does not reach this one:
// the policy class is the boundary, whatever is associated.
func TestContactsVisibility_AnAssociationAcrossWorkspacesStillDenies(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	e := ngac.NewDecisionEngine(w.g, nil)
	ask := func(user, oa string) string {
		return e.Decide(context.Background(), dirReq(user, oa, vocab.OpRead)).Decision
	}
	require.Equal(t, ngac.DecisionDeny, ask(vid("vm"), wid("dir_a2")))
	w.associate(vid("members"), wid("open"), []string{vocab.OpRead})
	assert.Equal(t, ngac.DecisionDeny, ask(vid("vm"), wid("dir_a2")), "different policy class")
	assert.Equal(t, ngac.DecisionAllow, ask(wid("m1"), wid("dir_a2")), "while this workspace's own member still reads it")
}

// dbAbove is what the database holds above a node the engine does not have in
// memory: the SQL fallback's input, as ngac_ancestors returns it.
func dbAbove(parents ...string) map[string]string {
	out := map[string]string{}
	for _, p := range parents {
		switch {
		case strings.HasSuffix(p, ":pc"):
			out[p] = "PC"
		case strings.Contains(p, ":role_"), strings.HasSuffix(p, ":members"), strings.HasSuffix(p, ":limited"):
			out[p] = "UA"
		default:
			out[p] = "OA"
		}
	}
	return out
}

// The SQL fallback cannot evaluate a prohibition, so the PDP evaluates it with
// the ancestors the database gives: the prohibition must hold for an object that
// memory has not caught up with, and only for what it targets.
func TestContactsVisibility_CTEFallbackStillAppliesTheLimit(t *testing.T) {
	w := buildDirectory(t, dirFlaws{})
	ghost := wid("dir_new") // in the database, under Open; not yet in memory
	cte := &fakeCTE{
		allowed: map[string]bool{
			wid("m1") + "|" + ghost + "|read":  true,
			wid("lim") + "|" + ghost + "|read": true,
		},
		ancestors: map[string]map[string]string{ghost: dbAbove(wid("open"), wid("directory"), wid("mgmt"), wid("pc"))},
	}
	e := ngac.NewDecisionEngine(w.g, cte)
	ctx := context.Background()
	batchOf := func(user string) bool {
		return e.(batchDecider).DecideBatch(ctx, ngac.BatchAccessRequest{UserNodeID: user, ObjectNodeIDs: []string{ghost}, Operations: []string{vocab.OpRead}})[ghost][vocab.OpRead]
	}

	// Allow: an unlimited member is answered by the fallback, with no ancestor lookup.
	assert.Equal(t, ngac.DecisionAllow, e.Decide(ctx, dirReq(wid("m1"), ghost, vocab.OpRead)).Decision)
	assert.True(t, batchOf(wid("m1")))
	assert.Zero(t, cte.ancCalls.Load(), "nobody covers m1: no database lookup")

	// Deny: the limited person is not, on both paths, with the prohibition named.
	d := e.Decide(ctx, dirReq(wid("lim"), ghost, vocab.OpRead))
	assert.Equal(t, ngac.DecisionDeny, d.Decision, "single path: the limit must hold on the CTE fallback")
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, vocab.DirectoryLimitedProhibitionName("w"), d.Explanation.ProhibitionDenied.ProhibitionName)
	assert.False(t, batchOf(wid("lim")), "batch path: the limit must hold on the CTE fallback")
}

func TestContactsVisibility_CTEFallbackIsPreciseAboutWhatItDenies(t *testing.T) {
	ctx := context.Background()
	ghostDir, ghostDrive := wid("dir_new"), wid("drive_oa")
	ghostUser := wid("ghost") // limited in the database, absent from memory
	setup := func(flaws dirFlaws, noProhibitionAtAll ...bool) (ngac.DecisionEngine, *fakeCTE) {
		w := buildDirectory(t, flaws)
		if len(noProhibitionAtAll) > 0 {
			w.g.RemoveProhibition(vocab.DirectoryLimitedProhibitionName("v"))
			require.Zero(t, w.g.ProhibitionCount())
		}
		cte := &fakeCTE{
			allowed: map[string]bool{},
			ancestors: map[string]map[string]string{
				ghostDir:   dbAbove(wid("open"), wid("directory"), wid("mgmt"), wid("pc")),
				ghostDrive: dbAbove(wid("pc")),
				ghostUser:  dbAbove(wid("role_limited"), wid("limited"), wid("members"), wid("pc")),
			},
		}
		for _, u := range []string{wid("m1"), wid("lim"), ghostUser} {
			for _, o := range []string{ghostDir, ghostDrive} {
				for _, op := range []string{vocab.OpRead, vocab.OpManage} {
					cte.allowed[u+"|"+o+"|"+op] = true
				}
			}
		}
		return ngac.NewDecisionEngine(w.g, cte), cte
	}
	single := func(e ngac.DecisionEngine, u, o, op string) string { return e.Decide(ctx, dirReq(u, o, op)).Decision }
	batch := func(e ngac.DecisionEngine, u string, objs []string, op string) map[string]map[string]bool {
		return e.(batchDecider).DecideBatch(ctx, ngac.BatchAccessRequest{UserNodeID: u, ObjectNodeIDs: objs, Operations: []string{op}})
	}

	t.Run("a limited person reading a non-directory object missing from memory is allowed", func(t *testing.T) {
		e, _ := setup(dirFlaws{})
		assert.Equal(t, ngac.DecisionAllow, single(e, wid("lim"), ghostDrive, vocab.OpRead), "the prohibition targets the directory, not a drive OA")
		assert.True(t, batch(e, wid("lim"), []string{ghostDrive}, vocab.OpRead)[ghostDrive][vocab.OpRead])
	})

	t.Run("a mixed batch denies only what the prohibition targets", func(t *testing.T) {
		e, cte := setup(dirFlaws{})
		got := batch(e, wid("lim"), []string{ghostDir, ghostDrive}, vocab.OpRead)
		assert.False(t, got[ghostDir][vocab.OpRead])
		assert.True(t, got[ghostDrive][vocab.OpRead])
		assert.EqualValues(t, 2, cte.ancCalls.Load(), "the user's subjects come from memory; one lookup per missing object, once")
	})

	t.Run("a ghost user, limited in the database and missing from memory, is denied", func(t *testing.T) {
		e, _ := setup(dirFlaws{})
		d := e.Decide(ctx, dirReq(ghostUser, ghostDir, vocab.OpRead))
		assert.Equal(t, ngac.DecisionDeny, d.Decision, "single path")
		require.NotNil(t, d.Explanation.ProhibitionDenied)
		assert.False(t, batch(e, ghostUser, []string{ghostDir}, vocab.OpRead)[ghostDir][vocab.OpRead], "batch path")
		// Same ghost, same fixture, an object the prohibition does not target: allowed.
		assert.Equal(t, ngac.DecisionAllow, single(e, ghostUser, ghostDrive, vocab.OpRead))
		assert.True(t, batch(e, ghostUser, []string{ghostDrive}, vocab.OpRead)[ghostDrive][vocab.OpRead])
	})

	t.Run("a different operation is not covered", func(t *testing.T) {
		e, _ := setup(dirFlaws{})
		assert.Equal(t, ngac.DecisionAllow, single(e, wid("lim"), ghostDir, vocab.OpManage))
		assert.True(t, batch(e, wid("lim"), []string{ghostDir}, vocab.OpManage)[ghostDir][vocab.OpManage])
		assert.Equal(t, ngac.DecisionDeny, single(e, wid("lim"), ghostDir, vocab.OpRead))
	})

	t.Run("no prohibition in the graph: the CTE answer stands with no lookup", func(t *testing.T) {
		e, cte := setup(dirFlaws{noRule: true}, true)
		assert.Equal(t, ngac.DecisionAllow, single(e, wid("lim"), ghostDir, vocab.OpRead))
		assert.Equal(t, ngac.DecisionAllow, single(e, ghostUser, ghostDir, vocab.OpRead))
		assert.True(t, batch(e, wid("lim"), []string{ghostDir}, vocab.OpRead)[ghostDir][vocab.OpRead])
		assert.Zero(t, cte.ancCalls.Load(), "the fast path touches no database")
	})

	t.Run("a lookup that fails closes the decision and is not cached", func(t *testing.T) {
		e, cte := setup(dirFlaws{})
		cte.ancErr = errDBDown
		d := e.Decide(ctx, dirReq(wid("lim"), ghostDir, vocab.OpRead))
		assert.Equal(t, ngac.DecisionDeny, d.Decision)
		assert.True(t, d.ErrorDerived(), "a DENY from a failed lookup must not be cached")
		assert.False(t, batch(e, wid("lim"), []string{ghostDir}, vocab.OpRead)[ghostDir][vocab.OpRead])
		// A person no prohibition covers never needs the lookup, so the outage does not touch them.
		assert.Equal(t, ngac.DecisionAllow, single(e, wid("m1"), ghostDir, vocab.OpRead))
	})

	t.Run("a denial from the prohibition is a policy answer, cacheable", func(t *testing.T) {
		e, _ := setup(dirFlaws{})
		assert.False(t, e.Decide(ctx, dirReq(wid("lim"), ghostDir, vocab.OpRead)).ErrorDerived(),
			"cached like any DENY (30 s TTL, dropped by the invalidation when the replica catches up)")
	})
}

// Freshness: a cached ALLOW must not outlive the change that makes it a DENY.
// Limiting a department (an assignment) and creating the rule (a prohibition) are
// the two changes the directory makes; each must reach the decision cache through
// the invalidation the write server performs for it.
func TestContactsVisibility_ChangesReachTheDecisionCache(t *testing.T) {
	rdb := testRedis(t, 12)
	ctx := context.Background()

	newRig := func(flaws dirFlaws) (*dirWorld, *ngac.AccessEvaluator, *ngac.CacheInvalidator) {
		w := buildDirectory(t, flaws)
		ev := ngac.NewAccessEvaluator(ngac.NewLayeredCache(rdb), ngac.NewDecisionEngine(w.g, nil))
		return w, ev, ngac.NewCacheInvalidator(rdb, func() *ngac.Graph { return w.g })
	}
	ask := func(ev *ngac.AccessEvaluator, user, oa string) *ngac.AccessDecision {
		return ev.Evaluate(ctx, ngac.AccessRequest{UserNodeID: user, ObjectNodeID: oa, Operation: vocab.OpRead, WorkspaceID: "w"})
	}
	cached := func(user, oa string) bool {
		n, err := rdb.Exists(ctx, ngac.DecisionCacheKey(ngac.AccessRequest{UserNodeID: user, ObjectNodeID: oa, Operation: vocab.OpRead, WorkspaceID: "w"})).Result()
		require.NoError(t, err)
		return n == 1
	}

	t.Run("a department is limited", func(t *testing.T) {
		w, ev, inv := newRig(dirFlaws{})
		require.Equal(t, ngac.DecisionAllow, ask(ev, wid("m1"), wid("dir_a2")).Decision)
		require.Equal(t, ngac.DecisionAllow, ask(ev, wid("m2"), wid("dir_a1")).Decision, "a bystander, cached too")
		require.True(t, cached(wid("m1"), wid("dir_a2")))

		w.limitDept("w", "a1") // what the write server sees as CreateAssignment(child, parent)

		// Without the invalidation the cache still answers ALLOW: this is what the rest proves.
		assert.Equal(t, ngac.DecisionAllow, ask(ev, wid("m1"), wid("dir_a2")).Decision, "stale until invalidated")

		inv.InvalidateForNodes(ctx, wid("dept_a1"), wid("limited"))
		assert.False(t, cached(wid("m1"), wid("dir_a2")), "the limited department's cached ALLOW is gone")
		assert.True(t, cached(wid("m2"), wid("dir_a1")), "a bystander's decision survives")
		d := ask(ev, wid("m1"), wid("dir_a2"))
		assert.Equal(t, ngac.DecisionDeny, d.Decision)
		require.NotNil(t, d.Explanation.ProhibitionDenied)
		assert.Equal(t, ngac.DecisionAllow, ask(ev, wid("m2"), wid("dir_a1")).Decision)
	})

	t.Run("the rule is created", func(t *testing.T) {
		w, ev, inv := newRig(dirFlaws{noRule: true})
		require.Equal(t, ngac.DecisionAllow, ask(ev, wid("lim"), wid("dir_a2")).Decision, "no rule yet: limited people read the open tree")
		require.True(t, cached(wid("lim"), wid("dir_a2")))

		require.NoError(t, w.g.AddProhibition(dirRule("w")))
		assert.Equal(t, ngac.DecisionAllow, ask(ev, wid("lim"), wid("dir_a2")).Decision, "stale until invalidated")

		// The write server invalidates the subject, the targets and the subject's users.
		inv.InvalidateForNodes(ctx, wid("limited"), wid("open"))
		assert.Equal(t, ngac.DecisionDeny, ask(ev, wid("lim"), wid("dir_a2")).Decision, "the cached ALLOW is gone")
		assert.Equal(t, ngac.DecisionAllow, ask(ev, wid("lim"), wid("peer_a1")).Decision, "the person's own tree is untouched")
	})

	t.Run("the rule is removed", func(t *testing.T) {
		w, ev, inv := newRig(dirFlaws{})
		require.Equal(t, ngac.DecisionDeny, ask(ev, wid("lim"), wid("dir_a2")).Decision)

		w.g.RemoveProhibition(vocab.DirectoryLimitedProhibitionName("w"))
		inv.InvalidateForNodes(ctx, wid("limited"), wid("open"))
		assert.Equal(t, ngac.DecisionAllow, ask(ev, wid("lim"), wid("dir_a2")).Decision)
	})
}

// The model is described in the spec by these names; keep the test and the spec
// from drifting apart on the one rule that is not an association.
func TestContactsVisibility_RuleShape(t *testing.T) {
	p := dirRule("w")
	assert.Equal(t, vocab.DirectoryLimitedProhibitionName("w"), p.Name)
	assert.Equal(t, []string{vocab.OpRead}, p.Operations, "only read is prohibited, so manage (the admin shortcut) is untouched")
	assert.False(t, p.Intersection)
	assert.True(t, strings.HasPrefix(p.SubjectID, "w:"))
	assert.Equal(t, []string{wid("open")}, p.TargetOAIDs, "Open is the only target: owners and peers are outside it")
}

// A user missing from memory takes the SQL fallback on both paths, whether or not
// the object is in memory: the single and the batch check must agree.
func TestContactsVisibility_MissingUserAgreesOnSingleAndBatch(t *testing.T) {
	ctx := context.Background()
	ghostUser := wid("ghost")
	inMemory := wid("dir_a2")
	build := func(noRule bool) ngac.DecisionEngine {
		w := buildDirectory(t, dirFlaws{noRule: noRule})
		if noRule {
			w.g.RemoveProhibition(vocab.DirectoryLimitedProhibitionName("v"))
		}
		cte := &fakeCTE{
			allowed:   map[string]bool{ghostUser + "|" + inMemory + "|read": true},
			ancestors: map[string]map[string]string{ghostUser: dbAbove(wid("members"), wid("pc"))},
		}
		return ngac.NewDecisionEngine(w.g, cte)
	}
	both := func(e ngac.DecisionEngine) (bool, bool) {
		single := e.Decide(ctx, dirReq(ghostUser, inMemory, vocab.OpRead)).Decision == ngac.DecisionAllow
		batch := e.(batchDecider).DecideBatch(ctx, ngac.BatchAccessRequest{
			UserNodeID: ghostUser, ObjectNodeIDs: []string{inMemory}, Operations: []string{vocab.OpRead},
		})[inMemory][vocab.OpRead]
		return single, batch
	}

	// No prohibition covers the ghost: SQL's ALLOW stands on both paths.
	single, batch := both(build(true))
	assert.True(t, single, "single path: SQL allows")
	assert.True(t, batch, "batch path agrees with the single path")

	// A covering prohibition in the database's ancestors: DENY on both paths.
	w := buildDirectory(t, dirFlaws{})
	cte := &fakeCTE{
		allowed:   map[string]bool{ghostUser + "|" + inMemory + "|read": true},
		ancestors: map[string]map[string]string{ghostUser: dbAbove(wid("role_limited"), wid("limited"), wid("members"), wid("pc"))},
	}
	e := ngac.NewDecisionEngine(w.g, cte)
	single, batch = both(e)
	assert.False(t, single, "single path: the limit applies to a user missing from memory")
	assert.False(t, batch, "batch path agrees")
}

// The fallback matches prohibitions with the same semantics as memory does,
// intersection included: every one of the prohibition's OAs must be above the object.
func TestContactsVisibility_CTEFallbackHonoursIntersectionProhibitions(t *testing.T) {
	g := failClosedGraphWith(t, &ngac.Prohibition{
		Name: "needs-both", SubjectID: "ua-staff", Operations: []string{"read"},
		TargetOAIDs: []string{"oa-docs", "oa-secret"}, Intersection: true,
	})
	cte := &fakeCTE{
		allowed: map[string]bool{"u-alice|o-both|read": true, "u-alice|o-one|read": true},
		ancestors: map[string]map[string]string{
			"o-both": {"oa-docs": "OA", "oa-secret": "OA"},
			"o-one":  {"oa-docs": "OA"},
		},
	}
	e := ngac.NewDecisionEngine(g, cte)
	assert.Equal(t, ngac.DecisionDeny, decide(t, e, "u-alice", "o-both", "read").Decision, "under both")
	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "u-alice", "o-one", "read").Decision, "under one only")
	res := batchOf(t, e, "u-alice", []string{"o-both", "o-one"}, []string{"read"})
	assert.False(t, res["o-both"]["read"])
	assert.True(t, res["o-one"]["read"])
}
