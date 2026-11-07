import { describe, expect, it } from 'vitest'
import { reconnectDelay } from './websocket.store'

describe('reconnectDelay', () => {
  it('spreads the first reconnect over five seconds', () => {
    expect(reconnectDelay(0, () => 0)).toBe(250)
    expect(reconnectDelay(0, () => 0.999)).toBeCloseTo(250 + 4995, 0)
  })

  it('doubles the spread on each attempt, up to thirty seconds', () => {
    expect(reconnectDelay(1, () => 1)).toBe(250 + 10000)
    expect(reconnectDelay(2, () => 1)).toBe(250 + 20000)
    expect(reconnectDelay(5, () => 1)).toBe(250 + 30000)
  })

  it('lands sessions dropped together at different moments', () => {
    let seed = 1
    const random = () => ((seed = (seed * 16807) % 2147483647) / 2147483647)
    const delays = Array.from({ length: 1000 }, () => reconnectDelay(0, random))
    const perSecond = new Map<number, number>()
    for (const d of delays) perSecond.set(Math.floor(d / 1000), (perSecond.get(Math.floor(d / 1000)) ?? 0) + 1)
    // 1,000 sessions over 5 s: no second takes much more than its share of 200.
    expect(Math.max(...perSecond.values())).toBeLessThan(260)
  })
})
