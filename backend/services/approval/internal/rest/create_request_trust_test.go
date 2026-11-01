package rest

import (
	"errors"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

// The body of POST /api/approval/requests cannot choose the request's
// department, and can file it only under a scope of the tenant.

func TestCreateRequest_BodyDepartmentIsIgnored(t *testing.T) {
	l := newLifecycle(t)
	if err := l.call(t, l.h.CreateRequest, "n-req", "tenant-a",
		`{"entity_type":"leave","department_id":"n-req"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, r := range l.store.requests {
		if r.DepartmentID != "d-req" {
			t.Errorf("department = %q, want the requester's own d-req", r.DepartmentID)
		}
	}
}

func TestCreateRequest_ForgedScopeIsForbidden(t *testing.T) {
	l := newLifecycle(t)
	err := l.call(t, l.h.CreateRequest, "n-req", "tenant-a", `{"entity_type":"leave","scope_oa_id":"oa-of-another-tenant"}`)
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusForbidden {
		t.Fatalf("err = %v, want 403", err)
	}
	if len(l.store.requests) != 0 {
		t.Errorf("%d requests stored", len(l.store.requests))
	}
	if err := l.call(t, l.h.CreateRequest, "n-req", "tenant-a", `{"entity_type":"leave","scope_oa_id":"oa-own"}`); err != nil {
		t.Fatalf("own scope: %v", err)
	}
}

func TestApprove_RequesterCannotApproveTheirOwnRequest(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)
	err := l.call(t, l.h.ApproveAction, "n-req", "tenant-a", body(map[string]any{"request_id": id}))
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusForbidden {
		t.Fatalf("self approve err = %v, want 403", err)
	}
	err = l.call(t, l.h.RejectAction, "n-req", "tenant-a", body(map[string]any{"request_id": id, "comment": "x"}))
	if !errors.As(err, &he) || he.Code != http.StatusForbidden {
		t.Fatalf("self reject err = %v, want 403", err)
	}
}
