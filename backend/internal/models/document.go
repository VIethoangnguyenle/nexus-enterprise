package models

import "time"

type Document struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	OwnerID    string    `json:"owner_id"`
	OwnerName  string    `json:"owner_name,omitempty"`
	NGACNodeID string    `json:"ngac_node_id"`
	Status     string    `json:"status"` // "draft" or "approved"
	IsPublic   bool      `json:"is_public"`
	CreatedAt  time.Time `json:"created_at"`
}

type ShareRequest struct {
	TargetUAID string   `json:"target_ua_id"`
	Operations []string `json:"operations"`
}

type ShareInfo struct {
	ID           string   `json:"id"`
	DocumentID   string   `json:"document_id"`
	TargetUAID   string   `json:"target_ua_id"`
	TargetUAName string   `json:"target_ua_name"`
	Operations   []string `json:"operations"`
	ShareOAID    string   `json:"share_oa_id"`
}

type AccessCheckRequest struct {
	UserID    string `json:"user_id"`
	ObjectID  string `json:"object_id"`
	Operation string `json:"operation"`
}
