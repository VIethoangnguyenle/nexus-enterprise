//go:build realpolicy

package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

// The adapter main wires into the approval service answers the membership question
// from the real policy service, through the writer connection, so a removal made a
// moment ago is already in the answer.
func TestRealPolicy_ApprovalGateFollowsTheGraph(t *testing.T) {
	read, write := testutil.StartPolicy(t)
	svc := domain.NewService(nil, newPolicyAdapter(read, write))
	w := testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)
	member := testutil.NewUser(t, write, w.Members)
	owner := testutil.NewUser(t, write, w.Owners)
	removed := testutil.NewUser(t, write, w.Members)
	stranger := testutil.NewUser(t, write, other.Members)
	ctx := func(node string) context.Context { return testutil.PolicyCaller(context.Background(), node) }

	// Allow: members and owners of the workspace.
	for name, node := range map[string]string{"member": member, "owner": owner, "soon removed": removed} {
		if err := svc.RequireMember(ctx(node), node, w.WorkspaceID); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Deny: removed a moment ago, a person of another workspace, a UA passed as a person.
	testutil.Unassign(t, write, removed, w.Members)
	for name, node := range map[string]string{
		"removed just now":      removed,
		"another workspace's":   stranger,
		"the Members UA itself": w.Members,
		"a node that is nobody": "no-such-node",
	} {
		if err := svc.RequireMember(ctx(node), node, w.WorkspaceID); !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("%s: err = %v, want ErrAccessDenied", name, err)
		}
	}
	// The same fixture, the one still in: the denials above are about the people.
	if err := svc.RequireMember(ctx(member), member, w.WorkspaceID); err != nil {
		t.Errorf("member after the removal of another: %v", err)
	}
}
