import { describe, expect, it } from 'vitest'
import {
  HOUR_PX, MIN_BLOCK_PX, dragResult, layoutDay, layoutSpans, moveByDays, snapDuration, snapMinutes, spansDays, timeAt, yOf,
  type TimedItem,
} from './calendar-layout'

const DAY = new Date(2026, 9, 14)
const at = (h: number, m = 0, day = 14) => new Date(2026, 9, day, h, m)
const item = (id: string, s: Date, e: Date): TimedItem => ({ id, start: s, end: e })
const byId = (list: ReturnType<typeof layoutDay>) => Object.fromEntries(list.map((b) => [b.id, b]))

describe('positions', () => {
  it('puts an hour at 48px and a quarter hour at 12px', () => {
    const [b] = layoutDay([item('a', at(9), at(10, 30))], DAY)
    expect(b!.top).toBe(9 * HOUR_PX)
    expect(b!.height).toBe(1.5 * HOUR_PX)
    expect(yOf(at(0, 15))).toBe(12)
  })

  it('never draws a block shorter than a line of text', () => {
    const [b] = layoutDay([item('a', at(9), at(9, 15))], DAY)
    expect(b!.height).toBe(MIN_BLOCK_PX)
  })

  it('cuts an event that crosses midnight at the day edge, on both days', () => {
    const night = item('n', at(22), at(2, 0, 15))
    const first = layoutDay([night], DAY)[0]!
    expect(first.top).toBe(22 * HOUR_PX)
    expect(first.top + first.height).toBe(24 * HOUR_PX)
    const next = layoutDay([night], new Date(2026, 9, 15))[0]!
    expect(next.top).toBe(0)
    expect(next.height).toBe(2 * HOUR_PX)
  })

  it('leaves out what is on another day, including what ends exactly at midnight', () => {
    expect(layoutDay([item('a', at(9), at(10, 0, 15)), item('b', at(20, 0, 13), at(0, 0, 14))], DAY).map((b) => b.id)).toEqual(['a'])
    expect(layoutDay([item('c', at(9, 0, 15), at(10, 0, 15))], DAY)).toEqual([])
  })
})

describe('overlapping blocks', () => {
  it('gives a lone block the whole width', () => {
    expect(layoutDay([item('a', at(9), at(10))], DAY)[0]).toMatchObject({ left: 0, width: 100, column: 0, columns: 1 })
  })

  it('does not split blocks that only touch', () => {
    const l = byId(layoutDay([item('a', at(9), at(10)), item('b', at(10), at(11))], DAY))
    expect(l.a).toMatchObject({ column: 0, columns: 1, width: 100 })
    expect(l.b).toMatchObject({ column: 0, columns: 1, width: 100 })
  })

  it('puts two overlapping blocks in two columns by start time, the later one over the right half of the first', () => {
    const l = byId(layoutDay([item('b', at(9, 30), at(10, 30)), item('a', at(9), at(10))], DAY))
    expect(l.a).toMatchObject({ column: 0, columns: 2, left: 0, width: 75 })
    expect(l.b).toMatchObject({ column: 1, columns: 2, left: 50, width: 50 })
  })

  it('reuses the first free column and keeps one cluster as wide as its deepest overlap', () => {
    // a 9-11, b 9-10, c 10-11 (fits under b), d 10:30-12 overlaps a and c.
    const l = byId(layoutDay([
      item('a', at(9), at(11)), item('b', at(9), at(10)), item('c', at(10), at(11)), item('d', at(10, 30), at(12)),
    ], DAY))
    expect(l.a!.column).toBe(0)
    expect(l.b!.column).toBe(1)
    expect(l.c!.column).toBe(1)
    expect(l.d!.column).toBe(2)
    expect(Object.values(l).every((b) => b.columns === 3)).toBe(true)
  })

  it('starts a new cluster after a gap, so a lone later block is full width again', () => {
    const l = byId(layoutDay([item('a', at(9), at(10)), item('b', at(9), at(10)), item('c', at(13), at(14))], DAY))
    expect(l.a!.columns).toBe(2)
    expect(l.c).toMatchObject({ columns: 1, width: 100 })
  })

  it('counts a short block as occupying its minimum height, so a 10-minute event does not hide behind its neighbour', () => {
    const l = byId(layoutDay([item('a', at(9), at(9, 10)), item('b', at(9, 15), at(10))], DAY))
    expect(l.a!.columns).toBe(2)
  })
})

describe('snapping', () => {
  it('snaps a pixel offset to a quarter of an hour, or five minutes with Alt', () => {
    expect(snapMinutes(9 * HOUR_PX + 5)).toBe(9 * 60) // 6.25 min -> 0
    expect(snapMinutes(9 * HOUR_PX + 7)).toBe(9 * 60 + 15) // 8.75 min -> 15
    expect(snapMinutes(9 * HOUR_PX + 7, 5)).toBe(9 * 60 + 10)
    expect(snapMinutes(-20)).toBe(0)
    expect(snapMinutes(99 * HOUR_PX)).toBe(24 * 60)
  })

  it('turns a click into a time of that day', () => {
    expect(timeAt(DAY, 10 * HOUR_PX + 13)).toEqual(at(10, 15))
  })

  it('keeps a duration to whole steps and at least one', () => {
    expect(snapDuration(40)).toBe(45)
    expect(snapDuration(2)).toBe(15)
  })
})

describe('bars across days', () => {
  const week = Array.from({ length: 7 }, (_, i) => new Date(2026, 9, 12 + i))
  const span = (id: string, s: Date, e: Date) => ({ id, start: s, end: e })

  it('places a one-day all-day item in its own column', () => {
    expect(layoutSpans([span('a', new Date(2026, 9, 14), new Date(2026, 9, 15))], week))
      .toEqual([{ id: 'a', startCol: 2, endCol: 2, lane: 0, continuesBefore: false, continuesAfter: false }])
  })

  it('stretches a multi-day item over its days, end exclusive', () => {
    const [s] = layoutSpans([span('a', new Date(2026, 9, 13), new Date(2026, 9, 16))], week)
    expect([s!.startCol, s!.endCol]).toEqual([1, 3])
  })

  it('clips an item that runs past the week and says so on the cut edges', () => {
    const [s] = layoutSpans([span('a', new Date(2026, 9, 9), new Date(2026, 9, 21))], week)
    expect(s).toMatchObject({ startCol: 0, endCol: 6, continuesBefore: true, continuesAfter: true })
  })

  it('drops an item outside the run', () => {
    expect(layoutSpans([span('a', new Date(2026, 9, 20), new Date(2026, 9, 22))], week)).toEqual([])
  })

  it('stacks colliding bars on separate lanes and reuses a lane that is free', () => {
    const l = layoutSpans([
      span('a', new Date(2026, 9, 12), new Date(2026, 9, 15)),
      span('b', new Date(2026, 9, 13), new Date(2026, 9, 14)),
      span('c', new Date(2026, 9, 15), new Date(2026, 9, 17)),
    ], week)
    const lane = Object.fromEntries(l.map((s) => [s.id, s.lane]))
    expect(lane).toEqual({ a: 0, b: 1, c: 0 })
  })

  it('knows when a timed item crosses midnight', () => {
    expect(spansDays(at(22), at(2, 0, 15))).toBe(true)
    expect(spansDays(at(9), at(10))).toBe(false)
    expect(spansDays(at(0), at(0, 0, 15))).toBe(false)
  })
})

describe('drag results', () => {
  const s = at(9)
  const e = at(10)

  it('moves by whole quarter hours and keeps the length', () => {
    // 20px down is 25 minutes, which snaps to 30.
    const r = dragResult({ mode: 'move', start: s, end: e, dy: 20, days: 0, step: 15 })
    expect(r.start).toEqual(at(9, 30))
    expect(r.end).toEqual(at(10, 30))
  })

  it('snaps to five minutes with Alt', () => {
    const r = dragResult({ mode: 'move', start: s, end: e, dy: 5, days: 0, step: 5 })
    expect(r.start).toEqual(at(9, 5))
  })

  it('carries a block to another day at the same time of day', () => {
    const r = dragResult({ mode: 'move', start: s, end: e, dy: 0, days: 2, step: 15 })
    expect(r.start).toEqual(at(9, 0, 16))
    expect(r.end).toEqual(at(10, 0, 16))
  })

  it('keeps a block inside its day at both edges', () => {
    expect(dragResult({ mode: 'move', start: s, end: e, dy: -2000, days: 0, step: 15 }).start).toEqual(at(0))
    const late = dragResult({ mode: 'move', start: s, end: e, dy: 2000, days: 0, step: 15 })
    expect(late.end).toEqual(new Date(2026, 9, 15, 0, 0))
    expect(late.start).toEqual(at(23))
  })

  it('resizes the end only, down to one step', () => {
    expect(dragResult({ mode: 'resize', start: s, end: e, dy: 24, days: 3, step: 15 })).toEqual({ start: s, end: at(10, 30) })
    expect(dragResult({ mode: 'resize', start: s, end: e, dy: -500, days: 0, step: 15 })).toEqual({ start: s, end: at(9, 15) })
  })

  it('moves a month-grid item by days without touching its time', () => {
    const r = moveByDays(at(9), at(10), -3)
    expect(r.start).toEqual(at(9, 0, 11))
    expect(r.end).toEqual(at(10, 0, 11))
  })
})
