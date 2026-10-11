import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useAuthStore } from '../../stores/auth.store'
import { OfflineBar } from './OfflineBar'

const connect = vi.fn()

beforeEach(() => {
  vi.useFakeTimers()
  connect.mockReset()
  useAuthStore.setState({ accessToken: 'tok' })
  useWebSocketStore.setState({ connected: true, reconnectAttempt: 0, connect })
})
afterEach(() => {
  vi.useRealTimers()
  useWebSocketStore.setState({ connected: false, reconnectAttempt: 0 })
})

const goOffline = () => act(() => { window.dispatchEvent(new Event('offline')) })
const goOnline = () => act(() => { window.dispatchEvent(new Event('online')) })
const wait = (ms: number) => act(() => { vi.advanceTimersByTime(ms) })

describe('OfflineBar', () => {
  it('shows nothing while connected', () => {
    render(<OfflineBar />)
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('appears after 2 seconds offline, not before', () => {
    render(<OfflineBar />)
    goOffline()
    wait(1_900)
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
    wait(200)
    expect(screen.getByRole('status')).toHaveTextContent('Mất kết nối. Đang thử kết nối lại…')
  })

  it('appears after a dropped socket', () => {
    render(<OfflineBar />)
    act(() => useWebSocketStore.setState({ connected: false, reconnectAttempt: 1 }))
    wait(2_100)
    expect(screen.getByRole('status')).toHaveTextContent('Mất kết nối')
  })

  it('retries the socket with the session token', () => {
    render(<OfflineBar />)
    goOffline()
    wait(2_100)
    fireEvent.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(connect).toHaveBeenCalledWith('tok')
  })

  it('says it is back for 2 seconds, then hides', () => {
    render(<OfflineBar />)
    goOffline()
    wait(2_100)
    goOnline()
    expect(screen.getByRole('status')).toHaveTextContent('Đã kết nối lại')
    expect(screen.queryByRole('button', { name: 'Thử lại' })).toBeNull()
    wait(1_900)
    expect(screen.getByRole('status')).toHaveTextContent('Đã kết nối lại')
    wait(200)
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })

  it('stays silent for a blip shorter than the grace period', () => {
    render(<OfflineBar />)
    goOffline()
    wait(1_000)
    goOnline()
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
    wait(5_000)
    expect(screen.getByRole('status')).toBeEmptyDOMElement()
  })
})
