import { describe, expect, it } from 'vitest'
import {
  buildRrule, defaultRule, describeRule, nthOfMonth, parseRrule, presetIdOf, recurrenceLabel, recurrencePresets,
  type CustomRule,
} from './calendar-recurrence'

// Wednesday 14 October 2026: the second Wednesday of its month.
const WED = new Date(2026, 9, 14)

describe('the ready-made choices', () => {
  it('are worked out from the chosen day and send the matching rule', () => {
    const presets = recurrencePresets(WED)
    expect(presets.map((p) => [p.label, p.rrule])).toEqual([
      ['Không lặp lại', null],
      ['Hằng ngày', 'FREQ=DAILY'],
      ['Hằng tuần vào thứ Tư', 'FREQ=WEEKLY;BYDAY=WE'],
      ['Hằng tháng vào thứ Tư thứ hai', 'FREQ=MONTHLY;BYDAY=2WE'],
      ['Hằng năm vào ngày 14 tháng 10', 'FREQ=YEARLY'],
      ['Mọi ngày trong tuần (thứ Hai đến thứ Sáu)', 'FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR'],
      ['Tùy chọn…', null],
    ])
  })

  it('follow the day: a Sunday and the last Friday of a month', () => {
    expect(recurrencePresets(new Date(2026, 9, 18)).find((p) => p.id === 'weekly')).toMatchObject({ label: 'Hằng tuần vào Chủ nhật', rrule: 'FREQ=WEEKLY;BYDAY=SU' })
    const lastFriday = new Date(2026, 9, 30)
    expect(nthOfMonth(lastFriday)).toBe(-1)
    expect(recurrencePresets(lastFriday).find((p) => p.id === 'monthly')).toMatchObject({ label: 'Hằng tháng vào thứ Sáu cuối cùng', rrule: 'FREQ=MONTHLY;BYDAY=-1FR' })
  })

  it('are recognised again from a stored rule, else the rule is custom', () => {
    expect(presetIdOf(null, WED)).toBe('none')
    expect(presetIdOf('FREQ=WEEKLY;BYDAY=WE', WED)).toBe('weekly')
    expect(presetIdOf('RRULE:FREQ=DAILY', WED)).toBe('daily')
    expect(presetIdOf('FREQ=WEEKLY;BYDAY=WE,FR', WED)).toBe('custom')
    expect(recurrenceLabel('FREQ=MONTHLY;BYDAY=2WE', WED)).toBe('Hằng tháng vào thứ Tư thứ hai')
  })
})

describe('the custom rule', () => {
  const rule = (over: Partial<CustomRule>): CustomRule => ({ ...defaultRule(WED), ...over })

  it('sends FREQ, INTERVAL and the days, and leaves out what is the default', () => {
    expect(buildRrule(rule({}), WED)).toBe('FREQ=WEEKLY;BYDAY=WE')
    expect(buildRrule(rule({ interval: 2, days: ['FR', 'WE'] }), WED)).toBe('FREQ=WEEKLY;INTERVAL=2;BYDAY=WE,FR')
    expect(buildRrule(rule({ unit: 'day', interval: 3, days: [] }), WED)).toBe('FREQ=DAILY;INTERVAL=3')
    expect(buildRrule(rule({ unit: 'month' }), WED)).toBe('FREQ=MONTHLY')
    expect(buildRrule(rule({ unit: 'year' }), WED)).toBe('FREQ=YEARLY')
  })

  it('falls back to the weekday of the date when no day is left', () => {
    expect(buildRrule(rule({ days: [] }), new Date(2026, 9, 16))).toBe('FREQ=WEEKLY;BYDAY=FR')
  })

  it('ends after N times or on a day', () => {
    expect(buildRrule(rule({ end: { kind: 'count', count: 13 } }), WED)).toBe('FREQ=WEEKLY;BYDAY=WE;COUNT=13')
    const until = buildRrule(rule({ end: { kind: 'until', date: '2026-12-31' } }), WED)
    expect(until).toMatch(/^FREQ=WEEKLY;BYDAY=WE;UNTIL=\d{8}T\d{6}Z$/)
    // The day survives the trip through UTC, whatever the zone of the machine.
    expect(parseRrule(until)?.end).toEqual({ kind: 'until', date: '2026-12-31' })
  })

  it('keeps the count within what the server accepts', () => {
    expect(buildRrule(rule({ end: { kind: 'count', count: 5000 } }), WED)).toContain('COUNT=730')
  })

  it('reads back what it wrote, and refuses what it cannot show', () => {
    const r = rule({ interval: 2, days: ['MO', 'FR'], end: { kind: 'count', count: 6 } })
    expect(parseRrule(buildRrule(r, WED))).toEqual(r)
    expect(parseRrule('FREQ=MONTHLY;BYDAY=2WE')).toBeNull()
    expect(parseRrule('FREQ=HOURLY')).toBeNull()
    expect(parseRrule('FREQ=WEEKLY;BYSETPOS=1')).toBeNull()
  })

  it('is described in words, never as a rule', () => {
    expect(describeRule(rule({ days: ['WE', 'FR'], end: { kind: 'until', date: '2026-12-31' } }), WED)).toBe('Hằng tuần vào thứ Tư và thứ Sáu, đến hết 31/12/2026')
    expect(describeRule(rule({ interval: 2, unit: 'day', end: { kind: 'count', count: 10 } }), WED)).toBe('Mỗi 2 ngày, 10 lần')
    expect(describeRule(rule({ unit: 'month' }), WED)).toBe('Hằng tháng')
    expect(recurrenceLabel('FREQ=WEEKLY;BYDAY=WE,FR', WED)).toBe('Hằng tuần vào thứ Tư và thứ Sáu')
  })
})
