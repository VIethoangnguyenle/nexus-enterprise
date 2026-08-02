package domain

import (
	"context"

	"ngac-platform/pkg/httputil"
)

// These alias the shared sentinels rather than declaring new ones. errors.Is
// compares identity, not message text, so a local errors.New with the same
// string would be a different value and httputil.MapDomainError would never
// match it — every failure would surface as 500, including denials.
var (
	ErrNotFound      = httputil.ErrNotFound
	ErrAccessDenied  = httputil.ErrAccessDenied
	ErrAlreadyExists = httputil.ErrAlreadyExists
	ErrInvalidInput  = httputil.ErrInvalidInput
)

// NotificationStore defines notification database operations for the REST handler.
type NotificationStore interface {
	ListByUser(ctx context.Context, userID string) (any, error)
	MarkRead(ctx context.Context, notifID string) error
	MarkAllRead(ctx context.Context, userID string) error
	UnreadCount(ctx context.Context, userID string) (int, error)
}
