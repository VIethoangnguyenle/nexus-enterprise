package rest

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// An access token names a tenant and outlives the membership it was issued for.
// Every route of the service therefore asks the graph, at the moment of the call,
// whether the person still belongs to that tenant: a removed person is refused at
// once, not when their token expires. The routes are read from the router, so a
// route added later is walked without a new test.

// membership answers the one question the gate asks.
type membership struct {
	noScopes
	member bool
	err    error
}

func (m membership) IsWorkspaceMember(context.Context, string, string) (bool, error) {
	return m.member, m.err
}

func TestMembershipGate_EveryRouteRefusesAPersonTheGraphNoLongerHolds(t *testing.T) {
	w := newWorkspaceEnv(t, "organization", true)

	// Allow: the same person, tenant and schema, while the graph holds them.
	w.e = routerWith(w.pool, membership{member: true})
	for _, r := range []walkedRoute{{"GET", "GET /api/approval/templates", "/api/approval/templates"}, {"GET", "GET /api/approval/pending", "/api/approval/pending"}} {
		if rec := w.do(t, r); rec.Code != http.StatusOK {
			t.Fatalf("%s as a member: status %d (%s), want 200", r.route, rec.Code, rec.Body)
		}
	}

	// Deny: removed from the workspace, every route says so.
	w.e = routerWith(w.pool, membership{member: false})
	routes := w.routes()
	if len(routes) < 15 {
		t.Fatalf("only %d routes walked; the router lost some", len(routes))
	}
	for _, r := range routes {
		if rec := w.do(t, r); rec.Code != http.StatusForbidden {
			t.Errorf("%s as a removed person: status %d (%s), want 403", r.route, rec.Code, rec.Body)
		}
	}
}

// The policy not answering stops the call too; it is not a yes, and it is a 503 the client can retry.
func TestMembershipGate_AnOutageOfThePolicyIsNotAnAllow(t *testing.T) {
	w := newWorkspaceEnv(t, "organization", true)
	w.e = routerWith(w.pool, membership{err: errors.New("policy: connection refused")})

	for _, r := range w.routes() {
		if rec := w.do(t, r); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s with the policy down: status %d, want 503 (try again), never a pass", r.route, rec.Code)
		}
	}
}
