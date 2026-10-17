package domain

import (
	"encoding/json"
	"strings"
)

// Values an event's enumerated fields take.
const (
	KindEvent = "event"
	KindFocus = "focus"
	KindOOO   = "ooo"

	TransparencyBusy = "busy"
	TransparencyFree = "free"

	VisibilityDefault = "default"
	VisibilityPrivate = "private"

	ScopeThis      = "this"
	ScopeFollowing = "following"
	ScopeAll       = "all"
)

// busyTitle is what an occurrence the caller may not read is called.
const busyTitle = "Bận"

// staleMessage is shown to a person whose edit started from an older version.
const staleMessage = "Sự kiện vừa được người khác sửa. Tải lại rồi thử lại."

// ReminderInput is one reminder: how long before the start it fires.
type ReminderInput struct {
	MinutesBefore int `json:"minutesBefore"`
}

// EventInput is the body of POST /events.
type EventInput struct {
	CalendarID  string  `json:"calendarId"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Location    string  `json:"location"`
	Start       string  `json:"start"`
	End         string  `json:"end"`
	AllDay      bool    `json:"allDay"`
	TZID        string  `json:"tzid"`
	RRule       *string `json:"rrule"`
	Kind        string  `json:"kind"`
	// Transparency and Visibility default to busy and default.
	Transparency string `json:"transparency"`
	Visibility   string `json:"visibility"`
	// Attendees are guests' node ids, InviteChannelID a group whose members are
	// all invited (expanded when the event is saved).
	Attendees        []string         `json:"attendees"`
	InviteChannelID  string           `json:"inviteChannelId"`
	GuestsCanInvite  bool             `json:"guestsCanInvite"`
	GuestsCanSeeList bool             `json:"guestsCanSeeList"`
	HasCall          bool             `json:"hasCall"`
	Reminders        *[]ReminderInput `json:"reminders"`
}

// OptionalString tells "not sent" from "sent as null" from "sent as a value".
type OptionalString struct {
	Set   bool
	Value *string
}

// UnmarshalJSON marks the field as sent, whatever it holds.
func (o *OptionalString) UnmarshalJSON(b []byte) error {
	o.Set = true
	if strings.TrimSpace(string(b)) == "null" {
		o.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	o.Value = &s
	return nil
}

// EventPatch is the body of PATCH /events/:id: the fields to change, and the
// version the edit started from.
type EventPatch struct {
	Version     int            `json:"version"`
	CalendarID  *string        `json:"calendarId"`
	Title       *string        `json:"title"`
	Description *string        `json:"description"`
	Location    *string        `json:"location"`
	Start       *string        `json:"start"`
	End         *string        `json:"end"`
	AllDay      *bool          `json:"allDay"`
	TZID        *string        `json:"tzid"`
	RRule       OptionalString `json:"rrule"`
	Kind        *string        `json:"kind"`

	Transparency     *string          `json:"transparency"`
	Visibility       *string          `json:"visibility"`
	Attendees        *[]string        `json:"attendees"`
	InviteChannelID  *string          `json:"inviteChannelId"`
	GuestsCanInvite  *bool            `json:"guestsCanInvite"`
	GuestsCanSeeList *bool            `json:"guestsCanSeeList"`
	HasCall          *bool            `json:"hasCall"`
	Reminders        *[]ReminderInput `json:"reminders"`
}

// ScopeQuery says which occurrences a change to a repeating event reaches.
type ScopeQuery struct {
	Scope      string
	Occurrence string
}

// ListEventsQuery asks for the occurrences inside [From, To) of some calendars
// (all the caller's visible ones when none is named).
type ListEventsQuery struct {
	From      string
	To        string
	Calendars []string
}

// OccurrenceView is one occurrence of an event as a grid draws it.
type OccurrenceView struct {
	EventID         string `json:"eventId"`
	OccurrenceStart string `json:"occurrenceStart"`
	Start           string `json:"start"`
	End             string `json:"end"`
	AllDay          bool   `json:"allDay"`
	CalendarID      string `json:"calendarId"`
	Title           string `json:"title"`
	Location        string `json:"location,omitempty"`
	// BusyOnly: the caller may not read the details; only the time is real.
	BusyOnly       bool       `json:"busyOnly"`
	MyResponse     string     `json:"myResponse,omitempty"`
	IsOrganizer    bool       `json:"isOrganizer"`
	HasCall        bool       `json:"hasCall"`
	IsRecurring    bool       `json:"isRecurring"`
	RecurrenceText string     `json:"recurrenceText,omitempty"`
	CanEdit        bool       `json:"canEdit"`
	Kind           string     `json:"kind"`
	GuestCount     int        `json:"guestCount"`
	Organizer      *PersonRef `json:"organizer,omitempty"`
	Version        int        `json:"version"`
}

// AttendeeView is one guest as the detail lists them.
type AttendeeView struct {
	Person      *PersonRef `json:"person"`
	Response    string     `json:"response"`
	Comment     string     `json:"comment,omitempty"`
	IsOrganizer bool       `json:"isOrganizer"`
}

// EventDetailView is the whole event behind an occurrence.
type EventDetailView struct {
	OccurrenceView
	Description      string          `json:"description,omitempty"`
	TZID             string          `json:"tzid"`
	RRule            string          `json:"rrule,omitempty"`
	Transparency     string          `json:"transparency"`
	Visibility       string          `json:"visibility"`
	GuestsCanInvite  bool            `json:"guestsCanInvite"`
	GuestsCanSeeList bool            `json:"guestsCanSeeList"`
	Attendees        []AttendeeView  `json:"attendees"`
	Reminders        []ReminderInput `json:"reminders"`
	// PreservedOverrides is how many individually edited occurrences an edit of
	// the whole series left as they were.
	PreservedOverrides int `json:"preservedOverrides,omitempty"`
}

// RsvpInput is the body of POST /events/:id/rsvp: the caller's answer, for one
// occurrence ("this", naming it) or for the whole event or series ("all").
type RsvpInput struct {
	Response   string `json:"response"`
	Comment    string `json:"comment"`
	Scope      string `json:"scope"`
	Occurrence string `json:"occurrence"`
}

// GuestsInput is the body of POST /events/:id/guests: people to invite and/or a
// group whose members are all invited.
type GuestsInput struct {
	Attendees       []string `json:"attendees"`
	InviteChannelID string   `json:"inviteChannelId"`
}

// BusySpan is a stretch of someone's time that is taken. The title is present only
// when the caller may read the event behind it.
type BusySpan struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Title string `json:"title,omitempty"`
}

// PersonFreeBusy is one person's busy time in a range, with their working hours.
type PersonFreeBusy struct {
	Person *PersonRef `json:"person"`
	Busy   []BusySpan `json:"busy"`
	// WorkStart and WorkEnd are HH:mm in the person's own zone.
	WorkStart string `json:"workStart"`
	WorkEnd   string `json:"workEnd"`
}

// TimeSlot is a stretch of time everyone asked about is free.
type TimeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// SettingsView is a person's calendar settings (also the body of PUT /settings).
type SettingsView struct {
	TZID            string `json:"tzid"`
	WorkStart       string `json:"workStart"`
	WorkEnd         string `json:"workEnd"`
	DefaultReminder int    `json:"defaultReminder"`
}
