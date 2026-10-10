package provision_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"ngac-platform/pkg/provision"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

func create(t *testing.T, w *testutil.FakePolicyWrite, name string) string {
	t.Helper()
	n, err := w.CreateNode(context.Background(), &policypb.CreateNodeRequest{Name: name, NodeType: "OA"})
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	return n.Id
}

func TestRunUndoesNewestFirst(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	var rb provision.Rollback
	for _, name := range []string{"a", "b", "c"} {
		rb.NodeCreated(w, create(t, w, name), name)
	}

	if err := rb.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if live := w.LiveNodes(); len(live) != 0 {
		t.Errorf("nodes left behind: %v", live)
	}
	want := []string{"create OA a", "create OA b", "create OA c", "delete c", "delete b", "delete a"}
	if !reflect.DeepEqual(w.Log(), want) {
		t.Errorf("log = %v, want %v", w.Log(), want)
	}
	if rb.Len() != 0 {
		t.Error("a run empties the stack")
	}
}

func TestDiscardKeepsTheWork(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	var rb provision.Rollback
	rb.NodeCreated(w, create(t, w, "kept"), "kept")

	rb.Discard()
	if err := rb.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if live := w.LiveNodes(); !reflect.DeepEqual(live, []string{"OA kept"}) {
		t.Errorf("live = %v, want the kept node", live)
	}
}

// One undo failing must not strand the steps recorded before it.
func TestRunContinuesPastAFailedUndo(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	var rb provision.Rollback
	rb.NodeCreated(w, create(t, w, "first"), "first")
	rb.Add("broken", func(context.Context) error { return errors.New("boom") })
	rb.NodeCreated(w, create(t, w, "last"), "last")

	err := rb.Run(context.Background())

	if err == nil || !strings.Contains(err.Error(), "undo broken") {
		t.Fatalf("err = %v, want the failed undo reported", err)
	}
	if live := w.LiveNodes(); len(live) != 0 {
		t.Errorf("the nodes on either side of the failed undo must still be removed, left: %v", live)
	}
}

// A cancelled request is the commonest reason a step fails; the compensation
// must still run.
func TestRunSurvivesACancelledContext(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	var rb provision.Rollback
	rb.NodeCreated(w, create(t, w, "x"), "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := rb.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if live := w.LiveNodes(); len(live) != 0 {
		t.Errorf("left: %v", live)
	}
}

func TestFailReturnsTheCauseAndNotesAnIncompleteRollback(t *testing.T) {
	cause := errors.New("insert failed")

	var clean provision.Rollback
	if got := clean.Fail(context.Background(), cause); got != cause {
		t.Errorf("nothing to undo: got %v, want the cause as is", got)
	}

	var rb provision.Rollback
	rb.Add("stuck", func(context.Context) error { return errors.New("delete refused") })
	got := rb.Fail(context.Background(), cause)
	if !errors.Is(got, cause) || !strings.Contains(got.Error(), "rollback incomplete") {
		t.Errorf("got %v, want the cause wrapped with a rollback note", got)
	}
}

func TestNodeCreatedIgnoresAnEmptyID(t *testing.T) {
	var rb provision.Rollback
	rb.NodeCreated(testutil.NewFakePolicyWrite(), "", "nothing")
	if rb.Len() != 0 {
		t.Error("an empty node ID records nothing")
	}
}
