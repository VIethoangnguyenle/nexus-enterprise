package domain

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
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
	return out, nil
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
	return &WorkspaceDetails{ID: ws.ID, Name: gotName, Description: gotDesc, CanManage: true}, nil
}
