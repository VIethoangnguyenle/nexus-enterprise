package grpcutil_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/grpcutil"
	"ngac-platform/pkg/httputil"
)

var errConflict = errors.New("step not active")

func TestStatusKeepsTheDomainsOwnRefusals(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"not found", httputil.ErrNotFound, codes.NotFound},
		{"denied", httputil.ErrAccessDenied, codes.PermissionDenied},
		{"exists", httputil.ErrAlreadyExists, codes.AlreadyExists},
		{"invalid", httputil.ErrInvalidInput, codes.InvalidArgument},
		{"wrapped", fmt.Errorf("create channel: %w", httputil.ErrAccessDenied), codes.PermissionDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := grpcutil.Status(tc.err)
			if status.Code(got) != tc.want {
				t.Fatalf("code = %v, want %v", status.Code(got), tc.want)
			}
			if status.Convert(got).Message() != tc.err.Error() {
				t.Errorf("message = %q, want %q", status.Convert(got).Message(), tc.err.Error())
			}
		})
	}
}

func TestStatusExtraMappingsAndTheirPrecedence(t *testing.T) {
	got := grpcutil.Status(errConflict, grpcutil.Mapping{Is: errConflict, Code: codes.FailedPrecondition})
	if status.Code(got) != codes.FailedPrecondition {
		t.Fatalf("code = %v, want FailedPrecondition", status.Code(got))
	}

	// An extra refines a shared sentinel.
	refined := fmt.Errorf("%w: %w", httputil.ErrNotFound, errConflict)
	got = grpcutil.Status(refined, grpcutil.Mapping{Is: errConflict, Code: codes.FailedPrecondition})
	if status.Code(got) != codes.FailedPrecondition {
		t.Fatalf("extra must win over the shared sentinel, got %v", status.Code(got))
	}
}

// An error that is not the domain's own is ours: the caller learns nothing
// about it, the log still can.
func TestStatusHidesInternalFailures(t *testing.T) {
	raw := errors.New(`ERROR: relation "ngac_nodes" does not exist (SQLSTATE 42P01)`)
	got := grpcutil.Status(fmt.Errorf("insert node: %w", raw))

	if status.Code(got) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(got))
	}
	wire := status.Convert(got).Message()
	if wire != httputil.InternalMessage {
		t.Errorf("wire message = %q, want %q", wire, httputil.InternalMessage)
	}
	if strings.Contains(wire, "SQLSTATE") || strings.Contains(wire, "ngac_nodes") {
		t.Errorf("internals reached the wire: %q", wire)
	}
	if !strings.Contains(got.Error(), "SQLSTATE 42P01") {
		t.Errorf("the detail must stay in the error text for the log, got %q", got.Error())
	}
	if !errors.Is(got, raw) {
		t.Error("the cause must stay reachable with errors.Is")
	}
}

func TestInternalIsGenericOnTheWire(t *testing.T) {
	raw := errors.New("pq: SQLSTATE 08006")
	got := grpcauth.Internal(fmt.Errorf("list: %w", raw))
	if status.Code(got) != codes.Internal || status.Convert(got).Message() != httputil.InternalMessage {
		t.Fatalf("got %v", got)
	}
}

func TestStatusNil(t *testing.T) {
	if grpcutil.Status(nil) != nil {
		t.Fatal("nil in, nil out")
	}
}

func TestIsFailedPrecondition(t *testing.T) {
	if !grpcutil.IsFailedPrecondition(status.Error(codes.FailedPrecondition, "x")) {
		t.Error("FailedPrecondition is one")
	}
	if grpcutil.IsFailedPrecondition(status.Error(codes.Unavailable, "x")) || grpcutil.IsFailedPrecondition(errors.New("x")) || grpcutil.IsFailedPrecondition(nil) {
		t.Error("only FailedPrecondition is one")
	}
}
