package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// A document or approval card carries no text of its own: the message only
// links the object, and the card is fetched under each viewer's rights.

func (k *talk) sendLinked(typ, id string) error {
	_, err := k.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode,
		LinkedEntityType: typ, LinkedEntityID: id,
	})
	return err
}

func TestSendMessage_LinkedCardNeedsNoText(t *testing.T) {
	for _, typ := range []string{"document", "approval"} {
		k := newTalk(t, "link-card-"+typ)
		before := k.messageCount(t)

		require.NoError(t, k.sendLinked(typ, "obj-1"), typ)

		assert.Equal(t, before+1, k.messageCount(t), typ)
	}
}

func TestSendMessage_LinkedCardRefusals(t *testing.T) {
	k := newTalk(t, "link-card-deny")
	before := k.messageCount(t)

	require.ErrorIs(t, k.sendLinked("spreadsheet", "obj-1"), domain.ErrInvalidInput, "unknown type")
	require.ErrorIs(t, k.sendLinked("document", ""), domain.ErrInvalidInput, "empty id")
	require.ErrorIs(t, k.sendLinked("document", "   "), domain.ErrInvalidInput, "blank id")
	require.ErrorIs(t, k.sendLinked("", "obj-1"), domain.ErrInvalidInput, "id without a type, no text")

	assert.Equal(t, before, k.messageCount(t), "a refused card stores nothing")
}

func TestSendMessage_LinkedCardNeedsWriteOnTheChannel(t *testing.T) {
	k := newTalk(t, "link-card-nowrite")
	delete(k.f.read.allow, grant{userNode, k.oaID, ngac.OpWrite})
	before := k.messageCount(t)

	require.ErrorIs(t, k.sendLinked("document", "obj-1"), domain.ErrAccessDenied)

	assert.Equal(t, before, k.messageCount(t))
}

func TestSendMessage_UnknownLinkedTypeRefusedEvenWithText(t *testing.T) {
	k := newTalk(t, "link-card-unknown-text")

	_, err := k.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "hi",
		LinkedEntityType: "spreadsheet", LinkedEntityID: "obj-1",
	})

	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// A conversation with no task answers an empty list, which the REST layer
// writes as `[]` and not `null`.
func TestListTasks_EmptyListIsNotNil(t *testing.T) {
	k := newTalk(t, "tasks-empty")

	tasks, err := k.f.svc.ListTasks(context.Background(), k.chID, userNode, "")

	require.NoError(t, err)
	assert.NotNil(t, tasks)
	assert.Empty(t, tasks)
}
