import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { BURST_MIN, REALTIME_MS, useArrivals } from './useArrivals'

interface Item { id: string; by: string }
const keyOf = (i: Item) => i.id
const authorOf = (i: Item) => i.by

function setup(initial: Item[], opts: { ready?: boolean; me?: string; scope?: string } = {}) {
  return renderHook(
    ({ items, ready, scope }) => useArrivals(items, { keyOf, authorOf, me: opts.me ?? 'me', ready, scope }),
    { initialProps: { items: initial, ready: opts.ready ?? true, scope: opts.scope } },
  )
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('useArrivals', () => {
  it('treats everything present at the first ready render as history', () => {
    const { result } = setup([{ id: 'a', by: 'bob' }])
    expect(result.current.fresh.size).toBe(0)
  })

  it('does not seed history while the list is still loading', () => {
    const { result, rerender } = setup([], { ready: false })
    rerender({ items: [{ id: 'a', by: 'bob' }], ready: false, scope: undefined })
    rerender({ items: [{ id: 'a', by: 'bob' }], ready: true, scope: undefined })
    expect(result.current.fresh.size).toBe(0)
  })

  it('marks a later arrival by someone else as other, and by me as mine', () => {
    const { result, rerender } = setup([{ id: 'a', by: 'bob' }])
    rerender({
      items: [{ id: 'a', by: 'bob' }, { id: 'b', by: 'bob' }, { id: 'c', by: 'me' }],
      ready: true,
      scope: undefined,
    })
    expect(result.current.fresh.get('b')?.source).toBe('other')
    expect(result.current.fresh.get('c')?.source).toBe('mine')
    expect(result.current.burst).toBeNull()
  })

  it('forgets the wash after the realtime window', () => {
    const { result, rerender } = setup([])
    rerender({ items: [{ id: 'b', by: 'bob' }], ready: true, scope: undefined })
    expect(result.current.fresh.size).toBe(1)
    act(() => { vi.advanceTimersByTime(REALTIME_MS) })
    expect(result.current.fresh.size).toBe(0)
  })

  it('coalesces three or more changes from others into one burst', () => {
    const { result, rerender } = setup([])
    const items = Array.from({ length: BURST_MIN }, (_, i) => ({ id: `n${i}`, by: i % 2 ? 'bob' : 'cam' }))
    rerender({ items, ready: true, scope: undefined })
    expect(result.current.burst?.count).toBe(BURST_MIN)
    expect([...(result.current.burst?.authors ?? [])].sort()).toEqual(['bob', 'cam'])
    act(() => { vi.advanceTimersByTime(REALTIME_MS) })
    expect(result.current.burst).toBeNull()
  })

  it('does not count my own changes toward a burst', () => {
    const { result, rerender } = setup([])
    rerender({ items: Array.from({ length: 4 }, (_, i) => ({ id: `m${i}`, by: 'me' })), ready: true, scope: undefined })
    expect(result.current.burst).toBeNull()
  })

  it('resets what was seen when the scope changes', () => {
    const { result, rerender } = setup([{ id: 'a', by: 'bob' }], { scope: 'ch1' })
    rerender({ items: [{ id: 'z', by: 'bob' }], ready: true, scope: 'ch2' })
    expect(result.current.fresh.size).toBe(0)
  })
})
