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
