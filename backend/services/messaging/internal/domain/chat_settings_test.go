package domain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

func str(s string) *string { return &s }
func yes(b bool) *bool     { return &b }

func nullStr(s string) domain.Nullable[string] {
	return domain.Nullable[string]{Set: true, Value: &s}
}

func (w *sectionWorld) cleanChatSettings() {
	w.f.pool.Exec(context.Background(), `DELETE FROM chat_user_settings WHERE workspace_id = $1`, w.f.wsID)
}

func (w *sectionWorld) storedChatRows(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, w.f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_user_settings WHERE workspace_id = $1`, w.f.wsID).Scan(&n))
	return n
}

func TestChatSettings_DeniedToNonMembers(t *testing.T) {
	w := newSectionWorld(t)
	t.Cleanup(w.cleanChatSettings)
	ctx := context.Background()
	svc := w.f.svc

	stranger := domain.Caller{UserID: "someone", NodeID: "ngac-user-stranger", TenantID: w.f.wsID}
	_, err := svc.GetChatSettings(ctx, w.f.wsID, stranger)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "not in the workspace: no read")
	_, err = svc.UpdateChatSettings(ctx, w.f.wsID, stranger, domain.ChatSettingsPatch{Sort: str("name")})
	require.ErrorIs(t, err, domain.ErrAccessDenied, "not in the workspace: no write")

	// A member of the workspace, but signed in to another tenant.
	otherTenant := w.me
	otherTenant.TenantID = "another-workspace"
	_, err = svc.GetChatSettings(ctx, w.f.wsID, otherTenant)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = svc.UpdateChatSettings(ctx, w.f.wsID, otherTenant, domain.ChatSettingsPatch{Sort: str("name")})
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	_, err = svc.GetChatSettings(ctx, w.f.wsID, domain.Caller{NodeID: userNode, TenantID: w.f.wsID})
	require.Error(t, err, "no signed-in user")
	assert.Zero(t, w.storedChatRows(t), "nothing was written")
}

func TestChatSettings_DefaultsThenRoundTripThenOwnRowOnly(t *testing.T) {
	w := newSectionWorld(t)
	t.Cleanup(w.cleanChatSettings)
	ctx := context.Background()
	svc := w.f.svc

	got, err := svc.GetChatSettings(ctx, w.f.wsID, w.me)
	require.NoError(t, err)
	assert.Equal(t, domain.ChatSettings{LinkPreview: true, OpenAt: "all", Sort: "activity", DoubleTap: "👍"}, got, "no row: the column defaults")
	assert.Zero(t, w.storedChatRows(t), "reading does not write")

	got, err = svc.UpdateChatSettings(ctx, w.f.wsID, w.me, domain.ChatSettingsPatch{
		EnterToSend: yes(true), Sort: str("name"), DoubleTap: str("❤️"), DefaultLevel: nullStr("off"),
	})
	require.NoError(t, err)
	want := domain.ChatSettings{EnterToSend: true, LinkPreview: true, OpenAt: "all", Sort: "name", DoubleTap: "❤️", DefaultLevel: str("off")}
	assert.Equal(t, want, got)

	// A partial write keeps the rest.
	got, err = svc.UpdateChatSettings(ctx, w.f.wsID, w.me, domain.ChatSettingsPatch{OpenAt: str("sections"), LinkPreview: yes(false), DoubleTap: str("")})
	require.NoError(t, err)
	want.OpenAt, want.LinkPreview, want.DoubleTap = "sections", false, ""
	assert.Equal(t, want, got, "'' turns the double-tap reaction off")
	again, err := svc.GetChatSettings(ctx, w.f.wsID, w.me)
	require.NoError(t, err)
	assert.Equal(t, want, again)

	// null clears the default level.
	got, err = svc.UpdateChatSettings(ctx, w.f.wsID, w.me, domain.ChatSettingsPatch{DefaultLevel: domain.Nullable[string]{Set: true}})
	require.NoError(t, err)
	assert.Nil(t, got.DefaultLevel)

	// Someone else in the workspace never sees or changes it.
	_, otherNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{otherNode, w.docsOA(t), ngac.OpRead}] = true
	otherID := "other-user"
	other := domain.Caller{UserID: otherID, NodeID: otherNode, TenantID: w.f.wsID}
	theirs, err := svc.GetChatSettings(ctx, w.f.wsID, other)
	require.NoError(t, err)
	assert.Equal(t, "all", theirs.OpenAt, "their own defaults, not mine")
}

func (w *sectionWorld) docsOA(t *testing.T) string {
	t.Helper()
	var oa string
	require.NoError(t, w.f.pool.QueryRow(context.Background(), `SELECT documents_oa_id FROM workspaces WHERE id = $1`, w.f.wsID).Scan(&oa))
	return oa
}

func TestChatSettings_RejectsInvalidValues(t *testing.T) {
	w := newSectionWorld(t)
	t.Cleanup(w.cleanChatSettings)
	ctx := context.Background()
	svc := w.f.svc

	for name, patch := range map[string]domain.ChatSettingsPatch{
		"open_at":          {OpenAt: str("everything")},
		"sort":             {Sort: str("random")},
		"default_level":    {DefaultLevel: nullStr("loud")},
		"empty level":      {DefaultLevel: nullStr("")},
		"double_tap text":  {DoubleTap: str("like")},
		"double_tap pair":  {DoubleTap: str("👍 ")},
		"double_tap long":  {DoubleTap: str(strings.Repeat("👍", 9))},
		"double_tap digit": {DoubleTap: str("1")},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.UpdateChatSettings(ctx, w.f.wsID, w.me, patch)
			require.ErrorIs(t, err, domain.ErrInvalidInput)
		})
	}
	assert.Zero(t, w.storedChatRows(t), "a refused value writes nothing")

	for _, ok := range []string{"👍", "❤️", "🎉", "👨‍👩‍👧", "👍🏽", ""} {
		_, err := svc.UpdateChatSettings(ctx, w.f.wsID, w.me, domain.ChatSettingsPatch{DoubleTap: str(ok)})
		require.NoError(t, err, "%q", ok)
	}
}

// A new group starts at the person's default level; a person who set none gets
// the system default, and a level they already chose is never overwritten.
func TestChatSettings_DefaultLevelSeedsANewGroup(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	t.Cleanup(func() { f.pool.Exec(ctx, `DELETE FROM chat_user_settings WHERE workspace_id = $1`, f.wsID) })

	levelOf := func(chID string) string {
		lvl, err := store.NewStore(f.pool).NotificationLevel(ctx, f.userID, chID)
		require.NoError(t, err)
		return lvl
	}

	plain, err := f.svc.CreateChannel(ctx, f.createInput(fmt.Sprintf("plain-%d", time.Now().UnixNano())))
	require.NoError(t, err)
	assert.Equal(t, domain.NotifyMain, levelOf(plain.Id), "no default chosen: the system default")

	_, err = f.pool.Exec(ctx, `INSERT INTO chat_user_settings (user_id, workspace_id, default_level) VALUES ($1, $2, 'off')`, f.userID, f.wsID)
	require.NoError(t, err)
	muted, err := f.svc.CreateChannel(ctx, f.createInput(fmt.Sprintf("muted-%d", time.Now().UnixNano())))
	require.NoError(t, err)
	assert.Equal(t, domain.NotifyOff, levelOf(muted.Id), "the creator's default level applies to a group they create")
	assert.Equal(t, domain.NotifyMain, levelOf(plain.Id), "an existing group is not touched")
}

func TestChatSettings_DefaultLevelSeedsAPersonAddedToAGroup(t *testing.T) {
	f := newFixture(t)
	ctx := f.callerCtx()
	chID, oaID := f.insertChannel(t, "added")
	f.read.allow[grant{userNode, oaID, ngac.OpInvite}] = true
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM chat_user_settings WHERE workspace_id = $1`, f.wsID)
	})

	view := &scriptedPolicyRead{allow: map[grant]bool{}}
	chosen, chosenNode := testutil.CreateUser(t, f.pool)
	plain, plainNode := testutil.CreateUser(t, f.pool)
	explicit, explicitNode := testutil.CreateUser(t, f.pool)
	for _, n := range []string{chosenNode, plainNode, explicitNode} {
		view.addMember(f.wsID, n)
	}
	w := testutil.NewFakePolicyWrite()
	w.Reads = view
	f.withFakeWrite(w)

	for _, id := range []string{chosen, explicit} {
		_, err := f.pool.Exec(context.Background(), `INSERT INTO chat_user_settings (user_id, workspace_id, default_level) VALUES ($1, $2, 'all')`, id, f.wsID)
		require.NoError(t, err)
	}
	_, err := f.pool.Exec(context.Background(), `INSERT INTO channel_notification_levels (user_id, channel_id, level) VALUES ($1, $2, 'off')`, explicit, chID)
	require.NoError(t, err)

	for _, node := range []string{chosenNode, plainNode, explicitNode} {
		require.NoError(t, f.svc.AddMember(ctx, chID, userNode, node))
	}
	level := func(userID string) string {
		lvl, err := store.NewStore(f.pool).NotificationLevel(context.Background(), userID, chID)
		require.NoError(t, err)
		return lvl
	}
	assert.Equal(t, domain.NotifyAll, level(chosen), "joins at their default level")
	assert.Equal(t, domain.NotifyMain, level(plain), "no default: the system default")
	assert.Equal(t, domain.NotifyOff, level(explicit), "a level already chosen for this group is kept")
}
