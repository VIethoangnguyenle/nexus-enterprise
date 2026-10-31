package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/domain"
)

// A message that mentions everyone keeps its marker through sending and an
// edit: the stored HTML still carries it, and so do the stored mentions.
func TestMentionAll_MarkerSurvivesSendAndEdit(t *testing.T) {
	k := newTalk(t, "mention-all-roundtrip")
	ctx := context.Background()
	body := `<p><span data-mention="` + domain.MentionAll + `">` + domain.MentionAll + `</span> họp lúc 9h</p>`
	msg, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, ContentFormat: "html",
		Content: body, Mentions: []string{domain.MentionAll},
	})
	require.NoError(t, err)
	assert.Equal(t, body, msg.Content)

	edited := `<p><span data-mention="` + domain.MentionAll + `">` + domain.MentionAll + `</span> họp lúc 10h</p>`
	out, err := k.f.svc.EditMessage(ctx, msg.Id, k.f.userID, userNode, edited, "html", []string{domain.MentionAll})
	require.NoError(t, err)
	assert.Equal(t, edited, out.Content)

	var stored string
	var mentions []string
	require.NoError(t, k.f.pool.QueryRow(ctx, `SELECT content, COALESCE(mentions, '{}') FROM messages WHERE id = $1`, msg.Id).
		Scan(&stored, &mentions))
	assert.Contains(t, stored, `data-mention="`+domain.MentionAll+`"`)
	assert.Contains(t, mentions, domain.MentionAll)
}

// Formatted content posted straight to the API is filtered before it is
// stored, and search reads its text, not its markup.
func TestSendMessage_StoresFilteredHTMLAndSearchesItsText(t *testing.T) {
	k := newTalk(t, "richtext")
	ctx := context.Background()
	msg, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, ContentFormat: "html",
		Content: `<p><strong>Quarterly</strong> report<img src=x onerror="alert(1)"><script>steal()</script></p>`,
	})
	require.NoError(t, err)
	assert.Equal(t, `<p><strong>Quarterly</strong> report</p>`, msg.Content, "the answer carries what was stored")

	var stored, text string
	require.NoError(t, k.f.pool.QueryRow(ctx, `SELECT content, content_text FROM messages WHERE id = $1`, msg.Id).
		Scan(&stored, &text))
	assert.NotContains(t, stored, "script")
	assert.NotContains(t, stored, "onerror")
	assert.Equal(t, "Quarterly report", text)

	found, err := k.f.svc.SearchMessages(ctx, k.chID, userNode, "quarterly", 10)
	require.NoError(t, err)
	require.Len(t, found.Messages, 1)
	assert.Equal(t, msg.Id, found.Messages[0].Id)

	none, err := k.f.svc.SearchMessages(ctx, k.chID, userNode, "strong", 10)
	require.NoError(t, err)
	assert.Empty(t, none.Messages, "a tag name is not text")
}
