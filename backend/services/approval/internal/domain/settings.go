package domain

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// ApprovalSettings is a person's approval preferences in a workspace.
type ApprovalSettings struct {
	DefaultTab    string `json:"default_tab"`
	Sort          string `json:"sort"`
	NotifyPending bool   `json:"notify_pending"`
	NotifyResult  bool   `json:"notify_result"`
}

// ApprovalSettingsPatch is the body of PUT /approval/settings: a field left out
// (nil) keeps its stored value, so a client that knows only some settings
// cannot reset the others.
type ApprovalSettingsPatch struct {
	DefaultTab    *string `json:"default_tab"`
	Sort          *string `json:"sort"`
	NotifyPending *bool   `json:"notify_pending"`
	NotifyResult  *bool   `json:"notify_result"`
}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// applyTo lays the fields the patch carries over cur.
func (p *ApprovalSettingsPatch) applyTo(cur *ApprovalSettings) {
	if p == nil {
		return
	}
	setIf(&cur.DefaultTab, p.DefaultTab)
	setIf(&cur.Sort, p.Sort)
	setIf(&cur.NotifyPending, p.NotifyPending)
	setIf(&cur.NotifyResult, p.NotifyResult)
}

// DefaultApprovalSettings returns the defaults for someone who never saved settings.
func DefaultApprovalSettings() *ApprovalSettings {
	return &ApprovalSettings{
		DefaultTab: "pending", Sort: "newest",
		NotifyPending: true, NotifyResult: true,
	}
}

var validApprovalTabs = []string{"pending", "approved", "rejected"}
var validApprovalSorts = []string{"newest", "oldest"}

// ValidateApprovalSettings checks that every field is one of the allowed values.
func ValidateApprovalSettings(s *ApprovalSettings) error {
	if !slices.Contains(validApprovalTabs, s.DefaultTab) {
		return fmt.Errorf("%w: default_tab must be one of: pending, approved, rejected", ErrInvalidInput)
	}
	if !slices.Contains(validApprovalSorts, s.Sort) {
		return fmt.Errorf("%w: sort must be one of: newest, oldest", ErrInvalidInput)
	}
	return nil
}

// SettingsRow is the database representation of a user's approval settings.
type SettingsRow struct {
	UserID        string
	DefaultTab    string
	Sort          string
	NotifyPending bool
	NotifyResult  bool
	UpdatedAt     time.Time
}

// SettingsStore defines settings data access methods.
type SettingsStore interface {
	EnsureSettingsTable(ctx context.Context) error
	GetApprovalUserSettings(ctx context.Context, userID string) (*SettingsRow, error)
	UpsertApprovalUserSettings(ctx context.Context, st *SettingsRow) error
}

// GetUserSettings returns the caller's settings, or defaults.
func (s *Service) GetUserSettings(ctx context.Context, userID string) (*ApprovalSettings, error) {
	if err := s.requireCallerOrganization(ctx); err != nil {
		return nil, err
	}
	ss, ok := s.store.(SettingsStore)
	if !ok {
		return DefaultApprovalSettings(), nil
	}
	return storedApprovalSettings(ctx, ss, userID)
}

// storedApprovalSettings reads the caller's saved settings, or the defaults.
func storedApprovalSettings(ctx context.Context, ss SettingsStore, userID string) (*ApprovalSettings, error) {
	if err := ss.EnsureSettingsTable(ctx); err != nil {
		return nil, fmt.Errorf("ensure settings table: %w", err)
	}
	row, err := ss.GetApprovalUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return DefaultApprovalSettings(), nil
	}
	return &ApprovalSettings{
		DefaultTab: row.DefaultTab, Sort: row.Sort,
		NotifyPending: row.NotifyPending, NotifyResult: row.NotifyResult,
	}, nil
}

// UpdateUserSettings merges the fields the patch carries into the caller's
// stored settings (or the defaults), checks the result and saves it.
func (s *Service) UpdateUserSettings(ctx context.Context, userID string, patch *ApprovalSettingsPatch) (*ApprovalSettings, error) {
	if err := s.requireCallerOrganization(ctx); err != nil {
		return nil, err
	}
	ss, ok := s.store.(SettingsStore)
	if !ok {
		return nil, fmt.Errorf("%w: settings not available", ErrInvalidInput)
	}
	in, err := storedApprovalSettings(ctx, ss, userID)
	if err != nil {
		return nil, err
	}
	patch.applyTo(in)
	if err := ValidateApprovalSettings(in); err != nil {
		return nil, err
	}
	if err := ss.UpsertApprovalUserSettings(ctx, &SettingsRow{
		UserID: userID, DefaultTab: in.DefaultTab, Sort: in.Sort,
		NotifyPending: in.NotifyPending, NotifyResult: in.NotifyResult,
	}); err != nil {
		return nil, err
	}
	return in, nil
}
