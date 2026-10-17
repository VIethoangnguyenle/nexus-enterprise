//go:build realpolicy

package testutil

import (
	"context"
	"testing"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// WorkspaceGraph holds the node ids of one seeded workspace topology.
type WorkspaceGraph struct {
	WorkspaceID string
	PC          string
	Owners      string
	Members     string
	Mgmt        string
	Documents   string
	Channels    string
}

// graphBuilder creates nodes through the write client and removes them when the
// test ends, newest first, so a run leaves the shared test database as it found
// it.
type graphBuilder struct {
	t     testing.TB
	write policypb.PolicyWriteServiceClient
}

func (b graphBuilder) node(name, nodeType string, props map[string]string) string {
	b.t.Helper()
	n, err := b.write.CreateNode(context.Background(), &policypb.CreateNodeRequest{
		Name: name, NodeType: nodeType, Properties: props,
	})
	if err != nil {
		b.t.Fatalf("create %s %s: %v", nodeType, name, err)
	}
	b.t.Cleanup(func() {
		// Best effort: a node already removed by the test is not an error here.
		_, _ = b.write.DeleteNode(context.Background(), &policypb.DeleteNodeRequest{NodeId: n.GetId()})
	})
	return n.GetId()
}

// Assign makes child a child of parent. A user's membership is an assignment.
func Assign(t testing.TB, write policypb.PolicyWriteServiceClient, child, parent string) {
	t.Helper()
	if _, err := write.CreateAssignment(context.Background(), &policypb.CreateAssignmentRequest{ChildId: child, ParentId: parent}); err != nil {
		t.Fatalf("assign %s -> %s: %v", child, parent, err)
	}
}

// Unassign removes the assignment of child to parent.
func Unassign(t testing.TB, write policypb.PolicyWriteServiceClient, child, parent string) {
	t.Helper()
	if _, err := write.RemoveAssignment(PolicyAdmin(context.Background()), &policypb.RemoveAssignmentRequest{ChildId: child, ParentId: parent}); err != nil {
		t.Fatalf("unassign %s -> %s: %v", child, parent, err)
	}
}

// Associate lets ua perform ops on oa.
func Associate(t testing.TB, write policypb.PolicyWriteServiceClient, ua, oa string, ops []string) {
	t.Helper()
	if _, err := write.CreateAssociation(PolicyAdmin(context.Background()), &policypb.CreateAssociationRequest{UaId: ua, OaId: oa, Operations: ops}); err != nil {
		t.Fatalf("associate %s -> %s: %v", ua, oa, err)
	}
}

// NewNode creates one node of nodeType under the given parents and returns its
// id. The node is removed when the test ends.
func NewNode(t testing.TB, write policypb.PolicyWriteServiceClient, name, nodeType string, parents ...string) string {
	t.Helper()
	id := graphBuilder{t: t, write: write}.node(name, nodeType, nil)
	for _, p := range parents {
		Assign(t, write, id, p)
	}
	return id
}

// NewUser creates a user node (unique name) under the given UAs.
func NewUser(t testing.TB, write policypb.PolicyWriteServiceClient, parents ...string) string {
	t.Helper()
	return NewNode(t, write, uniqueID("U_rp"), ngac.TypeU, parents...)
}

// SeedWorkspaceGraph builds the topology CreateWorkspace builds, in the running
// policy process, with ngac.* helpers for names and operation sets so a change
// to either follows into every test:
//
//	PC -> Owners, Members, Mgmt, Documents, Channels;  Owners -> Members
//	Owners:  Mgmt, Documents (owner ops), Channels (owner channel ops)
//	Members: Documents (member document ops), Channels (member channel ops)
//
// Owners is assigned under Members, so an owner inherits the member grants.
func SeedWorkspaceGraph(t testing.TB, write policypb.PolicyWriteServiceClient) WorkspaceGraph {
	t.Helper()
	wsID := uniqueID("rp-ws")
	id := ngac.WorkspaceID(wsID)
	b := graphBuilder{t: t, write: write}

	g := WorkspaceGraph{WorkspaceID: wsID}
	g.PC = b.node(ngac.PCName(id), ngac.TypePC, map[string]string{
		"workspace": wsID, "workspace_id": wsID, "scope": "tenant", "tenant_id": wsID,
	})
	g.Owners = b.node(ngac.OwnersUAName(id), ngac.TypeUA, nil)
	g.Members = b.node(ngac.MembersUAName(id), ngac.TypeUA, nil)
	g.Mgmt = b.node(ngac.MgmtOAName(id), ngac.TypeOA, nil)
	g.Documents = b.node(ngac.DocumentsOAName(id), ngac.TypeOA, nil)
	g.Channels = b.node(ngac.ChannelsOAName(id), ngac.TypeOA, nil)

	for _, a := range []struct{ child, parent string }{
		{g.Owners, g.PC}, {g.Members, g.PC},
		{g.Mgmt, g.PC}, {g.Documents, g.PC}, {g.Channels, g.PC},
		{g.Owners, g.Members},
	} {
		Assign(t, write, a.child, a.parent)
	}
	Associate(t, write, g.Owners, g.Mgmt, ngac.AllOwnerOps())
	Associate(t, write, g.Owners, g.Documents, ngac.AllOwnerOps())
	Associate(t, write, g.Owners, g.Channels, ngac.OwnerChannelOps())
	Associate(t, write, g.Members, g.Documents, ngac.MemberDocumentOps())
	Associate(t, write, g.Members, g.Channels, ngac.MemberChannelOps())
	return g
}

// Allowed asks the real policy service whether the user holds op on the OA. A
// transport failure fails the test: a deny assertion must never pass because the
// policy was unreachable.
func Allowed(t testing.TB, read policypb.PolicyReadServiceClient, userNodeID, oaNodeID, op string) bool {
	t.Helper()
	resp, err := read.CheckAccess(PolicyCaller(context.Background(), userNodeID), &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: oaNodeID, Operation: op,
	})
	if err != nil {
		t.Fatalf("check %s %s on %s: %v", userNodeID, op, oaNodeID, err)
	}
	return ngac.Allowed(resp.GetDecision(), nil)
}
