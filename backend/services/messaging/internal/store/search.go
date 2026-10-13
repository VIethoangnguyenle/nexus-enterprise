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

// SearchAllMessages searches the given channels — main messages and thread
// replies — newest first, leaving out deleted ones. hasFile keeps only messages
// with an attachment. before and beforeID are the send time and id of the last
// message of the previous page; the id breaks a tie between messages sent at
// the same instant.
func (s *Store) SearchAllMessages(ctx context.Context, channelIDs []string, query string, hasFile bool, before *time.Time, beforeID string, limit int) ([]*Message, bool, error) {
	head, tail, ok := searchTerms(query)
	if len(channelIDs) == 0 || !ok {
		return nil, false, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.channel_id = ANY($1::text[]) AND m.deleted_at IS NULL
		   AND m.search_vector @@ `+tsQuerySQL(2, 3)+`
		   AND (NOT $4 OR EXISTS (SELECT 1 FROM message_attachments a WHERE a.message_id = m.id)
		        OR m.linked_entity_type = 'drive_file')
		   AND ($5::timestamptz IS NULL OR m.created_at < $5
		        OR ($6 <> '' AND m.created_at = $5 AND m.id < $6))
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT $7`, channelIDs, head, tsTail(tail), hasFile, before, beforeID, limit+1)
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
