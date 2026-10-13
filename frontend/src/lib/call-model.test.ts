import { describe, it, expect } from 'vitest'
import {
  encodeMessage, formatElapsed, gridFor, layoutFor, pageCount, pageOf, parseHand, parseReaction, parseShare,
} from './call-model'

describe('layoutFor', () => {
  it('shows a lone person large', () => {
    expect(layoutFor(1)).toMatchObject({ kind: 'single' })
  })

  it('shows the other person large and you small when two are on stage', () => {
    expect(layoutFor(2)).toMatchObject({ kind: 'pip' })
  })

  it.each([[3, 2], [4, 2], [5, 3], [9, 3], [10, 4], [20, 4]])('puts %i people in %i columns', (tiles, cols) => {
    expect(layoutFor(tiles)).toMatchObject({ kind: 'grid', cols })
  })

  it('pages 20 people on a 4-column grid into two pages', () => {
    const l = gridFor(20)
    expect(l.perPage).toBe(12)
    expect(pageCount(20, l.perPage)).toBe(2)
  })

  it('does not page a grid that fits', () => {
    expect(pageCount(9, gridFor(9).perPage)).toBe(1)
  })
})

describe('pageOf', () => {
  const items = Array.from({ length: 20 }, (_, i) => i)
  it('slices a page and clamps the page number', () => {
    expect(pageOf(items, 0, 12)).toHaveLength(12)
    expect(pageOf(items, 1, 12)).toEqual([12, 13, 14, 15, 16, 17, 18, 19])
    expect(pageOf(items, 9, 12)).toEqual([12, 13, 14, 15, 16, 17, 18, 19])
    expect(pageOf(items, -3, 12)[0]).toBe(0)
  })
})

describe('formatElapsed', () => {
  it('reads minutes and seconds, then hours', () => {
    expect(formatElapsed(0)).toBe('00:00')
    expect(formatElapsed(252_000)).toBe('04:12')
    expect(formatElapsed(3_725_000)).toBe('1:02:05')
    expect(formatElapsed(-5)).toBe('00:00')
  })
})

describe('data messages', () => {
  it('reads a hand message and rejects anything else', () => {
    expect(parseHand(encodeMessage({ raised: true, at: 5 }))).toEqual({ raised: true, at: 5 })
    expect(parseHand(encodeMessage({ raised: 'yes' }))).toBeNull()
    expect(parseHand(new TextEncoder().encode('not json'))).toBeNull()
    expect(parseHand(new Uint8Array(0))).toBeNull()
    expect(parseHand(encodeMessage([1, 2]))).toBeNull()
  })

  it('lets only the six reactions through', () => {
    expect(parseReaction(encodeMessage({ emoji: '👏' }))).toEqual({ emoji: '👏' })
    expect(parseReaction(encodeMessage({ emoji: '<script>' }))).toBeNull()
    expect(parseReaction(encodeMessage({ emoji: '💩' }))).toBeNull()
  })

  it('refuses an oversized payload', () => {
    expect(parseHand(encodeMessage({ raised: true, pad: 'x'.repeat(600) }))).toBeNull()
  })

  it('reads a share start', () => {
    expect(parseShare(encodeMessage({ action: 'start' }))).toEqual({ action: 'start' })
    expect(parseShare(encodeMessage({ action: 'stop' }))).toBeNull()
  })
})
