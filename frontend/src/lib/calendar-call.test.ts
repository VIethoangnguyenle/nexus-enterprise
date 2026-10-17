import { describe, expect, it } from 'vitest'
import { callWindow } from './calendar-call'

const start = new Date(2026, 9, 14, 9, 0)
const end = new Date(2026, 9, 14, 10, 0)
const at = (h: number, m: number) => new Date(2026, 9, 14, h, m)

describe('callWindow', () => {
  it('opens 15 minutes before the start and closes at the end', () => {
    expect(callWindow(start, end, at(8, 44))).toBe('early')
    expect(callWindow(start, end, at(8, 45))).toBe('open')
    expect(callWindow(start, end, at(9, 30))).toBe('open')
    expect(callWindow(start, end, at(9, 59))).toBe('open')
    expect(callWindow(start, end, at(10, 0))).toBe('over')
  })
})
