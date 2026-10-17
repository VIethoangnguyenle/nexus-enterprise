import { apiFetch } from './client'

/*
 * REST contract of the calendar service (base `/api/workspaces/:wsId/calendar`).
 *
 * Conventions the backend must follow:
 *  - JSON is camelCase; absent optional fields are omitted, never `null` (except `rrule`, see EventInput).
 *  - Instants (`start`, `end`, `occurrenceStart`, freebusy `start`/`end`) are RFC 3339 strings with an
 *    offset, e.g. `2026-10-14T09:00:00+07:00`. The client parses them with the browser's own time zone.
 *  - An ALL-DAY event or occurrence (`allDay: true`) carries calendar DATES instead: `start` and `end` are
 *    `yyyy-MM-dd`, `end` EXCLUSIVE (one-day event on 14 Oct: start `2026-10-14`, end `2026-10-15`). Dates
 *    do not move with the viewer's time zone. `occurrenceStart` of an all-day instance is also `yyyy-MM-dd`.
 *  - `occurrenceStart` is opaque to the client: it is sent back unchanged as `occurrence` to name one
 *    instance of a recurring event.
 *  - PATCH / DELETE of a recurring event take `scope` + `occurrence` (the `occurrenceStart` of the instance
 *    acted on). `this` changes only that instance. `all` and `following` (that instance and every later one)
 *    apply to the series; when `start`/`end` are sent with them they are the NEW position of the named
 *    instance and the server shifts the whole series (or its tail) by the same delta, in the event's tzid.
 *    The PATCH response is the EventDetail of the event that now holds the instance: for `following` that is
 *    the new tail event (a different `eventId`). A single event is changed without `scope`.
 *  - Errors: `{ message }` with 403 (no longer allowed), 404 (gone), 409 (stale `version`), 422 (invalid).
 */

// --- Shared ---------------------------------------------------------------

/** A person as the calendar shows them. `nodeId` is the NGAC node the server addresses people by; never rendered. */
export interface PersonRef {
  nodeId: string
  displayName: string
  avatarUrl?: string
  /** Person hue index 0..7, drawn as `--color-person-<hue + 1>`. */
  hue: number
}

export type CalendarKind = 'personal' | 'workspace' | 'channel'

/** What the caller may do on a calendar, lowest to highest. `freebusy` shows only busy time, no titles. */
export type CalendarAccess = 'freebusy' | 'read' | 'write' | 'manage'

/** One calendar the caller sees in the side list (their own, a shared one, a workspace or a group's). */
export interface CalendarSummary {
  id: string
  kind: CalendarKind
  name: string
  /** Person hue index 0..7, or -1 for the accent colour (workspace calendars by default). */
  color: number
  /** Whether its events are drawn on the grid (the caller's checkbox). */
  visible: boolean
  access: CalendarAccess
  /** The caller's own personal calendar. */
  isMine: boolean
  /** Owner of a personal calendar. */
  owner?: PersonRef
  /** The group of a `channel` calendar. */
  channel?: { id: string; name: string }
}

export type RsvpResponse = 'needs_action' | 'accepted' | 'declined' | 'tentative'
export type EventKind = 'event' | 'focus' | 'ooo'
export type Transparency = 'busy' | 'free'
export type Visibility = 'default' | 'private'
/** Which instances of a recurring event a change applies to. */
export type EditScope = 'this' | 'following' | 'all'

// --- Events ---------------------------------------------------------------

/** One instance of an event inside a requested range (a recurring event yields one per occurrence). */
export interface Occurrence {
  eventId: string
  /** Identifies the instance within a recurring event; echo it as `occurrence`. */
  occurrenceStart: string
  start: string
  end: string
  allDay: boolean
  calendarId: string
  /** `"Bận"` when `busyOnly`. */
  title: string
  location?: string
  /** The caller may not read the details: only start, end and the title `"Bận"` are real; no popover. */
  busyOnly: boolean
  /** The caller's own answer, when they are a guest or the organizer. */
  myResponse?: RsvpResponse
  isOrganizer: boolean
  hasCall: boolean
  isRecurring: boolean
  /** The rule in Vietnamese ("Hằng tuần vào thứ Tư"); the client never translates RRULE. */
  recurrenceText?: string
  /** The caller may edit, move or delete it. */
  canEdit: boolean
  kind: EventKind
  guestCount: number
  organizer?: PersonRef
  /** Concurrency token of the event; send it back as `version` on PATCH. */
  version: number
}

export interface Attendee {
  person: PersonRef
  response: RsvpResponse
  comment?: string
  isOrganizer: boolean
}

export interface Reminder {
  minutesBefore: number
}

/** The whole event behind an occurrence. */
export interface EventDetail extends Occurrence {
  description?: string
  /** IANA zone the event was made in; its recurrence is expanded there. */
  tzid: string
  /** RFC 5545 RRULE body, only for the editor; the screens show `recurrenceText`. */
  rrule?: string
  transparency: Transparency
  visibility: Visibility
  guestsCanInvite: boolean
  guestsCanSeeList: boolean
  /** Empty when the caller may not see the guest list. */
  attendees: Attendee[]
  reminders: Reminder[]
}

/** Body of `POST /events`; also the writable fields of `PATCH /events/:id` (all optional there). */
export interface EventInput {
  calendarId: string
  title: string
  description?: string
  location?: string
  start: string
  end: string
  allDay: boolean
  tzid: string
  /** `null` on PATCH removes the recurrence; omitted keeps it. */
  rrule?: string | null
  kind: EventKind
  transparency: Transparency
  visibility: Visibility
  /** NGAC node ids of the guests (the person picker yields them). */
  attendees: string[]
  /** Invite every member of this group chat. */
  inviteChannelId?: string
  guestsCanInvite: boolean
  guestsCanSeeList: boolean
  hasCall: boolean
  /** Omitted: the server applies the creator's default reminder. An empty list: no reminder. */
  reminders?: Reminder[]
}

export type EventPatch = Partial<EventInput> & {
  /** The `version` the edit started from; a mismatch answers 409. */
  version: number
}

export interface RsvpInput {
  response: Exclude<RsvpResponse, 'needs_action'>
  comment?: string
  /** `this` answers only `occurrence`; `all` answers the whole series. */
  scope: 'this' | 'all'
  occurrence?: string
}

// --- Subscriptions and sharing ---------------------------------------------

export interface SubscriptionPatch {
  /** Person hue index 0..7, or -1 for the accent colour. */
  color?: number
  visible?: boolean
}

export type ShareLevel = 'view' | 'edit' | 'manage'

export interface Share {
  grantee: PersonRef
  level: ShareLevel
}

// --- Free/busy, search, settings --------------------------------------------

export interface BusySpan {
  start: string
  end: string
  title?: string
}

export interface PersonFreeBusy {
  person: PersonRef
  busy: BusySpan[]
  /** Working hours, `HH:mm` in the person's own zone. */
  workStart: string
  workEnd: string
}

export interface TimeSlot {
  start: string
  end: string
}

export interface CalendarSettings {
  tzid: string
  workStart: string
  workEnd: string
  /** Minutes before a new event; 0 means none. */
  defaultReminder: number
}

// --- Requests ---------------------------------------------------------------

/** Query string from the parameters that are set. */
function query(params: Record<string, string | number | undefined>): string {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '') continue
    qs.set(k, String(v))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

const json = (method: string, body: unknown): RequestInit => ({ method, body: JSON.stringify(body) })

/** `scope` and `occurrence` of a change to a recurring event; both absent for a single event. */
export interface ScopeQuery {
  scope?: EditScope
  occurrence?: string
}

export const calendarApi = {
  // Calendars and subscriptions
  listCalendars: (ws: string) => apiFetch<{ calendars: CalendarSummary[] }>(`/workspaces/${ws}/calendar/calendars`),
  updateSubscription: (ws: string, calId: string, patch: SubscriptionPatch) =>
    apiFetch<CalendarSummary>(`/workspaces/${ws}/calendar/calendars/${calId}/subscription`, json('PUT', patch)),
  /** Add the personal calendar of another person to the list. */
  subscribe: (ws: string, userNodeId: string) =>
    apiFetch<CalendarSummary>(`/workspaces/${ws}/calendar/subscriptions`, json('POST', { userNodeId })),
  unsubscribe: (ws: string, calId: string) =>
    apiFetch<Record<string, never>>(`/workspaces/${ws}/calendar/subscriptions/${calId}`, { method: 'DELETE' }),

  // Sharing
  listShares: (ws: string, calId: string) =>
    apiFetch<{ shares: Share[] }>(`/workspaces/${ws}/calendar/calendars/${calId}/shares`),
  setShare: (ws: string, calId: string, granteeNodeId: string, level: ShareLevel) =>
    apiFetch<Share>(`/workspaces/${ws}/calendar/calendars/${calId}/shares/${granteeNodeId}`, json('PUT', { level })),
  removeShare: (ws: string, calId: string, granteeNodeId: string) =>
    apiFetch<Record<string, never>>(`/workspaces/${ws}/calendar/calendars/${calId}/shares/${granteeNodeId}`, { method: 'DELETE' }),

  // Events
  /** Occurrences that touch `[from, to)`, of the named calendars (all the caller's visible ones when omitted). */
  listEvents: (ws: string, from: string, to: string, calendarIds?: string[]) =>
    apiFetch<{ occurrences: Occurrence[] }>(
      `/workspaces/${ws}/calendar/events${query({ from, to, calendars: calendarIds?.join(',') })}`,
    ),
  getEvent: (ws: string, id: string) => apiFetch<EventDetail>(`/workspaces/${ws}/calendar/events/${id}`),
  createEvent: (ws: string, input: EventInput) =>
    apiFetch<EventDetail>(`/workspaces/${ws}/calendar/events`, json('POST', input)),
  updateEvent: (ws: string, id: string, patch: EventPatch, scope: ScopeQuery = {}) =>
    apiFetch<EventDetail>(`/workspaces/${ws}/calendar/events/${id}${query({ ...scope })}`, json('PATCH', patch)),
  deleteEvent: (ws: string, id: string, scope: ScopeQuery = {}) =>
    apiFetch<Record<string, never>>(`/workspaces/${ws}/calendar/events/${id}${query({ ...scope })}`, { method: 'DELETE' }),
  rsvp: (ws: string, id: string, input: RsvpInput) =>
    apiFetch<EventDetail>(`/workspaces/${ws}/calendar/events/${id}/rsvp`, json('POST', input)),
  searchEvents: (ws: string, q: string) =>
    apiFetch<{ occurrences: Occurrence[] }>(`/workspaces/${ws}/calendar/events/search${query({ q })}`),

  // Free/busy
  freeBusy: (ws: string, users: string[], from: string, to: string) =>
    apiFetch<{ people: PersonFreeBusy[] }>(
      `/workspaces/${ws}/calendar/freebusy${query({ users: users.join(','), from, to })}`,
    ),
  suggestSlots: (ws: string, users: string[], durationMin: number, day: string) =>
    apiFetch<{ slots: TimeSlot[] }>(
      `/workspaces/${ws}/calendar/freebusy/suggest${query({ users: users.join(','), durationMin, day })}`,
    ),

  // Settings
  getSettings: (ws: string) => apiFetch<CalendarSettings>(`/workspaces/${ws}/calendar/settings`),
  updateSettings: (ws: string, s: CalendarSettings) =>
    apiFetch<CalendarSettings>(`/workspaces/${ws}/calendar/settings`, json('PUT', s)),
}
