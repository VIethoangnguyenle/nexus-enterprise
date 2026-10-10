package domain

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/store"
)

func TestRefusalsAreClassifiedAndKeepTheirMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		is   error
		msg  string
	}{
		{"not found", notFound("asset not found"), ErrNotFound, "asset not found"},
		{"denied", denied("no %s access", "read"), ErrAccessDenied, "no read access"},
		{"invalid", invalid("name is too long"), ErrInvalidInput, "name is too long"},
		{"conflict", conflict("busy"), ErrConflict, "busy"},
		{"unauthenticated", unauthenticated("who"), ErrUnauthenticated, "who"},
		{"unavailable", unavailable("later"), ErrUnavailable, "later"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.is) {
				t.Errorf("errors.Is(%v, %v) = false", tc.err, tc.is)
			}
			if tc.err.Error() != tc.msg {
				t.Errorf("message = %q, want %q", tc.err.Error(), tc.msg)
			}
			if Reason(tc.err) != "" {
				t.Errorf("a plain refusal has no reason, got %q", Reason(tc.err))
			}
		})
	}
	if ErrNotFound != httputil.ErrNotFound || ErrAccessDenied != httputil.ErrAccessDenied || ErrInvalidInput != httputil.ErrInvalidInput {
		t.Error("domain sentinels must alias httputil's")
	}
}

func TestARefusalCarriesItsReasonThroughWrapping(t *testing.T) {
	err := Refuse(ErrConflict, ReasonStateChanged, "asset changed since it was read")
	if !errors.Is(err, ErrConflict) || Reason(err) != ReasonStateChanged {
		t.Fatalf("class %v, reason %q", errors.Is(err, ErrConflict), Reason(err))
	}
	wrapped := fmt.Errorf("transition: %w", err)
	if Reason(wrapped) != ReasonStateChanged || !errors.Is(wrapped, ErrConflict) {
		t.Error("wrapping must not hide the class or the reason")
	}
	if Reason(errors.New("boom")) != "" {
		t.Error("an unclassified error has no reason")
	}
}

// What the store refuses inside a transaction becomes a refusal the caller can
// act on; everything else stays a failure of ours with its cause attached for the log.
func TestStoreErrMapsTheStoresRefusals(t *testing.T) {
	cases := []struct {
		err    error
		is     error
		reason string
	}{
		{store.ErrNotFound, ErrNotFound, ""},
		{store.ErrAssetUnavailable, ErrConflict, ReasonAssetUnavailable},
		{store.ErrRequestNotOpen, ErrConflict, ReasonRequestNotOpen},
		{store.ErrStateChanged, ErrConflict, ReasonStateChanged},
		{store.ErrWrongType, ErrInvalidInput, ReasonWrongType},
		{store.ErrNotAMember, ErrInvalidInput, ReasonNotAMember},
		{store.ErrSameHolder, ErrInvalidInput, ReasonSameHolder},
	}
	for _, tc := range cases {
		got := storeErr(fmt.Errorf("tx: %w", tc.err), "asset")
		if !errors.Is(got, tc.is) || Reason(got) != tc.reason {
			t.Errorf("%v -> class ok=%v reason=%q, want reason %q", tc.err, errors.Is(got, tc.is), Reason(got), tc.reason)
		}
	}
	if storeErr(nil, "asset") != nil {
		t.Error("nil in, nil out")
	}
	cause := errors.New(`ERROR: deadlock detected (SQLSTATE 40P01)`)
	got := storeErr(cause, "hand over")
	for _, class := range []error{ErrNotFound, ErrAccessDenied, ErrInvalidInput, ErrConflict} {
		if errors.Is(got, class) {
			t.Errorf("an unexpected store failure must stay unclassified, matched %v", class)
		}
	}
	if !errors.Is(got, cause) {
		t.Error("the cause must stay reachable for the log")
	}
}

type policyStub struct {
	policypb.PolicyReadServiceClient
	decision string
	err      error
	calls    int
}

func (p *policyStub) CheckAccess(context.Context, *policypb.CheckAccessRequest, ...grpc.CallOption) (*policypb.AccessDecision, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &policypb.AccessDecision{Decision: p.decision}, nil
}

// Every way the policy answer can fail to be an ALLOW is a denial.
func TestAuthorizeFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		policy  *policyStub
		user    string
		object  string
		wantErr bool
	}{
		{"allow", &policyStub{decision: ngac.DecisionAllow}, "u", "oa", false},
		{"deny", &policyStub{decision: ngac.DecisionDeny}, "u", "oa", true},
		{"unrecognised decision", &policyStub{decision: "MAYBE"}, "u", "oa", true},
		{"outage", &policyStub{err: status.Error(codes.Unavailable, "down")}, "u", "oa", true},
		{"no caller", &policyStub{decision: ngac.DecisionAllow}, "", "oa", true},
		{"no object (a type without an OA)", &policyStub{decision: ngac.DecisionAllow}, "u", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := authorize(context.Background(), tc.policy, tc.user, tc.object, ngac.OpRead)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrAccessDenied) {
				t.Errorf("a refusal must be ErrAccessDenied, got %v", err)
			}
			if tc.user == "" || tc.object == "" {
				if tc.policy.calls != 0 {
					t.Errorf("no identity must not reach the policy service, calls=%d", tc.policy.calls)
				}
			}
		})
	}
}

// A missing row is "not found" with a message that names nothing of the
// database; any other lookup failure is ours and is not dressed as a 404.
func TestLookupErr_SeparatesAMissingRowFromAFailure(t *testing.T) {
	missing := lookupErr(fmt.Errorf("getting asset: %w", store.ErrNotFound), "asset")
	if !errors.Is(missing, ErrNotFound) || missing.Error() != "asset not found" {
		t.Errorf("missing row: %v", missing)
	}

	cause := errors.New(`ERROR: invalid byte sequence for encoding "UTF8": 0x00 (SQLSTATE 22021)`)
	failed := lookupErr(fmt.Errorf("getting asset: %w", cause), "asset")
	if errors.Is(failed, ErrNotFound) {
		t.Errorf("a database failure must not be reported as not found: %v", failed)
	}
	if !errors.Is(failed, cause) {
		t.Error("the cause must stay reachable for the log")
	}
}
