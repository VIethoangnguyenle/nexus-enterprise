package domain

import (
	"context"
	"slices"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/store"
)

func viewOfSettings(st store.Settings) *SettingsView {
	return &SettingsView{
		TZID: st.TZID, WorkStart: st.WorkStart, WorkEnd: st.WorkEnd,
		DefaultReminder: st.DefaultReminder,
		DefaultView:     st.DefaultView, WeekStart: st.WeekStart,
		VisibleCalendars: st.VisibleCalendars, DefaultDuration: st.DefaultDuration,
		NotifyInvites: st.NotifyInvites, NotifyChanges: st.NotifyChanges,
		NotifyReminders: st.NotifyReminders,
	}
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

var validDefaultViews = []string{"agenda", "day", "week", "month"}

func setIf[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// merged lays the fields the patch carries over the stored settings.
func merged(st store.Settings, p SettingsPatch) SettingsView {
	in := *viewOfSettings(st)
	setIf(&in.TZID, p.TZID)
	setIf(&in.WorkStart, p.WorkStart)
	setIf(&in.WorkEnd, p.WorkEnd)
	setIf(&in.DefaultReminder, p.DefaultReminder)
	setIf(&in.DefaultView, p.DefaultView)
	setIf(&in.WeekStart, p.WeekStart)
	setIf(&in.DefaultDuration, p.DefaultDuration)
	setIf(&in.NotifyInvites, p.NotifyInvites)
	setIf(&in.NotifyChanges, p.NotifyChanges)
	setIf(&in.NotifyReminders, p.NotifyReminders)
	if p.VisibleCalendars != nil {
		in.VisibleCalendars = *p.VisibleCalendars
		if len(in.VisibleCalendars) == 0 {
			in.VisibleCalendars = nil
		}
	}
	return in
}

// UpdateSettings merges the fields the patch carries into the caller's stored
// settings, checks the result and saves it. A field the patch leaves out keeps
// its stored value.
func (s *Service) UpdateSettings(ctx context.Context, wsID string, patch SettingsPatch) (*SettingsView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	stored, err := s.store.Settings(ctx, ws.ID, []string{who.NGACNodeID})
	if err != nil {
		return nil, internal("load settings", err)
	}
	in := merged(stored[who.NGACNodeID], patch)
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
	// Validate new fields.
	if in.DefaultView != "" && !slices.Contains(validDefaultViews, in.DefaultView) {
		return nil, invalid("defaultView must be one of: agenda, day, week, month")
	}
	if in.DefaultView == "" {
		in.DefaultView = "agenda"
	}
	if in.WeekStart != 0 && in.WeekStart != 1 {
		return nil, invalid("weekStart must be 0 (Sunday) or 1 (Monday)")
	}
	if in.DefaultDuration == 0 {
		in.DefaultDuration = 30
	}
	if in.DefaultDuration < 5 || in.DefaultDuration > 1440 {
		return nil, invalid("defaultDuration must be 5 to 1440 minutes")
	}
	// Validate visible_calendars: each ID must be a calendar the caller can read.
	// Only a list the patch carries is checked; a stored one was checked on save.
	if patch.VisibleCalendars != nil {
		for _, cid := range in.VisibleCalendars {
			cal, err := s.store.CalendarByID(ctx, cid)
			if err != nil {
				return nil, denied("visible calendar %s not accessible", cid)
			}
			if err := s.authorize(ctx, who.NGACNodeID, cal, ngac.OpRead); err != nil {
				return nil, denied("visible calendar %s not accessible", cid)
			}
		}
	}
	st := store.Settings{
		TZID: in.TZID, WorkStart: in.WorkStart, WorkEnd: in.WorkEnd,
		DefaultReminder: in.DefaultReminder,
		DefaultView:     in.DefaultView, WeekStart: in.WeekStart,
		VisibleCalendars: in.VisibleCalendars, DefaultDuration: in.DefaultDuration,
		NotifyInvites: in.NotifyInvites, NotifyChanges: in.NotifyChanges,
		NotifyReminders: in.NotifyReminders,
	}
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
