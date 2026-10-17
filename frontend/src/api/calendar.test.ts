import { beforeEach, describe, expect, it, vi } from 'vitest'

import { calendarApi } from './calendar'

/** Pins the paths, query parameters and body field names the calendar service is built to. */
const apiFetch = vi.fn()
vi.mock('./client', () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }))

beforeEach(() => apiFetch.mockReset().mockResolvedValue({}))

const lastCall = () => {
  const [url, init] = apiFetch.mock.calls[apiFetch.mock.calls.length - 1] as [string, RequestInit | undefined]
  const u = new URL(url, 'http://x')
  return {
    path: u.pathname,
    params: Object.fromEntries(u.searchParams),
    method: init?.method ?? 'GET',
    body: init?.body ? JSON.parse(String(init.body)) : undefined,
  }
}

describe('calendar queries', () => {
  it('lists the calendars of a workspace', async () => {
    await calendarApi.listCalendars('w1')
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/calendars', method: 'GET' })
  })

  it('lists occurrences of a range, naming the calendars only when given', async () => {
    await calendarApi.listEvents('w1', '2026-10-12T00:00:00+07:00', '2026-10-19T00:00:00+07:00', ['a', 'b'])
    expect(lastCall()).toMatchObject({
      path: '/workspaces/w1/calendar/events',
      params: { from: '2026-10-12T00:00:00+07:00', to: '2026-10-19T00:00:00+07:00', calendars: 'a,b' },
    })
    await calendarApi.listEvents('w1', 'f', 't')
    expect(lastCall().params).toEqual({ from: 'f', to: 't' })
  })

  it('reads one event and searches', async () => {
    await calendarApi.getEvent('w1', 'e1')
    expect(lastCall().path).toBe('/workspaces/w1/calendar/events/e1')
    await calendarApi.searchEvents('w1', 'họp nhóm')
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/events/search', params: { q: 'họp nhóm' } })
  })

  it('asks free/busy and suggestions for a list of people', async () => {
    await calendarApi.freeBusy('w1', ['u1', 'u2'], 'f', 't')
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/freebusy', params: { users: 'u1,u2', from: 'f', to: 't' } })
    await calendarApi.suggestSlots('w1', ['u1'], 30, '2026-10-14')
    expect(lastCall()).toMatchObject({
      path: '/workspaces/w1/calendar/freebusy/suggest',
      params: { users: 'u1', durationMin: '30', day: '2026-10-14' },
    })
  })
})

describe('calendar writes', () => {
  it('moves one occurrence with the scope, the instance and the version it started from', async () => {
    await calendarApi.updateEvent('w1', 'e1', { start: 's', end: 'e', version: 4 }, { scope: 'this', occurrence: '2026-10-14T09:00:00+07:00' })
    expect(lastCall()).toMatchObject({
      path: '/workspaces/w1/calendar/events/e1',
      method: 'PATCH',
      params: { scope: 'this', occurrence: '2026-10-14T09:00:00+07:00' },
      body: { start: 's', end: 'e', version: 4 },
    })
  })

  it('sends no scope for a single event', async () => {
    await calendarApi.updateEvent('w1', 'e1', { title: 'x', version: 1 })
    expect(lastCall().params).toEqual({})
    await calendarApi.deleteEvent('w1', 'e1')
    expect(lastCall()).toMatchObject({ method: 'DELETE', params: {} })
    await calendarApi.deleteEvent('w1', 'e1', { scope: 'following', occurrence: 'o' })
    expect(lastCall().params).toEqual({ scope: 'following', occurrence: 'o' })
  })

  it('creates an event from the input as it is', async () => {
    const input = {
      calendarId: 'c1', title: 'Họp', start: 's', end: 'e', allDay: false, tzid: 'Asia/Ho_Chi_Minh', kind: 'event' as const,
      transparency: 'busy' as const, visibility: 'default' as const, attendees: [], guestsCanInvite: false,
      guestsCanSeeList: true, hasCall: false, reminders: [{ minutesBefore: 10 }],
    }
    await calendarApi.createEvent('w1', input)
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/events', method: 'POST', body: input })
  })

  it('answers an invitation', async () => {
    await calendarApi.rsvp('w1', 'e1', { response: 'accepted', scope: 'this', occurrence: 'o' })
    expect(lastCall()).toMatchObject({
      path: '/workspaces/w1/calendar/events/e1/rsvp', method: 'POST', body: { response: 'accepted', scope: 'this', occurrence: 'o' },
    })
  })

  it('changes a subscription and subscribes to a person', async () => {
    await calendarApi.updateSubscription('w1', 'c1', { color: 3, visible: false })
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/calendars/c1/subscription', method: 'PUT', body: { color: 3, visible: false } })
    await calendarApi.subscribe('w1', 'n1')
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/subscriptions', method: 'POST', body: { userNodeId: 'n1' } })
    await calendarApi.unsubscribe('w1', 'c1')
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/subscriptions/c1', method: 'DELETE' })
  })

  it('shares a calendar by level', async () => {
    await calendarApi.setShare('w1', 'c1', 'n1', 'edit')
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/calendars/c1/shares/n1', method: 'PUT', body: { level: 'edit' } })
    await calendarApi.removeShare('w1', 'c1', 'n1')
    expect(lastCall().method).toBe('DELETE')
    await calendarApi.listShares('w1', 'c1')
    expect(lastCall().path).toBe('/workspaces/w1/calendar/calendars/c1/shares')
  })

  it('reads and writes the settings', async () => {
    await calendarApi.updateSettings('w1', { tzid: 'Asia/Ho_Chi_Minh', workStart: '09:00', workEnd: '18:00', defaultReminder: 10 })
    expect(lastCall()).toMatchObject({ path: '/workspaces/w1/calendar/settings', method: 'PUT' })
    await calendarApi.getSettings('w1')
    expect(lastCall().method).toBe('GET')
  })
})
