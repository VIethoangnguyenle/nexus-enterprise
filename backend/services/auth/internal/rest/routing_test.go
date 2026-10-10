package rest

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The auth service is reached through two gateways: Traefik in Docker and the
// Vite proxy in native dev. A route registered here that neither forwards 404s
// in one of them while working in the other, which is how GET /api/me and the
// directory went missing in Docker. These tests read both configurations and
// hold them to the route table.

const repoRoot = "../../../../../"

func read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(repoRoot + rel)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// traefikRule returns the value of one router's labelled attribute.
func traefikLabel(t *testing.T, compose, router, attr string) string {
	t.Helper()
	re := regexp.MustCompile(`traefik\.http\.routers\.` + router + `\.` + attr + `=(.*)"`)
	m := re.FindStringSubmatch(compose)
	if m == nil {
		t.Fatalf("no label for router %s %s", router, attr)
	}
	return m[1]
}

// ruleMatches evaluates the PathPrefix and PathRegexp terms of a rule.
func ruleMatches(rule, path string) bool {
	rule = strings.ReplaceAll(rule, "$$", "$") // compose escapes a literal dollar
	for _, m := range regexp.MustCompile("PathPrefix\\(`([^`]+)`\\)").FindAllStringSubmatch(rule, -1) {
		if strings.HasPrefix(path, m[1]) {
			return true
		}
	}
	for _, m := range regexp.MustCompile("PathRegexp\\(`([^`]+)`\\)").FindAllStringSubmatch(rule, -1) {
		if regexp.MustCompile(m[1]).MatchString(path) {
			return true
		}
	}
	return false
}

func TestTraefikRoutesEveryAuthRouteToTheAuthService(t *testing.T) {
	compose := read(t, "docker-compose.yml")
	authRule := traefikLabel(t, compose, "auth", "rule")
	wsRule := traefikLabel(t, compose, "workspace", "rule")

	a := newAcctApp(t, false, nil)
	for _, r := range a.e.Routes() {
		path := strings.NewReplacer(":id", "abc").Replace(r.Path)
		switch r.Method {
		case "GET", "POST", "PATCH", "PUT", "DELETE":
		default:
			continue // Echo's own not-found marker
		}
		if strings.Contains(path, "*") {
			continue // the group's catch-all, not a route anyone calls
		}
		if !ruleMatches(authRule, path) {
			t.Errorf("Traefik does not send %s %s to the auth service", r.Method, r.Path)
		}
	}

	// The directory sits under /api/workspaces, which the workspace router also
	// matches; the auth router must win it.
	if !ruleMatches(wsRule, "/api/workspaces/abc/contacts") {
		return // the workspace router does not overlap; priority is moot
	}
	prio := func(router string) string { return traefikLabel(t, compose, router, "priority") }
	if prio("auth") <= prio("workspace") {
		t.Errorf("auth priority %s must exceed workspace %s or the directory goes to the workspace service", prio("auth"), prio("workspace"))
	}
}

func TestTraefikDoesNotRouteTheAccountListingThatNoLongerExists(t *testing.T) {
	rule := traefikLabel(t, read(t, "docker-compose.yml"), "auth", "rule")
	if ruleMatches(rule, "/api/users") || ruleMatches(rule, "/api/users/lookup") {
		t.Error("the auth router still sends /api/users to a service that has no such route")
	}
	if ruleMatches(rule, "/api/messages") {
		t.Error("the auth router's /api/me pattern also catches /api/messages")
	}
}

func TestTrustedProxiesAreConfiguredInDocker(t *testing.T) {
	if !strings.Contains(read(t, "docker-compose.yml"), "AUTH_TRUSTED_PROXIES") {
		t.Error("behind Traefik every client would count as the proxy: AUTH_TRUSTED_PROXIES must be set")
	}
}

func TestVitePrefixesCoverTheAuthRoutes(t *testing.T) {
	vite := read(t, "frontend/vite.config.js")
	for _, want := range []string{"'/api/auth'", "'/api/me'"} {
		re := regexp.MustCompile(regexp.QuoteMeta(want) + `: \{ target: 'http://localhost:8180'`)
		if !re.MatchString(vite) {
			t.Errorf("vite.config.js does not proxy %s to the auth service on :8180", want)
		}
	}
	if !regexp.MustCompile(`workspaces\\/\[\^\/\]\+\\/contacts[\s\S]{0,120}localhost:8180`).MatchString(vite) {
		t.Error("vite.config.js does not re-dispatch /api/workspaces/:id/contacts to :8180")
	}
	if strings.Contains(vite, "'/api/users'") {
		t.Error("vite.config.js still proxies /api/users, which no service serves")
	}
}
