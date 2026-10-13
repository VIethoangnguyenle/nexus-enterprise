// Package domain — searching message text across the conversations a person reads.
package domain

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

const (
	// searchMinRunes is the shortest text worth searching for: one letter
	// matches nearly every message.
	searchMinRunes = 2
	// searchMaxRunes caps the text put to the index: a pasted paragraph is not
	// a search, and every extra word narrows the query without helping anyone.
	searchMaxRunes = 200
	// searchPageSize is a page of hits when the caller names no limit.
	searchPageSize = 25
)

// SearchQuery is one search across conversations. Before and BeforeID are the
// send time (RFC 3339) and id of the last message of the previous page.
type SearchQuery struct {
	Text     string
	HasFile  bool
	Before   string
	BeforeID string
	Limit    int
}

// SearchAll finds, newest first, the messages — main stream and thread replies
// alike — whose text matches the query, in the conversations the caller reads
// in the workspace they are signed in to and in their direct messages (the
// scope of the Mentions feed). A message of any other conversation is never
// returned: a hit is message text, so it follows read access exactly. Text
// shorter than two characters finds nothing. A page is 25 hits unless Limit
// asks for another size (at most 50).
func (s *Service) SearchAll(ctx context.Context, userID, userNodeID, workspaceID string, q SearchQuery) (*pb.MessageList, error) {
	if userID == "" || userNodeID == "" {
		return nil, ErrInvalidInput
	}
	if utf8.RuneCountInString(strings.TrimSpace(q.Text)) < searchMinRunes {
		return &pb.MessageList{}, nil
	}
	if r := []rune(q.Text); len(r) > searchMaxRunes {
		q.Text = string(r[:searchMaxRunes])
	}
	if q.Limit <= 0 {
		q.Limit = searchPageSize
	}
	return s.messageFeed(ctx, userID, userNodeID, workspaceID, q.Before, q.BeforeID, q.Limit,
		func(ctx context.Context, _ string, channelIDs []string, before *time.Time, beforeID string, limit int) ([]*store.Message, bool, error) {
			return s.store.SearchAllMessages(ctx, channelIDs, q.Text, q.HasFile, before, beforeID, limit)
		})
}
