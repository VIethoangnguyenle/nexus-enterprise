package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
)

// postTextBy stores a message from the given sender, one second after the previous.
func (w *threadWorld) postTextBy(t *testing.T, sender, content, format, parent string, mentions ...string) *store.Message {
	t.Helper()
	w.clock = w.clock.Add(time.Second)
	m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: w.chID, SenderID: sender, Content: content,
		ContentFormat: format, MessageType: "user", ParentMessageID: parent, Mentions: mentions, CreatedAt: w.clock}
	require.NoError(t, w.st.InsertMessage(context.Background(), m))
	return m
}

// searchAllFiltered wraps SearchAllMessages with filters.
func (w *threadWorld) searchAllFiltered(t *testing.T, q string, hasFile bool, before *time.Time, beforeID string, limit int, sf store.SearchFilters) ([]*store.Message, bool) {
	t.Helper()
	msgs, more, err := w.st.SearchAllMessages(context.Background(), []string{w.chID}, q, hasFile, before, beforeID, limit, sf)
	require.NoError(t, err)
	return msgs, more
}

func TestSearchAllMessages_FromFilterKeepsOnlyThatSender(t *testing.T) {
	w := newThreadWorld(t, 2)
	alice, bob := w.users[0], w.users[1]
	aliceMsg := w.postTextBy(t, alice, "needle alice", "markdown", "")
	w.postTextBy(t, bob, "needle bob", "markdown", "")

	got, _ := w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{From: alice})
	assert.Equal(t, []string{aliceMsg.ID}, ids(got), "only alice's message")

	got, _ = w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{From: bob})
	assert.Len(t, got, 1, "only bob's message")
	assert.Equal(t, bob, got[0].SenderID)
}

func TestSearchAllMessages_ChannelIDFilterRestrictsToOneChannel(t *testing.T) {
	a := newThreadWorld(t, 1)
	b := newThreadWorld(t, 1)
	hitA := a.postText(t, "needle channel-a", "markdown", "")
	b.postText(t, "needle channel-b", "markdown", "")

	// When channel_id is in the readable set, only that channel's results.
	msgs, _, err := a.st.SearchAllMessages(context.Background(), []string{a.chID, b.chID}, "needle", false, nil, "", 25,
		store.SearchFilters{ChannelID: a.chID})
	require.NoError(t, err)
	assert.Equal(t, []string{hitA.ID}, ids(msgs))

	// When channel_id is NOT in the readable set, zero results, no error.
	msgs, _, err = a.st.SearchAllMessages(context.Background(), []string{a.chID}, "needle", false, nil, "", 25,
		store.SearchFilters{ChannelID: b.chID})
	require.NoError(t, err)
	assert.Empty(t, msgs, "unreadable channel: zero results, never an error")
}

func TestSearchAllMessages_AfterAndBeforeDateFilterByUTC7Day(t *testing.T) {
	w := newThreadWorld(t, 1)
	hcm := time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

	// Messages at different Asia/Ho_Chi_Minh dates.
	// 2025-03-15 00:30 HCM = 2025-03-14 17:30 UTC
	earlyDay := time.Date(2025, 3, 14, 17, 30, 0, 0, time.UTC)
	// 2025-03-15 12:00 HCM = 2025-03-15 05:00 UTC
	midDay := time.Date(2025, 3, 15, 5, 0, 0, 0, time.UTC)
	// 2025-03-16 06:00 HCM = 2025-03-15 23:00 UTC
	lateDay := time.Date(2025, 3, 15, 23, 0, 0, 0, time.UTC)

	w.clock = earlyDay.Add(-time.Second)
	early := w.postText(t, "needle early", "markdown", "")
	w.clock = midDay.Add(-time.Second)
	mid := w.postText(t, "needle mid", "markdown", "")
	w.clock = lateDay.Add(-time.Second)
	late := w.postText(t, "needle late", "markdown", "")

	// after=2025-03-15 → start of 2025-03-15 HCM = 2025-03-14T17:00Z
	afterT := time.Date(2025, 3, 15, 0, 0, 0, 0, hcm).UTC()
	got, _ := w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{After: &afterT})
	gotIDs := ids(got)
	assert.Contains(t, gotIDs, early.ID, "early is 2025-03-15 00:30 HCM ≥ start-of-day")
	assert.Contains(t, gotIDs, mid.ID)
	assert.Contains(t, gotIDs, late.ID)

	// before_date=2025-03-15 → end-of-day = start of 2025-03-16 HCM = 2025-03-15T17:00Z
	endT := time.Date(2025, 3, 16, 0, 0, 0, 0, hcm).UTC()
	got, _ = w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{BeforeDate: &endT})
	gotIDs = ids(got)
	assert.Contains(t, gotIDs, early.ID)
	assert.Contains(t, gotIDs, mid.ID)
	assert.NotContains(t, gotIDs, late.ID, "late is 2025-03-16 06:00 HCM, past end-of-day")

	// Combined: after + before_date on the same day.
	got, _ = w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{After: &afterT, BeforeDate: &endT})
	gotIDs = ids(got)
	assert.Contains(t, gotIDs, early.ID)
	assert.Contains(t, gotIDs, mid.ID)
	assert.NotContains(t, gotIDs, late.ID)
}

func TestSearchAllMessages_HasLinkKeepsOnlyMessagesWithLinks(t *testing.T) {
	w := newThreadWorld(t, 1)
	withLink := w.postText(t, "needle see https://example.com/page for details", "markdown", "")
	w.postText(t, "needle without a link", "markdown", "")
	withHTTP := w.postText(t, "needle check http://insecure.example.com now", "markdown", "")

	got, _ := w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{HasLink: true})
	gotIDs := ids(got)
	assert.Len(t, gotIDs, 2)
	assert.Contains(t, gotIDs, withLink.ID)
	assert.Contains(t, gotIDs, withHTTP.ID)
}

func TestSearchAllMessages_MentionsMeKeepsOnlyMentioningMessages(t *testing.T) {
	w := newThreadWorld(t, 2)
	me, other := w.users[0], w.users[1]
	mentioned := w.postTextBy(t, other, "needle @me here", "markdown", "", me)
	w.postTextBy(t, other, "needle no mention", "markdown", "")

	got, _ := w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{MentionsMe: true, CallerID: me})
	assert.Equal(t, []string{mentioned.ID}, ids(got))

	// Not mentioning the right person.
	got, _ = w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{MentionsMe: true, CallerID: other})
	assert.Empty(t, got)
}

func TestSearchAllMessages_FilterOnlySearchNoText(t *testing.T) {
	w := newThreadWorld(t, 2)
	alice, bob := w.users[0], w.users[1]
	w.postTextBy(t, alice, "something by alice", "markdown", "")
	w.postTextBy(t, bob, "something by bob", "markdown", "")

	// Filter-only: text is empty, but from is set.
	got, _ := w.searchAllFiltered(t, "", false, nil, "", 25,
		store.SearchFilters{From: alice})
	assert.Len(t, got, 1, "filter-only search works")
	assert.Equal(t, alice, got[0].SenderID)
}

func TestSearchAllMessages_CombinedFilters(t *testing.T) {
	w := newThreadWorld(t, 2)
	alice, bob := w.users[0], w.users[1]
	w.postTextBy(t, alice, "needle with https://link.com here", "markdown", "", bob)
	w.postTextBy(t, alice, "needle no link", "markdown", "")
	w.postTextBy(t, bob, "needle with https://other.com link", "markdown", "")

	// from=alice AND has_link AND mentions bob
	got, _ := w.searchAllFiltered(t, "needle", false, nil, "", 25,
		store.SearchFilters{From: alice, HasLink: true, MentionsMe: true, CallerID: bob})
	require.Len(t, got, 1)
	assert.Equal(t, alice, got[0].SenderID)
	assert.Contains(t, got[0].Content, "https://link.com")
}

func TestSearchAllMessages_PaginationWithFilter(t *testing.T) {
	w := newThreadWorld(t, 1)
	alice := w.users[0]
	var want []string
	for range 5 {
		want = append([]string{w.postTextBy(t, alice, "needle paged", "markdown", "").ID}, want...)
	}

	var got []string
	var before *time.Time
	beforeID := ""
	for range 10 {
		page, more := w.searchAllFiltered(t, "needle", false, before, beforeID, 2,
			store.SearchFilters{From: alice})
		require.NotEmpty(t, page)
		got = append(got, ids(page)...)
		if !more {
			break
		}
		last := page[len(page)-1]
		at := last.CreatedAt
		before, beforeID = &at, last.ID
	}
	assert.Equal(t, want, got, "pagination works with filters")
}
