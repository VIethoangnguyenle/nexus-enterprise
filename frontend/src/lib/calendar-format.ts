import {
  addDays, addMonths, differenceInCalendarDays, endOfMonth, format, formatISO, getISOWeek, isSameDay, isSameMonth,
  isSameYear, parseISO, startOfDay, startOfMonth, startOfWeek,
} from 'date-fns'
import type { Occurrence, RsvpResponse } from '../api/calendar'

/** The five ways to look at the calendar (DESIGN.md §6 Lịch). */
export const CALENDAR_VIEWS = ['day', '3day', 'week', 'month', 'agenda'] as const
export type CalendarView = (typeof CALENDAR_VIEWS)[number]

/** Monday first, as everywhere in the app. */
export const WEEK_OPTIONS = { weekStartsOn: 1 } as const

/** Names the view menu shows (D, W, M, A are the shortcuts). */
export const VIEW_LABEL: Record<CalendarView, string> = {
  day: 'Ngày',
  '3day': '3 ngày',
  week: 'Tuần',
  month: 'Tháng',
  agenda: 'Lịch biểu',
}

/** Days an agenda loads at a time, and so the span it asks the server for. */
export const AGENDA_DAYS = 30

const WEEKDAY_SHORT = ['CN', 'T2', 'T3', 'T4', 'T5', 'T6', 'T7'] as const
const WEEKDAY_LONG = ['Chủ nhật', 'Thứ Hai', 'Thứ Ba', 'Thứ Tư', 'Thứ Năm', 'Thứ Sáu', 'Thứ Bảy'] as const

export const weekdayShort = (d: Date) => WEEKDAY_SHORT[d.getDay()]!
export const weekdayLong = (d: Date) => WEEKDAY_LONG[d.getDay()]!

// --- Instants --------------------------------------------------------------

/** `yyyy-MM-dd` of the local day; the URL's `date` and the all-day wire format. */
export const toDateKey = (d: Date) => format(d, 'yyyy-MM-dd')

/** Local midnight of a `yyyy-MM-dd`; null when it is not a real date. */
export function parseDateKey(key: unknown): Date | null {
  if (typeof key !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(key)) return null
  const d = parseISO(key)
  return Number.isNaN(d.getTime()) || toDateKey(d) !== key ? null : d
}

/** RFC 3339 with the local offset, the wire format of an instant. */
export const toWire = (d: Date) => formatISO(d)

/** What the server needs for a start or end: a date for an all-day event, an instant otherwise. */
export const wireOf = (d: Date, allDay: boolean) => (allDay ? toDateKey(d) : toWire(d))

/** An occurrence's span as dates. An all-day `end` is exclusive and already the midnight after the last day. */
export function spanOf(o: Pick<Occurrence, 'start' | 'end'>): { start: Date; end: Date } {
  return { start: parseISO(o.start), end: parseISO(o.end) }
}

// --- Ranges ----------------------------------------------------------------

/** First day shown and the number of days, for the views that are a row of days. */
export function visibleDays(view: CalendarView, date: Date): Date[] {
  const day = startOfDay(date)
  if (view === 'day') return [day]
  if (view === '3day') return [0, 1, 2].map((n) => addDays(day, n))
  const first = startOfWeek(day, WEEK_OPTIONS)
  return [0, 1, 2, 3, 4, 5, 6].map((n) => addDays(first, n))
}

/** Weeks (Monday first) a month grid shows: as many rows as the month needs, at least five like the mockup. */
export function monthWeeks(date: Date): Date[][] {
  const first = startOfWeek(startOfMonth(date), WEEK_OPTIONS)
  const last = endOfMonth(date)
  const rows = Math.max(5, Math.ceil((differenceInCalendarDays(last, first) + 1) / 7))
  return Array.from({ length: rows }, (_, r) => Array.from({ length: 7 }, (_, c) => addDays(first, r * 7 + c)))
}

/** `[from, to)` the view needs events for. */
export function rangeOf(view: CalendarView, date: Date): { from: Date; to: Date } {
  if (view === 'month') {
    const weeks = monthWeeks(date)
    return { from: weeks[0]![0]!, to: addDays(weeks[weeks.length - 1]![6]!, 1) }
  }
  if (view === 'agenda') {
    const from = startOfDay(date)
    return { from, to: addDays(from, AGENDA_DAYS) }
  }
  const days = visibleDays(view, date)
  return { from: days[0]!, to: addDays(days[days.length - 1]!, 1) }
}

/** J and K, the arrows: the same view one step on. */
export function shiftDate(view: CalendarView, date: Date, steps: number): Date {
  switch (view) {
    case 'day': return addDays(date, steps)
    case '3day': return addDays(date, 3 * steps)
    case 'week': return addDays(date, 7 * steps)
    case 'month': return addMonths(date, steps)
    case 'agenda': return addDays(date, AGENDA_DAYS * steps)
  }
}

// --- Words -----------------------------------------------------------------

/** Heading of the top bar. */
export function titleOf(view: CalendarView, date: Date): string {
  if (view === 'month') return `Tháng ${format(date, 'M')} năm ${format(date, 'yyyy')}`
  if (view === 'day') return `${format(date, 'd')} tháng ${format(date, 'M')} năm ${format(date, 'yyyy')}`
  const { from, to } = rangeOf(view, date)
  const last = addDays(to, -1)
  if (isSameMonth(from, last)) return `Tháng ${format(from, 'M')} năm ${format(from, 'yyyy')}`
  if (isSameYear(from, last)) return `Tháng ${format(from, 'M')} – ${format(last, 'M')} năm ${format(last, 'yyyy')}`
  return `Tháng ${format(from, 'M')} năm ${format(from, 'yyyy')} – Tháng ${format(last, 'M')} năm ${format(last, 'yyyy')}`
}

/** "Tuần 42", the muted word beside the heading of the week view. */
export const weekLabel = (d: Date) => `Tuần ${getISOWeek(d)}`

/** 24-hour clock, `09:00`. */
export const formatClock = (d: Date) => format(d, 'HH:mm')

/** `GMT+7`, over the hour column. */
export function gmtLabel(d: Date): string {
  const offset = -d.getTimezoneOffset()
  const sign = offset < 0 ? '-' : '+'
  const hours = Math.floor(Math.abs(offset) / 60)
  const minutes = Math.abs(offset) % 60
  return `GMT${sign}${hours}${minutes ? `:${String(minutes).padStart(2, '0')}` : ''}`
}

/** "Thứ Tư, 14 tháng 10", the date line of a popover. */
export const formatDayLong = (d: Date) => `${weekdayLong(d)}, ${format(d, 'd')} tháng ${format(d, 'M')}`

/** "T4, TH 10", beside the day number of the agenda. */
export const formatDayShort = (d: Date) => `${weekdayShort(d)}, TH ${format(d, 'M')}`

/** "14 tháng 10", the name a "+ N nữa" button gives its day. */
export const formatDayMonth = (d: Date) => `${format(d, 'd')} tháng ${format(d, 'M')}`

/** When an event is: `Thứ Tư, 14 tháng 10 · 09:00 – 10:00`; all-day and multi-day events say so. */
export function formatWhen(start: Date, end: Date, allDay: boolean): string {
  if (allDay) {
    const lastDay = addDays(end, -1)
    return isSameDay(start, lastDay)
      ? formatDayLong(start)
      : `${formatDayLong(start)} – ${formatDayLong(lastDay)}`
  }
  if (isSameDay(start, end)) return `${formatDayLong(start)} · ${formatClock(start)} – ${formatClock(end)}`
  return `${formatDayLong(start)} ${formatClock(start)} – ${formatDayLong(end)} ${formatClock(end)}`
}

/** Short time of a block's second line: `09:00 – 10:00`. */
export const formatTimes = (start: Date, end: Date) => `${formatClock(start)} – ${formatClock(end)}`

const RESPONSE_WORD: Record<RsvpResponse, string> = {
  needs_action: 'chưa trả lời',
  accepted: 'đã nhận lời',
  declined: 'đã từ chối',
  tentative: 'chưa chắc tham gia',
}

/** The state of the caller's answer in words, for labels (colour is never the only carrier). */
export const responseWord = (r: RsvpResponse) => RESPONSE_WORD[r]

/** Name a screen reader gets for a block: "Phỏng vấn kế toán viên, 15:00 đến 16:00, chưa trả lời". */
export function eventAriaLabel(o: Occurrence, start: Date, end: Date): string {
  const when = o.allDay ? 'cả ngày' : `${formatClock(start)} đến ${formatClock(end)}`
  const parts = [o.title, when]
  if (o.myResponse && o.myResponse !== 'accepted' && !o.isOrganizer) parts.push(responseWord(o.myResponse))
  if (o.location && !o.busyOnly) parts.push(o.location)
  return parts.join(', ')
}

/** The toast after a drop: `Đã dời sang 10:00`, or the new day when it moved to another. */
export function movedToast(from: Date, to: Date, allDay: boolean): string {
  if (allDay || !isSameDay(from, to)) return `Đã dời sang ${formatDayLong(to)}${allDay ? '' : ` ${formatClock(to)}`}`
  return `Đã dời sang ${formatClock(to)}`
}

/** How a block is drawn; each is a state the design names (DESIGN.md §6, Khối sự kiện). */
export type BlockTone = 'solid' | 'past' | 'pending' | 'declined' | 'busy'

/** The look of an occurrence at `now`: details hidden, declined, unanswered, over, or plainly coming up. */
export function toneOf(o: Pick<Occurrence, 'busyOnly' | 'myResponse' | 'isOrganizer'>, end: Date, now: Date): BlockTone {
  if (o.busyOnly) return 'busy'
  if (o.myResponse === 'declined') return 'declined'
  if (o.myResponse === 'needs_action' && !o.isOrganizer) return 'pending'
  return end.getTime() < now.getTime() ? 'past' : 'solid'
}
