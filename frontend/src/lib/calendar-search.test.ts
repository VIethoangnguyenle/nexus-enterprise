import { describe, expect, it } from 'vitest'
import { validateCalendarSearch, validateEditEventSearch, validateNewEventSearch } from './calendar-search'

describe('validateCalendarSearch', () => {
  it('keeps a known view and a real date, and the workspace', () => {
    expect(validateCalendarSearch({ ws: 'w1', mode: 'month', date: '2026-10-14' })).toEqual({ ws: 'w1', mode: 'month', date: '2026-10-14' })
    expect(validateCalendarSearch({ mode: '3day' })).toEqual({ mode: '3day' })
  })

  it('drops what it does not know instead of refusing the page', () => {
    expect(validateCalendarSearch({ mode: 'year', date: '2026-02-31' })).toEqual({})
    expect(validateCalendarSearch({ date: 20261014 })).toEqual({})
  })
})

describe('validateNewEventSearch', () => {
  it('keeps a span, a title, a calendar and an event to copy', () => {
    expect(validateNewEventSearch({ ws: 'w', date: '2026-10-14', start: '09:00', end: '10:30', title: 'Họp', calendar: 'c1', copy: 'e1' }))
      .toEqual({ ws: 'w', date: '2026-10-14', start: '09:00', end: '10:30', title: 'Họp', calendar: 'c1', copy: 'e1' })
    expect(validateNewEventSearch({ allDay: '1' })).toEqual({ allDay: 1 })
  })

  it('drops what is not a date, a clock or a word', () => {
    expect(validateNewEventSearch({ date: '2026-13-40', start: '25:00', end: '9:5', title: '   ', copy: 12 })).toEqual({})
  })
})

describe('validateEditEventSearch', () => {
  it('keeps the instance, and the span only as a pair', () => {
    const at = '2026-10-16T09:00:00+07:00'
    expect(validateEditEventSearch({ occurrence: at, start: at, end: '2026-10-16T09:15:00+07:00' }))
      .toEqual({ occurrence: at, start: at, end: '2026-10-16T09:15:00+07:00' })
    expect(validateEditEventSearch({ start: at })).toEqual({})
    expect(validateEditEventSearch({ occurrence: 'tomorrow' })).toEqual({})
  })
})
