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
}

// Message represents a row in the messages table.
type Message struct {
	ID               string
	ChannelID        string
	SenderID         string
	SenderName       string
	Content          string
	MessageType      string
	ParentMessageID  string
	LinkedEntityType string
	LinkedEntityID   string
	ReplyCount       int32
	CreatedAt        time.Time
}
