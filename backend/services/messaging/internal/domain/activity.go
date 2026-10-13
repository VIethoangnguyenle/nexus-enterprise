// Package domain — what each conversation last said, for the list previews.
package domain

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// previewMaxRunes bounds a preview, its ellipsis included.
const previewMaxRunes = 140

// The kinds of message a preview names.
const (
	kindText       = "text"
	kindAttachment = "attachment"
	kindPoll       = "poll"
	kindTask       = "task"
	kindCard       = "card"
	kindCall       = "call"
)

// previewText is the opening of a message as one line of plain text. Longer
// than previewMaxRunes it is cut at a word boundary when there is one, else at
// the rune limit, and ends with an ellipsis.
func previewText(format, content string) string {
	text := strings.Join(strings.Fields(plainText(format, content)), " ")
	if utf8.RuneCountInString(text) <= previewMaxRunes {
		return text
	}
	runes := []rune(text)
	limit := previewMaxRunes - 1 // room for the ellipsis
	cut := runes[:limit]
	if !unicode.IsSpace(runes[limit]) {
		if i := lastSpace(cut); i > 0 {
			cut = cut[:i]
		}
	}
	return strings.TrimRightFunc(string(cut), unicode.IsSpace) + "…"
}

func lastSpace(rs []rune) int {
	for i := len(rs) - 1; i >= 0; i-- {
		if unicode.IsSpace(rs[i]) {
			return i
		}
	}
	return -1
}

// messageKind classifies a message from what it carries.
func messageKind(m *store.Message, attachments []store.Attachment) string {
	switch {
	case len(attachments) > 0:
		return kindAttachment
	case m.LinkedEntityType == linkedPoll:
		return kindPoll
	case m.LinkedEntityType == linkedTask:
		return kindTask
	case m.LinkedEntityType == linkedCall:
		return kindCall
	case isLinkedCard(m.LinkedEntityType):
		return kindCard
	}
	return kindText
}

func activityMessage(m *store.Message, attachments []store.Attachment) *pb.ActivityMessage {
	out := &pb.ActivityMessage{
		Id:              m.ID,
		SenderId:        m.SenderID,
		SenderName:      m.SenderName,
		Preview:         previewText(m.ContentFormat, m.Content),
		Kind:            messageKind(m, attachments),
		Mentions:        m.Mentions,
		CreatedAt:       timestamppb.New(m.CreatedAt),
		ParentMessageId: m.ParentMessageID,
	}
	if out.Kind == kindAttachment {
		out.AttachmentName = attachments[0].FileName
	}
	return out
}

// ConversationActivity returns, for every conversation the caller may read —
// the channels the navigator lists and the caller's direct messages — its
// newest main-stream message and its newest thread reply when that is newer.
// A conversation with no message is left out, and so is every conversation the
// caller may not read: the preview carries message text, so it follows the
// same read check as the unread counts.
func (s *Service) ConversationActivity(ctx context.Context, userID, userNodeID, workspaceID string) ([]*pb.ConversationActivity, error) {
	if userID == "" || userNodeID == "" {
		return nil, ErrInvalidInput
	}
	channelIDs, err := s.readableChannelIDs(ctx, workspaceID, userNodeID)
	if err != nil {
		return nil, err
	}
	main, replies, err := s.store.LatestActivity(ctx, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("conversation activity: %w", err)
	}

	ids := make([]string, 0, len(main)+len(replies))
	for _, m := range main {
		ids = append(ids, m.ID)
	}
	for ch, r := range replies {
		if m, ok := main[ch]; ok && r.CreatedAt.After(m.CreatedAt) {
			ids = append(ids, r.ID)
		}
	}
	attachments, err := s.store.ListAttachmentsForMessages(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("conversation activity: %w", err)
	}

	out := make([]*pb.ConversationActivity, 0, len(main))
	for _, chID := range channelIDs { // the readable order, stable between calls
		m, ok := main[chID]
		if !ok {
			continue
		}
		item := &pb.ConversationActivity{ChannelId: chID, Latest: activityMessage(m, attachments[m.ID])}
		if r, ok := replies[chID]; ok && r.CreatedAt.After(m.CreatedAt) {
			item.LatestReply = activityMessage(r, attachments[r.ID])
		}
		out = append(out, item)
	}
	return out, nil
}
