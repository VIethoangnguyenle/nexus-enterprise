/**
 * Fixture data and a fake REST layer for the calendar. Week of Monday 12 October 2026; "now" is
 * Wednesday 14 October 10:40 (the mockup's clock). Times are built from local dates so the suite does not
 * depend on the machine's zone. Every id is a real-looking UUID so tests can assert none reaches the screen.
 */
import { formatISO } from 'date-fns'
import type { CalendarSummary, EventDetail, Occurrence, PersonRef } from '../api/calendar'
import { U, WS_ID } from './chat-fixtures'

export { UUID_RE, WS_ID } from './chat-fixtures'

export const NOW = new Date(2026, 9, 14, 10, 40)

const at = (day: number, h: number, m = 0) => formatISO(new Date(2026, 9, day, h, m))
const dateKey = (day: number) => `2026-10-${String(day).padStart(2, '0')}`

export const CAL = {
  mine: 'c0000000-aaaa-4bbb-8ccc-000000000001',
  ws: 'c0000000-aaaa-4bbb-8ccc-000000000002',
  vinh: 'c0000000-aaaa-4bbb-8ccc-000000000003',
}

const person = (key: keyof typeof U, name: string, hue: number): PersonRef => ({ nodeId: U[key].node, displayName: name, hue })
export const PEOPLE = {
  hoa: person('hoa', 'Lê Thị Hoa', 5),
  yen: person('yen', 'Phạm Hải Yến', 2),
  lan: person('lan', 'Trần Ngọc Lan', 3),
}

export const CALENDARS: CalendarSummary[] = [
  { id: CAL.mine, kind: 'personal', name: 'Lê Thị Hoa', color: 5, visible: true, access: 'manage', isMine: true, owner: PEOPLE.hoa },
  { id: CAL.ws, kind: 'workspace', name: 'Khối Vận hành', color: -1, visible: true, access: 'read', isMine: false },
  { id: CAL.vinh, kind: 'personal', name: 'Lê Quang Vinh', color: 0, visible: true, access: 'read', isMine: false },
]

export const E = {
  standup: 'e0000000-aaaa-4bbb-8ccc-000000000001',
  giaoBan: 'e0000000-aaaa-4bbb-8ccc-000000000002',
  review: 'e0000000-aaaa-4bbb-8ccc-000000000003',
  interview: 'e0000000-aaaa-4bbb-8ccc-000000000004',
  report: 'e0000000-aaaa-4bbb-8ccc-000000000005',
  leave: 'e0000000-aaaa-4bbb-8ccc-000000000006',
  busy: 'e0000000-aaaa-4bbb-8ccc-000000000007',
  a: 'e0000000-aaaa-4bbb-8ccc-000000000008',
  b: 'e0000000-aaaa-4bbb-8ccc-000000000009',
  c: 'e0000000-aaaa-4bbb-8ccc-00000000000a',
}

const base = {
  allDay: false, busyOnly: false, isOrganizer: false, hasCall: false, isRecurring: false, canEdit: true,
  kind: 'event' as const, guestCount: 0, version: 3,
}
const occ = (eventId: string, title: string, start: string, end: string, over: Partial<Occurrence> = {}): Occurrence => ({
  ...base, eventId, occurrenceStart: start, start, end, calendarId: CAL.mine, title, ...over,
})

export const OCCURRENCES: Occurrence[] = [
  occ(E.leave, 'Nghỉ phép', dateKey(12), dateKey(14), { allDay: true }),
  occ(E.busy, 'Bận', at(13, 13), at(13, 14), { calendarId: CAL.vinh, busyOnly: true, canEdit: false }),
  occ(E.giaoBan, 'Họp giao ban', at(14, 9), at(14, 10), { isOrganizer: true, myResponse: 'accepted', guestCount: 4, hasCall: true }),
  occ(E.review, 'Review A', at(14, 9, 30), at(14, 10, 30)),
  occ(E.interview, 'Phỏng vấn kế toán viên', at(14, 15), at(14, 16), {
    calendarId: CAL.ws, myResponse: 'needs_action', guestCount: 3, canEdit: false, organizer: PEOPLE.yen, location: 'Phòng họp 3',
  }),
  occ(E.report, 'Báo cáo tuần', at(15, 14), at(15, 15)),
  occ(E.a, 'Việc A', at(15, 8), at(15, 9)),
  occ(E.b, 'Việc B', at(15, 11), at(15, 12)),
  occ(E.c, 'Việc C', at(15, 16), at(15, 17)),
  occ(E.standup, 'Standup', at(16, 9), at(16, 9, 15), { isRecurring: true, recurrenceText: 'Hằng tuần vào thứ Sáu' }),
]

export const DETAIL_ATTENDEES = [
  { person: PEOPLE.yen, response: 'accepted' as const, isOrganizer: true },
  { person: PEOPLE.hoa, response: 'needs_action' as const, isOrganizer: false },
  { person: PEOPLE.lan, response: 'declined' as const, isOrganizer: false },
]

export interface ApiCall { method: string; path: string; query: URLSearchParams; body?: Record<string, unknown> }
export const calls: ApiCall[] = []

export interface Mode {
  calendarsError: boolean
  eventsError: boolean
  noEvents: boolean
  /** Status the next write answers with, once. */
  writeStatus: number | null
}
const DEFAULT_MODE: Mode = { calendarsError: false, eventsError: false, noEvents: false, writeStatus: null }
export const mode: Mode = { ...DEFAULT_MODE }

/** The fake server's copy of the occurrences; PATCH and DELETE change it. */
let store: Occurrence[] = []
let calendars: CalendarSummary[] = []
export function resetFixtures() {
  Object.assign(mode, DEFAULT_MODE)
  calls.length = 0
  store = structuredClone(OCCURRENCES)
  calendars = structuredClone(CALENDARS)
}
resetFixtures()

const detailOf = (o: Occurrence): EventDetail => ({
  ...o, tzid: 'Asia/Ho_Chi_Minh', transparency: 'busy', visibility: 'default', guestsCanInvite: false, guestsCanSeeList: true,
  attendees: o.guestCount > 0 ? DETAIL_ATTENDEES : [], reminders: [{ minutesBefore: 10 }],
})

export function calendarFixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const [p, qs = ''] = path.split('?') as [string, string?]
  const query = new URLSearchParams(qs)
  const body = typeof init?.body === 'string' ? (JSON.parse(init.body) as Record<string, unknown>) : undefined
  calls.push({ method, path: p, query, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))
  // Loaded when needed: the screen test mocks the client module with a factory that imports this file.
  const fail = async (status: number): Promise<never> => {
    const { ApiError } = await import('../api/client')
    throw new ApiError('fixture refusal', status)
  }

  if (p === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
  if (p === `/workspaces/${WS_ID}/contacts`) return ok({ contacts: [], total: 0 })

  const root = `/workspaces/${WS_ID}/calendar`
  if (!p.startsWith(root)) return Promise.reject(new Error(`unexpected ${method} ${path}`))
  const rest = p.slice(root.length)

  if (method !== 'GET' && mode.writeStatus) {
    const status = mode.writeStatus
    mode.writeStatus = null
    return fail(status)
  }

  if (rest === '/calendars' && method === 'GET') {
    return mode.calendarsError ? fail(500) : ok({ calendars })
  }
  const sub = rest.match(/^\/calendars\/([^/]+)\/subscription$/)
  if (sub && method === 'PUT') {
    const target = calendars.find((c) => c.id === sub[1])!
    Object.assign(target, body)
    return ok(target)
  }

  if (rest === '/events' && method === 'GET') {
    if (mode.eventsError) return fail(500)
    // Only the calendars asked for, as the server does.
    const asked = query.get('calendars')?.split(',')
    return ok({ occurrences: mode.noEvents ? [] : store.filter((o) => !asked || asked.includes(o.calendarId)) })
  }
  if (rest === '/events' && method === 'POST') {
    const created = occ('e0000000-aaaa-4bbb-8ccc-0000000000ff', String(body?.title), String(body?.start), String(body?.end), {
      allDay: body?.allDay === true, calendarId: String(body?.calendarId),
    })
    store = [...store, created]
    return ok(detailOf(created))
  }
  const rsvp = rest.match(/^\/events\/([^/]+)\/rsvp$/)
  if (rsvp && method === 'POST') {
    const target = store.find((o) => o.eventId === rsvp[1])!
    target.myResponse = body?.response as Occurrence['myResponse']
    return ok(detailOf(target))
  }
  const one = rest.match(/^\/events\/([^/]+)$/)
  if (one) {
    const target = store.find((o) => o.eventId === one[1])
    if (!target) return fail(404)
    if (method === 'GET') return ok(detailOf(target))
    if (method === 'PATCH') {
      Object.assign(target, { start: body?.start, end: body?.end }, { version: target.version + 1 })
      return ok(detailOf(target))
    }
    if (method === 'DELETE') {
      store = store.filter((o) => o !== target)
      return ok({})
    }
  }
  return Promise.reject(new Error(`unexpected ${method} ${path}`))
}
