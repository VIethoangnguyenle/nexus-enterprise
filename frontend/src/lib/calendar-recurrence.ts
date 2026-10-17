import { endOfDay, format, getDate, getDay, getMonth } from 'date-fns'
import { parseDateKey, toDateKey } from './calendar-format'

/** RFC 5545 weekday codes, Monday first like the rest of the app. */
export const WEEKDAY_CODES = ['MO', 'TU', 'WE', 'TH', 'FR', 'SA', 'SU'] as const
export type WeekdayCode = (typeof WEEKDAY_CODES)[number]

const CODE_OF_JS_DAY: WeekdayCode[] = ['SU', 'MO', 'TU', 'WE', 'TH', 'FR', 'SA']
/** The chip names of the custom dialog. */
export const WEEKDAY_CHIP: Record<WeekdayCode, string> = { MO: 'T2', TU: 'T3', WE: 'T4', TH: 'T5', FR: 'T6', SA: 'T7', SU: 'CN' }
const WEEKDAY_NAME: Record<WeekdayCode, string> = {
  MO: 'thứ Hai', TU: 'thứ Ba', WE: 'thứ Tư', TH: 'thứ Năm', FR: 'thứ Sáu', SA: 'thứ Bảy', SU: 'Chủ nhật',
}
const NTH_WORD: Record<number, string> = { 1: 'thứ nhất', 2: 'thứ hai', 3: 'thứ ba', 4: 'thứ tư', [-1]: 'cuối cùng' }

export const weekdayCodeOf = (d: Date): WeekdayCode => CODE_OF_JS_DAY[getDay(d)]!

/** Which weekday of the month the date is: 1..4, or -1 for the fifth (the last one). */
export const nthOfMonth = (d: Date): number => {
  const n = Math.ceil(getDate(d) / 7)
  return n >= 5 ? -1 : n
}

// --- The ready-made choices ----------------------------------------------------

export type PresetId = 'none' | 'daily' | 'weekly' | 'monthly' | 'yearly' | 'weekdays' | 'custom'

export interface RecurrencePreset {
  id: PresetId
  label: string
  /** The rule this choice sends; `null` is no recurrence, and custom has none until the dialog makes one. */
  rrule: string | null
}

/** The choices of the Lặp lại select, worked out from the day the event is on. */
export function recurrencePresets(date: Date): RecurrencePreset[] {
  const code = weekdayCodeOf(date)
  const nth = nthOfMonth(date)
  return [
    { id: 'none', label: 'Không lặp lại', rrule: null },
    { id: 'daily', label: 'Hằng ngày', rrule: 'FREQ=DAILY' },
    { id: 'weekly', label: `Hằng tuần vào ${WEEKDAY_NAME[code]}`, rrule: `FREQ=WEEKLY;BYDAY=${code}` },
    { id: 'monthly', label: `Hằng tháng vào ${WEEKDAY_NAME[code]} ${NTH_WORD[nth]}`, rrule: `FREQ=MONTHLY;BYDAY=${nth}${code}` },
    { id: 'yearly', label: `Hằng năm vào ngày ${getDate(date)} tháng ${getMonth(date) + 1}`, rrule: 'FREQ=YEARLY' },
    { id: 'weekdays', label: 'Mọi ngày trong tuần (thứ Hai đến thứ Sáu)', rrule: 'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR' },
    { id: 'custom', label: 'Tùy chọn…', rrule: null },
  ]
}

// --- The custom rule -------------------------------------------------------------

export type RepeatUnit = 'day' | 'week' | 'month' | 'year'
export type RuleEnd = { kind: 'never' } | { kind: 'until'; date: string } | { kind: 'count'; count: number }

export interface CustomRule {
  interval: number
  unit: RepeatUnit
  /** Weekly only. */
  days: WeekdayCode[]
  end: RuleEnd
}

export const UNIT_LABEL: Record<RepeatUnit, string> = { day: 'ngày', week: 'tuần', month: 'tháng', year: 'năm' }
const FREQ_OF: Record<RepeatUnit, string> = { day: 'DAILY', week: 'WEEKLY', month: 'MONTHLY', year: 'YEARLY' }
const UNIT_OF: Record<string, RepeatUnit> = { DAILY: 'day', WEEKLY: 'week', MONTHLY: 'month', YEARLY: 'year' }
/** The server accepts COUNT up to this, INTERVAL up to `MAX_INTERVAL`. */
export const MAX_COUNT = 730
export const MAX_INTERVAL = 1000

const sortDays = (days: WeekdayCode[]) => [...new Set(days)].sort((a, b) => WEEKDAY_CODES.indexOf(a) - WEEKDAY_CODES.indexOf(b))

/** UNTIL is the end of the chosen local day, written in UTC (RFC 5545). */
function untilOf(dateKey: string): string | null {
  const day = parseDateKey(dateKey)
  if (!day) return null
  return endOfDay(day).toISOString().replace(/[-:]/g, '').replace(/\.\d{3}/, '')
}

/** The local day an UNTIL names (date or date-time form). */
function dayOfUntil(until: string): string | null {
  const m = until.match(/^(\d{4})(\d{2})(\d{2})(?:T(\d{2})(\d{2})(\d{2})(Z)?)?$/)
  if (!m) return null
  const [, y, mo, d, h, mi, s, z] = m
  const at = h === undefined
    ? new Date(Number(y), Number(mo) - 1, Number(d))
    : z ? new Date(Date.UTC(Number(y), Number(mo) - 1, Number(d), Number(h), Number(mi), Number(s))) : new Date(Number(y), Number(mo) - 1, Number(d), Number(h), Number(mi), Number(s))
  return Number.isNaN(at.getTime()) ? null : toDateKey(at)
}

/** The RRULE body of a custom rule. A weekly rule with no day falls back to `fallbackDay`. */
export function buildRrule(rule: CustomRule, fallbackDay: Date): string {
  const parts = [`FREQ=${FREQ_OF[rule.unit]}`]
  const interval = Math.min(MAX_INTERVAL, Math.max(1, Math.floor(rule.interval) || 1))
  if (interval > 1) parts.push(`INTERVAL=${interval}`)
  if (rule.unit === 'week') {
    const days = rule.days.length > 0 ? sortDays(rule.days) : [weekdayCodeOf(fallbackDay)]
    parts.push(`BYDAY=${days.join(',')}`)
  }
  if (rule.end.kind === 'count') parts.push(`COUNT=${Math.min(MAX_COUNT, Math.max(1, Math.floor(rule.end.count) || 1))}`)
  if (rule.end.kind === 'until') {
    const until = untilOf(rule.end.date)
    if (until) parts.push(`UNTIL=${until}`)
  }
  return parts.join(';')
}

/** Reads a rule back into the dialog; null when it uses parts the dialog cannot show (it is kept as it is). */
export function parseRrule(rrule: string): CustomRule | null {
  const map = new Map<string, string>()
  for (const part of rrule.replace(/^RRULE:/i, '').split(';')) {
    const [k, v] = part.split('=')
    if (!k || v === undefined) return null
    map.set(k.toUpperCase(), v)
  }
  const unit = UNIT_OF[map.get('FREQ') ?? '']
  if (!unit) return null
  for (const k of map.keys()) if (!['FREQ', 'INTERVAL', 'BYDAY', 'COUNT', 'UNTIL'].includes(k)) return null
  const interval = map.has('INTERVAL') ? Number(map.get('INTERVAL')) : 1
  if (!Number.isInteger(interval) || interval < 1) return null

  let days: WeekdayCode[] = []
  if (map.has('BYDAY')) {
    if (unit !== 'week') return null
    days = map.get('BYDAY')!.split(',') as WeekdayCode[]
    if (days.some((d) => !WEEKDAY_CODES.includes(d))) return null
  }
  let end: RuleEnd = { kind: 'never' }
  if (map.has('COUNT')) {
    const count = Number(map.get('COUNT'))
    if (!Number.isInteger(count) || count < 1) return null
    end = { kind: 'count', count }
  } else if (map.has('UNTIL')) {
    const date = dayOfUntil(map.get('UNTIL')!)
    if (!date) return null
    end = { kind: 'until', date }
  }
  return { interval, unit, days: sortDays(days), end }
}

const joinDays = (days: WeekdayCode[]) => {
  const names = days.map((d) => WEEKDAY_NAME[d])
  return names.length <= 1 ? (names[0] ?? '') : `${names.slice(0, -1).join(', ')} và ${names[names.length - 1]}`
}

/** The rule in words, under the dialog: "Hằng tuần vào thứ Tư và thứ Sáu, đến hết 31/12/2026". */
export function describeRule(rule: CustomRule, fallbackDay: Date): string {
  const every = rule.interval > 1 ? `Mỗi ${rule.interval} ${UNIT_LABEL[rule.unit]}` : `Hằng ${UNIT_LABEL[rule.unit]}`
  let text = every
  if (rule.unit === 'week') {
    const days = rule.days.length > 0 ? sortDays(rule.days) : [weekdayCodeOf(fallbackDay)]
    text += ` vào ${joinDays(days)}`
  }
  if (rule.end.kind === 'count') text += `, ${rule.end.count} lần`
  if (rule.end.kind === 'until') {
    const day = parseDateKey(rule.end.date)
    if (day) text += `, đến hết ${format(day, 'dd/MM/yyyy')}`
  }
  return text
}

/** Which choice of the select a stored rule is: a ready-made one when it matches exactly, else custom. */
export function presetIdOf(rrule: string | null | undefined, date: Date): PresetId {
  if (!rrule) return 'none'
  const body = rrule.replace(/^RRULE:/i, '')
  return recurrencePresets(date).find((p) => p.rrule === body)?.id ?? 'custom'
}

/** The label the select shows for a stored rule: the matching choice, else the rule in words. */
export function recurrenceLabel(rrule: string | null | undefined, date: Date): string {
  const id = presetIdOf(rrule, date)
  if (id !== 'custom') return recurrencePresets(date).find((p) => p.id === id)!.label
  const rule = rrule ? parseRrule(rrule) : null
  return rule ? describeRule(rule, date) : 'Tùy chọn…'
}

/** The dialog's first state for a date with no rule yet: every week on that day, never ending. */
export const defaultRule = (date: Date): CustomRule => ({ interval: 1, unit: 'week', days: [weekdayCodeOf(date)], end: { kind: 'never' } })
