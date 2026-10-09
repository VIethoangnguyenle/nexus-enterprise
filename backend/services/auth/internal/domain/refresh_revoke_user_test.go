package domain_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/domain"
)

func TestRevokeAllForUser_KillsOlderSessionsOnlyForThatUser(t *testing.T) {
	rdb := testRedisClient(t)
	rs := domain.NewRefreshStore(rdb)
	ctx := context.Background()
	victim := "user-" + time.Now().Format("150405.000000000")
	other := victim + "-other"

	old, _, err := rs.Issue(ctx, domain.RefreshIdentity{UserID: victim, Username: "v"})
	if err != nil {
		t.Fatal(err)
	}
	bystander, _, err := rs.Issue(ctx, domain.RefreshIdentity{UserID: other, Username: "o"})
	if err != nil {
		t.Fatal(err)
	}

	if err := rs.RevokeAllForUser(ctx, victim); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if _, _, err := rs.Rotate(ctx, old); err == nil {
		t.Error("a session started before the revocation must be rejected")
	}
	if _, _, err := rs.Rotate(ctx, bystander); err != nil {
		t.Errorf("another user's session must survive: %v", err)
	}
	fresh, _, err := rs.Issue(ctx, domain.RefreshIdentity{UserID: victim, Username: "v"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := rs.Rotate(ctx, fresh); err != nil {
		t.Errorf("a session started after the revocation must work: %v", err)
	}
}

// Tokens minted before session start times were recorded carry none; they
// must count as older than any revocation.
func TestRevokeAllForUser_RejectsLegacyRecordsWithoutStartTime(t *testing.T) {
	rdb := testRedisClient(t)
	rs := domain.NewRefreshStore(rdb)
	ctx := context.Background()
	user := "legacy-" + time.Now().Format("150405.000000000")
	token := "legacy-token-" + user

	payload, _ := json.Marshal(map[string]any{
		"user_id": user, "username": "l", "ngac_node_id": "n", "tenant_id": "", "session_id": "legacy-session-" + user, "spent": false,
	})
	if err := rdb.Set(ctx, "refresh:"+token, payload, time.Hour).Err(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { rdb.Del(ctx, "refresh:"+token) })

	if err := rs.RevokeAllForUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	if _, _, err := rs.Rotate(ctx, token); err == nil {
		t.Fatal("a legacy refresh token must not survive a user-wide revocation")
	}
}
