package domain

import (
	"context"
	"errors"
	"fmt"

	"ngac-platform/services/auth/internal/store"
)

// ProfileUpdateInput is a partial update: a nil field is left as it is and a
// pointer to "" clears it. There is no department (an administrator assigns it,
// per workspace) and no avatar (there is no image store to point at yet): the
// REST edge refuses both by name rather than ignoring them.
type ProfileUpdateInput struct {
	DisplayName *string
	Title       *string
	Location    *string
	Language    *string
	// WorkspaceID, when set, makes Title that workspace's title rather than the
	// account default. It must be a workspace the person is an active member of.
	WorkspaceID string
}

// validLanguages are the accepted values for the language field.
var validLanguages = map[string]bool{"vi": true, "en": true, "": true}

// UpdateProfile validates and saves the fields that were sent. The first save
// of a profile field (name, title, location) ends the "tell us who you are"
// step of sign-in; the language alone does not. Nothing is written unless
// every field sent is valid.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileUpdateInput) error {
	if userID == "" {
		return ErrInvalidInput
	}
	ch := store.ProfileChanges{WorkspaceID: in.WorkspaceID}
	sent := false

	text := func(dst **string, src *string, max int, required bool) error {
		if src == nil {
			return nil
		}
		v, err := cleanText(*src, max, required)
		if err != nil {
			return err
		}
		*dst, sent = &v, true
		return nil
	}
	if err := text(&ch.DisplayName, in.DisplayName, maxDisplayNameRunes, true); err != nil {
		return err
	}
	if err := text(&ch.Title, in.Title, maxProfileFieldRunes, false); err != nil {
		return err
	}
	if err := text(&ch.Location, in.Location, maxProfileFieldRunes, false); err != nil {
		return err
	}
	if in.Language != nil {
		if !validLanguages[*in.Language] {
			return ErrInvalidInput
		}
		ch.Language, sent = in.Language, true
	}
	if !sent {
		// An empty update would still mark the profile done.
		return ErrInvalidInput
	}

	if err := s.store.UpdateProfile(ctx, userID, ch); err != nil {
		if errors.Is(err, store.ErrNoSuchUser) {
			return ErrNotFound
		}
		if errors.Is(err, store.ErrNotMember) {
			return ErrAccessDenied
		}
		return fmt.Errorf("update profile: %w", err)
	}
	s.announceProfileChanged(ctx, userID)
	return nil
}
