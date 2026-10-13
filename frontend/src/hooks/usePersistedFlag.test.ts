import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { usePersistedFlag } from './usePersistedFlag'

beforeEach(() => localStorage.clear())
afterEach(() => vi.restoreAllMocks())

describe('usePersistedFlag', () => {
  it('starts off, and remembers an on switch on this device', () => {
    const { result, unmount } = renderHook(() => usePersistedFlag('k'))
    expect(result.current[0]).toBe(false)
    act(() => result.current[1](true))
    expect(result.current[0]).toBe(true)
    unmount()
    expect(renderHook(() => usePersistedFlag('k')).result.current[0]).toBe(true)
  })

  it('keeps one flag per key', () => {
    renderHook(() => usePersistedFlag('a')).result.current[1](true)
    expect(renderHook(() => usePersistedFlag('b')).result.current[0]).toBe(false)
  })

  it('follows a key that changes (another person signs in)', () => {
    localStorage.setItem('a', '1')
    const { result, rerender } = renderHook(({ k }) => usePersistedFlag(k), { initialProps: { k: 'a' } })
    expect(result.current[0]).toBe(true)
    rerender({ k: 'b' })
    expect(result.current[0]).toBe(false)
  })

  it('works, without remembering, when storage is blocked', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new Error('blocked') })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('full') })
    const { result } = renderHook(() => usePersistedFlag('k'))
    expect(result.current[0]).toBe(false)
    act(() => result.current[1](true))
    expect(result.current[0]).toBe(true)
  })
})
