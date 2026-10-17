package domain_test

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// Joining a workspace is a graph write first and a row after it: the graph is the
// truth, the row follows it. Every test here has an allow on the same fixture, so a
// join that always failed would not pass.

func assignment(child, parent string) *policypb.CreateAssignmentRequest {
	return &policypb.CreateAssignmentRequest{ChildId: child, ParentId: parent}
}

func membersOf(ws string) string { return "node-" + ngac.MembersUAName(ngac.WorkspaceID(ws)) }
func ownersOf(ws string) string  { return "node-" + ngac.OwnersUAName(ngac.WorkspaceID(ws)) }
func tenantMemberOf(ws string) string {
	return "node-" + ngac.TenantMemberUAName(ngac.WorkspaceID(ws))
}

// A workspace made without its TenantMember UA (every personal one, and the legacy
// organizations) still takes a member: they are assigned under its Members UA directly.
func TestJoinTenant_WorksWithoutTheTenantMemberUA(t *testing.T) {
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ctx := context.Background()
	ws := w.addTenant("Acme", "")
	u := w.addUser("join1@example.test")

	w.noTenantUAs = true
	if err := svc.JoinTenantForTest(ctx, ws, u.ID, u.NGACNodeID, "member"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if !w.assigned(u.NGACNodeID, membersOf(ws)) {
		t.Errorf("the member is not under %s; assignments: %v", membersOf(ws), w.assignments())
	}
	if w.assigned(u.NGACNodeID, tenantMemberOf(ws)) {
		t.Error("a TenantMember UA that does not exist was assigned")
	}
	if m := w.membership(ws, u.ID); m == nil || m.Status != "active" || m.Role != "member" {
		t.Errorf("row = %+v, want an active member", m)
	}

	// Allow, same shape: where the TenantMember UA exists the member is under it too.
	w.noTenantUAs = false
	ws2 := w.addTenant("Beta", "")
	if err := svc.JoinTenantForTest(ctx, ws2, u.ID, u.NGACNodeID, "member"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if !w.assigned(u.NGACNodeID, membersOf(ws2)) || !w.assigned(u.NGACNodeID, tenantMemberOf(ws2)) {
		t.Errorf("want Members and TenantMember edges; assignments: %v", w.assignments())
	}
}

// An owner is assigned under the workspace's Owners UA, which is under Members.
func TestJoinTenant_AnOwnerGoesUnderOwners(t *testing.T) {
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ws := w.addTenant("Acme", "")
	u := w.addUser("join2@example.test")
	w.noTenantUAs = true

	if err := svc.JoinTenantForTest(context.Background(), ws, u.ID, u.NGACNodeID, "owner"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if !w.assigned(u.NGACNodeID, ownersOf(ws)) {
		t.Errorf("owner not under Owners; assignments: %v", w.assignments())
	}
	if m := w.membership(ws, u.ID); m == nil || m.Role != "owner" {
		t.Errorf("row = %+v, want an owner", m)
	}
}

// The edge exists before the row does: a row written first and an edge that then fails
// is the drift this change removes.
func TestJoinTenant_TheEdgeIsWrittenBeforeTheRow(t *testing.T) {
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ws := w.addTenant("Acme", "")
	u := w.addUser("join3@example.test")
	var edgeAtInsert bool
	w.beforeInsert = func(tenantID, nodeID string) { edgeAtInsert = w.assigned(nodeID, membersOf(tenantID)) }

	if err := svc.JoinTenantForTest(context.Background(), ws, u.ID, u.NGACNodeID, "member"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if !edgeAtInsert {
		t.Error("the row was written before the Members edge")
	}
}

// A graph write that fails leaves no row and is reported; with the graph healthy the
// same join leaves both.
func TestJoinTenant_AFailedGraphWriteLeavesNoRow(t *testing.T) {
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ctx := context.Background()
	ws := w.addTenant("Acme", "")
	denied := w.addUser("join4@example.test")
	fine := w.addUser("join5@example.test")
	w.assignErr = func(child, _ string) error {
		if child == denied.NGACNodeID {
			return errors.New("policy: unavailable")
		}
		return nil
	}

	if err := svc.JoinTenantForTest(ctx, ws, denied.ID, denied.NGACNodeID, "member"); err == nil {
		t.Error("a join whose graph write failed reported success")
	}
	if m := w.membership(ws, denied.ID); m != nil {
		t.Errorf("row = %+v, want none: the graph has not agreed", m)
	}

	if err := svc.JoinTenantForTest(ctx, ws, fine.ID, fine.NGACNodeID, "member"); err != nil {
		t.Fatalf("join: %v", err)
	}
	if m := w.membership(ws, fine.ID); m == nil || m.Status != "active" {
		t.Errorf("row = %+v, want active", m)
	}
}

// A row that cannot be written takes back the edges this call made, and only those.
func TestJoinTenant_ARowFailureUndoesOnlyTheEdgesThisCallMade(t *testing.T) {
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ctx := context.Background()
	ws := w.addTenant("Acme", "")
	fresh := w.addUser("join6@example.test")
	already := w.addUser("join7@example.test")
	// already is under Members before the join (an invitation accepted a moment ago).
	if _, err := (&fakePolicyWrite{w: w}).CreateAssignment(ctx, assignment(already.NGACNodeID, membersOf(ws))); err != nil {
		t.Fatal(err)
	}
	w.insertErr = errors.New("database: down")

	if err := svc.JoinTenantForTest(ctx, ws, fresh.ID, fresh.NGACNodeID, "member"); err == nil {
		t.Error("a join whose row failed reported success")
	}
	if w.assigned(fresh.NGACNodeID, membersOf(ws)) || w.assigned(fresh.NGACNodeID, tenantMemberOf(ws)) {
		t.Errorf("edges this call made were left behind; assignments: %v", w.assignments())
	}

	if err := svc.JoinTenantForTest(ctx, ws, already.ID, already.NGACNodeID, "member"); err == nil {
		t.Error("a join whose row failed reported success")
	}
	if !w.assigned(already.NGACNodeID, membersOf(ws)) {
		t.Error("the Members edge that was there before the join was removed")
	}
	for _, a := range w.assignments() {
		if a == "-"+already.NGACNodeID+">"+membersOf(ws) {
			t.Error("the join removed an edge it did not create")
		}
	}
}
