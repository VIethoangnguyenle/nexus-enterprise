// Package domain — how Chat behaves for one person in one workspace.
//
// These options arrange and word the person's own view of Chat (and choose the
// notification level a new group starts at); none of them grants or hides
// anything, so the only check is that the caller is in the workspace and that the
// row is theirs: it is keyed by the verified user, never by a user in the request.
package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"unicode"
	"unicode/utf8"

	"ngac-platform/services/messaging/internal/store"
)

// ChatSettings are a person's Chat options in a workspace.
type ChatSettings = store.ChatSettings

// Nullable is a JSON field that can be absent (leave as it is), null (clear) or
// a value. Set is true when the field was present at all.
type Nullable[T any] struct {
	Set   bool
	Value *T
}

// UnmarshalJSON records that the field was present, and its value or null.
func (n *Nullable[T]) UnmarshalJSON(b []byte) error {
	n.Set = true
	if string(b) == "null" {
		n.Value = nil
		return nil
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	n.Value = &v
	return nil
}

// MarshalJSON writes the value, or null.
func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if n.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.Value)
}

// ChatSettingsPatch is a change to ChatSettings: a nil field is left as it is.
type ChatSettingsPatch struct {
	EnterToSend, LinkPreview *bool
	OpenAt, Sort, DoubleTap  *string
	// DefaultLevel: absent leaves it, null clears it (the system default).
	DefaultLevel Nullable[string]
}

// Limits on the double-tap reaction: one emoji, possibly a sequence (skin tone,
// a family joined by zero-width joiners), never text.
const maxDoubleTapRunes = 8

func validOpenAt(v string) bool { return v == "all" || v == "sections" }
func validSort(v string) bool   { return v == "activity" || v == "name" }

// validDoubleTap accepts "" (off) or a short run of symbols, marks and joiners.
func validDoubleTap(v string) bool {
	if v == "" {
		return true
	}
	if !utf8.ValidString(v) || utf8.RuneCountInString(v) > maxDoubleTapRunes {
		return false
	}
	symbols := 0
	for _, r := range v {
		switch {
		case unicode.IsSymbol(r):
			symbols++
		case unicode.IsMark(r), r == '‍':
		default:
			return false
		}
	}
	return symbols > 0
}

// chatSettingsScope checks the caller is in the workspace and names a user.
func (s *Service) chatSettingsScope(ctx context.Context, workspaceID string, c Caller) error {
	if err := s.inWorkspace(ctx, workspaceID, c); err != nil {
		return err
	}
	if c.UserID == "" {
		return fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	return nil
}

// GetChatSettings returns the caller's Chat options in a workspace they are in.
func (s *Service) GetChatSettings(ctx context.Context, workspaceID string, c Caller) (ChatSettings, error) {
	if err := s.chatSettingsScope(ctx, workspaceID, c); err != nil {
		return ChatSettings{}, err
	}
	return s.store.GetChatSettings(ctx, c.UserID, workspaceID)
}

// UpdateChatSettings applies the patch to the caller's own options and returns
// the result. Nothing is written unless every given value is valid.
func (s *Service) UpdateChatSettings(ctx context.Context, workspaceID string, c Caller, in ChatSettingsPatch) (ChatSettings, error) {
	if err := s.chatSettingsScope(ctx, workspaceID, c); err != nil {
		return ChatSettings{}, err
	}
	if in.OpenAt != nil && !validOpenAt(*in.OpenAt) {
		return ChatSettings{}, fmt.Errorf("%w: open_at must be \"all\" or \"sections\"", ErrInvalidInput)
	}
	if in.Sort != nil && !validSort(*in.Sort) {
		return ChatSettings{}, fmt.Errorf("%w: sort must be \"activity\" or \"name\"", ErrInvalidInput)
	}
	if in.DoubleTap != nil && !validDoubleTap(*in.DoubleTap) {
		return ChatSettings{}, fmt.Errorf("%w: double_tap must be one emoji, or empty to turn it off", ErrInvalidInput)
	}
	if in.DefaultLevel.Set && in.DefaultLevel.Value != nil && !validNotifyLevel(*in.DefaultLevel.Value) {
		return ChatSettings{}, fmt.Errorf("%w: default_level must be %q, %q, %q or null", ErrInvalidInput, NotifyAll, NotifyMain, NotifyOff)
	}
	cur, err := s.store.GetChatSettings(ctx, c.UserID, workspaceID)
	if err != nil {
		return ChatSettings{}, err
	}
	if in.EnterToSend != nil {
		cur.EnterToSend = *in.EnterToSend
	}
	if in.LinkPreview != nil {
		cur.LinkPreview = *in.LinkPreview
	}
	if in.OpenAt != nil {
		cur.OpenAt = *in.OpenAt
	}
	if in.Sort != nil {
		cur.Sort = *in.Sort
	}
	if in.DoubleTap != nil {
		cur.DoubleTap = *in.DoubleTap
	}
	if in.DefaultLevel.Set {
		cur.DefaultLevel = in.DefaultLevel.Value
	}
	if err := s.store.SaveChatSettings(ctx, c.UserID, workspaceID, cur); err != nil {
		return ChatSettings{}, err
	}
	return cur, nil
}

// seedNotificationLevel starts a person's level for a group they have just
// joined at the default they chose. The group already exists, so a failure is
// logged and the person keeps the system default rather than undoing the join.
func (s *Service) seedNotificationLevel(ctx context.Context, userID, workspaceID, channelID string) {
	if userID == "" {
		return
	}
	if err := s.store.SeedNotificationLevel(ctx, userID, workspaceID, channelID); err != nil {
		slog.Warn("default notification level not applied", "channel", channelID, "error", err)
	}
}
