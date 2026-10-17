//go:build realpolicy

package testutil_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

// A member holds what Members is associated with; a stranger holds nothing. Both
// halves run on the same fixture so an unknown node cannot fake the deny.
func TestSeededWorkspaceDecisions(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	ws := testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)

	member := testutil.NewUser(t, write, ws.Members)
	owner := testutil.NewUser(t, write, ws.Owners)
	stranger := testutil.NewUser(t, write)
	elsewhere := testutil.NewUser(t, write, other.Members)

	// Allow.
	if !testutil.Allowed(t, read, member, ws.Channels, ngac.OpRead) {
		t.Error("member must read Channels")
	}
	if !testutil.Allowed(t, read, owner, ws.Mgmt, ngac.OpManage) {
		t.Error("owner must manage Mgmt")
	}
	// Deny, on the same fixture.
	if testutil.Allowed(t, read, member, ws.Mgmt, ngac.OpRead) {
		t.Error("member must not read Mgmt")
	}
	if testutil.Allowed(t, read, stranger, ws.Channels, ngac.OpRead) {
		t.Error("a user in no workspace must not read Channels")
	}
	if testutil.Allowed(t, read, elsewhere, ws.Channels, ngac.OpRead) {
		t.Error("a member of another workspace must not read this one's Channels (different PC)")
	}
	if !testutil.Allowed(t, read, elsewhere, other.Channels, ngac.OpRead) {
		t.Error("the same user must read the Channels of the workspace they belong to")
	}
}

// A decision follows a graph write: deny before the association, allow after, and
// deny again once it is removed. This is the harness's reason to exist (a fake
// that answers from a table cannot show the policy reacting to a write).
func TestDecisionFollowsGraphWrites(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	ws := testutil.SeedWorkspaceGraph(t, write)
	member := testutil.NewUser(t, write, ws.Members)
	oa := testutil.NewNode(t, write, "OA_rp_"+ws.WorkspaceID, ngac.TypeOA, ws.PC)

	if testutil.Allowed(t, read, member, oa, ngac.OpRead) {
		t.Fatal("deny expected before any association")
	}
	testutil.Associate(t, write, ws.Members, oa, []string{ngac.OpRead})
	if !testutil.Allowed(t, read, member, oa, ngac.OpRead) {
		t.Fatal("allow expected after the association")
	}
	if testutil.Allowed(t, read, member, oa, ngac.OpWrite) {
		t.Fatal("an operation that was not associated must stay denied")
	}

	if _, err := write.RemoveAssociation(testutil.PolicyAdmin(context.Background()), &policypb.RemoveAssociationRequest{UaId: ws.Members, OaId: oa}); err != nil {
		t.Fatal(err)
	}
	if testutil.Allowed(t, read, member, oa, ngac.OpRead) {
		t.Fatal("deny expected once the association is removed")
	}
}

// The process must not touch the developer's Redis or Redpanda: its caches and
// topics are shared with real services.
func TestPolicyProcessUsesNoSharedInfrastructure(t *testing.T) {
	testutil.StartPolicy(t)
	log := testutil.PolicyLog()

	if !strings.Contains(log, "redis unavailable") {
		t.Errorf("policy should have tried the unreachable Redis and reported it unavailable; log:\n%s", log)
	}
	if !strings.Contains(log, "kafka unavailable") {
		t.Errorf("policy should have tried the unreachable Kafka and reported it unavailable; log:\n%s", log)
	}
	// The realtime producer is created without dialling, so it logs "ready"
	// whatever the address; what matters is that it was given the dead one.
	for _, shared := range []string{":19092", ":6379", "localhost", "redpanda"} {
		if strings.Contains(log, shared) {
			t.Errorf("policy log mentions shared infrastructure %q; log:\n%s", shared, log)
		}
	}
	if strings.Contains(log, "kafka producer connected") {
		t.Errorf("policy connected to Kafka; log:\n%s", log)
	}
}
