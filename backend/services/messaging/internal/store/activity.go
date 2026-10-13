// Package store — the newest message of each conversation, for the list previews.
package store

import (
	"context"
	"fmt"
)

// latestPerChannel selects, of each given channel, its newest live message of
// the main stream (replies = false) or of its threads (replies = true). One
// statement, one ordered probe of a partial index per channel
// (idx_messages_latest_main / idx_messages_latest_reply): DISTINCT ON would read
// every message of every channel and sort them.
func (s *Store) latestPerChannel(ctx context.Context, channelIDs []string, replies bool) (map[string]*Message, error) {
	parent := "IS NULL"
	if replies {
		parent = "IS NOT NULL"
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM unnest($1::text[]) AS c(id)
		 CROSS JOIN LATERAL (
		   SELECT x.* FROM messages x
		   WHERE x.channel_id = c.id AND x.parent_message_id `+parent+` AND x.deleted_at IS NULL
		   ORDER BY x.created_at DESC, x.id DESC LIMIT 1
		 ) m
		 LEFT JOIN users u ON m.sender_id = u.id`, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("latest activity: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, fmt.Errorf("latest activity: %w", err)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("latest activity: %w", err)
	}
	out := make(map[string]*Message, len(msgs))
	for _, m := range msgs {
		out[m.ChannelID] = m
	}
	return out, nil
}

// LatestActivity returns, per given channel, its newest main-stream message
// and its newest thread reply, both leaving out deleted messages; a channel
// with none has no entry. The messages carry the sender's name but no thread
// state, reactions or attachments.
func (s *Store) LatestActivity(ctx context.Context, channelIDs []string) (main, replies map[string]*Message, err error) {
	if len(channelIDs) == 0 {
		return map[string]*Message{}, map[string]*Message{}, nil
	}
	if main, err = s.latestPerChannel(ctx, channelIDs, false); err != nil {
		return nil, nil, err
	}
	if replies, err = s.latestPerChannel(ctx, channelIDs, true); err != nil {
		return nil, nil, err
	}
	return main, replies, nil
}
