import { describe, expect, it } from 'vitest'
import { pauseLabel, pauseUntil } from './pause'

const at = (h: number, m = 0) => new Date(2026, 9, 19, h, m, 0, 0)

describe('Tạm dừng thông báo', () => {
  it('ends a timed pause that long after now', () => {
    expect(pauseUntil('30m', at(14))).toBe(at(14, 30).toISOString())
    expect(pauseUntil('2h', at(23))).toBe(new Date(2026, 9, 20, 1, 0).toISOString())
  })

  it('ends "until morning" at 8:00 the next day, and "off" is no pause', () => {
    expect(pauseUntil('morning', at(23, 40))).toBe(new Date(2026, 9, 20, 8, 0).toISOString())
    expect(pauseUntil('off', at(9))).toBeNull()
  })

  it('reads Tắt when not paused or when the pause is over', () => {
    expect(pauseLabel(null, at(9))).toBe('Tắt')
    expect(pauseLabel(undefined, at(9))).toBe('Tắt')
    expect(pauseLabel(at(8).toISOString(), at(9))).toBe('Tắt')
    expect(pauseLabel('not a date', at(9))).toBe('Tắt')
  })

  it('says until when: the time today, the time and date on another day', () => {
    expect(pauseLabel(at(14, 30).toISOString(), at(9))).toBe('Đến 14:30')
    expect(pauseLabel(new Date(2026, 9, 20, 8, 0).toISOString(), at(23))).toBe('Đến 08:00 20/10/2026')
  })
})
