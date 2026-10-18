package domain

import (
	"context"
	"slices"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/asset/internal/store"
)

// AssetSettings is a person's asset preferences in a workspace.
type AssetSettings struct {
	DefaultTab        string `json:"default_tab"`
	Sort              string `json:"sort"`
	NotifyRequests    bool   `json:"notify_requests"`
	NotifyMaintenance bool   `json:"notify_maintenance"`
}

// AssetSettingsPatch is the body of PUT /workspaces/:id/asset-settings: a field
// left out (nil) keeps its stored value, so a client that knows only some
// settings cannot reset the others.
type AssetSettingsPatch struct {
	DefaultTab        *string `json:"default_tab"`
	Sort              *string `json:"sort"`
	NotifyRequests    *bool   `json:"notify_requests"`
	NotifyMaintenance *bool   `json:"notify_maintenance"`
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// applyTo lays the fields the patch carries over cur.
func (p *AssetSettingsPatch) applyTo(cur *AssetSettings) {
	if p == nil {
		return
	}
	setIf(&cur.DefaultTab, p.DefaultTab)
	setIf(&cur.Sort, p.Sort)
	setIf(&cur.NotifyRequests, p.NotifyRequests)
	setIf(&cur.NotifyMaintenance, p.NotifyMaintenance)
}

// DefaultAssetSettings returns the defaults for someone who never saved settings.
func DefaultAssetSettings() *AssetSettings {
	return &AssetSettings{
		DefaultTab: "overview", Sort: "name",
		NotifyRequests: true, NotifyMaintenance: true,
	}
}

var validAssetTabs = []string{"overview", "inventory", "requests"}
var ValidAssetSorts = []string{"name", "created", "updated", "status"}
var validAssetSorts = ValidAssetSorts

// ValidateAssetSettings checks that every field is one of the allowed values.
func ValidateAssetSettings(s *AssetSettings) error {
	if !slices.Contains(validAssetTabs, s.DefaultTab) {
		return invalid("default_tab must be one of: overview, inventory, requests")
	}
	if !slices.Contains(validAssetSorts, s.Sort) {
		return invalid("sort must be one of: name, created, updated, status")
	}
	return nil
}

// GetAssetSettings returns the caller's settings, or defaults.
func (s *AssetService) GetAssetSettings(ctx context.Context, userID, workspaceID string) (*AssetSettings, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	return s.storedAssetSettings(ctx, userID, workspaceID)
}

// storedAssetSettings reads the caller's saved settings, or the defaults.
func (s *AssetService) storedAssetSettings(ctx context.Context, userID, workspaceID string) (*AssetSettings, error) {
	row, err := s.store.GetAssetUserSettings(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return DefaultAssetSettings(), nil
	}
	return &AssetSettings{
		DefaultTab: row.DefaultTab, Sort: row.Sort,
		NotifyRequests: row.NotifyRequests, NotifyMaintenance: row.NotifyMaintenance,
	}, nil
}

// UpdateAssetSettings merges the fields the patch carries into the caller's
// stored settings (or the defaults), checks the result and saves it.
func (s *AssetService) UpdateAssetSettings(ctx context.Context, userID, workspaceID string, patch *AssetSettingsPatch) (*AssetSettings, error) {
	if err := s.requireMember(ctx, workspaceID, userID); err != nil {
		return nil, err
	}
	in, err := s.storedAssetSettings(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	patch.applyTo(in)
	if err := ValidateAssetSettings(in); err != nil {
		return nil, err
	}
	if err := s.store.UpsertAssetUserSettings(ctx, &store.AssetUserSettings{
		UserID: userID, WorkspaceID: workspaceID,
		DefaultTab: in.DefaultTab, Sort: in.Sort,
		NotifyRequests: in.NotifyRequests, NotifyMaintenance: in.NotifyMaintenance,
	}); err != nil {
		return nil, err
	}
	return in, nil
}

// requireMember checks that the caller is an active member of the workspace.
func (s *AssetService) requireMember(ctx context.Context, workspaceID, userID string) error {
	if workspaceID == "" {
		return invalid("workspace_id required")
	}
	who := grpcauth.CallerFrom(ctx)
	if !who.Authenticated() {
		return denied("access denied")
	}
	ok, err := s.store.IsWorkspaceMember(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return denied("not a member of this workspace")
	}
	return nil
}
