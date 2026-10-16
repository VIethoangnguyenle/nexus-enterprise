// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// SendMessageInput holds validated parameters for sending a message.
type SendMessageInput struct {
	ChannelID        string
	SenderID         string
	SenderNodeID     string
	Content          string
	ContentFormat    string
	Mentions         []string
	MessageType      string
	ParentMessageID  string
	LinkedEntityType string
	LinkedEntityID   string
	Attachments      []AttachmentInput
	QuotedMessageID  string
}

// The linked_entity_type values a message may carry.
const (
	linkedDocument  = "document"
	linkedDriveFile = "drive_file"
	linkedPoll      = "poll"
	linkedTask      = "task"
)

// isLinkedCard reports whether a message of this linked type is a card: it
// names an object and may have no text. The object is not disclosed by the
// message; each viewer's card is fetched under that viewer's own rights.
func isLinkedCard(linkedType string) bool {
	return linkedType == linkedDocument || linkedType == linkedApproval
}

// validateLinkedEntity refuses a link to a type the platform does not know, or
// a typed link with no id. No link at all is valid.
func validateLinkedEntity(linkedType, linkedID string) error {
	switch linkedType {
	case "":
		return nil
	case linkedDocument, linkedApproval, linkedDriveFile, linkedPoll, linkedTask:
		if strings.TrimSpace(linkedID) == "" {
			return fmt.Errorf("%w: a linked %s needs an id", ErrInvalidInput, linkedType)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown linked type", ErrInvalidInput)
	}
}

// SendMessage sends a message after access checking.
func (s *Service) SendMessage(ctx context.Context, in SendMessageInput) (*pb.Message, error) {
	ch, err := s.loadChannel(ctx, in.ChannelID)
	if err != nil {
		return nil, err
	}

	msgType := in.MessageType
	if msgType == "" {
		msgType = "user"
	}

	// Only what a person sends is judged: the service's own system lines carry
	// link types of their own.
	if msgType == "user" {
		if err := validateLinkedEntity(in.LinkedEntityType, in.LinkedEntityID); err != nil {
			return nil, err
		}
	}
	if err := s.requireApprovalsAvailable(ctx, ch, in.LinkedEntityType); err != nil {
		return nil, err
	}
	if msgType == "user" {
		if err := s.checkAccess(ctx, in.SenderNodeID, ch.NGACOaID, ngac.OpWrite); err != nil {
			return nil, err
		}
	}
	if in.ParentMessageID != "" {
		if err := s.checkReplyParent(ctx, in.ParentMessageID, in.ChannelID); err != nil {
			return nil, err
		}
	}
	if in.QuotedMessageID != "" {
		if err := s.checkQuote(ctx, in.QuotedMessageID, in.ChannelID); err != nil {
			return nil, err
		}
	}
	// An old-style file link is an attachment by another name: it goes through
	// the same check, so it cannot name a file outside the channel's drive.
	if in.LinkedEntityType == linkedDriveFile {
		in.Attachments = append([]AttachmentInput{{DriveFileID: in.LinkedEntityID}}, in.Attachments...)
		in.LinkedEntityType, in.LinkedEntityID = "", ""
	}
	attachments, err := s.resolveAttachments(ctx, in.ChannelID, in.Attachments)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	contentFormat := in.ContentFormat
	if contentFormat == "" {
		contentFormat = "markdown"
	}
	content, contentText, contentFormat, err := prepareContent(contentFormat, in.Content)
	if err != nil {
		return nil, err
	}
	// Judged after filtering: markup that filters down to nothing is no text.
	// A card links an object and carries no text of its own.
	if msgType == "user" && strings.TrimSpace(contentText) == "" && len(attachments) == 0 && !isLinkedCard(in.LinkedEntityType) {
		return nil, fmt.Errorf("%w: a message needs text or a file", ErrInvalidInput)
	}
	msg := &store.Message{
		ID:               uuid.New().String(),
		ChannelID:        in.ChannelID,
		SenderID:         in.SenderID,
		Content:          content,
		ContentText:      contentText,
		ContentFormat:    contentFormat,
		Mentions:         in.Mentions,
		MessageType:      msgType,
		ParentMessageID:  in.ParentMessageID,
		LinkedEntityType: in.LinkedEntityType,
		LinkedEntityID:   in.LinkedEntityID,
		Attachments:      attachments,
		QuotedMessageID:  in.QuotedMessageID,
		CreatedAt:        now,
	}

	if err := s.store.InsertMessage(ctx, msg); err != nil {
		if errors.Is(err, store.ErrParentGone) {
			return nil, fmt.Errorf("%w: the message replied to was deleted", ErrInvalidInput)
		}
		return nil, fmt.Errorf("send message: %w", err)
	}
	s.bumpChannel(ctx, ch)

	msg.SenderName = s.lookupUsername(ctx, in.SenderID)
	if err := s.attachQuotes(ctx, []*store.Message{msg}); err != nil {
		return nil, err
	}

	return messageToProto(msg), nil
}

// GetMessages returns paginated messages for a channel, each root message with
// the caller's thread summary.
func (s *Service) GetMessages(ctx context.Context, channelID, userID, userNodeID, before string, limit int) (*pb.MessageList, error) {
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 50 {
		limit = 50
	}

	var beforeTime *time.Time
	if before != "" {
		t, err := time.Parse(time.RFC3339Nano, before)
		if err != nil {
			return nil, fmt.Errorf("%w: before must be an RFC 3339 time", ErrInvalidInput)
		}
		beforeTime = &t
	}

	msgs, hasMore, err := s.store.ListMessages(ctx, channelID, beforeTime, limit)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	if err := s.EnrichMessagesWithMetadata(ctx, msgs, channelID); err != nil {
		return nil, err
	}
	if err := s.applyThreadStates(ctx, userID, msgs); err != nil {
		return nil, err
	}

	return &pb.MessageList{
		Messages: messagesToProto(msgs),
		HasMore:  hasMore,
	}, nil
}

// GetThread returns the parent message, with the caller's thread summary, plus all replies.
func (s *Service) GetThread(ctx context.Context, messageID, userID, userNodeID string) (*pb.MessageList, error) {
	if err := s.authorizeMessage(ctx, messageID, userNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	msgs, err := s.store.GetThread(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}

	// Enrich with reactions and pin status; thread messages share the parent's channel.
	if len(msgs) > 0 {
		if err := s.EnrichMessagesWithMetadata(ctx, msgs, msgs[0].ChannelID); err != nil {
			return nil, err
		}
		if err := s.applyThreadStates(ctx, userID, msgs[:1]); err != nil {
			return nil, err
		}
	}

	return &pb.MessageList{Messages: messagesToProto(msgs)}, nil
}

// FindThreadsByEntity returns messages linked to a specific entity.
//
// The lookup spans every channel that references the entity, so the result is
// filtered per channel rather than authorized up front: a user sees the linked
// messages from the channels they can read and nothing from the others.
func (s *Service) FindThreadsByEntity(ctx context.Context, entityType, entityID, userNodeID string) (*pb.MessageList, error) {
	msgs, err := s.store.FindByEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("find threads: %w", err)
	}

	readable := make(map[string]bool, 4)
	visible := msgs[:0]
	for _, m := range msgs {
		allowed, seen := readable[m.ChannelID]
		if !seen {
			_, err := s.authorizeChannel(ctx, m.ChannelID, userNodeID, ngac.OpRead)
			allowed = err == nil
			readable[m.ChannelID] = allowed
		}
		if allowed {
			visible = append(visible, m)
		}
	}

	return &pb.MessageList{Messages: messagesToProto(visible)}, nil
}

func messageToProto(m *store.Message) *pb.Message {
	var reactions []*pb.ReactionGroup
	for _, rg := range m.Reactions {
		reactions = append(reactions, &pb.ReactionGroup{
			Emoji:   rg.Emoji,
			Count:   rg.Count,
			UserIds: rg.UserIDs,
		})
	}
	var lastReplyAt *timestamppb.Timestamp
	if m.LastReplyAt != nil {
		lastReplyAt = timestamppb.New(*m.LastReplyAt)
	}
	return &pb.Message{
		Id:               m.ID,
		ChannelId:        m.ChannelID,
		SenderId:         m.SenderID,
		SenderName:       m.SenderName,
		Content:          m.Content,
		MessageType:      m.MessageType,
		ParentMessageId:  m.ParentMessageID,
		LinkedEntityType: m.LinkedEntityType,
		LinkedEntityId:   m.LinkedEntityID,
		ReplyCount:       m.ReplyCount,
		ContentFormat:    m.ContentFormat,
		Mentions:         m.Mentions,
		Reactions:        reactions,
		IsPinned:         m.IsPinned,
		CreatedAt:        timestamppb.New(m.CreatedAt),

		LastReplyAt:         lastReplyAt,
		ReplyParticipantIds: m.ReplyParticipantIDs,
		Following:           m.Following,
		ThreadUnreadCount:   m.ThreadUnread,
		Attachments:         attachmentsToProto(m.Attachments),
		EditedAt:            editedAtProto(m),
		Deleted:             m.Deleted,
		Quoted:              quoteToProto(m.Quoted),

		Starred:   m.StarredAt != nil,
		StarredAt: starredAtProto(m),
		Unread:    m.Unread,
	}
}

func messagesToProto(msgs []*store.Message) []*pb.Message {
	result := make([]*pb.Message, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, messageToProto(m))
	}
	return result
}
