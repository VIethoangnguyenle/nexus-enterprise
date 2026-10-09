import { describe, it, expect } from 'vitest'
import {
  toDate, formatTime, formatDate, formatRelative, formatListTime, formatBytes, formatMoney, formatCount,
} from './format'

// Local-time constructor so the tests read the same in any timezone.
const at = (y: number, mo: number, d: number, h = 0, mi = 0, s = 0) => new Date(y, mo - 1, d, h, mi, s)
const NOW = at(2026, 10, 9, 15, 0)

describe('toDate', () => {
  it('reads ISO strings, protobuf timestamps, unix seconds and Dates', () => {
    const d = at(2026, 10, 9, 9, 14)
    expect(toDate(d.toISOString())?.getTime()).toBe(d.getTime())
    expect(toDate({ seconds: d.getTime() / 1000, nanos: 0 })?.getTime()).toBe(d.getTime())
    expect(toDate({ seconds: String(d.getTime() / 1000) })?.getTime()).toBe(d.getTime())
    expect(toDate(d.getTime() / 1000)?.getTime()).toBe(d.getTime())
    expect(toDate(d)?.getTime()).toBe(d.getTime())
  })
  it('returns null for missing or invalid input', () => {
    expect(toDate(undefined)).toBeNull()
    expect(toDate('')).toBeNull()
    expect(toDate('not a date')).toBeNull()
    expect(toDate({})).toBeNull()
  })
})

describe('formatTime / formatDate', () => {
  it('formats a time as 24h HH:mm', () => {
    expect(formatTime(at(2026, 10, 9, 9, 14))).toBe('09:14')
    expect(formatTime(at(2026, 10, 9, 21, 5))).toBe('21:05')
  })
  it('formats a date as dd/MM/yyyy', () => {
    expect(formatDate(at(2026, 10, 9))).toBe('09/10/2026')
    expect(formatDate(at(2026, 1, 3))).toBe('03/01/2026')
  })
  it('returns an empty string for unreadable input', () => {
    expect(formatTime(undefined)).toBe('')
    expect(formatDate('nope')).toBe('')
  })
})

describe('formatRelative', () => {
  it('says "Vừa xong" under a minute', () => {
    expect(formatRelative(at(2026, 10, 9, 14, 59, 30), NOW)).toBe('Vừa xong')
  })
  it('counts minutes and hours today', () => {
    expect(formatRelative(at(2026, 10, 9, 14, 55), NOW)).toBe('5 phút trước')
    expect(formatRelative(at(2026, 10, 9, 12, 0), NOW)).toBe('3 giờ trước')
  })
  it('says "Hôm qua" for the previous calendar day', () => {
    expect(formatRelative(at(2026, 10, 8, 23, 50), NOW)).toBe('Hôm qua')
  })
  it('falls back to the date further back', () => {
    expect(formatRelative(at(2026, 9, 30, 10, 0), NOW)).toBe('30/09/2026')
  })
})

describe('formatListTime', () => {
  it('shows the time today, "Hôm qua", then a short date this year', () => {
    expect(formatListTime(at(2026, 10, 9, 9, 20), NOW)).toBe('09:20')
    expect(formatListTime(at(2026, 10, 8, 9, 20), NOW)).toBe('Hôm qua')
    expect(formatListTime(at(2026, 10, 7, 9, 20), NOW)).toBe('07/10')
    expect(formatListTime(at(2025, 12, 31, 9, 20), NOW)).toBe('31/12/2025')
  })
})

describe('formatBytes', () => {
  it('uses binary units with a Vietnamese decimal comma', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
    expect(formatBytes(412 * 1024)).toBe('412 KB')
    expect(formatBytes(1.5 * 1024 * 1024)).toBe('1,5 MB')
    expect(formatBytes(3 * 1024 * 1024 * 1024)).toBe('3 GB')
  })
})

describe('formatMoney', () => {
  it('groups with dots and suffixes the dong sign', () => {
    expect(formatMoney(12450000)).toBe('12.450.000 ₫')
    expect(formatMoney(1284500)).toBe('1.284.500 ₫')
    expect(formatMoney(0)).toBe('0 ₫')
  })
})

describe('formatCount', () => {
  it('caps large counters', () => {
    expect(formatCount(3)).toBe('3')
    expect(formatCount(99)).toBe('99')
    expect(formatCount(100)).toBe('99+')
  })
})
