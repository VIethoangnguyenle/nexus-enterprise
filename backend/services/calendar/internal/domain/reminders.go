package domain

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

const (
	// reminderEvery is how often due reminders are looked for.
	reminderEvery = 30 * time.Second
	// reminderGrace is how late a reminder may still fire: a service that was down
	// catches up with what fell due meanwhile, but does not wake people with a
	// reminder for something already under way.
	reminderGrace = 10 * time.Minute
)

// RunReminders fires due reminders every reminderEvery until ctx is done.
func (s *Service) RunReminders(ctx context.Context) {
	t := time.NewTicker(reminderEvery)
	defer t.Stop()
	for {
		if n, err := s.FireReminders(ctx); err != nil && ctx.Err() == nil {
			slog.Error("reminders not fired", "error", err)
		} else if n > 0 {
			slog.Info("reminders fired", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// reminderRound caches what one pass learns about a calendar or a workspace.
type reminderRound struct {
	workspaces map[string]*store.Workspace
	calendars  map[string]*store.Calendar
}

// FireReminders sends the reminders that have fallen due: each occurrence that
// has a reminder at start minus its minutes, now at or past that moment and not
// more than reminderGrace past it, to each person it concerns — the guests who
// have not declined it, and the owner of a personal calendar (the host of any
// other event). A reminder is sent once per person, occurrence and minutes, and
// claimed in the database first, so any number of services running this at once
// send it once. It returns how many it sent.
func (s *Service) FireReminders(ctx context.Context) (int, error) {
	now := s.now()
	// An occurrence that starts after the longest reminder an event may carry
	// cannot be due yet.
	masters, overrides, err := s.store.ReminderEvents(ctx, now.Add(-reminderGrace), now.Add(maxReminderMinutes*time.Minute))
	if err != nil {
		return 0, internal("find events with reminders", err)
	}
	if len(masters) == 0 {
		return 0, nil
	}
	ids := make([]string, len(masters))
	for i, m := range masters {
		ids[i] = m.ID
	}
	specs, err := s.store.ReminderSpecs(ctx, ids)
	if err != nil {
		return 0, internal("load reminders", err)
	}
	insts, err := expandEvents(masters, overrides, now.Add(-reminderGrace), now.Add(maxReminderMinutes*time.Minute))
	if err != nil {
		return 0, internal("expand events", err)
	}
	round := &reminderRound{workspaces: map[string]*store.Workspace{}, calendars: map[string]*store.Calendar{}}
	sent := 0
	for _, in := range insts {
		list := specs[in.Master.ID]
		if len(dueMinutes(list, in.Start, now)) == 0 {
			continue
		}
		n, err := s.remindOccurrence(ctx, round, in, list, now)
		sent += n
		if err != nil {
			// The rest are tried; this one is retried on the next pass while its grace lasts.
			slog.Error("could not remind an occurrence", "event", in.Master.ID, "error", err)
		}
	}
	return sent, nil
}

// dueMinutes lists the minutes among the specs that are due for an occurrence
// starting at start.
func dueMinutes(specs []store.ReminderSpec, start, now time.Time) []int {
	var out []int
	for _, sp := range specs {
		fireAt := start.Add(-time.Duration(sp.Minutes) * time.Minute)
		if !now.Before(fireAt) && now.Sub(fireAt) <= reminderGrace && !slices.Contains(out, sp.Minutes) {
			out = append(out, sp.Minutes)
		}
	}
	return out
}

// remindOccurrence sends the due reminders of one occurrence.
func (s *Service) remindOccurrence(ctx context.Context, round *reminderRound, in instance, specs []store.ReminderSpec, now time.Time) (int, error) {
	m := in.Master
	ws, err := round.workspace(ctx, s.store, m.WorkspaceID)
	if err != nil {
		return 0, internal("load workspace", err)
	}
	cal, err := round.calendar(ctx, s.store, m.CalendarID)
	if err != nil {
		return 0, internal("load calendar", err)
	}
	audience, err := s.reminderAudience(ctx, m, cal, in.Original)
	if err != nil {
		return 0, err
	}

	// A person's own reminders replace the event's.
	own := map[string][]store.ReminderSpec{}
	var shared []store.ReminderSpec
	for _, sp := range specs {
		if sp.UserNodeID == "" {
			shared = append(shared, sp)
		} else {
			own[sp.UserNodeID] = append(own[sp.UserNodeID], sp)
		}
	}
	shown := *in.Row
	shown.ID, shown.StartsAt, shown.EndsAt, shown.NgacOAID = m.ID, in.Start, in.End, m.NgacOAID

	sent := 0
	var firstErr error
	for _, node := range audience {
		list := shared
		if mine, ok := own[node]; ok {
			list = mine
		}
		for _, minutes := range dueMinutes(list, in.Start, now) {
			claimed, err := s.store.ClaimReminder(ctx, m.ID, node, in.Original, minutes)
			if err != nil {
				if firstErr == nil {
					firstErr = internal("claim reminder", err)
				}
				continue
			}
			if !claimed {
				continue
			}
			mins := minutes
			s.notify(ctx, events.TypeReminder, ws, cal, &shown, []string{node}, noticeExtras{occurrence: in.Original, minutes: &mins, toActor: true})
			sent++
		}
	}
	return sent, firstErr
}

// reminderAudience is who an occurrence reminds: the guests who have not declined
// it (an answer to the occurrence outranks the one to the series), and the owner
// of a personal calendar or the host of any other event, unless they declined.
func (s *Service) reminderAudience(ctx context.Context, m *store.Event, cal *store.Calendar, original time.Time) ([]string, error) {
	guests, err := s.store.Attendees(ctx, m.ID)
	if err != nil {
		return nil, internal("load guests", err)
	}
	answers, err := s.store.OccurrenceAnswers(ctx, m.ID, original)
	if err != nil {
		return nil, internal("load answers", err)
	}
	declined := map[string]bool{}
	var out []string
	for _, g := range guests {
		response := g.Response
		if a := answers[g.UserNodeID]; a != nil {
			response = a.Response
		}
		declined[g.UserNodeID] = response == ResponseDeclined
		if response != ResponseDeclined {
			out = append(out, g.UserNodeID)
		}
	}
	host := m.OrganizerNodeID
	if cal.Kind == store.KindPersonal {
		host = cal.OwnerNodeID
	}
	if _, listed := declined[host]; !listed && host != "" {
		out = append(out, host)
	}
	return distinct(out...), nil
}

func (r *reminderRound) workspace(ctx context.Context, st Store, id string) (*store.Workspace, error) {
	if w, ok := r.workspaces[id]; ok {
		return w, nil
	}
	w, err := st.Workspace(ctx, id)
	if err != nil {
		return nil, err
	}
	r.workspaces[id] = w
	return w, nil
}

func (r *reminderRound) calendar(ctx context.Context, st Store, id string) (*store.Calendar, error) {
	if c, ok := r.calendars[id]; ok {
		return c, nil
	}
	c, err := st.CalendarByID(ctx, id)
	if err != nil {
		return nil, err
	}
	r.calendars[id] = c
	return c, nil
}
