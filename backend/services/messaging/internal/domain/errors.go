package domain

import (
	"context"
	"errors"
)

var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAccessDenied indicates the user lacks permission for the operation.
	ErrAccessDenied = errors.New("access denied")
	// ErrAlreadyExists indicates a duplicate entity.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidInput indicates missing or malformed request fields.
	ErrInvalidInput = errors.New("invalid input")
)

// NotificationStore defines notification database operations for the REST handler.
type NotificationStore interface {
	ListByUser(ctx context.Context, userID string) (any, error)
	MarkRead(ctx context.Context, notifID string) error
	MarkAllRead(ctx context.Context, userID string) error
	UnreadCount(ctx context.Context, userID string) (int, error)
}
