import { addMinutes, differenceInMinutes, parseISO, startOfDay } from 'date-fns'
import type { BusySpan, PersonFreeBusy } from '../api/calendar'

/** Pixels an hour takes in Tìm giờ (the mockup's `--hh`). */
export const FIND_HOUR_PX = 40
export const FIND_DAY_PX = FIND_HOUR_PX * 24
const STEP = 15

export interface Span {
  start: Date
  end: Date
  title?: string
}

export const spansOf = (busy: BusySpan[]): Span[] =>
  busy.map((b) => ({ start: parseISO(b.start), end: parseISO(b.end), ...(b.title ? { title: b.title } : {}) }))

/** Busy spans that overlap `[start, end)`. */
export const overlapping = (busy: Span[], start: Date, end: Date): Span[] =>
  busy.filter((b) => b.start < end && b.end > start)

/** The line under a guest's name at the head of their column. */
export function statusLine(busy: Span[], start: Date, end: Date): string {
  const clash = overlapping(busy, start, end)
  if (clash.length > 0) return `Trùng ${clash.length} sự kiện`
  if (busy.length > 0 && busy.every((b) => !b.title)) return 'Chỉ thấy bận/rảnh'
  return 'Có thể tham gia'
}

/** One line under a guest of the editor: "Rảnh" or what they are busy with. */
export function guestStatus(busy: Span[], start: Date, end: Date): { free: boolean; text: string } {
  const clash = overlapping(busy, start, end)
  if (clash.length === 0) return { free: true, text: 'Rảnh' }
  const first = clash[0]!
  return { free: false, text: first.title ? `Bận lúc này · ${first.title}` : 'Bận lúc này' }
}

/** Top and height in pixels of a span on the day, clipped to it; null when it does not touch the day. */
export function boxOnDay(day: Date, start: Date, end: Date): { top: number; height: number } | null {
  const from = startOfDay(day)
  const to = addMinutes(from, 1440)
  if (end <= from || start >= to) return null
  const a = Math.max(start.getTime(), from.getTime())
  const b = Math.min(end.getTime(), to.getTime())
  return {
    top: ((a - from.getTime()) / 60000) * (FIND_HOUR_PX / 60),
    height: Math.max(2, ((b - a) / 60000) * (FIND_HOUR_PX / 60)),
  }
}

/** `HH:mm` → minutes since midnight; a bad value gives the fallback. */
export function minutesOfClock(hhmm: string, fallback: number): number {
  const m = hhmm.match(/^(\d{1,2}):(\d{2})$/)
  return m ? Number(m[1]) * 60 + Number(m[2]) : fallback
}

/** The two bands outside someone's working hours: before it starts and after it ends. */
export function outsideHours(p: Pick<PersonFreeBusy, 'workStart' | 'workEnd'>): { top: number; height: number }[] {
  const ws = minutesOfClock(p.workStart, 0)
  const we = minutesOfClock(p.workEnd, 1440)
  const px = FIND_HOUR_PX / 60
  const out: { top: number; height: number }[] = []
  if (ws > 0) out.push({ top: 0, height: ws * px })
  if (we < 1440) out.push({ top: we * px, height: (1440 - we) * px })
  return out
}

/** A pointer offset in the day → the start of the span it names, snapped to a quarter of an hour. */
export function snapToDay(day: Date, y: number): Date {
  const minutes = Math.round(y / (FIND_HOUR_PX / 60) / STEP) * STEP
  return addMinutes(startOfDay(day), Math.min(1440, Math.max(0, minutes)))
}

/** Moves a span by `dy` pixels (a quarter of an hour at a time) without leaving the day. */
export function dragSpan(start: Date, end: Date, dy: number): { start: Date; end: Date } {
  const length = differenceInMinutes(end, start)
  const day = startOfDay(start)
  const shift = Math.round(dy / (FIND_HOUR_PX / 60) / STEP) * STEP
  const from = Math.min(1440 - Math.min(length, 1440), Math.max(0, differenceInMinutes(start, day) + shift))
  const s = addMinutes(day, from)
  return { start: s, end: addMinutes(s, length) }
}

/** Changes the length by `dy` pixels, at least a quarter of an hour and not past midnight. */
export function resizeSpan(start: Date, end: Date, dy: number): { start: Date; end: Date } {
  const day = startOfDay(start)
  const startMin = differenceInMinutes(start, day)
  const shift = Math.round(dy / (FIND_HOUR_PX / 60) / STEP) * STEP
  const endMin = Math.min(1440, Math.max(startMin + STEP, differenceInMinutes(end, day) + shift))
  return { start, end: addMinutes(day, endMin) }
}
