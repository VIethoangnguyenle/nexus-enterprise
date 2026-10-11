import { describe, expect, it } from 'vitest'
import { feedItemMotion } from './feed-motion'

describe('feedItemMotion', () => {
  it('grows a new item in and collapses a removed one over 220ms', () => {
    const m = feedItemMotion(false, true)
    expect(m.initial).toEqual({ opacity: 0, height: 0 })
    expect(m.animate).toMatchObject({ opacity: 1, height: 'auto', transition: { duration: 0.22 } })
    expect(m.exit).toMatchObject({ opacity: 0, height: 0, transition: { duration: 0.22 } })
  })

  it('does not animate an item that is just there when it mounts', () => {
    expect(feedItemMotion(false, false).initial).toBe(false)
    expect(feedItemMotion(true, false).initial).toBe(false)
  })

  it('under reduced motion only fades, in at most 120ms, and never moves the height', () => {
    const m = feedItemMotion(true, true)
    expect(m.initial).toEqual({ opacity: 0 })
    for (const state of [m.animate, m.exit]) {
      expect(state).not.toHaveProperty('height')
      expect((state.transition as { duration: number }).duration).toBeLessThanOrEqual(0.12)
    }
  })
})
