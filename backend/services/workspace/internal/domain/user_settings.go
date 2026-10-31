package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// SettingMobileBar is the key of the phone's bottom bar: a JSON array of exactly
// MobileBarSlots distinct module ids, in order.
const SettingMobileBar = "mobile.bar"

// MobileBarSlots is how many modules the phone's bottom bar shows.
const MobileBarSlots = 4

// mobileBarModules are the modules a bar slot may hold. "admin" is deliberately
// absent: administration is reached from workspace settings, never from the bar.
var mobileBarModules = []string{"chat", "calendar", "drive", "approval", "assets", "contacts", "notifications"}

// modulesOf lists what a workspace of the given type offers: a personal workspace
// has no approval and no assets.
func modulesOf(wsType string) []string {
	if wsType != TypePersonal {
		return mobileBarModules
	}
	out := make([]string, 0, len(mobileBarModules))
	for _, m := range mobileBarModules {
		if m != "approval" && m != "assets" {
			out = append(out, m)
		}
	}
	return out
}

// normalizeMobileBar checks a bar value and returns it in canonical JSON.
func normalizeMobileBar(raw json.RawMessage, wsType string) (json.RawMessage, error) {
	var ids []string
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ids); err != nil || ids == nil {
		return nil, fmt.Errorf("%w: %s must be an array of %d module ids", ErrInvalidInput, SettingMobileBar, MobileBarSlots)
	}
	if dec.More() {
		return nil, fmt.Errorf("%w: %s must be a single array", ErrInvalidInput, SettingMobileBar)
	}
	if len(ids) != MobileBarSlots {
		return nil, fmt.Errorf("%w: %s must hold exactly %d modules", ErrInvalidInput, SettingMobileBar, MobileBarSlots)
	}
	allowed := modulesOf(wsType)
	for i, id := range ids {
		if !slices.Contains(allowed, id) {
			return nil, fmt.Errorf("%w: %q is not a module this workspace offers", ErrInvalidInput, id)
		}
		if slices.Contains(ids[:i], id) {
			return nil, fmt.Errorf("%w: %q appears twice", ErrInvalidInput, id)
		}
	}
	return json.Marshal(ids)
}

// normalizeSetting validates one workspace-scope setting this service owns.
func normalizeSetting(key string, raw json.RawMessage, wsType string) (json.RawMessage, error) {
	switch key {
	case SettingMobileBar:
		return normalizeMobileBar(raw, wsType)
	}
	return nil, fmt.Errorf("%w: unknown setting %q", ErrInvalidInput, key)
}

// GetUserSettings returns the caller's own workspace-scope settings that have been
// set; a key never written is absent (the client knows its defaults). The caller
// must belong to the workspace, and userID — from verified claims — scopes every
// read, so no one reaches another person's row.
func (s *Service) GetUserSettings(ctx context.Context, callerNodeID, userID, wsID string) (map[string]json.RawMessage, error) {
	ws, err := s.authorizeSettings(ctx, callerNodeID, userID, wsID)
	if err != nil {
		return nil, err
	}
	return s.store.GetUserSettings(ctx, userID, ws.ID)
}

// PutUserSettings validates every key of kv and then writes them all, or nothing.
// It answers with the caller's settings as they are afterwards.
func (s *Service) PutUserSettings(ctx context.Context, callerNodeID, userID, wsID string, kv map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	ws, err := s.authorizeSettings(ctx, callerNodeID, userID, wsID)
	if err != nil {
		return nil, err
	}
	if len(kv) == 0 {
		return nil, fmt.Errorf("%w: nothing to change", ErrInvalidInput)
	}
	clean := make(map[string]json.RawMessage, len(kv))
	for k, v := range kv {
		if clean[k], err = normalizeSetting(k, v, ws.Type); err != nil {
			return nil, err
		}
	}
	if err := s.store.PutUserSettings(ctx, userID, ws.ID, clean); err != nil {
		return nil, err
	}
	return s.store.GetUserSettings(ctx, userID, ws.ID)
}

// authorizeSettings requires a signed-in person who belongs to the workspace.
func (s *Service) authorizeSettings(ctx context.Context, callerNodeID, userID, wsID string) (*WorkspaceResult, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: requires an authenticated caller", ErrAccessDenied)
	}
	return s.authorizeMember(ctx, callerNodeID, wsID)
}
