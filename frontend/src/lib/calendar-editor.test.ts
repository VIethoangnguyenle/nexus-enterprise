import { describe, expect, it } from 'vitest'
import type { EventDetail } from '../api/calendar'
import {
  blankForm, durationLabel, formFromDetail, hasChanges, isDirty, joinMinutes, notifiesGuests, remindersShown, spanOfForm,
  splitMinutes, toInput, toPatch, validateForm, withStart,
} from './calendar-editor'

const at = (h: number, m = 0) => new Date(2026, 9, 14, h, m)
const form = (over = {}) => ({ ...blankForm({ start: at(9), end: at(10), allDay: false, calendarId: 'cal-1' }), ...over })

describe('validation', () => {
  it('lets an event end after it starts', () => {
    expect(validateForm(form())).toEqual({})
  })

  it('refuses an end that is not after the start', () => {
    expect(validateForm(form({ endTime: '09:00' }))).toEqual({ end: 'Giờ kết thúc phải sau giờ bắt đầu' })
    expect(validateForm(form({ endTime: '08:00' })).end).toBeDefined()
    expect(validateForm(form({ endDate: '2026-10-15', endTime: '08:00' }))).toEqual({})
  })

  it('checks an all-day event by days', () => {
    expect(validateForm(form({ allDay: true, endDate: '2026-10-14' }))).toEqual({})
    expect(validateForm(form({ allDay: true, endDate: '2026-10-13' })).end).toBeDefined()
  })

  it('asks for a date and a time that exist', () => {
    expect(validateForm(form({ startTime: '' })).start).toBeDefined()
  })
})

describe('the times', () => {
  it('keeps the length when the start moves', () => {
    const f = withStart(form(), { startTime: '13:15' })
    expect([f.startTime, f.endTime, f.endDate]).toEqual(['13:15', '14:15', '2026-10-14'])
    expect(withStart(form(), { startDate: '2026-10-16' }).endDate).toBe('2026-10-16')
  })

  it('writes the length beside the end time', () => {
    expect([60, 90, 45, 120, 2880].map(durationLabel)).toEqual(['1 giờ', '1 giờ 30 phút', '45 phút', '2 giờ', '2 ngày'])
  })

  it('makes an all-day event end the midnight after its last day', () => {
    const f = form({ allDay: true, startDate: '2026-10-14', endDate: '2026-10-15' })
    const span = spanOfForm(f)!
    expect(span.end.getDate()).toBe(16)
    expect(toInput(f)).toMatchObject({ allDay: true, start: '2026-10-14', end: '2026-10-16' })
  })
})

describe('reminders', () => {
  it('splits into the largest unit and joins back', () => {
    expect(splitMinutes(10)).toEqual({ n: 10, unit: 'minute' })
    expect(splitMinutes(120)).toEqual({ n: 2, unit: 'hour' })
    expect(splitMinutes(1440)).toEqual({ n: 1, unit: 'day' })
    expect(splitMinutes(90)).toEqual({ n: 90, unit: 'minute' })
    expect(joinMinutes(2, 'hour')).toBe(120)
  })

  it('shows the default until the person touches them', () => {
    expect(remindersShown(form(), 10)).toEqual([10])
    expect(remindersShown(form(), 0)).toEqual([])
    expect(remindersShown(form({ reminders: [5, 60] }), 10)).toEqual([5, 60])
  })
})

describe('the body of a new event', () => {
  it('names the guests by node, leaves reminders to the server until chosen, and never sends an empty text', () => {
    const f = form({
      title: '  Duyệt ngân sách ', guests: [{ nodeId: 'node-1', displayName: 'A', hue: 1 }], inviteChannelId: 'ch-1',
      rrule: 'FREQ=WEEKLY;BYDAY=WE', hasCall: true,
    })
    const input = toInput(f)!
    expect(input).toMatchObject({
      calendarId: 'cal-1', title: 'Duyệt ngân sách', attendees: ['node-1'], inviteChannelId: 'ch-1', rrule: 'FREQ=WEEKLY;BYDAY=WE', hasCall: true,
    })
    expect(input).not.toHaveProperty('reminders')
    expect(input).not.toHaveProperty('location')
    expect(input.start).toMatch(/^2026-10-14T09:00:00[+-]\d{2}:\d{2}$/)
    expect(toInput(form({ reminders: [] }))?.reminders).toEqual([])
  })

  it('gives an untitled event the usual name', () => {
    expect(toInput(form())?.title).toBe('(Không có tiêu đề)')
  })
})

describe('the patch of an edit', () => {
  const detail: EventDetail = {
    eventId: 'e1', occurrenceStart: '2026-10-14T09:00:00+07:00', start: '2026-10-14T09:00:00+07:00',
    end: '2026-10-14T10:00:00+07:00', allDay: false, calendarId: 'cal-1', title: 'Họp', busyOnly: false, isOrganizer: true,
    hasCall: false, isRecurring: true, canEdit: true, kind: 'event', guestCount: 1, version: 4, tzid: 'Asia/Ho_Chi_Minh',
    rrule: 'FREQ=WEEKLY;BYDAY=WE', transparency: 'busy', visibility: 'default', guestsCanInvite: false, guestsCanSeeList: true,
    attendees: [
      { person: { nodeId: 'me', displayName: 'Tôi', hue: 0 }, response: 'accepted', isOrganizer: true },
      { person: { nodeId: 'g1', displayName: 'Khách', hue: 1 }, response: 'needs_action', isOrganizer: false },
    ],
    reminders: [{ minutesBefore: 10 }],
  }
  const original = () => formFromDetail(detail)

  it('reads the event into the form, the organizer not among the guests', () => {
    const f = original()
    expect(f).toMatchObject({ title: 'Họp', startTime: '09:00', endTime: '10:00', reminders: [10] })
    expect(f.guests.map((g) => g.nodeId)).toEqual(['g1'])
  })

  it('shows the instance that was opened, not the first of the series', () => {
    const f = formFromDetail(detail, { start: '2026-10-21T09:00:00+07:00', end: '2026-10-21T10:00:00+07:00' })
    expect(f.startDate).toBe('2026-10-21')
  })

  it('has nothing to send when nothing changed', () => {
    const p = toPatch(original(), original(), 4)!
    expect(p).toEqual({ version: 4 })
    expect(hasChanges(p)).toBe(false)
  })

  it('sends only the fields that changed, with the version it started from', () => {
    const f = { ...original(), title: 'Họp mới', location: 'Phòng 3' }
    expect(toPatch(f, original(), 4)).toEqual({ version: 4, title: 'Họp mới', location: 'Phòng 3' })
  })

  it('sends the new position when the time moved', () => {
    const p = toPatch(withStart(original(), { startTime: '10:00' }), original(), 4)!
    expect(p.start).toMatch(/T10:00:00/)
    expect(p.end).toMatch(/T11:00:00/)
    expect(p.allDay).toBe(false)
  })

  it('removes the recurrence with null and changes guests as a whole list', () => {
    const f = { ...original(), rrule: null, guests: [] }
    expect(toPatch(f, original(), 4)).toMatchObject({ rrule: null, attendees: [] })
  })

  it('tells the guests of a change of time but not of a reminder', () => {
    expect(notifiesGuests(toPatch(withStart(original(), { startTime: '10:00' }), original(), 4)!)).toBe(true)
    const quiet = toPatch({ ...original(), reminders: [5], transparency: 'free' }, original(), 4)!
    expect(quiet).toMatchObject({ reminders: [{ minutesBefore: 5 }], transparency: 'free' })
    expect(notifiesGuests(quiet)).toBe(false)
  })

  it('knows when the form is unchanged', () => {
    expect(isDirty(original(), original())).toBe(false)
    expect(isDirty({ ...original(), title: 'x' }, original())).toBe(true)
  })
})
