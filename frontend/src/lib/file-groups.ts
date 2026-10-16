import { fileKind } from './file-kind'

/** How recent a file is, in the words the Tệp tab uses for its headings. */
export type FileGroupKey = 'today' | 'week' | 'month' | 'older'

export const FILE_GROUP_LABELS: Record<FileGroupKey, string> = {
  today: 'Hôm nay',
  week: 'Tuần này',
  month: 'Tháng này',
  older: 'Cũ hơn',
}

const ORDER: FileGroupKey[] = ['today', 'week', 'month', 'older']
const DAY_MS = 86_400_000

/** The calendar date `ms` falls on in `timeZone`, as a day number since 1970 plus its month key. */
function calendarDay(ms: number, timeZone?: string): { day: number; month: number } {
  const parts = new Intl.DateTimeFormat('en-US', { timeZone, year: 'numeric', month: 'numeric', day: 'numeric' })
    .formatToParts(new Date(ms))
  const get = (type: string) => Number(parts.find((p) => p.type === type)?.value)
  const y = get('year')
  const m = get('month')
  return { day: Date.UTC(y, m - 1, get('day')) / DAY_MS, month: y * 12 + (m - 1) }
}

/**
 * Which group a moment belongs to, judged on the calendar of `timeZone` (the
 * user's own when omitted): today; earlier this week (Monday first); earlier
 * this month; anything older. A moment ahead of the clock counts as today.
 */
export function fileGroupOf(ms: number, now: Date = new Date(), timeZone?: string): FileGroupKey {
  if (!Number.isFinite(ms)) return 'older'
  const then = calendarDay(ms, timeZone)
  const today = calendarDay(now.getTime(), timeZone)
  if (then.day >= today.day) return 'today'
  // 1970-01-01 was a Thursday, so (day + 3) % 7 counts days since Monday.
  const sinceMonday = (today.day + 3) % 7
  if (then.day >= today.day - sinceMonday) return 'week'
  if (then.month === today.month) return 'month'
  return 'older'
}

export interface FileGroup<T> {
  key: FileGroupKey
  label: string
  items: T[]
}

/** Splits items into the non-empty groups, newest group first; order inside a group is the caller's. */
export function groupFiles<T>(
  items: readonly T[],
  timeOf: (item: T) => number,
  now: Date = new Date(),
  timeZone?: string,
): FileGroup<T>[] {
  const buckets = new Map<FileGroupKey, T[]>()
  for (const item of items) {
    const key = fileGroupOf(timeOf(item), now, timeZone)
    const bucket = buckets.get(key)
    if (bucket) bucket.push(item)
    else buckets.set(key, [item])
  }
  return ORDER.filter((k) => buckets.has(k)).map((key) => ({ key, label: FILE_GROUP_LABELS[key], items: buckets.get(key)! }))
}

/** A picture or a video: what a group of may be laid out as a tight square album. */
export function isMediaFile(name: string, mime?: string): boolean {
  const label = fileKind(name, mime).label
  return label === 'Ảnh' || label === 'Video'
}
