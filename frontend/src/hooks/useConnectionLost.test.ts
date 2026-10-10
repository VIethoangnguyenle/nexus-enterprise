import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useWebSocketStore } from '../stores/websocket.store'
import { useConnectionLost } from './useConnectionLost'

beforeEach(() => {
  vi.useFakeTimers()
  useWebSocketStore.setState({ connected: true, reconnectAttempt: 0 })
})
afterEach(() => {
  vi.useRealTimers()
  useWebSocketStore.setState({ connected: false, reconnectAttempt: 0 })
})

describe('useConnectionLost', () => {
  it('is false for a socket that has never connected', () => {
    useWebSocketStore.setState({ connected: false, reconnectAttempt: 0 })
    const { result } = renderHook(() => useConnectionLost())
    act(() => { vi.advanceTimersByTime(10_000) })
    expect(result.current).toBe(false)
  })

  it('waits out the 3s grace period before reporting a dropped socket', () => {
    const { result } = renderHook(() => useConnectionLost())
    act(() => useWebSocketStore.setState({ connected: false, reconnectAttempt: 1 }))
    act(() => { vi.advanceTimersByTime(2_900) })
    expect(result.current).toBe(false)
    act(() => { vi.advanceTimersByTime(200) })
    expect(result.current).toBe(true)
  })

  it('recovers at once when the socket comes back', () => {
    const { result } = renderHook(() => useConnectionLost())
    act(() => useWebSocketStore.setState({ connected: false, reconnectAttempt: 2 }))
    act(() => { vi.advanceTimersByTime(3_100) })
    expect(result.current).toBe(true)
    act(() => useWebSocketStore.setState({ connected: true, reconnectAttempt: 0 }))
    expect(result.current).toBe(false)
  })

  it('treats a browser offline event as lost after the same grace', () => {
    const { result } = renderHook(() => useConnectionLost())
    act(() => { window.dispatchEvent(new Event('offline')) })
    act(() => { vi.advanceTimersByTime(3_100) })
    expect(result.current).toBe(true)
    act(() => { window.dispatchEvent(new Event('online')) })
    expect(result.current).toBe(false)
  })
})
