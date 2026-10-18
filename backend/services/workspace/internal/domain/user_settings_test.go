package domain_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/domain"
)

const (
	userMember = "user-member"
	userTarget = "user-target"
)

func bar(ids ...string) json.RawMessage {
	b, _ := json.Marshal(ids)
	return b
}

func barSetting(ids ...string) map[string]json.RawMessage {
	return map[string]json.RawMessage{domain.SettingMobileBar: bar(ids...)}
}

func TestUserSettings_NonMemberIsRefusedAndNothingIsWritten(t *testing.T) {
	f := newFixture(t)
	for _, caller := range []string{outsider, otherOwner, ""} {
		_, err := f.svc.GetUserSettings(ctx(), caller, "user-x", ws1)
		require.ErrorIs(t, err, domain.ErrAccessDenied, "get as %q", caller)
		_, err = f.svc.PutUserSettings(ctx(), caller, "user-x", ws1, barSetting("chat", "calendar", "drive", "approval"))
		require.ErrorIs(t, err, domain.ErrAccessDenied, "put as %q", caller)
	}
	assert.False(t, f.mutated(), "a refused write leaves no row")
}

func TestUserSettings_UnknownWorkspaceIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.GetUserSettings(ctx(), member, userMember, "no-such-ws")
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserSettings_MemberRoundTripsTheirBar(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.GetUserSettings(ctx(), member, userMember, ws1)
	require.NoError(t, err)
	assert.Empty(t, got, "nothing stored yet")

	out, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, barSetting("chat", "drive", "approval", "assets"))
	require.NoError(t, err)
	assert.JSONEq(t, `["chat","drive","approval","assets"]`, string(out[domain.SettingMobileBar]))

	got, err = f.svc.GetUserSettings(ctx(), member, userMember, ws1)
	require.NoError(t, err)
	assert.JSONEq(t, `["chat","drive","approval","assets"]`, string(got[domain.SettingMobileBar]))
}

// A person only ever reaches their own rows: another member of the same workspace sees nothing of it.
func TestUserSettings_OtherUsersSettingsAreUnreachable(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, barSetting("chat", "drive", "approval", "assets"))
	require.NoError(t, err)

	got, err := f.svc.GetUserSettings(ctx(), target, userTarget, ws1)
	require.NoError(t, err)
	assert.Empty(t, got, "the other member's bar is not mine")

	_, err = f.svc.PutUserSettings(ctx(), target, userTarget, ws1, barSetting("chat", "calendar", "drive", "contacts"))
	require.NoError(t, err)
	mine, err := f.svc.GetUserSettings(ctx(), member, userMember, ws1)
	require.NoError(t, err)
	assert.JSONEq(t, `["chat","drive","approval","assets"]`, string(mine[domain.SettingMobileBar]), "writing theirs left mine alone")
}

func TestUserSettings_AnonymousCallerWithAUserIDStillNeedsMembership(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.GetUserSettings(ctx(), member, "", ws1)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "no user id in the claims is no one")
}

func TestUserSettings_InvalidBarIsRefusedAndNothingIsWritten(t *testing.T) {
	for name, raw := range map[string]json.RawMessage{
		"three modules":         bar("chat", "drive", "approval"),
		"five modules":          bar("chat", "drive", "approval", "assets", "contacts"),
		"duplicate":             bar("chat", "chat", "drive", "approval"),
		"admin is never a slot": bar("chat", "drive", "approval", "admin"),
		"unknown module":        bar("chat", "drive", "approval", "bogus"),
		"empty id":              bar("chat", "drive", "approval", ""),
		"not an array":          json.RawMessage(`"chat"`),
		"object":                json.RawMessage(`{"chat":1}`),
		"array of numbers":      json.RawMessage(`[1,2,3,4]`),
		"null":                  json.RawMessage(`null`),
		"empty":                 bar(),
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, map[string]json.RawMessage{domain.SettingMobileBar: raw})
			require.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.False(t, f.mutated())
		})
	}
}

func TestUserSettings_UnknownKeyOrEmptyBodyIsRefused(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, map[string]json.RawMessage{"chat.enterToSend": json.RawMessage(`true`)})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "this service owns mobile.bar only")
	_, err = f.svc.PutUserSettings(ctx(), member, userMember, ws1, map[string]json.RawMessage{})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.False(t, f.mutated())
}

func TestUserSettings_OneBadKeyWritesNoneOfTheBody(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, map[string]json.RawMessage{
		domain.SettingMobileBar: bar("chat", "drive", "approval", "assets"),
		"nope":                  json.RawMessage(`1`),
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.False(t, f.mutated())
}

// A personal workspace has no approval and no assets.
func TestUserSettings_PersonalWorkspaceOffersNoApprovalOrAssets(t *testing.T) {
	f := newFixture(t)
	f.wsStore.ws[ws1].Type = domain.TypePersonal
	for _, ids := range [][]string{
		{"chat", "calendar", "drive", "approval"},
		{"chat", "calendar", "drive", "assets"},
	} {
		_, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, barSetting(ids...))
		require.ErrorIs(t, err, domain.ErrInvalidInput, "%v", ids)
	}
	assert.False(t, f.mutated())

	_, err := f.svc.PutUserSettings(ctx(), member, userMember, ws1, barSetting("chat", "calendar", "drive", "contacts"))
	require.NoError(t, err)
	_, err = f.svc.PutUserSettings(ctx(), member, userMember, ws1, barSetting("chat", "calendar", "drive", "notifications"))
	require.NoError(t, err)
}
