// Package domain — a person's stars and the feed of messages that mention them.
package domain

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// StarConversation stars a conversation the caller reads. A star is the
// caller's own; it never widens what they may read.
func (s *Service) StarConversation(ctx context.Context, channelID, userID, userNodeID string) error {
	return s.setConversationStar(ctx, channelID, userID, userNodeID, true)
}

// UnstarConversation removes the caller's star from a conversation they read.
func (s *Service) UnstarConversation(ctx context.Context, channelID, userID, userNodeID string) error {
	return s.setConversationStar(ctx, channelID, userID, userNodeID, false)
}

func (s *Service) setConversationStar(ctx context.Context, channelID, userID, userNodeID string, starred bool) error {
	if channelID == "" || userID == "" {
		return ErrInvalidInput
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return err
	}
	return s.store.SetConversationStar(ctx, userID, channelID, starred)
}

// StarMessage stars a message in a conversation the caller reads. A deleted
// message cannot be starred.
func (s *Service) StarMessage(ctx context.Context, messageID, userID, userNodeID string) error {
	return s.setMessageStar(ctx, messageID, userID, userNodeID, true)
}

// UnstarMessage removes the caller's star from a message, deleted or not, in a
// conversation they read.
func (s *Service) UnstarMessage(ctx context.Context, messageID, userID, userNodeID string) error {
	return s.setMessageStar(ctx, messageID, userID, userNodeID, false)
}

func (s *Service) setMessageStar(ctx context.Context, messageID, userID, userNodeID string, starred bool) error {
	if messageID == "" || userID == "" {
		return ErrInvalidInput
	}
	facts, err := s.store.GetMessageFacts(ctx, messageID)
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	if facts == nil {
		return fmt.Errorf("%w: message not found", ErrNotFound)
	}
	// Messages are not graph nodes: read is checked on the message's channel.
	if _, err := s.authorizeChannel(ctx, facts.ChannelID, userNodeID, ngac.OpRead); err != nil {
		return err
	}
	if starred && facts.Deleted {
		return fmt.Errorf("%w: a deleted message cannot be starred", ErrInvalidInput)
	}
	return s.store.SetMessageStar(ctx, userID, messageID, starred)
}

// GetChannelFor returns a conversation the caller reads, with whether they
// starred it.
func (s *Service) GetChannelFor(ctx context.Context, channelID, userID, userNodeID string) (*pb.Channel, error) {
	ch, err := s.GetChannel(ctx, channelID, userNodeID)
	if err != nil {
		return nil, err
	}
	starred, err := s.store.StarredConversations(ctx, userID, []string{channelID})
	if err != nil {
		return nil, fmt.Errorf("load star: %w", err)
	}
	_, ch.Starred = starred[channelID]
	return ch, nil
}

// feedPage parses a feed's paging cursor and caps its page size.
func feedPage(before string, limit int) (*time.Time, int, error) {
	if limit <= 0 || limit > threadPageLimit {
		limit = threadPageLimit
	}
	if before == "" {
		return nil, limit, nil
	}
	t, err := time.Parse(time.RFC3339Nano, before)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: before must be an RFC 3339 time", ErrInvalidInput)
	}
	return &t, limit, nil
}

// StarredMessages returns the messages the caller starred, newest star first,
// kept only where the caller still reads the conversation — the workspace they
// are signed in to and their direct messages. before is an RFC 3339 star time
// to page from and beforeID the id of the last message of the previous page.
func (s *Service) StarredMessages(ctx context.Context, userID, userNodeID, workspaceID, before, beforeID string, limit int) (*pb.MessageList, error) {
	return s.messageFeed(ctx, userID, userNodeID, workspaceID, before, beforeID, limit, s.store.StarredMessages)
}

// Mentions returns the messages by others that mention the caller, newest
// first, in the conversations they read (the same scope as StarredMessages),
// each flagged unread or not. before is an RFC 3339 send time to page from.
func (s *Service) Mentions(ctx context.Context, userID, userNodeID, workspaceID, before, beforeID string, limit int) (*pb.MessageList, error) {
	return s.messageFeed(ctx, userID, userNodeID, workspaceID, before, beforeID, limit, s.store.MentionsOf)
}

// UnreadMentionCount is the number on the "mentioned" shortcut: the unread
// messages of the Mentions feed, counted over the same conversations.
func (s *Service) UnreadMentionCount(ctx context.Context, userID, userNodeID, workspaceID string) (int, error) {
	if userID == "" || userNodeID == "" {
		return 0, ErrInvalidInput
	}
	channelIDs, err := s.readableChannelIDs(ctx, workspaceID, userNodeID)
	if err != nil {
		return 0, err
	}
	return s.store.CountUnreadMentions(ctx, userID, channelIDs)
}

type feedQuery func(ctx context.Context, userID string, channelIDs []string, before *time.Time, beforeID string, limit int) ([]*store.Message, bool, error)

func (s *Service) messageFeed(ctx context.Context, userID, userNodeID, workspaceID, before, beforeID string, limit int, query feedQuery) (*pb.MessageList, error) {
	if userID == "" || userNodeID == "" {
		return nil, ErrInvalidInput
	}
	beforeTime, limit, err := feedPage(before, limit)
	if err != nil {
		return nil, err
	}
	channelIDs, err := s.readableChannelIDs(ctx, workspaceID, userNodeID)
	if err != nil {
		return nil, err
	}
	msgs, hasMore, err := query(ctx, userID, channelIDs, beforeTime, beforeID, limit)
	if err != nil {
		return nil, err
	}
	// Reactions, pins and attachments are per channel.
	byChannel := map[string][]*store.Message{}
	for _, m := range msgs {
		byChannel[m.ChannelID] = append(byChannel[m.ChannelID], m)
	}
	for ch, group := range byChannel {
		if err := s.EnrichMessagesWithMetadata(ctx, group, ch); err != nil {
			return nil, err
		}
	}
	if err := s.applyThreadStates(ctx, userID, msgs); err != nil {
		return nil, err
	}
	return &pb.MessageList{Messages: messagesToProto(msgs), HasMore: hasMore}, nil
}

func starredAtProto(m *store.Message) *timestamppb.Timestamp {
	if m.StarredAt == nil {
		return nil
	}
	return timestamppb.New(*m.StarredAt)
}
