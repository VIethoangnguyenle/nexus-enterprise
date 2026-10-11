package store

import "time"

// Channel represents a row in the channels table.
type Channel struct {
	ID          string
	Name        string
	ChannelType string
	WorkspaceID string
	NGACOaID    string
	NGACUaID    string
	CreatedBy   string
	CreatedAt   time.Time
	Topic       string
	Description string
	MemberCount int32
}

// Message represents a row in the messages table.
type Message struct {
	ID         string
	ChannelID  string
	SenderID   string
	SenderName string
	Content    string
	// ContentText is the readable text (no markup) kept for search and previews;
	// empty means the content is its own text.
	ContentText      string
	MessageType      string
	ParentMessageID  string
	LinkedEntityType string
	LinkedEntityID   string
	ReplyCount       int32
	LastReplyAt      *time.Time // nil until the first reply
	ContentFormat    string
	Mentions         []string
	IsPinned         bool
	Reactions        []ReactionGroup
	Attachments      []Attachment
	CreatedAt        time.Time

	EditedAt        *time.Time // nil until its sender changes the text
	Deleted         bool       // soft-deleted: body and files are never returned
	QuotedMessageID string     // the message this one quotes, when any
	Quoted          *Quote     // filled for display where a page is enriched

	// Thread state as seen by one caller; filled only where a caller is known.
	Following           bool
	ThreadUnread        int32
	ReplyParticipantIDs []string
}
