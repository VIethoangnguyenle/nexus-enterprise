// Package domain — editing, deleting, quoting and un-reading messages.
//
// Messages are not NGAC nodes: every check here is on the content OA of the
// message's channel, and "whose message" is the sender recorded on the row.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// liveMessage returns a message that exists and is not deleted.
func (s *Service) liveMessage(ctx context.Context, id string) (*store.MessageFacts, error) {
	if id == "" {
		return nil, ErrInvalidInput
	}
	f, err := s.store.GetMessageFacts(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load message: %w", err)
	}
	if f == nil || f.Deleted {
		return nil, fmt.Errorf("%w: message not found", ErrNotFound)
	}
	return f, nil
}

// EditMessage changes the text of the caller's own message. It needs write on
// the channel; a card (poll, task…) is not edited as text, and an edit to
// nothing is refused — removing the text is a delete.
//
// mentions is the message's mention list after the edit; nil leaves it as it
// was. Only the people the edit adds to it are notified (see pushNewMentions);
// people already mentioned are not told again, and removing a mention tells
// nobody.
func (s *Service) EditMessage(ctx context.Context, messageID, userID, userNodeID, content, format string, mentions []string) (*pb.Message, error) {
	f, err := s.liveMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	ch, err := s.authorizeChannel(ctx, f.ChannelID, userNodeID, ngac.OpWrite)
	if err != nil {
		return nil, err
	}
	if f.SenderID != userID {
		return nil, fmt.Errorf("%w: only its sender edits a message", ErrAccessDenied)
	}
	if f.LinkedEntityType != "" && f.LinkedEntityType != "drive_file" {
		return nil, fmt.Errorf("%w: a card is not edited as text", ErrInvalidInput)
	}
	if format == "" {
		format = "markdown"
	}
	stored, text, format, err := prepareContent(format, content)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: an edit needs text", ErrInvalidInput)
	}
	added, err := s.store.UpdateMessageContent(ctx, messageID, stored, text, format, cleanMentions(mentions))
	if err != nil {
		if errors.Is(err, store.ErrMessageGone) {
			return nil, fmt.Errorf("%w: message not found", ErrNotFound)
		}
		return nil, err
	}
	msgs, err := s.store.GetThread(ctx, messageID)
	if err != nil || len(msgs) == 0 {
		return nil, fmt.Errorf("reload message: %w", err)
	}
	edited := msgs[0]
	if err := s.EnrichMessagesWithMetadata(ctx, []*store.Message{edited}, ch.ID); err != nil {
		return nil, err
	}
	out := messageToProto(edited)
	s.pushNewMentions(ctx, out, added)
	return out, nil
}

// cleanMentions drops blank and repeated ids; nil stays nil (no change).
func cleanMentions(ids []string) []string {
	if ids == nil {
		return nil
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; id == "" || dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// DeleteOutcome is what a delete tells the live views: the deletion itself,
// and — when a manager removed someone else's message — the line recorded in
// the conversation saying so.
type DeleteOutcome struct {
	Event  *pb.MessageDeletedEvent
	Notice *pb.Message
}

// DeleteMessage deletes a message: its sender may (with write on the channel),
// and so may anyone who manages the channel. A manager's deletion of someone
// else's message is recorded on the row and as a line in the conversation.
func (s *Service) DeleteMessage(ctx context.Context, messageID, userID, userNodeID string) (*DeleteOutcome, error) {
	f, err := s.liveMessage(ctx, messageID)
	if err != nil {
		return nil, err
	}
	op := ngac.OpManage
	if f.SenderID == userID {
		op = ngac.OpWrite
	}
	if _, err := s.authorizeChannel(ctx, f.ChannelID, userNodeID, op); err != nil {
		return nil, err
	}
	d, err := s.store.SoftDeleteMessage(ctx, messageID, userID)
	if err != nil {
		if errors.Is(err, store.ErrMessageGone) {
			return nil, fmt.Errorf("%w: message not found", ErrNotFound)
		}
		return nil, err
	}
	out := &DeleteOutcome{Event: &pb.MessageDeletedEvent{
		MessageId: messageID, ChannelId: d.ChannelID, ParentMessageId: d.ParentMessageID,
		KeepPlaceholder: d.KeepPlaceholder, ReplyCount: d.ParentReplyCount,
	}}
	if f.SenderID != userID {
		deleter, err := s.store.DisplayName(ctx, userID)
		if err != nil {
			return nil, err
		}
		author, err := s.store.DisplayName(ctx, f.SenderID)
		if err != nil {
			return nil, err
		}
		notice, err := s.SendMessage(ctx, SendMessageInput{
			ChannelID: d.ChannelID, SenderID: userID, SenderNodeID: userNodeID, MessageType: "system",
			ContentFormat:    "plain",
			Content:          fmt.Sprintf("%s đã xoá một tin nhắn của %s", deleter, author),
			LinkedEntityType: "message_deleted", LinkedEntityID: messageID,
		})
		if err != nil {
			return nil, fmt.Errorf("record deletion: %w", err)
		}
		out.Notice = notice
	}
	return out, nil
}

// MarkMessageUnread makes a message, and everything after it, unread for the
// caller: in the channel for a root message, in its thread for a reply.
func (s *Service) MarkMessageUnread(ctx context.Context, messageID, userID, userNodeID string) error {
	f, err := s.liveMessage(ctx, messageID)
	if err != nil {
		return err
	}
	if _, err := s.authorizeChannel(ctx, f.ChannelID, userNodeID, ngac.OpRead); err != nil {
		return err
	}
	if f.ParentMessageID != "" {
		return s.store.SetThreadReadBefore(ctx, userID, f.ParentMessageID, f.CreatedAt)
	}
	return s.store.SetChannelReadBefore(ctx, userID, f.ChannelID, f.CreatedAt)
}

// checkQuote refuses a quote of a message outside the channel, or deleted: a
// quote shows the quoted text to everyone who reads this channel.
func (s *Service) checkQuote(ctx context.Context, quotedID, channelID string) error {
	f, err := s.store.GetMessageFacts(ctx, quotedID)
	if err != nil {
		return fmt.Errorf("load quoted message: %w", err)
	}
	if f == nil || f.Deleted || f.ChannelID != channelID {
		return fmt.Errorf("%w: a quote names a message of this channel", ErrInvalidInput)
	}
	return nil
}

// attachQuotes fills the quoted message of every message of a page that has one.
func (s *Service) attachQuotes(ctx context.Context, msgs []*store.Message) error {
	var ids []string
	for _, m := range msgs {
		if m.QuotedMessageID != "" {
			ids = append(ids, m.QuotedMessageID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	quotes, err := s.store.QuotesByID(ctx, ids)
	if err != nil {
		return err
	}
	for _, m := range msgs {
		// A quote shows only a message of the same channel, whatever a row says.
		if q, ok := quotes[m.QuotedMessageID]; ok && q.ChannelID == m.ChannelID {
			q := q
			m.Quoted = &q
		}
	}
	return nil
}

func quoteToProto(q *store.Quote) *pb.QuotedMessage {
	if q == nil {
		return nil
	}
	return &pb.QuotedMessage{Id: q.ID, SenderId: q.SenderID, SenderName: q.SenderName, Text: q.Text, Deleted: q.Deleted}
}

func editedAtProto(m *store.Message) *timestamppb.Timestamp {
	if m.EditedAt == nil {
		return nil
	}
	return timestamppb.New(*m.EditedAt)
}
