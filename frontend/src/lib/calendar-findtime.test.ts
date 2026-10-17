import { describe, expect, it } from 'vitest'
import {
  FIND_HOUR_PX, boxOnDay, dragSpan, guestStatus, outsideHours, resizeSpan, snapToDay, spansOf, statusLine,
} from './calendar-findtime'

const at = (h: number, m = 0) => new Date(2026, 9, 14, h, m)
const iso = (h: number, m = 0) => at(h, m).toISOString()

describe('Tìm giờ', () => {
  const busy = spansOf([{ start: iso(9), end: iso(9, 30), title: 'Gặp ngân hàng' }])

  it('says what is under a name', () => {
    expect(statusLine(busy, at(9), at(10))).toBe('Trùng 1 sự kiện')
    expect(statusLine(busy, at(10), at(11))).toBe('Có thể tham gia')
    expect(statusLine(spansOf([{ start: iso(9), end: iso(10) }]), at(13), at(14))).toBe('Chỉ thấy bận/rảnh')
  })

  it('tells a guest free from busy, with what they are busy with', () => {
    expect(guestStatus(busy, at(11), at(12))).toEqual({ free: true, text: 'Rảnh' })
    expect(guestStatus(busy, at(9, 15), at(10))).toEqual({ free: false, text: 'Bận lúc này · Gặp ngân hàng' })
  })

  it('places a span on the day and clips it at midnight', () => {
    expect(boxOnDay(at(0), at(9), at(10))).toEqual({ top: 9 * FIND_HOUR_PX, height: FIND_HOUR_PX })
    expect(boxOnDay(at(0), new Date(2026, 9, 13, 23), at(1))?.top).toBe(0)
    expect(boxOnDay(at(0), new Date(2026, 9, 15, 1), new Date(2026, 9, 15, 2))).toBeNull()
  })

  it('draws the hours outside work as stripes', () => {
    expect(outsideHours({ workStart: '08:00', workEnd: '17:30' })).toEqual([
      { top: 0, height: 8 * FIND_HOUR_PX },
      { top: 17.5 * FIND_HOUR_PX, height: 6.5 * FIND_HOUR_PX },
    ])
  })

  it('drags a band a quarter of an hour at a time and keeps it inside the day', () => {
    const moved = dragSpan(at(9), at(10), FIND_HOUR_PX + 3)
    expect([moved.start.getHours(), moved.start.getMinutes()]).toEqual([10, 0])
    expect(moved.end.getHours()).toBe(11)
    const top = dragSpan(at(9), at(10), -5000)
    expect(top.start.getHours()).toBe(0)
    const bottom = dragSpan(at(9), at(10), 5000)
    expect(bottom.end.getHours() * 60 + bottom.end.getMinutes()).toBe(0)
    expect(bottom.start.getHours()).toBe(23)
  })

  it('resizes from the lower edge, never below a quarter of an hour', () => {
    expect(resizeSpan(at(9), at(10), FIND_HOUR_PX / 2).end).toEqual(at(10, 30))
    expect(resizeSpan(at(9), at(10), -9999).end).toEqual(at(9, 15))
  })

  it('names a pointer offset as a quarter of an hour', () => {
    expect(snapToDay(at(0), 9.4 * FIND_HOUR_PX)).toEqual(at(9, 30))
  })
})
