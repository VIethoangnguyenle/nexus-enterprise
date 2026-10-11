// Package domain — in-line threads: follow, per-thread read position, thread list.
package domain

import (
	"context"
	"fmt"
	"time"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// threadPageLimit caps a page of the thread list.
const threadPageLimit = 50

// checkReplyParent refuses a reply whose parent is not a root message of the
// channel the reply is sent to. A parent in another channel would let a writer
// of this channel add to a thread they may not write, and nested replies are
// not part of the in-line thread model.
func (s *Service) checkReplyParent(ctx context.Context, parentID, channelID string) error {
	parentChannel, grandparent, found, err := s.store.MessageThreadInfo(ctx, parentID)
	if err != nil {
		return fmt.Errorf("load parent message: %w", err)
	}
	switch {
	case !found:
		return fmt.Errorf("%w: unknown parent message %s", ErrInvalidInput, parentID)
	case parentChannel != channelID:
		return fmt.Errorf("%w: parent message %s is not in this channel", ErrInvalidInput, parentID)
	case grandparent != "":
		return fmt.Errorf("%w: a reply cannot be replied to; reply to the thread's root message", ErrInvalidInput)
	}
	return nil
}

// authorizeThreadRoot checks that messageID is a root message and that the
// caller may read its channel. Threads are not NGAC nodes; the check is read
// on the channel's content OA.
func (s *Service) authorizeThreadRoot(ctx context.Context, messageID, userNodeID string) error {
	if messageID == "" {
		return ErrInvalidInput
	}
	channelID, parentID, found, err := s.store.MessageThreadInfo(ctx, messageID)
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	if !found {
		return fmt.Errorf("%w: message not found", ErrNotFound)
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return err
	}
	if parentID != "" {
		return fmt.Errorf("%w: a thread is named by its root message", ErrInvalidInput)
	}
	return nil
}

// FollowThread makes the caller follow a thread.
func (s *Service) FollowThread(ctx context.Context, messageID, userID, userNodeID string) error {
	if err := s.authorizeThreadRoot(ctx, messageID, userNodeID); err != nil {
		return err
	}
	return s.store.SetThreadFollow(ctx, userID, messageID, true)
}

// UnfollowThread stops the caller following a thread until they reply to it,
// are mentioned in it, or follow it again.
func (s *Service) UnfollowThread(ctx context.Context, messageID, userID, userNodeID string) error {
	if err := s.authorizeThreadRoot(ctx, messageID, userNodeID); err != nil {
		return err
	}
	return s.store.SetThreadFollow(ctx, userID, messageID, false)
}

// MarkThreadRead marks every reply of a thread as read by the caller.
func (s *Service) MarkThreadRead(ctx context.Context, messageID, userID, userNodeID string) error {
	if err := s.authorizeThreadRoot(ctx, messageID, userNodeID); err != nil {
		return err
	}
	return s.store.MarkThreadRead(ctx, userID, messageID)
}

// ListThreads returns the threads of a channel, most recently active first,
// each with the caller's thread summary. filter is "all" (or empty),
// "following" or "unread"; before is an RFC 3339 last-reply time to page from,
// and beforeID the root id of the last thread of the previous page.
func (s *Service) ListThreads(ctx context.Context, channelID, userID, userNodeID, filter, before, beforeID string, limit int) (*pb.MessageList, error) {
	switch filter {
	case "":
		filter = store.ThreadFilterAll
	case store.ThreadFilterAll, store.ThreadFilterFollowing, store.ThreadFilterUnread:
	default:
		return nil, fmt.Errorf("%w: filter must be all, following or unread", ErrInvalidInput)
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > threadPageLimit {
		limit = threadPageLimit
	}
	var beforeTime *time.Time
	if before != "" {
		t, err := time.Parse(time.RFC3339Nano, before)
		if err != nil {
			return nil, fmt.Errorf("%w: before must be an RFC 3339 time", ErrInvalidInput)
		}
		beforeTime = &t
	}

	msgs, hasMore, err := s.store.ListThreads(ctx, channelID, userID, filter, beforeTime, beforeID, limit)
	if err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	if err := s.EnrichMessagesWithMetadata(ctx, msgs, channelID); err != nil {
		return nil, err
	}
	return &pb.MessageList{Messages: messagesToProto(msgs), HasMore: hasMore}, nil
}

// applyThreadStates fills the caller's thread summary into the root messages
// of a page, with one query for the whole page. Replies are left as they are.
func (s *Service) applyThreadStates(ctx context.Context, userID string, msgs []*store.Message) error {
	if userID == "" {
		return nil
	}
	roots := make([]string, 0, len(msgs))
	for _, m := range msgs {
		if m.ParentMessageID == "" {
			roots = append(roots, m.ID)
		}
	}
	if len(roots) == 0 {
		return nil
	}
	states, err := s.store.ThreadStates(ctx, userID, roots)
	if err != nil {
		return fmt.Errorf("load thread states: %w", err)
	}
	for _, m := range msgs {
		if st, ok := states[m.ID]; ok && m.ParentMessageID == "" {
			m.Following = st.Following
			m.ThreadUnread = st.Unread
			m.ReplyParticipantIDs = st.Participants
		}
	}
	return nil
}
