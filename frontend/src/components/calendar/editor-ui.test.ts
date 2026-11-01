import { describe, expect, it } from 'vitest'
import { zoneLabel, zoneNames } from './editor-ui'

/** Minutes east of GMT read back from a label, e.g. "(GMT+5:30) Kolkata" → 330. */
function offsetOf(label: string): number {
  const m = /^\(GMT([+-])(\d{1,2})(?::(\d{2}))?\)/.exec(label)
  if (!m) throw new Error(`no offset in ${label}`)
  const mins = Number(m[2]) * 60 + Number(m[3] ?? 0)
  return m[1] === '-' ? -mins : mins
}

describe('zoneLabel', () => {
  it("names Vietnam's zone by its offset and Hà Nội, under either id", () => {
    expect(zoneLabel('Asia/Ho_Chi_Minh')).toBe('(GMT+7) Hà Nội')
    expect(zoneLabel('Asia/Saigon')).toBe('(GMT+7) Hà Nội')
  })

  it('writes a half-hour offset with its minutes and falls back to the place in the id', () => {
    expect(zoneLabel('Asia/Kolkata')).toBe('(GMT+5:30) Kolkata')
  })

  it('reads UTC as GMT+0 with a Vietnamese name', () => {
    expect(zoneLabel('UTC')).toBe('(GMT+0) Giờ quốc tế (UTC)')
  })

  it('turns underscores into spaces for a zone outside the map', () => {
    expect(zoneLabel('America/Argentina/Buenos_Aires')).toBe('(GMT-3) Buenos Aires')
  })

  it('never throws on a zone the browser does not know, and still shows no raw path', () => {
    expect(() => zoneLabel('Mars/Olympus_Mons')).not.toThrow()
    expect(zoneLabel('Mars/Olympus_Mons')).toBe('Olympus Mons')
    expect(() => zoneLabel('')).not.toThrow()
  })
})

describe('zoneNames', () => {
  it('lists the zones by offset, then by name', () => {
    const labels = zoneNames('Asia/Ho_Chi_Minh').map(zoneLabel)
    for (let i = 1; i < labels.length; i++) {
      const [a, b] = [labels[i - 1]!, labels[i]!]
      const [oa, ob] = [offsetOf(a), offsetOf(b)]
      expect(oa < ob || (oa === ob && a.localeCompare(b, 'vi') <= 0), `${a} before ${b}`).toBe(true)
    }
  })

  it('always holds the zone in use, and no two entries read the same', () => {
    const names = zoneNames('Asia/Ho_Chi_Minh')
    expect(names).toContain('Asia/Ho_Chi_Minh')
    const labels = names.map(zoneLabel)
    expect(new Set(labels).size).toBe(labels.length)
    expect(labels.filter((l) => l === '(GMT+7) Hà Nội')).toHaveLength(1)
  })

  it('keeps an unknown zone in use rather than dropping it', () => {
    expect(zoneNames('Mars/Olympus_Mons')).toContain('Mars/Olympus_Mons')
  })
})
