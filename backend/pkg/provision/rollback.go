// Package provision compensates for multi-step provisioning that fails part way.
//
// Provisioning a workspace, department or channel is a run of separate writes to
// the policy service followed by a database insert. None of them are in one
// transaction, so a failure at step N used to leave steps 1..N-1 behind: nodes
// no row refers to, that no later request will ever clean up, and that sit in
// the graph deciding access. A Rollback records how to undo each step as it
// succeeds and, on failure, undoes them newest first.
package provision

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"

	policypb "ngac-platform/proto/policy"
)

// undoTimeout bounds the whole compensation. It runs on a context detached from
// the request's: the commonest reason a step fails is that the request was
// cancelled, and a compensation that dies with it would leave the very orphans
// it exists to remove.
const undoTimeout = 15 * time.Second

// NodeDeleter is the part of the policy write client compensation needs.
type NodeDeleter interface {
	DeleteNode(ctx context.Context, in *policypb.DeleteNodeRequest, opts ...grpc.CallOption) (*policypb.Empty, error)
}

type step struct {
	label string
	undo  func(ctx context.Context) error
}

// Rollback is a stack of undo actions. The zero value is ready to use. It is not
// safe for concurrent use: one provisioning call owns one Rollback.
type Rollback struct {
	steps []step
}

// Add records how to undo a step that has just succeeded.
func (r *Rollback) Add(label string, undo func(ctx context.Context) error) {
	r.steps = append(r.steps, step{label: label, undo: undo})
}

// NodeCreated records a node that has just been created. Undoing it deletes the
// node, which removes every assignment and association that touches it.
func (r *Rollback) NodeCreated(w NodeDeleter, nodeID, label string) {
	if nodeID == "" {
		return
	}
	r.Add(label, func(ctx context.Context) error {
		_, err := w.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: nodeID})
		return err
	})
}

// Len reports how many undo actions are recorded.
func (r *Rollback) Len() int { return len(r.steps) }

// Discard forgets every recorded step: the provisioning succeeded and there is
// nothing left to undo.
func (r *Rollback) Discard() { r.steps = nil }

// Run undoes every recorded step, newest first, and empties the stack. A step
// that fails to undo does not stop the ones before it: each orphan removed is
// one fewer left behind. The returned error joins every undo failure and is nil
// when everything was undone; callers log it next to the error that caused the
// rollback rather than replacing that error.
func (r *Rollback) Run(ctx context.Context) error {
	if len(r.steps) == 0 {
		return nil
	}
	steps := r.steps
	r.steps = nil

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), undoTimeout)
	defer cancel()

	var errs []error
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if err := s.undo(ctx); err != nil {
			slog.Error("provisioning rollback step failed; an orphan may remain", "step", s.label, "error", err)
			errs = append(errs, fmt.Errorf("undo %s: %w", s.label, err))
		}
	}
	return errors.Join(errs...)
}

// Fail rolls back and returns cause, with a note when the rollback itself was
// incomplete. It is the one-line exit for a failed step:
//
//	return nil, rb.Fail(ctx, fmt.Errorf("create channels OA: %w", err))
func (r *Rollback) Fail(ctx context.Context, cause error) error {
	if rbErr := r.Run(ctx); rbErr != nil {
		return fmt.Errorf("%w (rollback incomplete: %v)", cause, rbErr)
	}
	return cause
}
