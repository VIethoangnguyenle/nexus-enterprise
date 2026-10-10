package domain

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// DirectoryStore is where the people behind user nodes are read from: names,
// pictures, email, standing in the workspace. The graph knows who belongs;
// this knows what to call them.
type DirectoryStore interface {
	ProfilesByNodeIDs(ctx context.Context, tenantID string, nodeIDs []string) (map[string]*store.Profile, error)
	EnsureTenantUser(ctx context.Context, tenantID, userID, nodeID string) error
	RemoveTenantUser(ctx context.Context, tenantID, nodeID string) error
}

// WithDirectory wires in the people directory. Operations that show or invite
// people need it; the rest of the service does not.
func (s *Service) WithDirectory(d DirectoryStore) *Service {
	s.directory = d
	return s
}

// neutralPerson is what a member is called when nothing readable is known about
// them. An identifier is never a name.
const neutralPerson = "Thành viên"

var uuidPattern = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// readableName returns the first candidate a person could be shown: not empty,
// not an identifier and not the name of a platform node (U_…).
func readableName(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" || uuidPattern.MatchString(c) || strings.HasPrefix(c, "U_") {
			continue
		}
		return c
	}
	return neutralPerson
}

// RoleRef is a role as a pill: its name only.
type RoleRef struct {
	ID   string
	Name string
}

// DeptRef is a department as a label.
type DeptRef struct {
	ID   string
	Name string
}

// MemberView is a row of the people table.
type MemberView struct {
	NodeID      string
	UserID      string // colour key for the avatar; never shown
	DisplayName string
	Email       string
	AvatarURL   string
	Title       string
	Status      string
	Owner       bool
	Department  *DeptRef
	Roles       []RoleRef
}

// profiles reads the people behind some nodes. Without a directory, or when it
// fails, the answer is empty and names fall back to what the graph holds: a
// table with plainer names is better than none.
func (s *Service) profiles(ctx context.Context, wsID string, nodes []*policypb.NGACNode) map[string]*store.Profile {
	if s.directory == nil || len(nodes) == 0 {
		return map[string]*store.Profile{}
	}
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.Id)
	}
	got, err := s.directory.ProfilesByNodeIDs(ctx, wsID, ids)
	if err != nil {
		slog.Warn("could not read member profiles; showing graph names", "workspace", wsID, "error", err)
		return map[string]*store.Profile{}
	}
	return got
}

// peopleFor names some user nodes.
func (s *Service) peopleFor(ctx context.Context, wsID string, nodes []*policypb.NGACNode) []*PersonRef {
	profs := s.profiles(ctx, wsID, nodes)
	out := make([]*PersonRef, 0, len(nodes))
	for _, n := range nodes {
		p := profs[n.Id]
		ref := &PersonRef{NodeID: n.Id, DisplayName: readableName(ngac.DisplayName(n.Name, n.Properties))}
		if p != nil {
			ref.UserID = p.UserID
			ref.AvatarURL = p.AvatarURL
			ref.DisplayName = readableName(p.DisplayName, p.Username, ngac.DisplayName(n.Name, n.Properties))
		}
		out = append(out, ref)
	}
	return out
}

// requireMemberUser confirms nodeID is a user who already belongs to the
// workspace: a U node whose ancestors include the workspace's PC. The role and
// department routes take node IDs from the client, and holding manage must not
// be a way to bring somebody in; inviting is the invite operation's job.
func (s *Service) requireMemberUser(ctx context.Context, ws *WorkspaceResult, nodeID string) error {
	if nodeID == "" {
		return fmt.Errorf("%w: person required", ErrInvalidInput)
	}
	n, err := s.policyWrite.GetNode(ctx, &policypb.GetNodeRequest{NodeId: nodeID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return fmt.Errorf("%w: person not in this workspace", ErrNotFound)
		}
		return fmt.Errorf("resolve person: %w", err)
	}
	if n.GetNodeType() != ngac.TypeU {
		return fmt.Errorf("%w: person not in this workspace", ErrNotFound)
	}
	return s.requireInPC(ctx, ws, nodeID)
}

// requireInPC confirms the workspace's PC is among the node's ancestors.
func (s *Service) requireInPC(ctx context.Context, ws *WorkspaceResult, nodeID string) error {
	member, err := s.reachesPC(ctx, ws, nodeID)
	if err != nil {
		return err
	}
	if !member {
		return fmt.Errorf("%w: person not in this workspace", ErrNotFound)
	}
	return nil
}

func (s *Service) reachesPC(ctx context.Context, ws *WorkspaceResult, nodeID string) (bool, error) {
	anc, err := s.policyWrite.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: nodeID})
	if err != nil {
		return false, fmt.Errorf("resolve membership: %w", err)
	}
	for _, n := range anc.GetNodes() {
		if n.Id == ws.PcNodeID {
			return true, nil
		}
	}
	return false, nil
}

// ListMemberDirectory returns every person of the workspace as a row: name,
// email, department, roles, standing. The caller must hold manage on the Mgmt
// OA; the table carries email addresses and what each person may do.
func (s *Service) ListMemberDirectory(ctx context.Context, callerNodeID, wsID string) ([]*MemberView, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	people := uniqueNodes(userNodes(desc.GetNodes()))

	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}

	// Who holds each attribute: one read per owner/role/department.
	owners := map[string]bool{}
	rolesOf := map[string][]RoleRef{}
	for _, n := range children.GetNodes() {
		kind, ok := classifyRole(n, ws.ID)
		if !ok || kind == RoleMembers {
			continue
		}
		holders, err := s.roleHolders(ctx, kind, n.Id)
		if err != nil {
			return nil, err
		}
		for _, h := range holders {
			if kind == RoleOwners {
				owners[h.Id] = true
			} else {
				rolesOf[h.Id] = append(rolesOf[h.Id], RoleRef{ID: n.Id, Name: ngac.DisplayName(n.Name, n.Properties)})
			}
		}
	}
	deptOf := map[string]*DeptRef{}
	depts, err := s.deptStore.ListDepartmentsByWorkspace(ctx, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	for _, d := range depts {
		kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: d.NGACUaID})
		if err != nil {
			return nil, fmt.Errorf("get department members: %w", err)
		}
		for _, u := range userNodes(kids.GetNodes()) {
			if deptOf[u.Id] == nil {
				deptOf[u.Id] = &DeptRef{ID: d.ID, Name: d.Name}
			}
		}
	}

	profs := s.profiles(ctx, ws.ID, people)
	out := make([]*MemberView, 0, len(people))
	for _, n := range people {
		graphName := ngac.DisplayName(n.Name, n.Properties)
		v := &MemberView{
			NodeID:      n.Id,
			DisplayName: readableName(graphName),
			Status:      "active",
			Owner:       owners[n.Id],
			Department:  deptOf[n.Id],
			Roles:       rolesOf[n.Id],
		}
		if p := profs[n.Id]; p != nil {
			v.UserID = p.UserID
			v.DisplayName = readableName(p.DisplayName, p.Username, graphName)
			v.Email = p.Email
			v.AvatarURL = p.AvatarURL
			v.Title = p.Title
			if p.Status != "" {
				v.Status = p.Status
			}
		}
		sort.SliceStable(v.Roles, func(i, j int) bool { return strings.ToLower(v.Roles[i].Name) < strings.ToLower(v.Roles[j].Name) })
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName) })
	return out, nil
}

// maxEmailLen is the longest address SMTP allows.
const maxEmailLen = 254

// normalizeEmail trims and lower-cases an address and checks it is one.
func normalizeEmail(in string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(in))
	if e == "" || len(e) > maxEmailLen || strings.ContainsAny(e, " \t\r\n<>,;") {
		return "", fmt.Errorf("%w: not a valid email address", ErrInvalidInput)
	}
	a, err := mail.ParseAddress(e)
	if err != nil || a.Address != e || !strings.Contains(e[strings.LastIndex(e, "@")+1:], ".") {
		return "", fmt.Errorf("%w: not a valid email address", ErrInvalidInput)
	}
	return e, nil
}
