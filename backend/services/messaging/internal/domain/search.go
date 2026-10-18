// Package domain — searching message text across the conversations a person reads.
package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	// The zone database travels with the binary: a slim image has no /usr/share/zoneinfo.
	_ "time/tzdata"

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

// defaultSearchZone is the zone the days of a date filter are in when the caller names none.
const defaultSearchZone = "Asia/Ho_Chi_Minh"

// SearchQuery is one search across conversations. Before and BeforeID are the
// send time (RFC 3339) and id of the last message of the previous page.
type SearchQuery struct {
	Text     string
	HasFile  bool
	Before   string
	BeforeID string
	Limit    int

	// New filters — all optional.
	From       string // sender user ID
	ChannelID  string // restrict to one conversation
	After      string // ISO date YYYY-MM-DD inclusive lower bound (in TZ)
	BeforeDate string // ISO date YYYY-MM-DD inclusive upper bound (in TZ)
	TZ         string // IANA zone the two days are in; empty means Asia/Ho_Chi_Minh
	HasLink    bool   // only messages containing http:// or https://
	MentionsMe bool   // only messages that mention the caller
}

// hasFilter reports whether any filter beyond text is set.
func (q SearchQuery) hasFilter() bool {
	return q.HasFile || q.From != "" || q.ChannelID != "" ||
		q.After != "" || q.BeforeDate != "" || q.HasLink || q.MentionsMe
}

// searchZone is the zone named by the caller (an IANA name such as "America/New_York"), or Asia/Ho_Chi_Minh
// when none is. A name the zone database does not know is invalid input, never a silent fallback.
func searchZone(name string) (*time.Location, error) {
	if name == "" {
		name = defaultSearchZone
	}
	// "Local" would be the server's own zone, which says nothing about the caller.
	if name == "Local" {
		return nil, fmt.Errorf("%w: tz must be an IANA zone name", ErrInvalidInput)
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("%w: tz must be an IANA zone name", ErrInvalidInput)
	}
	return loc, nil
}

// parseSearchDates validates and converts After/BeforeDate to UTC times.
// After becomes start-of-day in loc; BeforeDate becomes end-of-day (the
// start of the next day) in loc.
func parseSearchDates(after, beforeDate string, loc *time.Location) (afterTime, beforeDateTime *time.Time, err error) {
	if after != "" {
		t, err := time.ParseInLocation("2006-01-02", after, loc)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: after must be an ISO date (YYYY-MM-DD)", ErrInvalidInput)
		}
		utc := t.UTC()
		afterTime = &utc
	}
	if beforeDate != "" {
		t, err := time.ParseInLocation("2006-01-02", beforeDate, loc)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: before_date must be an ISO date (YYYY-MM-DD)", ErrInvalidInput)
		}
		end := t.AddDate(0, 0, 1).UTC() // end of day = start of next day
		beforeDateTime = &end
	}
	return afterTime, beforeDateTime, nil
}

// SearchAll finds, newest first, the messages — main stream and thread replies
// alike — whose text matches the query, in the conversations the caller reads
// in the workspace they are signed in to and in their direct messages (the
// scope of the Mentions feed). A message of any other conversation is never
// returned: a hit is message text, so it follows read access exactly.
//
// Text shorter than two characters finds nothing — unless at least one filter
// is set (a filter-only search is allowed). With no text and no filter, the
// answer is an empty list.
//
// A page is 25 hits unless Limit asks for another size (at most 50).
func (s *Service) SearchAll(ctx context.Context, userID, userNodeID, workspaceID string, q SearchQuery) (*pb.MessageList, error) {
	if userID == "" || userNodeID == "" {
		return nil, ErrInvalidInput
	}

	hasText := utf8.RuneCountInString(strings.TrimSpace(q.Text)) >= searchMinRunes
	if !hasText && !q.hasFilter() {
		return &pb.MessageList{}, nil
	}
	// When no meaningful text is given but filters are set, clear Text so the
	// store knows not to apply the full-text predicate.
	if !hasText {
		q.Text = ""
	}

	if r := []rune(q.Text); len(r) > searchMaxRunes {
		q.Text = string(r[:searchMaxRunes])
	}
	if q.Limit <= 0 {
		q.Limit = searchPageSize
	}

	// Validate date filters.
	loc, err := searchZone(q.TZ)
	if err != nil {
		return nil, err
	}
	afterTime, beforeDateTime, err := parseSearchDates(q.After, q.BeforeDate, loc)
	if err != nil {
		return nil, err
	}

	sf := store.SearchFilters{
		From:       q.From,
		ChannelID:  q.ChannelID,
		After:      afterTime,
		BeforeDate: beforeDateTime,
		HasLink:    q.HasLink,
		MentionsMe: q.MentionsMe,
		CallerID:   userID,
	}

	return s.messageFeed(ctx, userID, userNodeID, workspaceID, q.Before, q.BeforeID, q.Limit,
		func(ctx context.Context, _ string, channelIDs []string, before *time.Time, beforeID string, limit int) ([]*store.Message, bool, error) {
			return s.store.SearchAllMessages(ctx, channelIDs, q.Text, q.HasFile, before, beforeID, limit, sf)
		})
}
