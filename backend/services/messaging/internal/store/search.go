// Package store — full-text search over message text.
package store

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// searchPrefixMinRunes is the shortest last word read as a prefix.
const searchPrefixMinRunes = 2

// searchTerms splits what the user typed into the two inputs of one tsquery:
// head, everything before the last word, read by plainto_tsquery — which takes
// any text and never fails on punctuation or operators — and tail, the last
// word when the text ends in one and has at least searchPrefixMinRunes letters,
// matched as a prefix so a word still being typed finds its message (a shorter
// word would match a large share of the whole index, so it stays in head as a
// whole word). The tail holds only letters, digits and marks, so
// it carries no tsquery syntax. ok is false when the text has no word at all.
func searchTerms(q string) (head, tail string, ok bool) {
	runes := []rune(strings.TrimSpace(q))
	isWord := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) }
	start := len(runes)
	for start > 0 && isWord(runes[start-1]) {
		start--
	}
	if len(runes)-start >= searchPrefixMinRunes { // ends in a long enough word: the prefix
		head, tail = string(runes[:start]), string(runes[start:])
	} else {
		head = string(runes)
	}
	for _, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return head, tail, true
		}
	}
	return "", "", false
}

// tsQuerySQL is the text-search query for the head and tail passed as the
// parameters $headArg and $tailArg; to_tsquery of an empty tail is the empty
// query, which && drops.
func tsQuerySQL(headArg, tailArg int) string {
	return fmt.Sprintf(`(plainto_tsquery('simple', $%d) && to_tsquery('simple', $%d))`, headArg, tailArg)
}

// tsTail is the tail as the to_tsquery operand: one quoted word, as a prefix.
func tsTail(tail string) string {
	if tail == "" {
		return ""
	}
	return "'" + strings.ReplaceAll(tail, "'", "''") + "':*"
}

// SearchMessages performs full-text search on messages in a channel, best
// match first.
func (s *Store) SearchMessages(ctx context.Context, channelID, query string, limit int) ([]*Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	head, tail, ok := searchTerms(query)
	if !ok {
		return nil, nil
	}
	tsq := tsQuerySQL(2, 3)
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.channel_id = $1 AND m.deleted_at IS NULL AND m.search_vector @@ `+tsq+`
		 ORDER BY ts_rank(m.search_vector, `+tsq+`) DESC, m.created_at DESC
		 LIMIT $4`,
		channelID, head, tsTail(tail), limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// SearchFilters holds optional filters for SearchAllMessages beyond the
// full-text query and has-file flag. All zero values mean "no filter".
type SearchFilters struct {
	From       string     // only messages by this sender
	ChannelID  string     // restrict to one channel (must be in the readable set)
	After      *time.Time // created_at >= this (start-of-day in UTC)
	BeforeDate *time.Time // created_at < this (end-of-day in UTC)
	HasLink    bool       // content contains http:// or https://
	MentionsMe bool       // m.mentions @> ARRAY[CallerID]
	CallerID   string     // the authenticated user, for MentionsMe
}

// SearchAllMessages searches the given channels — main messages and thread
// replies — newest first, leaving out deleted ones. hasFile keeps only messages
// with an attachment. before and beforeID are the send time and id of the last
// message of the previous page; the id breaks a tie between messages sent at
// the same instant. The filters in sf are combined with AND.
//
// When query is empty (filter-only search), the full-text predicate is skipped.
func (s *Store) SearchAllMessages(ctx context.Context, channelIDs []string, query string, hasFile bool, before *time.Time, beforeID string, limit int, sf SearchFilters) ([]*Message, bool, error) {
	// When a channel_id filter is set, intersect it with the readable set.
	if sf.ChannelID != "" {
		found := false
		for _, id := range channelIDs {
			if id == sf.ChannelID {
				found = true
				break
			}
		}
		if !found {
			// The requested channel is not readable: zero results, never an error.
			return nil, false, nil
		}
		channelIDs = []string{sf.ChannelID}
	}

	if len(channelIDs) == 0 {
		return nil, false, nil
	}

	// Build the WHERE clause and args dynamically.
	args := []any{channelIDs} // $1
	argN := 1
	next := func(v any) int { argN++; args = append(args, v); return argN }

	var clauses []string
	clauses = append(clauses, `m.channel_id = ANY($1::text[])`, `m.deleted_at IS NULL`)

	// Full-text search predicate (skipped when query is empty = filter-only).
	if query != "" {
		head, tail, ok := searchTerms(query)
		if !ok {
			return nil, false, nil
		}
		headArg := next(head)
		tailArg := next(tsTail(tail))
		clauses = append(clauses, `m.search_vector @@ `+tsQuerySQL(headArg, tailArg))
	}

	// has_file
	hasFileArg := next(hasFile)
	clauses = append(clauses, fmt.Sprintf(`(NOT $%d OR EXISTS (SELECT 1 FROM message_attachments a WHERE a.message_id = m.id)
		        OR m.linked_entity_type = 'drive_file')`, hasFileArg))

	// from (sender)
	if sf.From != "" {
		n := next(sf.From)
		clauses = append(clauses, fmt.Sprintf(`m.sender_id = $%d`, n))
	}

	// after (created_at >= start-of-day in UTC)
	if sf.After != nil {
		n := next(*sf.After)
		clauses = append(clauses, fmt.Sprintf(`m.created_at >= $%d`, n))
	}

	// before_date (created_at < end-of-day in UTC)
	if sf.BeforeDate != nil {
		n := next(*sf.BeforeDate)
		clauses = append(clauses, fmt.Sprintf(`m.created_at < $%d`, n))
	}

	// has_link
	if sf.HasLink {
		clauses = append(clauses, `(m.content ~* 'https?://')`)
	}

	// mentions_me — same definition as the Mentions feed: m.mentions @> ARRAY[$caller]
	if sf.MentionsMe && sf.CallerID != "" {
		n := next(sf.CallerID)
		clauses = append(clauses, fmt.Sprintf(`m.mentions @> ARRAY[$%d]::text[]`, n))
	}

	// Pagination cursor
	beforeArg := next(before)
	beforeIDArg := next(beforeID)
	clauses = append(clauses, fmt.Sprintf(`($%d::timestamptz IS NULL OR m.created_at < $%d
		        OR ($%d <> '' AND m.created_at = $%d AND m.id < $%d))`,
		beforeArg, beforeArg, beforeIDArg, beforeArg, beforeIDArg))

	limitArg := next(limit + 1)

	sql := `SELECT ` + messageCols + `
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE ` + strings.Join(clauses, "\n\t\t   AND ") + `
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT $` + fmt.Sprint(limitArg)

	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, false, fmt.Errorf("search all messages: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, false, fmt.Errorf("search all messages: %w", err)
	}
	return trimPage(msgs, limit)
}

