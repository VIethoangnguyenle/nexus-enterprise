package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/workspace/internal/store"
)

// Limits of a workspace's name and description, in characters.
const (
	MaxWorkspaceNameRunes        = 100
	MaxWorkspaceDescriptionRunes = 500
)

// WorkspaceDetails is what the workspace settings screen shows and edits.
// CanManage says whether the caller may change it.
type WorkspaceDetails struct {
	ID          string
	Name        string
	Description string
	CanManage   bool
	// Type is TypePersonal or TypeOrganization.
	Type string
	// IsOwner is whether the caller is the workspace's owner; OwnerName is who
	// that is, as a name.
	IsOwner   bool
	OwnerName string
	// ConvertedByName and ConvertedAt say who turned a personal workspace into an
	// organization and when; empty and nil for one that was never converted.
	ConvertedByName string
	ConvertedAt     *time.Time
}

// withOwnership fills in the type, the owners and the conversion record. The
// owners are the people in the workspace's Owners group, the same source the
// owner changes and the last-owner rule use. If the group cannot be read, the
// caller is not shown as an owner (fail closed) and the owners go unnamed.
func (s *Service) withOwnership(ctx context.Context, d *WorkspaceDetails, ws *WorkspaceResult, callerNodeID, wsType string) (*WorkspaceDetails, error) {
	rec, err := s.store.ConversionRecord(ctx, d.ID)
	if err != nil {
		if errors.Is(err, store.ErrWorkspaceNotFound) {
			return nil, fmt.Errorf("%w: workspace", ErrNotFound)
		}
		return nil, err
	}
	d.Type, d.ConvertedAt = wsType, rec.At
	if rec.At != nil {
		d.ConvertedByName = readableName(rec.ByName, neutralPerson)
	}
	if _, owners, err := s.ownersOf(ctx, ws); err != nil {
		slog.Warn("could not read the workspace owners", "workspace", d.ID, "error", err)
	} else {
		d.IsOwner = containsNode(owners, callerNodeID) && callerNodeID != ""
		names := make([]string, 0, len(owners))
		for _, p := range s.peopleFor(ctx, d.ID, owners) {
			names = append(names, p.DisplayName)
		}
		d.OwnerName = strings.Join(names, ", ")
	}
	return d, nil
}

// WorkspaceDetails returns a workspace's name and description to a member of it,
// with whether that member holds manage on its management OA.
func (s *Service) WorkspaceDetails(ctx context.Context, callerNodeID, wsID string) (*WorkspaceDetails, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	row, err := s.store.GetByID(ctx, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("%w: workspace", ErrNotFound)
	}
	out := &WorkspaceDetails{ID: row.ID, Name: row.Name, Description: row.Desc}
	if mgmtID, err := s.mgmtOAID(ctx, ws); err == nil {
		out.CanManage = s.checkAccess(ctx, callerNodeID, mgmtID, ngac.OpManage) == nil
	}
	return s.withOwnership(ctx, out, ws, callerNodeID, row.Type)
}

// UpdateWorkspaceDetails changes a workspace's name and/or description. It
// takes manage on the workspace's management OA, like every other change to the
// workspace as a whole. A nil field is left alone.
func (s *Service) UpdateWorkspaceDetails(ctx context.Context, callerNodeID, wsID string, name, description *string) (*WorkspaceDetails, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if name == nil && description == nil {
		return nil, fmt.Errorf("%w: nothing to change", ErrInvalidInput)
	}
	var newName, newDesc *string
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" || utf8.RuneCountInString(n) > MaxWorkspaceNameRunes || !utf8.ValidString(n) || strings.ContainsRune(n, 0) {
			return nil, fmt.Errorf("%w: workspace name must be 1 to %d characters", ErrInvalidInput, MaxWorkspaceNameRunes)
		}
		newName = &n
	}
	if description != nil {
		d := strings.TrimSpace(*description)
		if utf8.RuneCountInString(d) > MaxWorkspaceDescriptionRunes || !utf8.ValidString(d) || strings.ContainsRune(d, 0) {
			return nil, fmt.Errorf("%w: description must be at most %d characters", ErrInvalidInput, MaxWorkspaceDescriptionRunes)
		}
		newDesc = &d
	}
	// One statement sets only the fields sent and answers with the row as it is
	// after, so two people editing different fields at once both land and the
	// answer is never a mix of a stale read and a write.
	gotName, gotDesc, err := s.store.UpdateDetails(ctx, ws.ID, newName, newDesc)
	if err != nil {
		return nil, err
	}
	s.announce(ctx, realtime.KindUpdated, ws.ID, ws.ID)
	return s.withOwnership(ctx, &WorkspaceDetails{ID: ws.ID, Name: gotName, Description: gotDesc, CanManage: true}, ws, callerNodeID, ws.Type)
}
