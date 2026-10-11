// Package domain — reactions, pins, read receipts, search domain logic.
package domain

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// --- Reactions ---

// AddReaction adds an emoji reaction to a message.
func (s *Service) AddReaction(ctx context.Context, messageID, userNodeID, userID, emoji string) error {
	if messageID == "" || emoji == "" {
		return ErrInvalidInput
	}
	if err := s.authorizeMessage(ctx, messageID, userNodeID, ngac.OpWrite); err != nil {
		return err
	}
	return s.store.InsertReaction(ctx, messageID, userID, emoji)
}

// RemoveReaction removes an emoji reaction from a message.
func (s *Service) RemoveReaction(ctx context.Context, messageID, userNodeID, userID, emoji string) error {
	if messageID == "" || emoji == "" {
		return ErrInvalidInput
	}
	if err := s.authorizeMessage(ctx, messageID, userNodeID, ngac.OpWrite); err != nil {
		return err
	}
	return s.store.DeleteReaction(ctx, messageID, userID, emoji)
}

// ListReactions returns aggregated reactions for a message.
func (s *Service) ListReactions(ctx context.Context, messageID, userNodeID string) ([]*pb.ReactionGroup, error) {
	if err := s.authorizeMessage(ctx, messageID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
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
func (s *Service) PinMessage(ctx context.Context, channelID, messageID, userNodeID, userID string) error {
	if channelID == "" || messageID == "" {
		return ErrInvalidInput
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpWrite); err != nil {
		return err
	}
	// A pin is shown to everyone who can read the channel, with the message's
	// text. The message must therefore be one of this channel's.
	owner, err := s.store.GetChannelIDForMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	if owner != channelID {
		return fmt.Errorf("%w: message %s is not in this channel", ErrInvalidInput, messageID)
	}
	return s.store.InsertPin(ctx, channelID, messageID, userID)
}

// UnpinMessage removes a pin from a message.
func (s *Service) UnpinMessage(ctx context.Context, channelID, messageID, userNodeID string) error {
	if channelID == "" || messageID == "" {
		return ErrInvalidInput
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpWrite); err != nil {
		return err
	}
	return s.store.DeletePin(ctx, channelID, messageID)
}

// ListPins returns pinned messages for a channel with full message data.
func (s *Service) ListPins(ctx context.Context, channelID, userNodeID string) ([]*pb.PinnedMessage, error) {
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	pins, err := s.store.ListPinsByChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}

	var result []*pb.PinnedMessage
	for _, p := range pins {
		// Load the full message for each pin
		msgs, err := s.store.GetThread(ctx, p.MessageID)
		if err != nil {
			return nil, fmt.Errorf("load pinned message: %w", err)
		}
		// A pin that names another channel's message is never shown.
		if len(msgs) == 0 || msgs[0].ChannelID != channelID {
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
func (s *Service) MarkChannelRead(ctx context.Context, userID, userNodeID, channelID, lastMessageID string) error {
	if channelID == "" {
		return ErrInvalidInput
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return err
	}

	// read_receipts has a foreign key onto messages, so an id the client
	// invented — an optimistic placeholder, or a message from another channel —
	// used to surface as a 500 from a constraint violation. It is client input:
	// reject it as such, and require the message to belong to the channel being
	// marked read so a receipt cannot reference someone else's conversation.
	if lastMessageID != "" {
		owner, err := s.store.GetChannelIDForMessage(ctx, lastMessageID)
		if err != nil {
			return fmt.Errorf("load message: %w", err)
		}
		if owner == "" {
			return fmt.Errorf("%w: unknown message %s", ErrInvalidInput, lastMessageID)
		}
		if owner != channelID {
			return fmt.Errorf("%w: message %s is not in this channel", ErrInvalidInput, lastMessageID)
		}
	}

	return s.store.UpsertReadReceipt(ctx, userID, channelID, lastMessageID)
}

// GetUnreadCounts returns the caller's unread counts for every channel they may
// read: the channels of the workspace they are signed in to and their DMs —
// the channels the sidebar lists. A channel the caller may not read is never
// named; the counts come from the caller's own receipts and thread follows.
func (s *Service) GetUnreadCounts(ctx context.Context, userID, userNodeID, workspaceID string) ([]*pb.ChannelUnread, error) {
	if userID == "" || userNodeID == "" {
		return nil, ErrInvalidInput
	}
	channelIDs, err := s.readableChannelIDs(ctx, workspaceID, userNodeID)
	if err != nil {
		return nil, err
	}
	unreads, err := s.store.GetUnreadCounts(ctx, userID, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("unread counts: %w", err)
	}
	starred, err := s.store.StarredConversations(ctx, userID, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("unread counts: %w", err)
	}
	var result []*pb.ChannelUnread
	for _, u := range unreads {
		result = append(result, &pb.ChannelUnread{
			ChannelId:         u.ChannelID,
			UnreadCount:       u.UnreadCount,
			LastReadMessageId: u.LastReadMsgID,
			ThreadUnreadCount: u.ThreadUnread,
			NotificationLevel: u.Level,
			MentionCount:      u.MentionCount,
			Starred:           starred[u.ChannelID],
		})
	}
	return result, nil
}

// readableChannelIDs lists the channels the sidebar shows the user: the
// workspace's channels and the DMs, each kept only when the user holds read on
// its content OA (one batch check, failing closed like the channel list).
func (s *Service) readableChannelIDs(ctx context.Context, workspaceID, userNodeID string) ([]string, error) {
	var candidates []*store.Channel
	if workspaceID != "" {
		channels, err := s.store.ListChannelsByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, fmt.Errorf("list channels: %w", err)
		}
		candidates = append(candidates, channels...)
	}
	dms, err := s.store.ListAllDMs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list DMs: %w", err)
	}
	candidates = append(candidates, dms...)

	readable := s.filterAccessible(ctx, candidates, userNodeID)
	ids := make([]string, 0, len(readable))
	for _, ch := range readable {
		ids = append(ids, ch.Id)
	}
	return ids, nil
}

// --- Search ---

// SearchMessages performs full-text search on messages in a channel.
func (s *Service) SearchMessages(ctx context.Context, channelID, userNodeID, query string, limit int) (*pb.MessageList, error) {
	if channelID == "" || query == "" {
		return &pb.MessageList{}, nil
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	msgs, err := s.store.SearchMessages(ctx, channelID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	return &pb.MessageList{Messages: messagesToProto(msgs)}, nil
}

// EnrichMessagesWithMetadata adds reactions, pin status and attachments to a slice of messages.
func (s *Service) EnrichMessagesWithMetadata(ctx context.Context, msgs []*store.Message, channelID string) error {
	if len(msgs) == 0 {
		return nil
	}

	// Batch load reactions
	msgIDs := make([]string, len(msgs))
	for i, m := range msgs {
		msgIDs[i] = m.ID
	}
	reactionsMap, err := s.store.ListReactionsForMessages(ctx, msgIDs)
	if err != nil {
		return fmt.Errorf("load reactions: %w", err)
	}

	// Batch load pin status
	pinnedSet, err := s.store.PinnedMessageIDs(ctx, channelID)
	if err != nil {
		return fmt.Errorf("load pins: %w", err)
	}

	// Batch load attachments
	attachments, err := s.store.ListAttachmentsForMessages(ctx, msgIDs)
	if err != nil {
		return fmt.Errorf("load attachments: %w", err)
	}

	for _, m := range msgs {
		if groups, ok := reactionsMap[m.ID]; ok {
			m.Reactions = groups
		}
		m.IsPinned = pinnedSet[m.ID]
		if !m.Deleted {
			m.Attachments = attachments[m.ID]
		}
	}
	return s.attachQuotes(ctx, msgs)
}
