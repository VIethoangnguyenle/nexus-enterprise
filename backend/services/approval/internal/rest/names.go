package rest

import (
	"context"
	"log/slog"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/approval/internal/domain"
)

// NameResolver turns the ids stored on approval records (NGAC node ids,
// department ids) into the names people use for them.
type NameResolver interface {
	DisplayNames(ctx context.Context, tenantID string, keys []string) (map[string]string, error)
}

// WithNames makes every response carry display names beside the ids it holds,
// so the client shows people by name and never has to fall back on an id.
// Without a resolver responses are unchanged.
func (h *Handler) WithNames(n NameResolver) *Handler {
	h.names = n
	return h
}

// named is everything about to leave the handler that can carry a name.
type named struct {
	requests    []*domain.Request
	assignments []*domain.AssignmentRecord
	steps       []*domain.Step
	audit       []*domain.AuditEntry
	templates   []*domain.Template
}

// fillNames resolves every key in n with one lookup and writes the names
// back. A failed lookup is logged and the response goes out without names:
// they are a courtesy, and the client shows a neutral word in their place.
func (h *Handler) fillNames(ctx context.Context, n named) {
	if h.names == nil {
		return
	}
	seen := map[string]struct{}{}
	var keys []string
	add := func(k string) {
		if k == "" {
			return
		}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		keys = append(keys, k)
	}
	for _, r := range n.requests {
		add(r.CreatedBy)
		add(r.DepartmentID)
	}
	for _, a := range n.assignments {
		add(a.UserNodeID)
	}
	for _, s := range n.steps {
		add(s.ApproverValue)
	}
	for _, e := range n.audit {
		add(e.ActorNodeID)
	}
	for _, t := range n.templates {
		add(t.CreatedBy)
		for _, s := range t.Steps {
			add(s.ApproverValue)
		}
	}
	if len(keys) == 0 {
		return
	}
	// Names are looked up inside the caller's tenant only.
	names, err := h.names.DisplayNames(ctx, grpcauth.CallerFrom(ctx).TenantID, keys)
	if err != nil {
		slog.Warn("resolve approval display names", "error", err)
		return
	}
	for _, r := range n.requests {
		r.CreatedByName = names[r.CreatedBy]
		r.DepartmentName = names[r.DepartmentID]
	}
	for _, a := range n.assignments {
		a.UserName = names[a.UserNodeID]
	}
	for _, s := range n.steps {
		s.ApproverName = names[s.ApproverValue]
	}
	for _, e := range n.audit {
		e.ActorName = names[e.ActorNodeID]
	}
	for _, t := range n.templates {
		t.CreatedByName = names[t.CreatedBy]
		for _, s := range t.Steps {
			s.ApproverName = names[s.ApproverValue]
		}
	}
}

// withAssignments collects the requests and assignments of pending/history rows.
func withAssignments(items []*domain.RequestWithAssignment) named {
	var n named
	for _, it := range items {
		if it.Request != nil {
			n.requests = append(n.requests, it.Request)
		}
		if it.Assignment != nil {
			n.assignments = append(n.assignments, it.Assignment)
		}
	}
	return n
}
