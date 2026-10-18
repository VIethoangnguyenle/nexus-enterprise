package domain

import (
	"context"
	"errors"
	"testing"
)

// settingsMemStore answers the workspace type and keeps settings rows in memory;
// any other store call panics through the nil embedded interface.
type settingsMemStore struct {
	Store
	kind string
	rows map[string]SettingsRow
}

func newSettingsMemStore(kind string) *settingsMemStore {
	return &settingsMemStore{kind: kind, rows: map[string]SettingsRow{}}
}

func (s *settingsMemStore) WorkspaceType(context.Context, string) (string, error) {
	return s.kind, nil
}

func (s *settingsMemStore) GetApprovalUserSettings(_ context.Context, userID string) (*SettingsRow, error) {
	row, ok := s.rows[userID]
	if !ok {
		return nil, nil
	}
	return &row, nil
}

func (s *settingsMemStore) UpsertApprovalUserSettings(_ context.Context, st *SettingsRow) error {
	s.rows[st.UserID] = *st
	return nil
}

func strPtr(v string) *string { return &v }
func boolPtr(v bool) *bool    { return &v }

// A client that sends only some settings must not reset the others.
func TestApprovalSettingsSaveKeepsFieldsTheBodyLeavesOut(t *testing.T) {
	svc := NewService(newSettingsMemStore(workspaceTypeOrganization), nil)
	ctx := callerIn("tenant-1")
	if _, err := svc.UpdateUserSettings(ctx, "u", &ApprovalSettingsPatch{
		DefaultTab: strPtr("rejected"), Sort: strPtr("oldest"),
		NotifyPending: boolPtr(false), NotifyResult: boolPtr(false),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.UpdateUserSettings(ctx, "u", &ApprovalSettingsPatch{Sort: strPtr("newest")})
	if err != nil {
		t.Fatal(err)
	}
	want := ApprovalSettings{DefaultTab: "rejected", Sort: "newest"}
	if *got != want {
		t.Fatalf("answered %+v, want %+v", *got, want)
	}
	stored, err := svc.GetUserSettings(ctx, "u")
	if err != nil {
		t.Fatal(err)
	}
	if *stored != want {
		t.Fatalf("stored %+v, want %+v", *stored, want)
	}
}

func TestApprovalSettingsFirstPartialSaveStartsFromTheDefaults(t *testing.T) {
	svc := NewService(newSettingsMemStore(workspaceTypeOrganization), nil)
	got, err := svc.UpdateUserSettings(callerIn("tenant-1"), "u", &ApprovalSettingsPatch{NotifyResult: boolPtr(false)})
	if err != nil {
		t.Fatal(err)
	}
	want := *DefaultApprovalSettings()
	want.NotifyResult = false
	if *got != want {
		t.Fatalf("answered %+v, want %+v", *got, want)
	}
}

func TestApprovalSettingsMergedValueIsStillChecked(t *testing.T) {
	st := newSettingsMemStore(workspaceTypeOrganization)
	svc := NewService(st, nil)
	ctx := callerIn("tenant-1")
	for name, p := range map[string]*ApprovalSettingsPatch{
		"unknown tab":  {DefaultTab: strPtr("all")},
		"empty sort":   {Sort: strPtr("")},
		"unknown sort": {Sort: strPtr("alphabetical")},
	} {
		if _, err := svc.UpdateUserSettings(ctx, "u", p); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: error = %v; want ErrInvalidInput", name, err)
		}
	}
	if len(st.rows) != 0 {
		t.Fatalf("a rejected save stored %+v", st.rows)
	}
}

func TestApprovalSettingsPartialSaveIsRefusedInAPersonalWorkspace(t *testing.T) {
	st := newSettingsMemStore(workspaceTypePersonal)
	svc := NewService(st, nil)
	for name, p := range map[string]*ApprovalSettingsPatch{
		"partial": {Sort: strPtr("oldest")},
		"empty":   {},
	} {
		if _, err := svc.UpdateUserSettings(callerIn("tenant-1"), "u", p); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("%s: error = %v; want ErrAccessDenied", name, err)
		}
	}
	if len(st.rows) != 0 {
		t.Fatalf("a refused save stored %+v", st.rows)
	}
}
