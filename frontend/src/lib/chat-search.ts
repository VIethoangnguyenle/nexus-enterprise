/**
 * What Chat's search results can be narrowed by (DESIGN.md "Gợi ý trong ô tìm", Trang kết quả). The filters and the
 * words live in the address, so back, forward and reload keep them; the keys there are short and never reach the
 * screen, which shows names and dates only.
 */

export interface ChatSearchFilters {
  /** Who wrote it: a user id, picked from the directory. */
  from?: string
  /** Where it was said: a conversation id, picked from the list the person reads. */
  channelId?: string
  /** First day, `YYYY-MM-DD` in the reader's zone. */
  after?: string
  /** Last day, inclusive, `YYYY-MM-DD` in the reader's zone. */
  beforeDate?: string
  hasFile?: boolean
  hasLink?: boolean
  mentionsMe?: boolean
}

/** The words and filters as the address carries them (`/channels/search?q=…`). */
export interface ChatSearchUrl {
  q?: string
  from?: string
  in?: string
  since?: string
  until?: string
  file?: 1
  link?: 1
  me?: 1
}

const KEYS = ['q', 'from', 'in', 'since', 'until', 'file', 'link', 'me'] as const satisfies readonly (keyof ChatSearchUrl)[]
/** The address keys, for a route that merges them into its own search. */
export const CHAT_SEARCH_KEYS: readonly (keyof ChatSearchUrl)[] = KEYS

const pad = (n: number) => String(n).padStart(2, '0')

/** A day as `YYYY-MM-DD` in the reader's own zone (not UTC, which would move a late evening to the next day). */
export const isoDate = (d: Date): string => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

const DAY = /^(\d{4})-(\d{2})-(\d{2})$/

/** A real calendar day, not just the right shape. */
function validDay(s: unknown): s is string {
  if (typeof s !== 'string') return false
  const m = DAY.exec(s)
  if (!m) return false
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])]
  const at = new Date(y, mo - 1, d)
  return at.getFullYear() === y && at.getMonth() === mo - 1 && at.getDate() === d
}

const word = (v: unknown): string | undefined => (typeof v === 'string' && v.trim() ? v.trim() : undefined)
const flag = (v: unknown): 1 | undefined => (v === 1 || v === '1' || v === true ? 1 : undefined)

/** `validateSearch` for the keys above: anything malformed is dropped rather than shown as an error page. */
export function parseChatSearch(raw: Record<string, unknown>): ChatSearchUrl {
  const out: ChatSearchUrl = {}
  const q = word(raw.q)
  if (q) out.q = q
  const from = word(raw.from)
  if (from) out.from = from
  const channel = word(raw.in)
  if (channel) out.in = channel
  if (validDay(raw.since)) out.since = raw.since
  if (validDay(raw.until)) out.until = raw.until
  if (flag(raw.file)) out.file = 1
  if (flag(raw.link)) out.link = 1
  if (flag(raw.me)) out.me = 1
  return out
}

export function filtersFromUrl(url: ChatSearchUrl): ChatSearchFilters {
  const f: ChatSearchFilters = {}
  if (url.from) f.from = url.from
  if (url.in) f.channelId = url.in
  if (url.since) f.after = url.since
  if (url.until) f.beforeDate = url.until
  if (url.file) f.hasFile = true
  if (url.link) f.hasLink = true
  if (url.me) f.mentionsMe = true
  return f
}

/** The address keys for words and filters; a filter that is off has no key. */
export function urlFromFilters(q: string, f: ChatSearchFilters): ChatSearchUrl {
  const url: ChatSearchUrl = {}
  const words = q.trim()
  if (words) url.q = words
  if (f.from) url.from = f.from
  if (f.channelId) url.in = f.channelId
  if (f.after) url.since = f.after
  if (f.beforeDate) url.until = f.beforeDate
  if (f.hasFile) url.file = 1
  if (f.hasLink) url.link = 1
  if (f.mentionsMe) url.me = 1
  return url
}

/** Filters in force; a date range, however many ends it has, is one. */
export function activeFilterCount(f: ChatSearchFilters): number {
  return [f.from, f.channelId, f.after || f.beforeDate, f.hasFile, f.hasLink, f.mentionsMe].filter(Boolean).length
}

export type DatePreset = 'today' | 'week' | 'month'
const PRESET_DAYS: Record<DatePreset, number> = { today: 1, week: 7, month: 30 }

/** The last `n` days, today included: `week` is today and the six before it. */
export function presetRange(preset: DatePreset, today: Date): Pick<ChatSearchFilters, 'after' | 'beforeDate'> {
  const from = new Date(today.getFullYear(), today.getMonth(), today.getDate() - (PRESET_DAYS[preset] - 1))
  return { after: isoDate(from), beforeDate: isoDate(today) }
}

/** `01/10`, or `01/10/2025` when the range reaches outside the current year. */
function shortDay(iso: string, withYear: boolean): string {
  const [y, m, d] = iso.split('-')
  return withYear ? `${d}/${m}/${y}` : `${d}/${m}`
}

/** What the Ngày chip says: the preset's own name when the range is exactly one, otherwise the days. */
export function describeRange(r: Pick<ChatSearchFilters, 'after' | 'beforeDate'>, today: Date): string {
  for (const [preset, label] of [['today', 'Hôm nay'], ['week', '7 ngày qua'], ['month', '30 ngày qua']] as const) {
    const p = presetRange(preset, today)
    if (p.after === r.after && p.beforeDate === r.beforeDate) return label
  }
  const year = String(today.getFullYear())
  const withYear = [r.after, r.beforeDate].some((d) => d !== undefined && !d.startsWith(year))
  const a = r.after && shortDay(r.after, withYear)
  const b = r.beforeDate && shortDay(r.beforeDate, withYear)
  if (a && b) return `${a} – ${b}`
  if (a) return `Từ ${a}`
  if (b) return `Đến ${b}`
  return ''
}
