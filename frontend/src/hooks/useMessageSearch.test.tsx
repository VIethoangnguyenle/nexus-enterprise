import type { ReactNode } from 'react'
import { cleanup, renderHook, waitFor, act } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { resetClient } from '../test/render'
import { apiFetch } from '../api/client'
import type { Message } from '../api/messaging'
import { useMessageSearch } from './useMessageSearch'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
const msg = (n: number): Message => ({
  id: `m-${n}`, channel_id: 'c1', sender_id: 'u1', content: `đối soát ${n}`, created_at: { seconds: 1_760_000_000 - n, nanos: 0 },
})
const searches = () => api.mock.calls.map(([path]) => new URLSearchParams(String(path).split('?')[1]))

beforeEach(() => {
  queryClient.setDefaultOptions({ queries: { retry: false } })
  api.mockReset()
})
afterEach(() => {
  cleanup()
  resetClient()
})

describe('useMessageSearch', () => {
  it('asks nothing for fewer than two characters', async () => {
    const { result } = renderHook(() => useMessageSearch(' n ', false), { wrapper })
    await new Promise((r) => setTimeout(r, 350))
    expect(api).not.toHaveBeenCalled()
    expect(result.current.active).toBe(false)
    expect(result.current.messages).toEqual([])
  })

  it('waits for a pause in typing, then asks once with the trimmed words', async () => {
    api.mockResolvedValue({ messages: [msg(1)], has_more: false })
    const { result, rerender } = renderHook(({ q }) => useMessageSearch(q, false), { wrapper, initialProps: { q: 'đố' } })
    expect(result.current.waiting).toBe(true)
    rerender({ q: 'đối' })
    rerender({ q: 'đối soát ' })
    expect(api).not.toHaveBeenCalled()
    await waitFor(() => expect(result.current.messages).toHaveLength(1))
    expect(searches()).toHaveLength(1)
    expect(searches()[0]!.get('q')).toBe('đối soát')
    expect(searches()[0]!.get('limit')).toBe('25')
    expect(result.current.waiting).toBe(false)
  })

  it('sends the file filter', async () => {
    api.mockResolvedValue({ messages: [], has_more: false })
    renderHook(() => useMessageSearch('báo cáo', true), { wrapper })
    await waitFor(() => expect(api).toHaveBeenCalled())
    expect(searches()[0]!.get('has_file')).toBe('true')
  })

  it('continues after the last message it has when more are left', async () => {
    const first = Array.from({ length: 25 }, (_, i) => msg(i + 1))
    api.mockResolvedValueOnce({ messages: first, has_more: true })
    api.mockResolvedValueOnce({ messages: [msg(26)], has_more: false })
    const { result } = renderHook(() => useMessageSearch('đối soát', false), { wrapper })
    await waitFor(() => expect(result.current.messages).toHaveLength(25))
    expect(result.current.hasMore).toBe(true)
    await act(async () => { await result.current.loadMore() })
    await waitFor(() => expect(result.current.messages).toHaveLength(26))
    expect(searches()[1]!.get('before_id')).toBe('m-25')
    expect(searches()[1]!.get('before')).toMatch(/^2025-|^2026-/)
    expect(result.current.hasMore).toBe(false)
  })

  it('reports a failure and asks again on retry', async () => {
    api.mockRejectedValueOnce(new Error('boom'))
    api.mockResolvedValueOnce({ messages: [msg(1)], has_more: false })
    const { result } = renderHook(() => useMessageSearch('đối soát', false), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
    act(() => result.current.retry())
    await waitFor(() => expect(result.current.messages).toHaveLength(1))
    expect(result.current.isError).toBe(false)
  })
})

describe('useMessageSearch while messages are hidden', () => {
  it('asks nothing', async () => {
    const { result } = renderHook(() => useMessageSearch('đối soát', false, false), { wrapper })
    await new Promise((r) => setTimeout(r, 350))
    expect(api).not.toHaveBeenCalled()
    expect(result.current.active).toBe(false)
  })
})
