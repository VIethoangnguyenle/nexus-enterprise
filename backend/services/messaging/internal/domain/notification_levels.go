// Package domain — how much of a conversation counts for one person.
package domain

import (
	"context"
	"fmt"

	"ngac-platform/ngac"
)

// The notification levels of a conversation, as in Google Chat.
const (
	// NotifyAll: every message and every reply counts and notifies.
	NotifyAll = "all"
	// NotifyMain: the main messages and the replies in followed threads (default).
	NotifyMain = "main"
	// NotifyOff: nothing counts and nothing notifies.
	NotifyOff = "off"
)

func validNotifyLevel(l string) bool { return l == NotifyAll || l == NotifyMain || l == NotifyOff }

// GetNotificationLevel returns the caller's level for a conversation they read.
func (s *Service) GetNotificationLevel(ctx context.Context, channelID, userID, userNodeID string) (string, error) {
	if channelID == "" || userID == "" {
		return "", ErrInvalidInput
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead); err != nil {
		return "", err
	}
	return s.store.NotificationLevel(ctx, userID, channelID)
}

// SetNotificationLevel stores the caller's level for a conversation they read.
// The level is the person's own; it never widens what they may read.
func (s *Service) SetNotificationLevel(ctx context.Context, channelID, userID, userNodeID, level string) error {
	if channelID == "" || userID == "" {
		return ErrInvalidInput
	}
	if !validNotifyLevel(level) {
		return fmt.Errorf("%w: level must be %q, %q or %q", ErrInvalidInput, NotifyAll, NotifyMain, NotifyOff)
	}
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	// A direct message has no main messages and threads to tell apart: it is
	// on ("main") or off.
	if ch.ChannelType == "dm" && level == NotifyAll {
		return fmt.Errorf("%w: a direct message is on or off", ErrInvalidInput)
	}
	return s.store.SetNotificationLevel(ctx, userID, channelID, level)
}
