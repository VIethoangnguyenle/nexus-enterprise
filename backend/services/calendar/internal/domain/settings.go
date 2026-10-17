package domain

import (
	"context"
	"time"

	"ngac-platform/services/calendar/internal/store"
)

func viewOfSettings(st store.Settings) *SettingsView {
	return &SettingsView{TZID: st.TZID, WorkStart: st.WorkStart, WorkEnd: st.WorkEnd, DefaultReminder: st.DefaultReminder}
}

// GetSettings returns the caller's calendar settings: their zone, working hours
// and default reminder, or the defaults (08:00 to 17:30 in Ho Chi Minh City) when
// they never saved any.
func (s *Service) GetSettings(ctx context.Context, wsID string) (*SettingsView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	got, err := s.store.Settings(ctx, ws.ID, []string{who.NGACNodeID})
	if err != nil {
		return nil, internal("load settings", err)
	}
	return viewOfSettings(got[who.NGACNodeID]), nil
}

// UpdateSettings saves the caller's settings after checking them.
func (s *Service) UpdateSettings(ctx context.Context, wsID string, in SettingsView) (*SettingsView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if _, err := loadZone(in.TZID); err != nil {
		return nil, err
	}
	start, err := hhmm(in.WorkStart)
	if err != nil {
		return nil, invalid("workStart must be HH:mm")
	}
	end, err := hhmm(in.WorkEnd)
	if err != nil {
		return nil, invalid("workEnd must be HH:mm")
	}
	if !end.After(start) {
		return nil, invalid("the working day must end after it starts")
	}
	if in.DefaultReminder < 0 || in.DefaultReminder > maxReminderMinutes {
		return nil, invalid("a reminder is 0 to %d minutes before", maxReminderMinutes)
	}
	st := store.Settings{TZID: in.TZID, WorkStart: in.WorkStart, WorkEnd: in.WorkEnd, DefaultReminder: in.DefaultReminder}
	if err := s.store.SaveSettings(ctx, ws.ID, who.NGACNodeID, st); err != nil {
		return nil, internal("save settings", err)
	}
	return viewOfSettings(st), nil
}

// hhmm reads a wall clock written exactly as HH:mm.
func hhmm(v string) (time.Time, error) {
	if len(v) != 5 {
		return time.Time{}, invalid("not HH:mm")
	}
	return time.Parse("15:04", v)
}
