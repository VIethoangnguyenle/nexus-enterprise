import { addDays, addMinutes, differenceInCalendarDays, differenceInMinutes, isSameDay, startOfDay } from 'date-fns'

/** One hour of the time grid is 48px (DESIGN.md §6 Lịch). */
export const HOUR_PX = 48
export const MINUTE_PX = HOUR_PX / 60
export const DAY_PX = HOUR_PX * 24
/** Dragging and drawing snap to a quarter of an hour; Alt snaps to five minutes. */
export const SNAP_MINUTES = 15
export const SNAP_MINUTES_FINE = 5
/** A block never draws shorter than this, so a 15-minute event still holds a line of text. */
export const MIN_BLOCK_PX = 20
/** What a block occupies for overlap purposes, in minutes: the shortest it can draw. */
const MIN_VISUAL_MINUTES = MIN_BLOCK_PX / MINUTE_PX

export interface TimedItem {
  id: string
  start: Date
  end: Date
}

/** A block of one day column: pixels from the top, and percent of the column's width. */
export interface PlacedBlock {
  id: string
  top: number
  height: number
  left: number
  width: number
  /** Which overlap column it sits in, and how many its cluster has. */
  column: number
  columns: number
}

/** Pixels below the top of the day for a time of that day. */
export function yOf(d: Date): number {
  return (d.getHours() * 60 + d.getMinutes() + d.getSeconds() / 60) * MINUTE_PX
}

/** Minutes after midnight for a pixel offset in the day column, snapped to `step`. */
export function snapMinutes(y: number, step: number = SNAP_MINUTES): number {
  const minutes = Math.round(y / MINUTE_PX / step) * step
  return Math.min(24 * 60, Math.max(0, minutes))
}

/** The time at a pixel offset of the column of `day`, snapped. */
export function timeAt(day: Date, y: number, step: number = SNAP_MINUTES): Date {
  return addMinutes(startOfDay(day), snapMinutes(y, step))
}

/** Snaps a length in minutes to `step`, at least one step. */
export function snapDuration(minutes: number, step: number = SNAP_MINUTES): number {
  return Math.max(step, Math.round(minutes / step) * step)
}

/** The part of each item that falls on `day`, with the item's own order kept by start and then length. */
function clipToDay<T extends TimedItem>(items: T[], day: Date): { item: T; start: Date; end: Date }[] {
  const dayStart = startOfDay(day)
  const dayEnd = addDays(dayStart, 1)
  const out: { item: T; start: Date; end: Date }[] = []
  for (const item of items) {
    if (item.end <= dayStart && !(item.end.getTime() === item.start.getTime() && item.start >= dayStart)) continue
    if (item.start >= dayEnd) continue
    out.push({
      item,
      start: item.start < dayStart ? dayStart : item.start,
      end: item.end > dayEnd ? dayEnd : item.end,
    })
  }
  return out.sort((a, b) => a.start.getTime() - b.start.getTime() || b.end.getTime() - a.end.getTime())
}

/**
 * Positions the timed items of one day.
 *
 * Items that overlap in time form a cluster. Inside it each item takes the first column whose last item has
 * ended, so columns follow the start time. A block is `1/columns` of the width and, except in the last column,
 * runs on under the next one by half a column, so a later block lies over the right half of the earlier one
 * and no title is hidden entirely (DESIGN.md §6, Khối sự kiện).
 */
export function layoutDay(items: TimedItem[], day: Date): PlacedBlock[] {
  const clipped = clipToDay(items, day)
  const placed: PlacedBlock[] = []

  let cluster: { id: string; start: Date; end: Date; column: number }[] = []
  let columnEnds: Date[] = []
  let clusterEnd = 0

  const flush = () => {
    if (cluster.length === 0) return
    const columns = columnEnds.length
    const colWidth = 100 / columns
    for (const c of cluster) {
      const left = c.column * colWidth
      const last = c.column === columns - 1
      placed.push({
        id: c.id,
        top: yOf(c.start),
        height: Math.max(MIN_BLOCK_PX, (differenceInMinutes(c.end, c.start, { roundingMethod: 'ceil' }) || 0) * MINUTE_PX),
        left,
        width: last ? colWidth : Math.min(100 - left, colWidth * 1.5),
        column: c.column,
        columns,
      })
    }
    cluster = []
    columnEnds = []
  }

  for (const { item, start, end } of clipped) {
    if (cluster.length > 0 && start.getTime() >= clusterEnd) flush()
    const visualEnd = new Date(Math.max(end.getTime(), start.getTime() + MIN_VISUAL_MINUTES * 60_000))
    let column = columnEnds.findIndex((e) => e.getTime() <= start.getTime())
    if (column === -1) {
      column = columnEnds.length
      columnEnds.push(visualEnd)
    } else {
      columnEnds[column] = visualEnd
    }
    cluster.push({ id: item.id, start, end, column })
    clusterEnd = Math.max(cluster.length === 1 ? 0 : clusterEnd, visualEnd.getTime())
  }
  flush()
  return placed
}

// --- Rows of days: the all-day row and month weeks ---------------------------

export interface SpanItem {
  id: string
  start: Date
  /** Exclusive: the midnight after the last day. */
  end: Date
}

/** A bar across consecutive day columns, on a lane (row) of its own where it would collide. */
export interface PlacedSpan {
  id: string
  /** First and last column it covers (0-based, inclusive). */
  startCol: number
  endCol: number
  lane: number
  /** The item goes on before the first or after the last visible day (draw the edge square). */
  continuesBefore: boolean
  continuesAfter: boolean
}

/**
 * Places bars over a run of consecutive `days` (a week, or the visible days of the time grid). Longer bars
 * and earlier ones take the upper lanes; a bar takes the first lane that is free along its whole length.
 */
export function layoutSpans(items: SpanItem[], days: Date[]): PlacedSpan[] {
  if (days.length === 0) return []
  const first = startOfDay(days[0]!)
  const lastIdx = days.length - 1
  const out: PlacedSpan[] = []
  const lanes: number[][] = [] // lanes[l] = last column occupied + 1... kept as per-lane list of busy ends

  const sorted = items
    .map((item) => {
      const rawStart = differenceInCalendarDays(item.start, first)
      // An exclusive end at midnight makes the last day the one before; a zero-length item still takes its day.
      const rawEnd = Math.max(rawStart, differenceInCalendarDays(addMinutes(item.end, -1), first))
      return { item, rawStart, rawEnd }
    })
    .filter((x) => x.rawEnd >= 0 && x.rawStart <= lastIdx)
    .sort((a, b) => a.rawStart - b.rawStart || b.rawEnd - a.rawEnd)

  for (const { item, rawStart, rawEnd } of sorted) {
    const startCol = Math.max(0, rawStart)
    const endCol = Math.min(lastIdx, rawEnd)
    let lane = lanes.findIndex((ends) => ends.every((_, col) => col < startCol || col > endCol || ends[col] === 0))
    if (lane === -1) {
      lane = lanes.length
      lanes.push(new Array<number>(days.length).fill(0))
    }
    for (let c = startCol; c <= endCol; c++) lanes[lane]![c] = 1
    out.push({
      id: item.id,
      startCol,
      endCol,
      lane,
      continuesBefore: rawStart < 0,
      continuesAfter: rawEnd > lastIdx,
    })
  }
  return out
}

/** True when the item touches more than one calendar day (a month grid draws it as a bar). */
export function spansDays(start: Date, end: Date): boolean {
  return !isSameDay(start, addMinutes(end, -1)) && end > start
}

// --- Dragging ------------------------------------------------------------------

export interface DragInput {
  /** `move` slides the block; `resize` drags its lower edge. */
  mode: 'move' | 'resize'
  start: Date
  end: Date
  /** Pointer travel since the press, in pixels down the column. */
  dy: number
  /** Columns the pointer crossed (moves only). */
  days: number
  /** Snap step in minutes: 15, or 5 with Alt. */
  step: number
}

/**
 * Where a dragged block lands. A move keeps the length, snaps the start to the step and, for a block that sits
 * inside one day, stays inside the day it lands on. A resize moves only the end and keeps at least one step.
 */
export function dragResult({ mode, start, end, dy, days, step }: DragInput): { start: Date; end: Date } {
  const delta = Math.round(dy / MINUTE_PX / step) * step
  if (mode === 'resize') {
    const minEnd = addMinutes(start, step)
    const next = addMinutes(end, delta)
    return { start, end: next < minEnd ? minEnd : next }
  }
  const length = end.getTime() - start.getTime()
  let moved = addMinutes(start, delta)
  if (isSameDay(start, addMinutes(end, -1)) && length < 24 * 60 * 60_000) {
    const lengthMin = Math.round(length / 60_000)
    const minutes = Math.min(Math.max(differenceInMinutes(moved, startOfDay(start)), 0), Math.max(0, 24 * 60 - lengthMin))
    moved = addMinutes(startOfDay(start), minutes)
  }
  moved = addDays(moved, days)
  return { start: moved, end: new Date(moved.getTime() + length) }
}

/** A month-grid drop: the same times, `days` later. */
export function moveByDays(start: Date, end: Date, days: number): { start: Date; end: Date } {
  const length = end.getTime() - start.getTime()
  const moved = addDays(start, days)
  return { start: moved, end: new Date(moved.getTime() + length) }
}
