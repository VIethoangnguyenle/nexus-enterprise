/**
 * Calendar query keys. Everything belongs to a workspace. Event lists are keyed by the range and the calendars
 * asked for; `eventsAll(wsId)` is the prefix that covers every range, for writes and realtime events that
 * cannot say which ranges they touch.
 */
export const calendarKeys = {
  all: (wsId: string) => ['calendar', wsId] as const,
  calendars: (wsId: string) => ['calendar', wsId, 'calendars'] as const,
  eventsAll: (wsId: string) => ['calendar', wsId, 'events'] as const,
  events: (wsId: string, from: string, to: string, calendarIds: readonly string[]) =>
    ['calendar', wsId, 'events', from, to, calendarIds] as const,
  event: (wsId: string, id: string) => ['calendar', wsId, 'event', id] as const,
  eventDetailsAll: (wsId: string) => ['calendar', wsId, 'event'] as const,
  search: (wsId: string, q: string) => ['calendar', wsId, 'search', q] as const,
  shares: (wsId: string, calId: string) => ['calendar', wsId, 'shares', calId] as const,
  freeBusy: (wsId: string, users: readonly string[], from: string, to: string) =>
    ['calendar', wsId, 'freebusy', users, from, to] as const,
  freeBusyAll: (wsId: string) => ['calendar', wsId, 'freebusy'] as const,
  suggest: (wsId: string, users: readonly string[], durationMin: number, day: string) =>
    ['calendar', wsId, 'suggest', users, durationMin, day] as const,
  settings: (wsId: string) => ['calendar', wsId, 'settings'] as const,
  /** Every calendar query of every workspace, for a resync after a reconnect. */
  everything: () => ['calendar'] as const,
}
