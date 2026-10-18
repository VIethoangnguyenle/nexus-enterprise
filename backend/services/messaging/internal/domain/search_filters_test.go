package domain_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func searchAllIDs(t *testing.T, k *talk, node string, q domain.SearchQuery) []string {
	t.Helper()
	list, err := k.f.svc.SearchAll(context.Background(), k.f.userID, node, k.f.wsID, q)
	require.NoError(t, err)
	out := []string{}
	for _, m := range list.Messages {
		out = append(out, m.Id)
	}
	return out
}

func TestSearchAll_FromFilterKeepsOnlySendersMessages(t *testing.T) {
	k := newTalk(t, "search-from")
	other, otherNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true

	k.send(t, k.f.userID, userNode, "unique_from_me xxyyzz", "")
	k.send(t, other, otherNode, "unique_from_me xxyyzz", "")
	t.Cleanup(func() {
		k.f.pool.Exec(context.Background(), `DELETE FROM messages WHERE channel_id = $1 AND sender_id = $2`, k.chID, other)
	})

	got := searchAllIDs(t, k, userNode, domain.SearchQuery{Text: "xxyyzz", From: k.f.userID})
	require.Len(t, got, 1, "only messages from the caller")

	got = searchAllIDs(t, k, userNode, domain.SearchQuery{Text: "xxyyzz", From: other})
	require.Len(t, got, 1, "only messages from the other user")
}

func TestSearchAll_ChannelIDFilterReturnsNothingForUnreadableChannel(t *testing.T) {
	k := newTalk(t, "search-ch-filter")
	// Create a second channel the caller can read.
	ch2, oa2 := k.f.insertChannel(t, "search-ch-filter-2")
	k.f.read.allow[grant{userNode, oa2, ngac.OpRead}] = true
	k.f.read.allow[grant{userNode, oa2, ngac.OpWrite}] = true
	msg2, err := k.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: ch2, SenderID: k.f.userID, SenderNodeID: userNode, Content: "needle in ch2",
	})
	require.NoError(t, err)
	t.Cleanup(func() { k.f.pool.Exec(context.Background(), `DELETE FROM messages WHERE channel_id = $1`, ch2) })

	// Filter to ch2 only.
	got := searchAllIDs(t, k, userNode, domain.SearchQuery{Text: "needle", ChannelID: ch2})
	assert.Equal(t, []string{msg2.Id}, got)

	// Filter to an unreadable channel: no results, no error.
	got = searchAllIDs(t, k, userNode, domain.SearchQuery{Text: "needle", ChannelID: "non-existent-ch"})
	assert.Empty(t, got, "unreadable channel yields empty, never error")
}

func TestSearchAll_DateFilterInvalidInput(t *testing.T) {
	k := newTalk(t, "search-date-err")
	ctx := context.Background()

	_, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{
		Text: "needle", After: "not-a-date"})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "invalid after date")

	_, err = k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{
		Text: "needle", BeforeDate: "2025-13-99"})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "invalid before_date")
}

func TestSearchAll_HasLinkFilterKeepsOnlyLinkedMessages(t *testing.T) {
	k := newTalk(t, "search-link")
	linked := k.send(t, k.f.userID, userNode, "needle see https://example.com", "")
	k.send(t, k.f.userID, userNode, "needle plain text", "")

	got := searchAllIDs(t, k, userNode, domain.SearchQuery{Text: "needle", HasLink: true})
	assert.Equal(t, []string{linked.Id}, got)
}

func TestSearchAll_MentionsMeFilterUsesTheMentionDefinition(t *testing.T) {
	k := newTalk(t, "search-mentions-me")
	other, otherNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true
	// Clean up messages by the other user before the user itself is deleted.
	t.Cleanup(func() {
		k.f.pool.Exec(context.Background(), `DELETE FROM messages WHERE channel_id = $1 AND sender_id = $2`, k.chID, other)
	})

	mentioned := k.send(t, other, otherNode, "needle @me unique_ment", "", k.f.userID)
	k.send(t, other, otherNode, "needle no mention unique_ment", "")

	got := searchAllIDs(t, k, userNode, domain.SearchQuery{Text: "unique_ment", MentionsMe: true})
	assert.Equal(t, []string{mentioned.Id}, got)
}

func TestSearchAll_FilterOnlySearchWithNoText(t *testing.T) {
	k := newTalk(t, "search-filter-only")
	// The initial message is "hello needle" from newTalk.
	got := searchAllIDs(t, k, userNode, domain.SearchQuery{From: k.f.userID})
	require.Len(t, got, 1, "filter-only search (no text) returns results")
	assert.Equal(t, k.msgID, got[0])

	// No text AND no filter: empty list.
	got = searchAllIDs(t, k, userNode, domain.SearchQuery{})
	assert.Empty(t, got, "no text + no filter = empty")
}

func TestSearchAll_CombinedFilters(t *testing.T) {
	k := newTalk(t, "search-combined")
	other, otherNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpWrite}] = true
	t.Cleanup(func() {
		k.f.pool.Exec(context.Background(), `DELETE FROM messages WHERE channel_id = $1 AND sender_id = $2`, k.chID, other)
	})

	hit := k.send(t, other, otherNode, "unique_combo https://link.com for you", "", k.f.userID)
	k.send(t, other, otherNode, "unique_combo plain", "")
	k.send(t, k.f.userID, userNode, "unique_combo https://mine.com link", "")

	got := searchAllIDs(t, k, userNode, domain.SearchQuery{
		Text: "unique_combo", From: other, HasLink: true, MentionsMe: true,
	})
	assert.Equal(t, []string{hit.Id}, got)
}

func TestSearchAll_PaginationWithFilter(t *testing.T) {
	k := newTalk(t, "search-page-filter")
	ctx := context.Background()
	for range 3 {
		k.send(t, k.f.userID, userNode, "needle extra", "")
	}

	first, err := k.f.svc.SearchAll(ctx, k.f.userID, userNode, k.f.wsID, domain.SearchQuery{
		Text: "needle", From: k.f.userID, Limit: 2})
	require.NoError(t, err)
	require.Len(t, first.Messages, 2)
	assert.True(t, first.HasMore)
}

func TestSearchAll_TimeZoneDecidesWhichDayAMessageIsOn(t *testing.T) {
	k := newTalk(t, "search-tz")
	ctx := context.Background()
	m := k.send(t, k.f.userID, userNode, "needle tz boundary", "")
	// 2025-03-15 20:00 UTC is already 2025-03-16 03:00 in Asia/Ho_Chi_Minh.
	_, err := k.f.pool.Exec(ctx, `UPDATE messages SET created_at = $2 WHERE id = $1`, m.Id,
		time.Date(2025, 3, 15, 20, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	day := domain.SearchQuery{Text: "tz boundary", After: "2025-03-15", BeforeDate: "2025-03-15"}
	assert.Empty(t, searchAllIDs(t, k, userNode, day), "no zone given: days are Asia/Ho_Chi_Minh, where it is the 16th")

	day.TZ = "UTC"
	assert.Equal(t, []string{m.Id}, searchAllIDs(t, k, userNode, day), "in UTC it is still the 15th")

	day.TZ = "America/New_York"
	assert.Equal(t, []string{m.Id}, searchAllIDs(t, k, userNode, day), "in New York it is the afternoon of the 15th")

	day.TZ = "Asia/Ho_Chi_Minh"
	assert.Empty(t, searchAllIDs(t, k, userNode, day))
}

func TestSearchAll_UnknownTimeZoneIsInvalidInput(t *testing.T) {
	k := newTalk(t, "search-tz-bad")
	for _, tz := range []string{"Mars/Olympus", "../etc/passwd", "UTC+7"} {
		_, err := k.f.svc.SearchAll(context.Background(), k.f.userID, userNode, k.f.wsID, domain.SearchQuery{
			Text: "needle", After: "2025-03-15", TZ: tz})
		require.ErrorIs(t, err, domain.ErrInvalidInput, tz)
	}
}

func TestSearchAll_MalformedIDFiltersAreInvalidInput(t *testing.T) {
	k := newTalk(t, "search-bad-ids")
	for _, q := range []domain.SearchQuery{
		{Text: "needle", From: " "}, {Text: "needle", From: "a b"}, {Text: "needle", ChannelID: "a/b"},
		{Text: "needle", ChannelID: strings.Repeat("x", 200)},
	} {
		_, err := k.f.svc.SearchAll(context.Background(), k.f.userID, userNode, k.f.wsID, q)
		require.ErrorIs(t, err, domain.ErrInvalidInput, "%+v", q)
	}
}
