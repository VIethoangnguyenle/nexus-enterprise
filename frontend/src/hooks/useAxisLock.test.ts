import { describe, expect, it } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useAxisLock, LOCK_THRESHOLD } from './useAxisLock'

describe('useAxisLock', () => {
  it('returns null until the threshold is reached', () => {
    const { result } = renderHook(() => useAxisLock())
    act(() => result.current.start(100, 100))
    expect(result.current.update(100 + LOCK_THRESHOLD - 1, 100)).toBeNull()
    expect(result.current.current()).toBeNull()
  })

  it('locks to x when horizontal movement dominates', () => {
    const { result } = renderHook(() => useAxisLock())
    act(() => result.current.start(100, 100))
    const axis = result.current.update(100 + LOCK_THRESHOLD, 101)
    expect(axis).toBe('x')
    expect(result.current.current()).toBe('x')
  })

  it('locks to y when vertical movement dominates', () => {
    const { result } = renderHook(() => useAxisLock())
    act(() => result.current.start(100, 100))
    const axis = result.current.update(101, 100 + LOCK_THRESHOLD)
    expect(axis).toBe('y')
    expect(result.current.current()).toBe('y')
  })

  it('stays locked after the first decision', () => {
    const { result } = renderHook(() => useAxisLock())
    act(() => result.current.start(100, 100))
    result.current.update(100 + LOCK_THRESHOLD, 101) // locks x
    // Large vertical move afterwards: still x
    expect(result.current.update(100, 100 + 200)).toBe('x')
  })

  it('resets the lock', () => {
    const { result } = renderHook(() => useAxisLock())
    act(() => result.current.start(100, 100))
    result.current.update(100 + LOCK_THRESHOLD, 101) // locks x
    act(() => result.current.reset())
    expect(result.current.current()).toBeNull()
  })

  it('returns null when update is called without start', () => {
    const { result } = renderHook(() => useAxisLock())
    expect(result.current.update(200, 200)).toBeNull()
  })

  it('locks to x when dx === dy (horizontal wins ties)', () => {
    const { result } = renderHook(() => useAxisLock())
    act(() => result.current.start(100, 100))
    const axis = result.current.update(100 + LOCK_THRESHOLD, 100 + LOCK_THRESHOLD)
    expect(axis).toBe('x')
  })
})
