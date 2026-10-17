import { addDays, addMinutes, differenceInMinutes, format, parseISO, startOfDay } from 'date-fns'
import type {
  EventDetail, EventInput, EventKind, EventPatch, PersonRef, Transparency, Visibility,
} from '../api/calendar'
import { parseDateKey, toDateKey, wireOf } from './calendar-format'

/** A guest as the editor keeps them: enough to draw them, and the node the server wants. */
export interface GuestPick {
  nodeId: string
  displayName: string
  avatarUrl?: string
  hue: number
}

export const guestOf = (p: PersonRef): GuestPick => ({ nodeId: p.nodeId, displayName: p.displayName, avatarUrl: p.avatarUrl, hue: p.hue })

/** What the editor edits. Dates and times are the strings the date and time inputs speak. */
export interface EditorForm {
  title: string
  calendarId: string
  kind: EventKind
  allDay: boolean
  startDate: string
  startTime: string
  /** For an all-day event the LAST day (inclusive), as people say it; the wire end is the day after. */
  endDate: string
  endTime: string
  tzid: string
  rrule: string | null
  hasCall: boolean
  location: string
  description: string
  /** Minutes before the start; `null` leaves the choice to the server's default for the creator. */
  reminders: number[] | null
  transparency: Transparency
  visibility: Visibility
  guests: GuestPick[]
  /** A whole group chat invited at once. */
  inviteChannelId: string | null
  inviteChannelName: string | null
  guestsCanInvite: boolean
  guestsCanSeeList: boolean
}

const HHMM = (d: Date) => format(d, 'HH:mm')
const DATE = (d: Date) => format(d, 'yyyy-MM-dd')

export const browserZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Ho_Chi_Minh'

/** The next whole hour after `now`, an hour long, on `day`'s date when it is not today. */
export function defaultSpan(day: Date, now: Date): { start: Date; end: Date } {
  const sameDay = toDateKey(day) === toDateKey(now)
  const start = new Date(sameDay ? now : day)
  if (sameDay) start.setHours(now.getHours() + 1, 0, 0, 0)
  else start.setHours(9, 0, 0, 0)
  if (startOfDay(start).getTime() !== startOfDay(day).getTime()) start.setHours(23, 0, 0, 0)
  return { start, end: addMinutes(start, 60) }
}

/** An empty form for a new event on the span, in the given calendar. */
export function blankForm(o: { start: Date; end: Date; allDay: boolean; calendarId: string; title?: string }): EditorForm {
  const lastDay = o.allDay ? addDays(o.end, -1) : o.end
  return {
    title: o.title ?? '',
    calendarId: o.calendarId,
    kind: 'event',
    allDay: o.allDay,
    startDate: DATE(o.start),
    startTime: o.allDay ? '09:00' : HHMM(o.start),
    endDate: DATE(o.allDay && lastDay < o.start ? o.start : lastDay),
    endTime: o.allDay ? '10:00' : HHMM(o.end),
    tzid: browserZone(),
    rrule: null,
    hasCall: false,
    location: '',
    description: '',
    reminders: null,
    transparency: 'busy',
    visibility: 'default',
    guests: [],
    inviteChannelId: null,
    inviteChannelName: null,
    guestsCanInvite: false,
    guestsCanSeeList: true,
  }
}

/**
 * The form of an existing event. `at` is the instance being edited (a recurring event's detail describes the
 * series, the screen the person came from knows which day they meant); it replaces the span of the detail.
 */
export function formFromDetail(d: EventDetail, at?: { start: string; end: string }): EditorForm {
  const startRaw = at?.start ?? d.start
  const endRaw = at?.end ?? d.end
  const start = parseISO(startRaw)
  const end = parseISO(endRaw)
  const lastDay = d.allDay ? addDays(end, -1) : end
  return {
    title: d.title,
    calendarId: d.calendarId,
    kind: d.kind,
    allDay: d.allDay,
    startDate: DATE(start),
    startTime: d.allDay ? '09:00' : HHMM(start),
    endDate: DATE(lastDay < start ? start : lastDay),
    endTime: d.allDay ? '10:00' : HHMM(end),
    tzid: d.tzid,
    rrule: d.rrule ?? null,
    hasCall: d.hasCall,
    location: d.location ?? '',
    description: d.description ?? '',
    reminders: d.reminders.map((r) => r.minutesBefore),
    transparency: d.transparency,
    visibility: d.visibility,
    guests: d.attendees.filter((a) => !a.isOrganizer).map((a) => guestOf(a.person)),
    inviteChannelId: null,
    inviteChannelName: null,
    guestsCanInvite: d.guestsCanInvite,
    guestsCanSeeList: d.guestsCanSeeList,
  }
}

/** Local date and time of the form as `Date`s; an all-day event runs from midnight to the midnight after its last day. */
export function spanOfForm(f: EditorForm): { start: Date; end: Date } | null {
  const sd = parseDateKey(f.startDate)
  const ed = parseDateKey(f.endDate)
  if (!sd || !ed) return null
  if (f.allDay) return { start: sd, end: addDays(ed, 1) }
  const at = (day: Date, time: string) => {
    const m = time.match(/^(\d{2}):(\d{2})$/)
    if (!m) return null
    const d = new Date(day)
    d.setHours(Number(m[1]), Number(m[2]), 0, 0)
    return d
  }
  const start = at(sd, f.startTime)
  const end = at(ed, f.endTime)
  return start && end ? { start, end } : null
}

export interface FormErrors {
  start?: string
  end?: string
}

export const END_BEFORE_START = 'Giờ kết thúc phải sau giờ bắt đầu'

/** What is wrong with the times; empty when the form can be saved. */
export function validateForm(f: EditorForm): FormErrors {
  const span = spanOfForm(f)
  if (!span) return { start: 'Chọn ngày và giờ' }
  if (f.allDay) return span.end <= span.start ? { end: 'Ngày kết thúc phải từ ngày bắt đầu trở đi' } : {}
  return span.end <= span.start ? { end: END_BEFORE_START } : {}
}

/** Moves the start and keeps the length, as the design asks of the start time. */
export function withStart(f: EditorForm, patch: { startDate?: string; startTime?: string }): EditorForm {
  const before = spanOfForm(f)
  const next = { ...f, ...patch }
  const after = spanOfForm(next)
  if (!before || !after || f.allDay) return next
  const end = new Date(after.start.getTime() + (before.end.getTime() - before.start.getTime()))
  return { ...next, endDate: DATE(end), endTime: HHMM(end) }
}

/** Sets both ends from instants (a chip of Tìm giờ, a drag of its band). */
export function withSpan(f: EditorForm, start: Date, end: Date): EditorForm {
  return { ...f, allDay: false, startDate: DATE(start), startTime: HHMM(start), endDate: DATE(end), endTime: HHMM(end) }
}

/** "1 giờ", "1 giờ 30 phút", "45 phút", "2 ngày": the length beside the end time. */
export function durationLabel(minutes: number): string {
  if (minutes <= 0) return ''
  const days = Math.floor(minutes / 1440)
  if (days >= 1 && minutes % 1440 === 0) return `${days} ngày`
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  if (h === 0) return `${m} phút`
  return m === 0 ? `${h} giờ` : `${h} giờ ${m} phút`
}

export function formDuration(f: EditorForm): number {
  const span = spanOfForm(f)
  return span ? differenceInMinutes(span.end, span.start) : 0
}

// --- Reminders ---------------------------------------------------------------------

export type ReminderUnit = 'minute' | 'hour' | 'day' | 'week'
export const REMINDER_UNIT_LABEL: Record<ReminderUnit, string> = { minute: 'phút', hour: 'giờ', day: 'ngày', week: 'tuần' }
const UNIT_MINUTES: Record<ReminderUnit, number> = { minute: 1, hour: 60, day: 1440, week: 10080 }

/** 90 → 90 phút; 120 → 2 giờ: the largest unit that divides it. */
export function splitMinutes(minutes: number): { n: number; unit: ReminderUnit } {
  for (const unit of ['week', 'day', 'hour'] as const) {
    if (minutes >= UNIT_MINUTES[unit] && minutes % UNIT_MINUTES[unit] === 0) return { n: minutes / UNIT_MINUTES[unit], unit }
  }
  return { n: minutes, unit: 'minute' }
}
export const joinMinutes = (n: number, unit: ReminderUnit) => Math.max(0, Math.round(n)) * UNIT_MINUTES[unit]

/** The reminders the form shows: its own, or the person's default while it has not been touched. */
export function remindersShown(f: EditorForm, defaultReminder: number): number[] {
  return f.reminders ?? (defaultReminder > 0 ? [defaultReminder] : [])
}

// --- To the wire ---------------------------------------------------------------------

/** Body of `POST /events`. */
export function toInput(f: EditorForm): EventInput | null {
  const span = spanOfForm(f)
  if (!span) return null
  return {
    calendarId: f.calendarId,
    title: f.title.trim() || '(Không có tiêu đề)',
    ...(f.description.trim() ? { description: f.description } : {}),
    ...(f.location.trim() ? { location: f.location.trim() } : {}),
    start: wireOf(span.start, f.allDay),
    end: wireOf(span.end, f.allDay),
    allDay: f.allDay,
    tzid: f.tzid,
    ...(f.rrule ? { rrule: f.rrule } : {}),
    kind: f.kind,
    transparency: f.transparency,
    visibility: f.visibility,
    attendees: f.guests.map((g) => g.nodeId),
    ...(f.inviteChannelId ? { inviteChannelId: f.inviteChannelId } : {}),
    guestsCanInvite: f.guestsCanInvite,
    guestsCanSeeList: f.guestsCanSeeList,
    hasCall: f.hasCall,
    ...(f.reminders ? { reminders: f.reminders.map((minutesBefore) => ({ minutesBefore })) } : {}),
  }
}

const sameList = (a: readonly unknown[] | null, b: readonly unknown[] | null) => JSON.stringify(a) === JSON.stringify(b)

/** Fields whose change tells the guests (a change of reminders or colour tells nobody). */
const NOTIFYING: (keyof EventPatch)[] = [
  'title', 'description', 'location', 'start', 'end', 'allDay', 'rrule', 'attendees', 'inviteChannelId', 'hasCall',
]

/** Only what differs from the event as loaded, so an untouched time is never moved and nothing is re-sent. */
export function toPatch(f: EditorForm, from: EditorForm, version: number): EventPatch | null {
  const now = toInput(f)
  const before = toInput(from)
  if (!now || !before) return null
  const patch: EventPatch = { version }
  const put = <K extends keyof EventInput>(key: K, a: EventInput[K], b: EventInput[K]) => {
    if (JSON.stringify(a) !== JSON.stringify(b)) (patch as Record<string, unknown>)[key] = a
  }
  put('title', now.title, before.title)
  put('description', f.description.trim() ? f.description : '', from.description.trim() ? from.description : '')
  put('location', f.location.trim(), from.location.trim())
  if (now.start !== before.start || now.end !== before.end || now.allDay !== before.allDay) {
    patch.start = now.start
    patch.end = now.end
    patch.allDay = now.allDay
  }
  if (f.tzid !== from.tzid) patch.tzid = f.tzid
  if (f.rrule !== from.rrule) patch.rrule = f.rrule
  if (f.calendarId !== from.calendarId) patch.calendarId = f.calendarId
  put('kind', now.kind, before.kind)
  put('transparency', now.transparency, before.transparency)
  put('visibility', now.visibility, before.visibility)
  if (!sameList(f.guests.map((g) => g.nodeId), from.guests.map((g) => g.nodeId))) patch.attendees = now.attendees
  if (f.inviteChannelId && f.inviteChannelId !== from.inviteChannelId) patch.inviteChannelId = f.inviteChannelId
  put('guestsCanInvite', now.guestsCanInvite, before.guestsCanInvite)
  put('guestsCanSeeList', now.guestsCanSeeList, before.guestsCanSeeList)
  put('hasCall', now.hasCall, before.hasCall)
  if (!sameList(f.reminders, from.reminders)) patch.reminders = (f.reminders ?? []).map((minutesBefore) => ({ minutesBefore }))
  return patch
}

/** Whether a patch has anything beyond its `version`. */
export const hasChanges = (p: EventPatch) => Object.keys(p).some((k) => k !== 'version')

/** Whether saving this patch should tell the guests. */
export const notifiesGuests = (p: EventPatch) => NOTIFYING.some((k) => k in p)

/** Whether the form differs from how it started. */
export const isDirty = (f: EditorForm, from: EditorForm) => JSON.stringify(f) !== JSON.stringify(from)
