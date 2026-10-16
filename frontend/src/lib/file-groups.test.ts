import { describe, expect, it } from 'vitest'
import { fileGroupOf, groupFiles, isMediaFile } from './file-groups'

const VN = 'Asia/Ho_Chi_Minh'
// Wednesday 2026-10-14, 10:00 in Ho Chi Minh (UTC+7).
const NOW = new Date('2026-10-14T03:00:00Z')
const at = (iso: string) => new Date(iso).getTime()

describe('fileGroupOf', () => {
  it('puts the same calendar day in today, from midnight to the end of the day', () => {
    expect(fileGroupOf(at('2026-10-13T17:00:00Z'), NOW, VN)).toBe('today') // 00:00 local
    expect(fileGroupOf(at('2026-10-14T16:59:59Z'), NOW, VN)).toBe('today') // 23:59:59 local
  })

  it('treats a timestamp ahead of the clock as today', () => {
    expect(fileGroupOf(at('2026-10-20T00:00:00Z'), NOW, VN)).toBe('today')
  })

  it('puts earlier days since Monday in this week', () => {
    expect(fileGroupOf(at('2026-10-13T16:59:59Z'), NOW, VN)).toBe('week') // Tue 23:59 local
    expect(fileGroupOf(at('2026-10-11T17:00:00Z'), NOW, VN)).toBe('week') // Mon 00:00 local
  })

  it('starts the week on Monday: Sunday belongs to the previous week', () => {
    expect(fileGroupOf(at('2026-10-11T16:59:59Z'), NOW, VN)).toBe('month') // Sun 23:59 local
  })

  it('puts earlier days of the month in this month and the rest in older', () => {
    expect(fileGroupOf(at('2026-09-30T17:00:00Z'), NOW, VN)).toBe('month') // Oct 1 00:00 local
    expect(fileGroupOf(at('2026-09-30T16:59:59Z'), NOW, VN)).toBe('older') // Sep 30 23:59 local
    expect(fileGroupOf(at('2025-10-14T03:00:00Z'), NOW, VN)).toBe('older') // same month, a year ago
  })

  it('keeps this week even when it began in the previous month', () => {
    const wednesday = new Date('2026-12-02T05:00:00Z') // Wed Dec 2; its Monday is Nov 30
    expect(fileGroupOf(at('2026-11-30T10:00:00Z'), wednesday, VN)).toBe('week') // Mon Nov 30, last month
    expect(fileGroupOf(at('2026-11-29T10:00:00Z'), wednesday, VN)).toBe('older') // Sun Nov 29
  })

  it('reads the day in the given timezone, not UTC', () => {
    const ts = at('2026-10-14T20:00:00Z') // Oct 14 in LA, Oct 15 in Ho Chi Minh
    const nowLa = new Date('2026-10-14T23:00:00Z') // 16:00 Oct 14 in LA
    expect(fileGroupOf(ts, nowLa, 'America/Los_Angeles')).toBe('today')
    expect(fileGroupOf(ts, nowLa, 'UTC')).toBe('today')
    const nowVn = new Date('2026-10-14T23:00:00Z') // 06:00 Oct 15 in Ho Chi Minh
    expect(fileGroupOf(ts, nowVn, VN)).toBe('today')
    expect(fileGroupOf(at('2026-10-14T10:00:00Z'), nowVn, VN)).toBe('week') // yesterday there
  })

  it('treats an unreadable time as older', () => {
    expect(fileGroupOf(Number.NaN, NOW, VN)).toBe('older')
    expect(fileGroupOf(0, NOW, VN)).toBe('older')
  })
})

describe('groupFiles', () => {
  it('returns non-empty groups in reading order, keeping the caller order inside each', () => {
    const items = [
      { n: 'old', t: at('2024-01-01T00:00:00Z') },
      { n: 'a', t: at('2026-10-14T02:00:00Z') },
      { n: 'month', t: at('2026-10-03T02:00:00Z') },
      { n: 'b', t: at('2026-10-14T01:00:00Z') },
    ]
    const groups = groupFiles(items, (i) => i.t, NOW, VN)
    expect(groups.map((g) => g.key)).toEqual(['today', 'month', 'older'])
    expect(groups.map((g) => g.label)).toEqual(['Hôm nay', 'Tháng này', 'Cũ hơn'])
    expect(groups[0]!.items.map((i) => i.n)).toEqual(['a', 'b'])
  })

  it('gives no groups for no items', () => {
    expect(groupFiles([], () => 0, NOW, VN)).toEqual([])
  })
})

describe('isMediaFile', () => {
  it('is true for pictures and videos by extension or mime, false for documents', () => {
    expect(isMediaFile('a.jpg')).toBe(true)
    expect(isMediaFile('clip', 'video/mp4')).toBe(true)
    expect(isMediaFile('bao-cao.pdf', 'application/pdf')).toBe(false)
    expect(isMediaFile('song.mp3')).toBe(false)
  })
})
