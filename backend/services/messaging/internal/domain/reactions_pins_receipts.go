// Package domain — reactions, pins, read receipts, search domain logic.
package domain

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// --- Reactions ---

// AddReaction adds an emoji reaction to a message.
func (s *Service) AddReaction(ctx context.Context, messageID, userID, emoji string) error {
	if messageID == "" || emoji == "" {
		return ErrInvalidInput
	}
	return s.store.InsertReaction(ctx, messageID, userID, emoji)
}

// RemoveReaction removes an emoji reaction from a message.
func (s *Service) RemoveReaction(ctx context.Context, messageID, userID, emoji string) error {
	if messageID == "" || emoji == "" {
		return ErrInvalidInput
	}
	return s.store.DeleteReaction(ctx, messageID, userID, emoji)
}

// ListReactions returns aggregated reactions for a message.
func (s *Service) ListReactions(ctx context.Context, messageID string) ([]*pb.ReactionGroup, error) {
	groups, err := s.store.ListReactionsByMessage(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("list reactions: %w", err)
	}
	var result []*pb.ReactionGroup
	for _, g := range groups {
		result = append(result, &pb.ReactionGroup{
			Emoji:   g.Emoji,
			Count:   g.Count,
			UserIds: g.UserIDs,
		})
	}
	return result, nil
}

// GetChannelIDForMessage returns the channel_id for a given message.
func (s *Service) GetChannelIDForMessage(ctx context.Context, messageID string) (string, error) {
	return s.store.GetChannelIDForMessage(ctx, messageID)
}

// --- Pins ---

// PinMessage pins a message in a channel.
func (s *Service) PinMessage(ctx context.Context, channelID, messageID, userID string) error {
	if channelID == "" || messageID == "" {
		return ErrInvalidInput
	}
	return s.store.InsertPin(ctx, channelID, messageID, userID)
}

// UnpinMessage removes a pin from a message.
func (s *Service) UnpinMessage(ctx context.Context, channelID, messageID string) error {
	if channelID == "" || messageID == "" {
		return ErrInvalidInput
	}
	return s.store.DeletePin(ctx, channelID, messageID)
}

// ListPins returns pinned messages for a channel with full message data.
func (s *Service) ListPins(ctx context.Context, channelID string) ([]*pb.PinnedMessage, error) {
	pins, err := s.store.ListPinsByChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}

	var result []*pb.PinnedMessage
	for _, p := range pins {
		// Load the full message for each pin
		msgs, _ := s.store.GetThread(ctx, p.MessageID)
		if len(msgs) == 0 {
			continue
		}
		result = append(result, &pb.PinnedMessage{
			Message:  messageToProto(msgs[0]),
			PinnedBy: p.PinnedBy,
			PinnedAt: timestamppb.New(p.CreatedAt),
		})
	}
	return result, nil
}

// --- Read Receipts ---

// MarkChannelRead marks a channel as read up to a specific message.
func (s *Service) MarkChannelRead(ctx context.Context, userID, channelID, lastMessageID string) error {
	if channelID == "" {
		return ErrInvalidInput
	}
	return s.store.UpsertReadReceipt(ctx, userID, channelID, lastMessageID)
}

// GetUnreadCounts returns unread message counts for all channels a user is a member of.
func (s *Service) GetUnreadCounts(ctx context.Context, userID string) ([]*pb.ChannelUnread, error) {
	unreads, err := s.store.GetUnreadCounts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("unread counts: %w", err)
	}
	var result []*pb.ChannelUnread
	for _, u := range unreads {
		result = append(result, &pb.ChannelUnread{
			ChannelId:         u.ChannelID,
			UnreadCount:       u.UnreadCount,
			LastReadMessageId: u.LastReadMsgID,
		})
	}
	return result, nil
}

// --- Search ---

// SearchMessages performs full-text search on messages in a channel.
func (s *Service) SearchMessages(ctx context.Context, channelID, query string, limit int) (*pb.MessageList, error) {
	if channelID == "" || query == "" {
		return &pb.MessageList{}, nil
	}
	msgs, err := s.store.SearchMessages(ctx, channelID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	return &pb.MessageList{Messages: messagesToProto(msgs)}, nil
}

// EnrichMessagesWithMetadata adds reactions and pin status to a slice of messages.
func (s *Service) EnrichMessagesWithMetadata(ctx context.Context, msgs []*store.Message, channelID string) {
	if len(msgs) == 0 {
		return
	}

	// Batch load reactions
	msgIDs := make([]string, len(msgs))
	for i, m := range msgs {
		msgIDs[i] = m.ID
	}
	reactionsMap, _ := s.store.ListReactionsForMessages(ctx, msgIDs)

	// Batch load pin status
	pinnedSet, _ := s.store.PinnedMessageIDs(ctx, channelID)

	for _, m := range msgs {
		if groups, ok := reactionsMap[m.ID]; ok {
			m.Reactions = groups
		}
		m.IsPinned = pinnedSet[m.ID]
	}
}
