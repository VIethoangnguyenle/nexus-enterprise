import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useDelayedFlag } from './useDelayedFlag'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('useDelayedFlag', () => {
  it('stays false for a flag that comes and goes inside the delay', () => {
    const { result, rerender } = renderHook(({ on }) => useDelayedFlag(on, 300), { initialProps: { on: true } })
    act(() => { vi.advanceTimersByTime(299) })
    expect(result.current).toBe(false)
    rerender({ on: false })
    act(() => { vi.advanceTimersByTime(500) })
    expect(result.current).toBe(false)
  })

  it('turns true once the flag has held for the delay, and false the moment it drops', () => {
    const { result, rerender } = renderHook(({ on }) => useDelayedFlag(on, 300), { initialProps: { on: true } })
    act(() => { vi.advanceTimersByTime(300) })
    expect(result.current).toBe(true)
    rerender({ on: false })
    expect(result.current).toBe(false)
  })
})
