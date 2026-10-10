package domain

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/reason"
)

// A refusal is classified by errors.Is and keeps the exact message the caller
// is shown, so the transports map it without parsing text.
func TestRefusalsAreClassifiedAndKeepTheirMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		is   error
		msg  string
	}{
		{"not found", notFound("item not found"), ErrNotFound, "item not found"},
		{"denied", denied("access denied"), ErrAccessDenied, "access denied"},
		{"invalid", invalid("invalid share permission: %q", "x"), ErrInvalidInput, `invalid share permission: "x"`},
		{"conflict", conflict("file not pending"), ErrConflict, "file not pending"},
		{"quota", quotaExceeded("storage quota exceeded"), ErrQuotaExceeded, "storage quota exceeded"},
		{"aborted", aborted("retry"), ErrAborted, "retry"},
		{"unavailable", unavailable("wait: %v", "x"), ErrUnavailable, "wait: x"},
		{"folder with documents", ErrFolderHasDocuments, ErrConflict, reason.FolderHasDocuments},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.is) {
				t.Errorf("errors.Is(%v, %v) = false", tc.err, tc.is)
			}
			if tc.err.Error() != tc.msg {
				t.Errorf("message = %q, want %q", tc.err.Error(), tc.msg)
			}
		})
	}
}

func TestRefusalsAreNotMistakenForEachOther(t *testing.T) {
	if errors.Is(notFound("x"), ErrAccessDenied) || errors.Is(denied("x"), ErrNotFound) {
		t.Error("distinct classes must not match")
	}
	if errors.Is(conflict("x"), ErrQuotaExceeded) || errors.Is(quotaExceeded("x"), ErrConflict) {
		t.Error("distinct classes must not match")
	}
	if errors.Is(errors.New("pq: boom"), ErrNotFound) {
		t.Error("an unclassified error is nobody's refusal")
	}
	// The shared sentinels are the platform's, not this package's own copies.
	if ErrNotFound != httputil.ErrNotFound || ErrAccessDenied != httputil.ErrAccessDenied || ErrInvalidInput != httputil.ErrInvalidInput {
		t.Error("domain sentinels must alias httputil's")
	}
}

type policyStub struct {
	policypb.PolicyReadServiceClient
	decision string
	err      error
}

func (p *policyStub) CheckAccess(context.Context, *policypb.CheckAccessRequest, ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &policypb.AccessDecision{Decision: p.decision}, nil
}

// Every way the policy answer can fail to be an ALLOW is a denial.
func TestCheckAccessFailsClosed(t *testing.T) {
	cases := []struct {
		name    string
		policy  *policyStub
		user    string
		wantErr bool
	}{
		{"allow", &policyStub{decision: ngac.DecisionAllow}, "u", false},
		{"deny", &policyStub{decision: ngac.DecisionDeny}, "u", true},
		{"unrecognised decision", &policyStub{decision: "MAYBE"}, "u", true},
		{"policy outage", &policyStub{err: status.Error(codes.Unavailable, "down")}, "u", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewService(nil, tc.policy, nil, nil)
			err := svc.checkAccess(context.Background(), tc.user, "oa-1", ngac.OpRead)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrAccessDenied) {
				t.Errorf("a refusal must be ErrAccessDenied, got %v", err)
			}
		})
	}
}
