package store_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
)

// everyoneIsInTheRole answers the policy questions of a tenant where each of the
// approvers belongs to the one role a step is assigned to.
type everyoneIsInTheRole struct{ role string }

func (p everyoneIsInTheRole) ResolveAccessibleScopes(context.Context, string, string) ([]string, error) {
	return []string{"oa"}, nil
}
func (p everyoneIsInTheRole) CheckAccess(context.Context, string, string, string) (bool, error) {
	return true, nil
}
func (p everyoneIsInTheRole) GetAncestors(context.Context, string) ([]string, error) {
	return []string{p.role}, nil
}
func (p everyoneIsInTheRole) GetMembers(context.Context, string) ([]string, error) { return nil, nil }

// twoApprovalsTemplate stores a one-step template for a role whose step needs two approvals.
func twoApprovalsTemplate(t *testing.T, ctx context.Context, role string, st *store.Store) {
	t.Helper()
	tmpl := &domain.Template{ID: uuid.NewString(), Name: "Two eyes", EntityType: "expense", IsActive: true, CreatedBy: "u",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Steps: []*domain.Step{{ID: uuid.NewString(), StepOrder: 1, Name: "Review", ApproverType: "role_in_dept", ApproverValue: role, RequiredCount: 2}}}
	if err := st.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("template: %v", err)
	}
}

func openRequests(t *testing.T, n int) (context.Context, *domain.Service, func(string) *domain.Request, []*domain.Request, *store.Store) {
	t.Helper()
	ctx, st, _ := scratchTenant(t)
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	t.Cleanup(cancel)
	role := "role-" + uuid.NewString()
	twoApprovalsTemplate(t, ctx, role, st)
	svc := domain.NewService(st, everyoneIsInTheRole{role})
	var reqs []*domain.Request
	for i := 0; i < n; i++ {
		r, err := svc.CreateApprovalRequest(ctx, domain.CreateRequestInput{
			EntityType: "expense", ScopeOAID: "oa", DepartmentID: "dept", CreatedBy: "creator",
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		reqs = append(reqs, r)
	}
	get := func(id string) *domain.Request {
		r, err := st.GetRequest(ctx, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		return r
	}
	return ctx, svc, get, reqs, st
}

// Two people approving at the same moment each count the other's decision: a
// step that needs two approvals advances, it is not left waiting for a third
// who is never asked.
func TestApprove_TwoConcurrentApprovalsMeetAQuorumOfTwo(t *testing.T) {
	ctx, svc, get, reqs, _ := openRequests(t, 12)

	var wg sync.WaitGroup
	errs := make(chan error, 2*len(reqs))
	for _, r := range reqs {
		for _, who := range []string{"alice", "bob"} {
			wg.Add(1)
			go func(id, who string) {
				defer wg.Done()
				errs <- svc.Approve(ctx, domain.ApproveInput{RequestID: id, UserNodeID: who})
			}(r.ID, who)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("approve: %v", err)
		}
	}
	for _, r := range reqs {
		if got := get(r.ID); got.Status != "approved" {
			t.Errorf("request %s is %q: both approvals counted only themselves and the step never completed", r.ID[:8], got.Status)
		}
	}
}

// An approval racing a rejection neither deadlocks nor leaves a mix: the
// request ends in exactly one terminal state, and a rejected one has nothing
// left pending.
func TestApproveAndRejectRacing_EndInOneConsistentState(t *testing.T) {
	ctx, svc, get, reqs, st := openRequests(t, 12)

	var wg sync.WaitGroup
	for _, r := range reqs {
		wg.Add(2)
		go func(id string) {
			defer wg.Done()
			err := svc.Approve(ctx, domain.ApproveInput{RequestID: id, UserNodeID: "alice"})
			if err != nil && !errors.Is(err, domain.ErrRequestCompleted) {
				t.Errorf("approve: %v", err)
			}
		}(r.ID)
		go func(id string) {
			defer wg.Done()
			err := svc.Reject(ctx, domain.RejectInput{RequestID: id, UserNodeID: "bob", Comment: "no"})
			if err != nil && !errors.Is(err, domain.ErrRequestCompleted) {
				t.Errorf("reject: %v", err)
			}
		}(r.ID)
	}
	wg.Wait()

	for _, r := range reqs {
		got := get(r.ID)
		switch got.Status {
		case "rejected":
			rows, err := st.ListAssignments(ctx, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range rows {
				if a.Status == "pending" {
					t.Errorf("request %s is rejected but still has a pending assignment", r.ID[:8])
				}
			}
		case "pending":
			// Only the approval won, and a quorum of two is not met by one.
		default:
			t.Errorf("request %s ended %q", r.ID[:8], got.Status)
		}
	}
}
