import { describe, expect, it } from 'vitest'
import {
  activeFilterCount, describeRange, filtersFromUrl, isoDate, parseChatSearch, presetRange, urlFromFilters,
} from './chat-search'

const TODAY = new Date(2026, 9, 18, 14, 30)

describe('isoDate', () => {
  it('is the calendar day in the reader’s zone, zero padded', () => {
    expect(isoDate(new Date(2026, 0, 5, 23, 59))).toBe('2026-01-05')
  })
})

describe('presetRange', () => {
  it('counts back whole days, today included', () => {
    expect(presetRange('today', TODAY)).toEqual({ after: '2026-10-18', beforeDate: '2026-10-18' })
    expect(presetRange('week', TODAY)).toEqual({ after: '2026-10-12', beforeDate: '2026-10-18' })
    expect(presetRange('month', TODAY)).toEqual({ after: '2026-09-19', beforeDate: '2026-10-18' })
  })
})

describe('describeRange', () => {
  it('names the presets the way the picker does', () => {
    expect(describeRange(presetRange('today', TODAY), TODAY)).toBe('Hôm nay')
    expect(describeRange(presetRange('week', TODAY), TODAY)).toBe('7 ngày qua')
    expect(describeRange(presetRange('month', TODAY), TODAY)).toBe('30 ngày qua')
  })

  it('writes any other range as day/month – day/month', () => {
    expect(describeRange({ after: '2026-10-01', beforeDate: '2026-10-15' }, TODAY)).toBe('01/10 – 15/10')
  })

  it('writes an open end as "Từ …" or "Đến …"', () => {
    expect(describeRange({ after: '2026-10-01' }, TODAY)).toBe('Từ 01/10')
    expect(describeRange({ beforeDate: '2026-10-15' }, TODAY)).toBe('Đến 15/10')
  })

  it('adds the year to a range that is not in this one', () => {
    expect(describeRange({ after: '2025-12-20', beforeDate: '2026-01-03' }, TODAY)).toBe('20/12/2025 – 03/01/2026')
  })
})

describe('the address', () => {
  it('reads only well-formed values and drops the rest', () => {
    expect(parseChatSearch({
      q: ' đối soát ', from: 'u-1', in: 'c-1', since: '2026-10-01', until: '2026-13-45', file: 1, link: '1', me: 'yes', junk: 'x',
    })).toEqual({ q: 'đối soát', from: 'u-1', in: 'c-1', since: '2026-10-01', file: 1, link: 1 })
  })

  it('round-trips filters through the address keys', () => {
    const filters = { from: 'u-1', channelId: 'c-1', after: '2026-10-01', beforeDate: '2026-10-15', hasFile: true, hasLink: true, mentionsMe: true }
    const url = urlFromFilters('báo cáo', filters)
    expect(url).toEqual({ q: 'báo cáo', from: 'u-1', in: 'c-1', since: '2026-10-01', until: '2026-10-15', file: 1, link: 1, me: 1 })
    expect(filtersFromUrl(url)).toEqual(filters)
  })

  it('leaves an off filter out of the address altogether', () => {
    expect(urlFromFilters('a b', { hasFile: false })).toEqual({ q: 'a b' })
  })
})

describe('activeFilterCount', () => {
  it('counts a date range as one filter', () => {
    expect(activeFilterCount({})).toBe(0)
    expect(activeFilterCount({ after: '2026-10-01', beforeDate: '2026-10-15', hasFile: true })).toBe(2)
  })
})
