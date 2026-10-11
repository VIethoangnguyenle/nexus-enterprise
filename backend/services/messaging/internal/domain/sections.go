// Package domain — personal sections of the chat navigator.
//
// A section only arranges one user's own list in one workspace: it grants
// nothing, nobody else sees it, and it is not an NGAC node. What is checked is
// that the caller is in the workspace, that the section is theirs, and that a
// conversation placed in it is one they may read.
package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/store"
)

// Caller is who acts, as the verified session says.
type Caller struct {
	UserID   string
	NodeID   string
	TenantID string
}

// Section is one of the caller's navigator sections.
type Section = store.Section

// NewSection is a section to create; Position and ChannelIDs restore one after
// an Undo (nil Position: last).
type NewSection struct {
	Name       string
	Position   *int
	ChannelIDs []string
}

const maxSectionName = 40

func cleanSectionName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxSectionName {
		return "", fmt.Errorf("%w: a section name is 1 to %d characters", ErrInvalidInput, maxSectionName)
	}
	return name, nil
}

func sectionErr(err error) error {
	if errors.Is(err, store.ErrSectionNameTaken) {
		return fmt.Errorf("%w: you already have a section with this name", ErrAlreadyExists)
	}
	return err
}

// inWorkspace checks the caller belongs to the workspace (read on its
// workspace-level attribute, inside their own tenant).
func (s *Service) inWorkspace(ctx context.Context, workspaceID string, c Caller) error {
	return s.AuthorizeWorkspaceAccess(ctx, workspaceID, c.NodeID, c.TenantID, ngac.OpRead)
}

// ownSection returns the caller's own section; another user's is not found.
func (s *Service) ownSection(ctx context.Context, workspaceID string, c Caller, sectionID string) (*store.Section, error) {
	if err := s.inWorkspace(ctx, workspaceID, c); err != nil {
		return nil, err
	}
	sec, err := s.store.GetSection(ctx, workspaceID, c.UserID, sectionID)
	if err != nil {
		return nil, err
	}
	if sec == nil {
		return nil, fmt.Errorf("%w: section not found", ErrNotFound)
	}
	return sec, nil
}

// placeable checks a conversation may sit in a section of this workspace: the
// caller reads it, and it is a channel of this workspace or a direct message.
func (s *Service) placeable(ctx context.Context, workspaceID string, c Caller, channelID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, c.NodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if ch.ChannelType != "dm" && ch.WorkspaceID != workspaceID {
		return fmt.Errorf("%w: the conversation belongs to another workspace", ErrInvalidInput)
	}
	return nil
}

// ListSections returns the caller's sections in a workspace, in order.
func (s *Service) ListSections(ctx context.Context, workspaceID string, c Caller) ([]*Section, error) {
	if err := s.inWorkspace(ctx, workspaceID, c); err != nil {
		return nil, err
	}
	return s.store.ListSections(ctx, workspaceID, c.UserID)
}

// CreateSection adds a section for the caller.
func (s *Service) CreateSection(ctx context.Context, workspaceID string, c Caller, in NewSection) (*Section, error) {
	if err := s.inWorkspace(ctx, workspaceID, c); err != nil {
		return nil, err
	}
	name, err := cleanSectionName(in.Name)
	if err != nil {
		return nil, err
	}
	for _, ch := range in.ChannelIDs {
		if err := s.placeable(ctx, workspaceID, c, ch); err != nil {
			return nil, err
		}
	}
	sec := &store.Section{ID: uuid.NewString(), Name: name, ChannelIDs: in.ChannelIDs}
	if err := s.store.InsertSection(ctx, workspaceID, c.UserID, sec, in.Position); err != nil {
		return nil, sectionErr(err)
	}
	return sec, nil
}

// RenameSection renames one of the caller's sections.
func (s *Service) RenameSection(ctx context.Context, workspaceID string, c Caller, sectionID, name string) error {
	if _, err := s.ownSection(ctx, workspaceID, c, sectionID); err != nil {
		return err
	}
	clean, err := cleanSectionName(name)
	if err != nil {
		return err
	}
	return sectionErr(s.store.RenameSection(ctx, c.UserID, sectionID, clean))
}

// MoveSection moves one of the caller's sections one place "up" or "down".
// The first section cannot move up nor the last down.
func (s *Service) MoveSection(ctx context.Context, workspaceID string, c Caller, sectionID, direction string) error {
	if direction != "up" && direction != "down" {
		return fmt.Errorf("%w: direction is up or down", ErrInvalidInput)
	}
	if _, err := s.ownSection(ctx, workspaceID, c, sectionID); err != nil {
		return err
	}
	secs, err := s.store.ListSections(ctx, workspaceID, c.UserID)
	if err != nil {
		return err
	}
	for i, sec := range secs {
		if sec.ID != sectionID {
			continue
		}
		j := i - 1
		if direction == "down" {
			j = i + 1
		}
		if j < 0 || j >= len(secs) {
			return fmt.Errorf("%w: the section cannot move %s", ErrInvalidInput, direction)
		}
		return s.store.SwapSections(ctx, c.UserID, sectionID, secs[j].ID)
	}
	return fmt.Errorf("%w: section not found", ErrNotFound)
}

// DeleteSection removes one of the caller's sections and returns it as it was,
// for Undo; its conversations return to the default lists.
func (s *Service) DeleteSection(ctx context.Context, workspaceID string, c Caller, sectionID string) (*Section, error) {
	sec, err := s.ownSection(ctx, workspaceID, c, sectionID)
	if err != nil {
		return nil, err
	}
	if err := s.store.DeleteSection(ctx, c.UserID, sectionID); err != nil {
		return nil, err
	}
	return sec, nil
}

// PlaceChannel puts a conversation the caller reads into one of their sections.
func (s *Service) PlaceChannel(ctx context.Context, workspaceID string, c Caller, sectionID, channelID string) error {
	if _, err := s.ownSection(ctx, workspaceID, c, sectionID); err != nil {
		return err
	}
	if err := s.placeable(ctx, workspaceID, c, channelID); err != nil {
		return err
	}
	return s.store.PlaceChannel(ctx, c.UserID, sectionID, channelID)
}

// UnplaceChannel takes a conversation out of the caller's sections.
func (s *Service) UnplaceChannel(ctx context.Context, workspaceID string, c Caller, channelID string) error {
	if err := s.inWorkspace(ctx, workspaceID, c); err != nil {
		return err
	}
	if channelID == "" {
		return ErrInvalidInput
	}
	return s.store.UnplaceChannel(ctx, c.UserID, channelID)
}
