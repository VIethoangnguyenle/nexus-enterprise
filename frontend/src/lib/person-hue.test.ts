import { describe, it, expect } from 'vitest'
import { personHue, personStyle } from './person-hue'

describe('personHue', () => {
  it('is deterministic: the same key always lands on the same hue', () => {
    const key = '6f1c2a90-4b7e-4e8e-9c55-1f0d3b2a7c11'
    const first = personHue(key)
    for (let i = 0; i < 20; i++) expect(personHue(key)).toBe(first)
  })

  it('always returns an integer between 1 and 8', () => {
    const keys = ['', 'a', 'b', 'Lê Thị Hoa', 'u-1', 'u-2', 'x'.repeat(500)]
    for (let i = 0; i < 500; i++) keys.push(`user-${i}-${(i * 7919).toString(36)}`)
    for (const k of keys) {
      const h = personHue(k)
      expect(Number.isInteger(h)).toBe(true)
      expect(h).toBeGreaterThanOrEqual(1)
      expect(h).toBeLessThanOrEqual(8)
    }
  })

  it('spreads keys across all eight hues', () => {
    const seen = new Set<number>()
    for (let i = 0; i < 400; i++) seen.add(personHue(`member-${i}`))
    expect(seen.size).toBe(8)
  })

  it('distinguishes keys that differ by one character', () => {
    // Not a guarantee for every pair, but a reasonable hash separates most neighbours.
    let differing = 0
    for (let i = 0; i < 50; i++) if (personHue(`user-${i}a`) !== personHue(`user-${i}b`)) differing++
    expect(differing).toBeGreaterThan(30)
  })
})

describe('personStyle', () => {
  it('exposes the hue and its wash as custom properties, never the key itself', () => {
    const key = 'secret-user-id-123'
    const style = personStyle(key) as Record<string, string>
    const n = personHue(key)
    expect(style['--p']).toBe(`var(--color-person-${n})`)
    expect(style['--pw']).toBe(`var(--color-person-${n}-wash)`)
    expect(JSON.stringify(style)).not.toContain(key)
  })
})
