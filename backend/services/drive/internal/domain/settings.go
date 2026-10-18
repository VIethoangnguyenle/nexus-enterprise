package domain

import (
	"context"
	"slices"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/drive/internal/store"
)

// DriveSettings is a person's drive preferences in a workspace.
type DriveSettings struct {
	View             string `json:"view"`
	Sort             string `json:"sort"`
	FoldersFirst     bool   `json:"folders_first"`
	CellularPreviews bool   `json:"cellular_previews"`
	NotifyShared     bool   `json:"notify_shared"`
}

// DriveSettingsPatch is the body of PUT /drive/settings: a field left out (nil)
// keeps its stored value, so a client that knows only some settings cannot
// reset the others.
type DriveSettingsPatch struct {
	View             *string `json:"view"`
	Sort             *string `json:"sort"`
	FoldersFirst     *bool   `json:"folders_first"`
	CellularPreviews *bool   `json:"cellular_previews"`
	NotifyShared     *bool   `json:"notify_shared"`
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// applyTo lays the fields the patch carries over cur.
func (p *DriveSettingsPatch) applyTo(cur *DriveSettings) {
	if p == nil {
		return
	}
	setIf(&cur.View, p.View)
	setIf(&cur.Sort, p.Sort)
	setIf(&cur.FoldersFirst, p.FoldersFirst)
	setIf(&cur.CellularPreviews, p.CellularPreviews)
	setIf(&cur.NotifyShared, p.NotifyShared)
}

// DefaultDriveSettings returns the defaults for someone who never saved settings.
func DefaultDriveSettings() *DriveSettings {
	return &DriveSettings{
		View: "auto", Sort: "modified", FoldersFirst: true,
		CellularPreviews: true, NotifyShared: true,
	}
}

var validViews = []string{"auto", "grid", "list"}
var validSorts = []string{"modified", "name", "size"}

// ValidateDriveSettings checks that every field is one of the allowed values.
func ValidateDriveSettings(s *DriveSettings) error {
	if !slices.Contains(validViews, s.View) {
		return invalid("view must be one of: auto, grid, list")
	}
	if !slices.Contains(validSorts, s.Sort) {
		return invalid("sort must be one of: modified, name, size")
	}
	return nil
}

// GetDriveSettings returns the caller's settings, or defaults.
func (s *Service) GetDriveSettings(ctx context.Context, userID, workspaceID string) (*DriveSettings, error) {
	if err := s.requireWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.storedDriveSettings(ctx, userID, workspaceID)
}

// storedDriveSettings reads the caller's saved settings, or the defaults.
func (s *Service) storedDriveSettings(ctx context.Context, userID, workspaceID string) (*DriveSettings, error) {
	row, err := s.store.GetDriveUserSettings(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return DefaultDriveSettings(), nil
	}
	return &DriveSettings{
		View: row.View, Sort: row.Sort, FoldersFirst: row.FoldersFirst,
		CellularPreviews: row.CellularPreviews, NotifyShared: row.NotifyShared,
	}, nil
}

// UpdateDriveSettings merges the fields the patch carries into the caller's
// stored settings (or the defaults), checks the result and saves it.
func (s *Service) UpdateDriveSettings(ctx context.Context, userID, workspaceID string, patch *DriveSettingsPatch) (*DriveSettings, error) {
	if err := s.requireWorkspaceMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	in, err := s.storedDriveSettings(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	patch.applyTo(in)
	if err := ValidateDriveSettings(in); err != nil {
		return nil, err
	}
	if err := s.store.UpsertDriveUserSettings(ctx, &store.DriveUserSettings{
		UserID: userID, WorkspaceID: workspaceID,
		View: in.View, Sort: in.Sort, FoldersFirst: in.FoldersFirst,
		CellularPreviews: in.CellularPreviews, NotifyShared: in.NotifyShared,
	}); err != nil {
		return nil, err
	}
	return in, nil
}

// requireWorkspaceMember checks that the caller is an active member.
func (s *Service) requireWorkspaceMember(ctx context.Context, workspaceID, userID string) error {
	if workspaceID == "" {
		return invalid("workspace_id required")
	}
	// When called from gRPC the caller is on the context; for REST the claims
	// middleware put it there. Either way, an unauthenticated request denies.
	who := grpcauth.CallerFrom(ctx)
	if !who.Authenticated() {
		return denied("access denied")
	}
	ok, err := s.store.IsMemberOfWorkspace(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return denied("not a member of this workspace")
	}
	return nil
}
