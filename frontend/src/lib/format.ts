/**
 * Vietnamese display formats (DESIGN.md §8). Every date, time, size and amount
 * on screen goes through here so the app speaks one dialect:
 *   time 09:14 · date 09/10/2026 · relative "5 phút trước" / "Hôm qua"
 *   size 412 KB · money 12.450.000 ₫
 */

const pad = (n: number) => String(n).padStart(2, '0')

/**
 * Reads the timestamp shapes the backend emits: ISO strings, protobuf
 * Timestamps serialised as `{ seconds, nanos }` (seconds may be a string when
 * int64 crosses JSON), unix seconds, or a Date.
 */
export function toDate(value: unknown): Date | null {
  if (value == null || value === '') return null
  let d: Date
  if (value instanceof Date) d = new Date(value.getTime())
  else if (typeof value === 'string') d = new Date(value)
  else if (typeof value === 'number') d = new Date(value * 1000)
  else if (typeof value === 'object' && 'seconds' in (value as object)) {
    const { seconds, nanos } = value as { seconds: number | string; nanos?: number }
    d = new Date(Number(seconds) * 1000 + Math.floor((nanos ?? 0) / 1e6))
  } else return null
  return isNaN(d.getTime()) ? null : d
}

/** Milliseconds since epoch, or 0 when unreadable. Handy for sorting. */
export function toMillis(value: unknown): number {
  return toDate(value)?.getTime() ?? 0
}

/** `09:14` */
export function formatTime(value: unknown): string {
  const d = toDate(value)
  return d ? `${pad(d.getHours())}:${pad(d.getMinutes())}` : ''
}

/** `09/10/2026` */
export function formatDate(value: unknown): string {
  const d = toDate(value)
  return d ? `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}` : ''
}

/** `09/10/2026 09:14`, for tooltips that back a relative label. */
export function formatDateTime(value: unknown): string {
  const d = toDate(value)
  return d ? `${formatDate(d)} ${formatTime(d)}` : ''
}

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

function dayDiff(d: Date, now: Date): number {
  return Math.round((startOfDay(now) - startOfDay(d)) / 86_400_000)
}

/** "Vừa xong", "5 phút trước", "3 giờ trước", "Hôm qua", then the date. */
export function formatRelative(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  const diffMin = Math.floor((now.getTime() - d.getTime()) / 60_000)
  const days = dayDiff(d, now)
  if (days === 0) {
    if (diffMin < 1) return 'Vừa xong'
    if (diffMin < 60) return `${diffMin} phút trước`
    return `${Math.floor(diffMin / 60)} giờ trước`
  }
  if (days === 1) return 'Hôm qua'
  return formatDate(d)
}

/** Compact stamp for list rows: `09:20` today, "Hôm qua", `07/10`, `31/12/2025`. */
export function formatListTime(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  const days = dayDiff(d, now)
  if (days === 0) return formatTime(d)
  if (days === 1) return 'Hôm qua'
  if (d.getFullYear() === now.getFullYear()) return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
  return formatDate(d)
}

const decimal = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 })
const integer = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 0 })

/** `412 KB`, `1,5 MB`. Binary units, Vietnamese decimal comma. */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = bytes
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${i === 0 ? integer.format(v) : decimal.format(v)} ${units[i]}`
}

/** `12.450.000 ₫` */
export function formatMoney(amount: number): string {
  return `${integer.format(Math.round(amount))} ₫`
}

/** Counter pill text: `3`, `99`, `99+`. */
export function formatCount(n: number): string {
  return n > 99 ? '99+' : String(n)
}
