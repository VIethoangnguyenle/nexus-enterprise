import { describe, expect, it } from 'vitest'
import {
  eventAriaLabel, toneOf, formatDayShort, formatWhen, gmtLabel, monthWeeks, movedToast, parseDateKey, rangeOf, shiftDate, spanOf,
  titleOf, toDateKey, toWire, visibleDays, weekLabel, wireOf,
} from './calendar-format'
import type { Occurrence } from '../api/calendar'

// Wednesday 14 October 2026.
const WED = new Date(2026, 9, 14, 10, 40)

const occ = (over: Partial<Occurrence> = {}): Occurrence => ({
  eventId: 'e1', occurrenceStart: '2026-10-14T15:00:00+07:00', start: '2026-10-14T15:00:00+07:00', end: '2026-10-14T16:00:00+07:00',
  allDay: false, calendarId: 'c1', title: 'Phỏng vấn kế toán viên', busyOnly: false, isOrganizer: false, hasCall: false,
  isRecurring: false, canEdit: true, kind: 'event', guestCount: 3, version: 1, ...over,
})

describe('date keys', () => {
  it('round-trips a day and refuses what is not a date', () => {
    expect(toDateKey(WED)).toBe('2026-10-14')
    expect(parseDateKey('2026-10-14')?.getDate()).toBe(14)
    expect(parseDateKey('2026-02-30')).toBeNull()
    expect(parseDateKey('14/10/2026')).toBeNull()
    expect(parseDateKey(undefined)).toBeNull()
  })

  it('sends a date for an all-day edge and an instant for a timed one', () => {
    expect(wireOf(WED, true)).toBe('2026-10-14')
    expect(wireOf(WED, false)).toMatch(/^2026-10-14T10:40:00[+-]\d\d:\d\d$/)
    expect(toWire(WED)).toBe(wireOf(WED, false))
  })

  it('reads a date-only edge as local midnight and an instant as that instant', () => {
    const allDay = spanOf({ start: '2026-10-14', end: '2026-10-16' })
    expect(allDay.start.getHours()).toBe(0)
    expect(allDay.end.getDate()).toBe(16)
    expect(spanOf(occ()).end.getTime() - spanOf(occ()).start.getTime()).toBe(3_600_000)
  })
})

describe('visible range', () => {
  it('starts the week on Monday, so a Sunday belongs to the week before it', () => {
    const days = visibleDays('week', WED)
    expect(days.map((d) => d.getDate())).toEqual([12, 13, 14, 15, 16, 17, 18])
    const sunday = new Date(2026, 9, 18)
    expect(visibleDays('week', sunday)[0]!.getDate()).toBe(12)
  })

  it('shows one day, three days from the date, or a week', () => {
    expect(visibleDays('day', WED)).toHaveLength(1)
    expect(visibleDays('3day', WED).map((d) => d.getDate())).toEqual([14, 15, 16])
  })

  it('fills the month grid with whole weeks from the Monday on or before the 1st', () => {
    const weeks = monthWeeks(WED)
    expect(weeks).toHaveLength(5)
    expect(weeks[0]![0]!.getDate()).toBe(28) // 28 Sep 2026 is a Monday
    expect(weeks[0]![0]!.getMonth()).toBe(8)
    expect(weeks[4]![6]!.getDate()).toBe(1) // Sun 1 Nov
  })

  it('adds a sixth row to a month that needs it', () => {
    // August 2026 starts on a Saturday and has 31 days.
    expect(monthWeeks(new Date(2026, 7, 10))).toHaveLength(6)
  })

  it('asks the server for [from, to) of the whole view', () => {
    const w = rangeOf('week', WED)
    expect(toDateKey(w.from)).toBe('2026-10-12')
    expect(toDateKey(w.to)).toBe('2026-10-19')
    const m = rangeOf('month', WED)
    expect(toDateKey(m.from)).toBe('2026-09-28')
    expect(toDateKey(m.to)).toBe('2026-11-02')
    const a = rangeOf('agenda', WED)
    expect(toDateKey(a.from)).toBe('2026-10-14')
    expect(toDateKey(a.to)).toBe('2026-11-13')
  })

  it('moves by the unit of the view', () => {
    expect(toDateKey(shiftDate('week', WED, 1))).toBe('2026-10-21')
    expect(toDateKey(shiftDate('day', WED, -1))).toBe('2026-10-13')
    expect(toDateKey(shiftDate('3day', WED, 1))).toBe('2026-10-17')
    expect(toDateKey(shiftDate('month', WED, 1))).toBe('2026-11-14')
  })
})

describe('words', () => {
  it('titles each view', () => {
    expect(titleOf('month', WED)).toBe('Tháng 10 năm 2026')
    expect(titleOf('week', WED)).toBe('Tháng 10 năm 2026')
    expect(titleOf('day', WED)).toBe('14 tháng 10 năm 2026')
    expect(titleOf('week', new Date(2026, 9, 1))).toBe('Tháng 9 – 10 năm 2026')
    expect(titleOf('week', new Date(2026, 11, 30))).toBe('Tháng 12 năm 2026 – Tháng 1 năm 2027')
  })

  it('numbers the week by ISO', () => {
    expect(weekLabel(WED)).toBe('Tuần 42')
  })

  it('names the zone from the offset of the date', () => {
    expect(gmtLabel(WED)).toMatch(/^GMT[+-]\d+(:\d\d)?$/)
  })

  it('says when an event is', () => {
    const s = new Date(2026, 9, 14, 9, 0)
    expect(formatWhen(s, new Date(2026, 9, 14, 10, 0), false)).toBe('Thứ Tư, 14 tháng 10 · 09:00 – 10:00')
    expect(formatWhen(new Date(2026, 9, 14), new Date(2026, 9, 15), true)).toBe('Thứ Tư, 14 tháng 10')
    expect(formatWhen(new Date(2026, 9, 14), new Date(2026, 9, 17), true)).toBe('Thứ Tư, 14 tháng 10 – Thứ Sáu, 16 tháng 10')
  })

  it('labels the agenda day column', () => {
    expect(formatDayShort(WED)).toBe('T4, TH 10')
    expect(formatDayShort(new Date(2026, 9, 18))).toBe('CN, TH 10')
  })

  it('reads a block aloud with its time and answer, not just its colour', () => {
    expect(eventAriaLabel(occ({ myResponse: 'needs_action' }), new Date(2026, 9, 14, 15), new Date(2026, 9, 14, 16)))
      .toBe('Phỏng vấn kế toán viên, 15:00 đến 16:00, chưa trả lời')
    expect(eventAriaLabel(occ({ allDay: true, myResponse: 'accepted' }), WED, WED)).toBe('Phỏng vấn kế toán viên, cả ngày')
    expect(eventAriaLabel(occ({ isOrganizer: true, myResponse: 'needs_action' }), WED, WED)).not.toContain('chưa trả lời')
  })

  it('confirms a drop with the new time, or the new day when the day changed', () => {
    expect(movedToast(new Date(2026, 9, 14, 9), new Date(2026, 9, 14, 10), false)).toBe('Đã dời sang 10:00')
    expect(movedToast(new Date(2026, 9, 14, 9), new Date(2026, 9, 15, 10), false)).toBe('Đã dời sang Thứ Năm, 15 tháng 10 10:00')
  })
})

describe('block tone', () => {
  const NOW = new Date(2026, 9, 14, 12, 0)
  const base = { busyOnly: false, isOrganizer: false } as const
  const after = new Date(2026, 9, 14, 15)
  const before = new Date(2026, 9, 14, 9)

  it('hides detail first, then declined, then unanswered, then over', () => {
    expect(toneOf({ ...base, busyOnly: true, myResponse: 'declined' }, after, NOW)).toBe('busy')
    expect(toneOf({ ...base, myResponse: 'declined' }, before, NOW)).toBe('declined')
    expect(toneOf({ ...base, myResponse: 'needs_action' }, after, NOW)).toBe('pending')
    expect(toneOf({ ...base, myResponse: 'accepted' }, before, NOW)).toBe('past')
    expect(toneOf(base, after, NOW)).toBe('solid')
  })

  it('never calls the organizer\'s own event unanswered', () => {
    expect(toneOf({ ...base, isOrganizer: true, myResponse: 'needs_action' }, after, NOW)).toBe('solid')
  })
})
