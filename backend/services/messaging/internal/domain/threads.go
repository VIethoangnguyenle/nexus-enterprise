// Package domain — in-line threads: follow, per-thread read position, thread list.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

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
	parent, err := s.store.GetMessageFacts(ctx, parentID)
	if err != nil {
		return fmt.Errorf("load parent message: %w", err)
	}
	switch {
	case parent == nil || parent.ChannelID != channelID:
		// One answer for both, so a writer learns nothing of other channels' ids.
		return fmt.Errorf("%w: the parent message is not one of this channel's", ErrInvalidInput)
	case parent.Deleted:
		return fmt.Errorf("%w: the message replied to was deleted", ErrInvalidInput)
	case parent.ParentMessageID != "":
		return fmt.Errorf("%w: a reply cannot be replied to; reply to the thread's root message", ErrInvalidInput)
	}
	return nil
}

// authorizeThreadRoot checks that messageID is a root message and that the
// caller may read its channel, and returns that channel. Threads are not NGAC
// nodes; the check is read on the channel's content OA.
func (s *Service) authorizeThreadRoot(ctx context.Context, messageID, userNodeID string) (string, error) {
	if messageID == "" {
		return "", ErrInvalidInput
	}
	channelID, parentID, found, err := s.store.MessageThreadInfo(ctx, messageID)
	if err != nil {
		return "", fmt.Errorf("load message: %w", err)
	}
	if !found {
		return "", fmt.Errorf("%w: message not found", ErrNotFound)
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return "", err
	}
	if parentID != "" {
		return "", fmt.Errorf("%w: a thread is named by its root message", ErrInvalidInput)
	}
	return channelID, nil
}

// threadStateFor is the caller's own state of one thread, as sent to their sessions.
func (s *Service) threadStateFor(ctx context.Context, channelID, messageID, userID string) (*pb.ThreadStateEvent, error) {
	asOf := timestamppb.Now() // stamped before the read, as in ThreadReplyFanout
	states, err := s.store.ThreadStates(ctx, userID, []string{messageID})
	if err != nil {
		return nil, fmt.Errorf("load thread state: %w", err)
	}
	st := states[messageID]
	return &pb.ThreadStateEvent{
		RootMessageId: messageID, ChannelId: channelID, Following: st.Following, UnreadCount: st.Unread,
		AsOf: asOf,
	}, nil
}

// FollowThread makes the caller follow a thread and returns their new state.
func (s *Service) FollowThread(ctx context.Context, messageID, userID, userNodeID string) (*pb.ThreadStateEvent, error) {
	channelID, err := s.authorizeThreadRoot(ctx, messageID, userNodeID)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetThreadFollow(ctx, userID, messageID, true); err != nil {
		return nil, err
	}
	return s.threadStateFor(ctx, channelID, messageID, userID)
}

// UnfollowThread stops the caller following a thread until they reply to it,
// are mentioned in it, or follow it again, and returns their new state.
func (s *Service) UnfollowThread(ctx context.Context, messageID, userID, userNodeID string) (*pb.ThreadStateEvent, error) {
	channelID, err := s.authorizeThreadRoot(ctx, messageID, userNodeID)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetThreadFollow(ctx, userID, messageID, false); err != nil {
		return nil, err
	}
	return s.threadStateFor(ctx, channelID, messageID, userID)
}

// MarkThreadRead marks every reply of a thread as read by the caller and
// returns their new state.
func (s *Service) MarkThreadRead(ctx context.Context, messageID, userID, userNodeID string) (*pb.ThreadStateEvent, error) {
	channelID, err := s.authorizeThreadRoot(ctx, messageID, userNodeID)
	if err != nil {
		return nil, err
	}
	if err := s.store.MarkThreadRead(ctx, userID, messageID); err != nil {
		return nil, err
	}
	return s.threadStateFor(ctx, channelID, messageID, userID)
}

// ThreadFanout is what one reply tells the live sessions: the thread summary
// for everyone watching the channel, and each follower's own state, keyed by
// the NGAC node their sessions authenticate as.
type ThreadFanout struct {
	Reply *pb.ThreadReplyEvent
	// States holds only followers who may read the channel now.
	States map[string]*pb.ThreadStateEvent
	// TenantID is the boundary the states are delivered inside.
	TenantID string
}

// ThreadReplyFanout builds the live events for a reply that was just stored.
// A root message is not a reply and yields nil. It reads only state the reply's
// sender was already allowed to change; delivery decides who receives what.
func (s *Service) ThreadReplyFanout(ctx context.Context, reply *pb.Message, callerTenantID string) (*ThreadFanout, error) {
	if reply == nil || reply.ParentMessageId == "" {
		return nil, nil
	}
	rootID := reply.ParentMessageId
	summary, err := s.store.ThreadSummary(ctx, rootID)
	if err != nil {
		return nil, err
	}
	if summary == nil {
		return nil, nil
	}
	ch, err := s.loadChannel(ctx, summary.ChannelID)
	if err != nil {
		return nil, err
	}
	ev := &pb.ThreadReplyEvent{
		Message:             ChatMessageOf(reply),
		ParentMessageId:     rootID,
		ReplyCount:          summary.ReplyCount,
		ReplyParticipantIds: summary.Participants,
	}
	if summary.LastReplyAt != nil {
		ev.LastReplyAt = timestamppb.New(*summary.LastReplyAt)
	}
	// A workspace channel's events stay in its workspace; a DM has none, and
	// goes to the tenant the reply was sent from.
	fan := &ThreadFanout{Reply: ev, TenantID: ch.WorkspaceID, States: map[string]*pb.ThreadStateEvent{}}
	if fan.TenantID == "" {
		fan.TenantID = callerTenantID
	}

	// Stamped before the read: a frame stamped later saw no older data.
	asOf := timestamppb.Now()
	followers, err := s.store.ThreadFollowers(ctx, rootID)
	if err != nil {
		return nil, err
	}
	for _, f := range followers {
		if f.NodeID == "" {
			continue
		}
		// Following is not access: a mention or a membership since revoked
		// leaves a follow behind. Only a follower who reads the channel now is
		// told anything about it; a policy error tells nobody.
		if err := s.checkAccess(ctx, f.NodeID, ch.NGACOaID, ngac.OpRead); err != nil {
			continue
		}
		fan.States[f.NodeID] = &pb.ThreadStateEvent{
			RootMessageId: rootID, ChannelId: summary.ChannelID, Following: true, UnreadCount: f.Unread, AsOf: asOf,
		}
	}
	return fan, nil
}

// ReplyAnnouncer is what delivers a reply's live events: the hub.
type ReplyAnnouncer interface {
	BroadcastThreadReply(channelID string, ev *pb.ThreadReplyEvent)
	SendThreadStates(tenantID string, states map[string]*pb.ThreadStateEvent)
}

// announceTimeout bounds the work of announcing one reply.
const announceTimeout = 10 * time.Second

// AnnounceReply tells the live sessions about a reply that was just stored,
// off the request path: the sender's response does not wait for the summary
// or one publish per follower, and a client that hangs up does not cancel it.
// When the summary cannot be built the reply still goes out, without it.
func (s *Service) AnnounceReply(ctx context.Context, reply *pb.Message, callerTenantID string, to ReplyAnnouncer) {
	if to == nil || reply == nil || reply.ParentMessageId == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), announceTimeout)
		defer cancel()
		fan, err := s.ThreadReplyFanout(ctx, reply, callerTenantID)
		if err != nil || fan == nil {
			slog.Warn("thread summary not built; announcing the reply alone", "message_id", reply.Id, "error", err)
			to.BroadcastThreadReply(reply.ChannelId, &pb.ThreadReplyEvent{
				Message: ChatMessageOf(reply), ParentMessageId: reply.ParentMessageId,
			})
			return
		}
		to.BroadcastThreadReply(reply.ChannelId, fan.Reply)
		to.SendThreadStates(fan.TenantID, fan.States)
	}()
}

// ChatMessageOf is the live-stream form of a stored message: the one
// conversion every frame that carries a message uses.
func ChatMessageOf(m *pb.Message) *pb.ChatMessage {
	cm := &pb.ChatMessage{
		Id: m.Id, ChannelId: m.ChannelId, SenderId: m.SenderId, SenderName: m.SenderName,
		Content: m.Content, CreatedAt: m.CreatedAt, MessageType: m.MessageType,
		ParentMessageId: m.ParentMessageId, ReplyCount: m.ReplyCount,
		ContentFormat: m.ContentFormat, Mentions: m.Mentions,
		LinkedEntityType: m.LinkedEntityType, LinkedEntityId: m.LinkedEntityId,
		Attachments: chatAttachments(m.Attachments),
	}
	if q := m.Quoted; q != nil {
		cm.Quoted = &pb.ChatQuote{Id: q.Id, SenderId: q.SenderId, SenderName: q.SenderName, Text: q.Text, Deleted: q.Deleted}
	}
	return cm
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
