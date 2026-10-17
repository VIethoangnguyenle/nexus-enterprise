import { CALENDAR_VIEWS, parseDateKey, type CalendarView } from './calendar-format'
import { validateWorkspaceSearch } from './workspace'

/**
 * What the Lịch URL carries: the workspace, the view and the day in the middle of it. The view is `mode`, not
 * `view`: Tài liệu already has a `?view=` with other values, and the router types `prev` in a search reducer
 * as the union of every route's search, so two routes cannot give one key two types.
 */
export interface CalendarSearch {
  ws?: string
  /** Absent means the person's remembered view (Tuần the first time). */
  mode?: CalendarView
  /** `yyyy-MM-dd`; absent means today. */
  date?: string
}

/** `validateSearch` for `/calendar`: an unknown view or a date that is not one is dropped, not rejected. */
export function validateCalendarSearch(search: Record<string, unknown>): CalendarSearch {
  const out: CalendarSearch = { ...validateWorkspaceSearch(search) }
  if ((CALENDAR_VIEWS as readonly unknown[]).includes(search.mode)) out.mode = search.mode as CalendarView
  if (typeof search.date === 'string' && parseDateKey(search.date)) out.date = search.date
  return out
}

/** What `/calendar/new` carries: the span a drag or a click chose, a title already typed, or an event to copy. */
export interface NewEventSearch {
  ws?: string
  /** `yyyy-MM-dd` of the first day. */
  date?: string
  /** `HH:mm`. */
  start?: string
  end?: string
  allDay?: 1
  /** The calendar to put it in. */
  calendar?: string
  title?: string
  /** The event whose details are copied (Nhân bản). */
  copy?: string
}

const CLOCK = /^([01]\d|2[0-3]):[0-5]\d$/
const word = (v: unknown, max = 200) => (typeof v === 'string' && v.trim() && v.length <= max ? v : undefined)

export function validateNewEventSearch(search: Record<string, unknown>): NewEventSearch {
  const out: NewEventSearch = { ...validateWorkspaceSearch(search) }
  if (typeof search.date === 'string' && parseDateKey(search.date)) out.date = search.date
  if (typeof search.start === 'string' && CLOCK.test(search.start)) out.start = search.start
  if (typeof search.end === 'string' && CLOCK.test(search.end)) out.end = search.end
  if (search.allDay === 1 || search.allDay === '1' || search.allDay === true) out.allDay = 1
  const calendar = word(search.calendar, 80)
  if (calendar) out.calendar = calendar
  const title = word(search.title)
  if (title) out.title = title
  const copy = word(search.copy, 80)
  if (copy) out.copy = copy
  return out
}

/**
 * What `/calendar/event/:id` carries: the instance of a recurring event that was opened. `occurrence` goes
 * back to the server unchanged; `start` and `end` are what that instance shows, so the editor opens on its day.
 */
export interface EditEventSearch {
  ws?: string
  occurrence?: string
  start?: string
  end?: string
}

const WIRE = /^\d{4}-\d{2}-\d{2}(T[\d:.+\-Z]+)?$/
const wire = (v: unknown) => (typeof v === 'string' && WIRE.test(v) ? v : undefined)

export function validateEditEventSearch(search: Record<string, unknown>): EditEventSearch {
  const out: EditEventSearch = { ...validateWorkspaceSearch(search) }
  const occurrence = wire(search.occurrence)
  const start = wire(search.start)
  const end = wire(search.end)
  if (occurrence) out.occurrence = occurrence
  if (start && end) {
    out.start = start
    out.end = end
  }
  return out
}
