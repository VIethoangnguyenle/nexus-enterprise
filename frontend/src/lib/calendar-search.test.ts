import { describe, expect, it } from 'vitest'
import { validateCalendarSearch } from './calendar-search'

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
