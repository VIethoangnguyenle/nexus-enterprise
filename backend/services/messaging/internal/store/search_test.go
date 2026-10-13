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

// postText stores a message with the given body, one second after the previous.
func (w *threadWorld) postText(t *testing.T, content, format, parent string) *store.Message {
	t.Helper()
	w.clock = w.clock.Add(time.Second)
	m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: w.chID, SenderID: w.users[0], Content: content,
		ContentFormat: format, MessageType: "user", ParentMessageID: parent, CreatedAt: w.clock}
	require.NoError(t, w.st.InsertMessage(context.Background(), m))
	return m
}

func (w *threadWorld) searchAll(t *testing.T, q string, hasFile bool, before *time.Time, beforeID string, limit int) ([]*store.Message, bool) {
	t.Helper()
	msgs, more, err := w.st.SearchAllMessages(context.Background(), []string{w.chID}, q, hasFile, before, beforeID, limit)
	require.NoError(t, err)
	return msgs, more
}

func ids(msgs []*store.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.ID)
	}
	return out
}

func TestSearchAllMessages_FindsVietnameseWithDiacritics(t *testing.T) {
	w := newThreadWorld(t, 1)
	hit := w.postText(t, "Báo cáo đối soát tháng 9", "markdown", "")
	w.postText(t, "Họp kế hoạch quý", "markdown", "")

	got, more := w.searchAll(t, "đối soát", false, nil, "", 25)
	assert.Equal(t, []string{hit.ID}, ids(got))
	assert.False(t, more)

	got, _ = w.searchAll(t, "ĐỐI SOÁT", false, nil, "", 25)
	assert.Equal(t, []string{hit.ID}, ids(got), "case does not matter")

	got, _ = w.searchAll(t, "doi soat", false, nil, "", 25)
	assert.Empty(t, got, "unaccented search is a known limit")
}

func TestSearchAllMessages_MainMessagesAndThreadRepliesWithParent(t *testing.T) {
	w := newThreadWorld(t, 1)
	root := w.postText(t, "needle in the main stream", "markdown", "")
	reply := w.postText(t, "needle in a reply", "markdown", root.ID)

	got, _ := w.searchAll(t, "needle", false, nil, "", 25)
	require.Equal(t, []string{reply.ID, root.ID}, ids(got), "newest first")
	assert.Equal(t, root.ID, got[0].ParentMessageID)
	assert.Empty(t, got[1].ParentMessageID)
	assert.NotEmpty(t, got[0].SenderName)
}

func TestSearchAllMessages_MarkupIsNotSearchable(t *testing.T) {
	w := newThreadWorld(t, 1)
	html := w.postText(t, `<p>xem <strong>kế hoạch</strong></p>`, "html", "")

	got, _ := w.searchAll(t, "strong", false, nil, "", 25)
	assert.Empty(t, got, "a tag name is not message text")
	got, _ = w.searchAll(t, "kế hoạch", false, nil, "", 25)
	assert.Equal(t, []string{html.ID}, ids(got))

	// A tag left in a stored body (a plain message that quotes markup) is also not text.
	w.postText(t, "see <em>this</em> now", "markdown", "")
	got, _ = w.searchAll(t, "em", false, nil, "", 25)
	assert.Empty(t, got)
}

func TestSearchAllMessages_LastWordMatchesAsPrefix(t *testing.T) {
	w := newThreadWorld(t, 1)
	hit := w.postText(t, "đối soát công nợ", "markdown", "")
	w.postText(t, "công việc khác", "markdown", "")

	got, _ := w.searchAll(t, "đối so", false, nil, "", 25)
	assert.Equal(t, []string{hit.ID}, ids(got), "an unfinished last word still matches")
	got, _ = w.searchAll(t, "đ", false, nil, "", 25)
	assert.Empty(t, got, "one letter is read as a whole word, not a prefix over the index")
	got, _ = w.searchAll(t, "đối s", false, nil, "", 25)
	assert.Empty(t, got, "a one-letter last word does not become a prefix either")
	got, _ = w.searchAll(t, "đối sơ", false, nil, "", 25)
	assert.Empty(t, got, "two letters are a prefix, and no word starts with them")
	got, _ = w.searchAll(t, "đối so", false, nil, "", 25)
	assert.Equal(t, []string{hit.ID}, ids(got), "two letters are enough for a prefix")
	got, _ = w.searchAll(t, "so đối", false, nil, "", 25)
	assert.Empty(t, got, "only the last word is a prefix; the others are whole words")
	got, _ = w.searchAll(t, "đối soát ", false, nil, "", 25)
	assert.Equal(t, []string{hit.ID}, ids(got), "a finished last word is matched whole")
}

func TestSearchAllMessages_UserInputNeverBreaksTheQuery(t *testing.T) {
	w := newThreadWorld(t, 1)
	hit := w.postText(t, "alpha beta", "markdown", "")

	for _, q := range []string{"a & | ! :*", "(((", "alpha &", "'; DROP TABLE messages; --", `\`, "alpha:*", "!!", "<->", "alpha <-> beta", "'", "alpha'"} {
		_, _, err := w.st.SearchAllMessages(context.Background(), []string{w.chID}, q, false, nil, "", 25)
		require.NoError(t, err, "query %q", q)
	}
	got, _ := w.searchAll(t, "alpha & beta", false, nil, "", 25)
	assert.Equal(t, []string{hit.ID}, ids(got), "an operator is just a separator")
	got, _ = w.searchAll(t, "!!", false, nil, "", 25)
	assert.Empty(t, got, "no word, nothing found")
	var n int
	require.NoError(t, w.pool.QueryRow(context.Background(), `SELECT count(*) FROM messages WHERE id = $1`, hit.ID).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestSearchAllMessages_LeavesOutDeletedAndOtherChannels(t *testing.T) {
	w := newThreadWorld(t, 1)
	other := newThreadWorld(t, 1)
	kept := w.postText(t, "needle kept", "markdown", "")
	gone := w.postText(t, "needle gone", "markdown", "")
	other.postText(t, "needle elsewhere", "markdown", "")
	_, err := w.pool.Exec(context.Background(), `UPDATE messages SET deleted_at = now() WHERE id = $1`, gone.ID)
	require.NoError(t, err)

	got, _ := w.searchAll(t, "needle", false, nil, "", 25)
	assert.Equal(t, []string{kept.ID}, ids(got))

	none, more, err := w.st.SearchAllMessages(context.Background(), nil, "needle", false, nil, "", 25)
	require.NoError(t, err)
	assert.Empty(t, none, "no readable channel, no result")
	assert.False(t, more)
}

func TestSearchAllMessages_HasFileKeepsOnlyMessagesWithAnAttachment(t *testing.T) {
	d := newDriveWorld(t)
	root := d.item(t, "folder", "", "channel", d.chID, "active", true)
	file := d.item(t, "file", root, "channel", d.chID, "active", false)

	d.postText(t, "needle without a file", "markdown", "")
	d.clock = d.clock.Add(time.Second)
	withFile := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: d.chID, SenderID: d.users[0], Content: "needle with a file",
		MessageType: "user", CreatedAt: d.clock,
		Attachments: []store.Attachment{{DriveFileID: file, FileName: "bao-cao.png", MimeType: "image/png", SizeBytes: 2048}}}
	require.NoError(t, d.st.InsertMessage(context.Background(), withFile))

	got, _ := d.searchAll(t, "needle", false, nil, "", 25)
	assert.Len(t, got, 2)
	got, _ = d.searchAll(t, "needle", true, nil, "", 25)
	assert.Equal(t, []string{withFile.ID}, ids(got))
}

func TestSearchAllMessages_PagesWithoutRepeatingOrSkipping(t *testing.T) {
	w := newThreadWorld(t, 1)
	var want []string
	for range 5 {
		want = append([]string{w.postText(t, "needle", "markdown", "").ID}, want...)
	}
	// Two messages sent at the same instant: the id breaks the tie.
	_, err := w.pool.Exec(context.Background(), `UPDATE messages SET created_at = (SELECT created_at FROM messages WHERE id = $1) WHERE id = $2`, want[1], want[2])
	require.NoError(t, err)
	if want[1] < want[2] {
		want[1], want[2] = want[2], want[1]
	}

	var got []string
	var before *time.Time
	beforeID := ""
	for range 10 {
		page, more := w.searchAll(t, "needle", false, before, beforeID, 2)
		require.NotEmpty(t, page)
		got = append(got, ids(page)...)
		if !more {
			break
		}
		last := page[len(page)-1]
		at := last.CreatedAt
		before, beforeID = &at, last.ID
	}
	assert.Equal(t, want, got)
}

func TestSearchMessages_InAChannelUsesTheSameMatching(t *testing.T) {
	w := newThreadWorld(t, 1)
	hit := w.postText(t, "Báo cáo đối soát", "markdown", "")
	w.postText(t, `<p><strong>other</strong></p>`, "html", "")

	got, err := w.st.SearchMessages(context.Background(), w.chID, "đối so", 20)
	require.NoError(t, err)
	assert.Equal(t, []string{hit.ID}, ids(got), "prefix and diacritics")
	got, err = w.st.SearchMessages(context.Background(), w.chID, "strong", 20)
	require.NoError(t, err)
	assert.Empty(t, got, "markup is not text")
	got, err = w.st.SearchMessages(context.Background(), w.chID, "a & | ! :*", 20)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPinnedMessageIDsIn_CoversSeveralChannelsInOneCall(t *testing.T) {
	a, b := newThreadWorld(t, 1), newThreadWorld(t, 1)
	ctx := context.Background()
	pinA, plainA, pinB := a.post(t, a.users[0], ""), a.post(t, a.users[0], ""), b.post(t, b.users[0], "")
	for _, p := range []struct {
		w *threadWorld
		m *store.Message
	}{{a, pinA}, {b, pinB}} {
		_, err := p.w.pool.Exec(ctx, `INSERT INTO message_pins (channel_id, message_id, pinned_by) VALUES ($1, $2, $3)`,
			p.w.chID, p.m.ID, p.w.users[0])
		require.NoError(t, err)
	}

	got, err := a.st.PinnedMessageIDsIn(ctx, []string{a.chID, b.chID})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{pinA.ID: true, pinB.ID: true}, got)
	assert.NotContains(t, got, plainA.ID)

	only, err := a.st.PinnedMessageIDsIn(ctx, []string{a.chID})
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{pinA.ID: true}, only, "a channel not asked for is not read")
	none, err := a.st.PinnedMessageIDsIn(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}
