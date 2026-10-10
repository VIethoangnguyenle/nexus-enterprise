import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../api/client'
import { useContacts } from './useContacts'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const person = (n: number) => ({
  user_id: `u${n}`, ngac_node_id: `n${n}`, username: `p${n}`, display_name: `Người ${n}`, email: '', title: '',
  department: n === 1 ? 'Đối soát' : '', location: '', avatar_url: '', is_online: false,
})

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{children}</QueryClientProvider>
)

beforeEach(() => api.mockReset())

// The wire with the auth service's directory: `cursor` goes out, `next_cursor`
// and the true `total` come back (backend/services/auth/internal/rest: see
// TestContacts_PagesByCursor_AndTheTotalIsTheStores).
describe('useContacts: the directory wire', () => {
  it('follows next_cursor until the server stops giving one, and reports the true total', async () => {
    api
      .mockResolvedValueOnce({ contacts: [person(1), person(2)], total: 5, next_cursor: 'c1' })
      .mockResolvedValueOnce({ contacts: [person(3), person(4)], total: 5, next_cursor: 'c2' })
      .mockResolvedValueOnce({ contacts: [person(5)], total: 5 })

    const { result } = renderHook(() => useContacts('w1'), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(api.mock.calls.map((c) => c[0])).toEqual([
      '/workspaces/w1/contacts',
      '/workspaces/w1/contacts?cursor=c1',
      '/workspaces/w1/contacts?cursor=c2',
    ])
    expect(result.current.data?.contacts.map((c) => c.user_id)).toEqual(['u1', 'u2', 'u3', 'u4', 'u5'])
    expect(result.current.data?.total).toBe(5)
  })

  it('keeps the filters on every page and asks for the cursor by the name the server reads', async () => {
    api
      .mockResolvedValueOnce({ contacts: [person(1)], total: 2, next_cursor: 'abc_-9' })
      .mockResolvedValueOnce({ contacts: [person(2)], total: 2 })

    const { result } = renderHook(
      () => useContacts('w1', { department: 'Đối soát', location: 'Hà Nội', search: 'lan' }),
      { wrapper },
    )
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    const second = new URL(String(api.mock.calls[1]![0]), 'http://x')
    expect(second.searchParams.get('cursor')).toBe('abc_-9')
    for (const call of api.mock.calls) {
      const q = new URL(String(call[0]), 'http://x').searchParams
      expect([q.get('department'), q.get('location'), q.get('search')]).toEqual(['Đối soát', 'Hà Nội', 'lan'])
    }
  })

  it('reads the department the server assigned, per person', async () => {
    api.mockResolvedValueOnce({ contacts: [person(1), person(2)], total: 2 })
    const { result } = renderHook(() => useContacts('w1'), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.contacts.map((c) => c.department)).toEqual(['Đối soát', ''])
  })

  it('is one request when the directory fits one page', async () => {
    api.mockResolvedValueOnce({ contacts: [person(1)], total: 1 })
    const { result } = renderHook(() => useContacts('w1'), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api).toHaveBeenCalledTimes(1)
  })

  it('a page that fails fails the whole directory instead of listing half of it', async () => {
    api
      .mockResolvedValueOnce({ contacts: [person(1)], total: 2, next_cursor: 'c1' })
      .mockRejectedValueOnce(new Error('boom'))
    const { result } = renderHook(() => useContacts('w1'), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(result.current.data).toBeUndefined()
  })

  it('does not ask without a workspace', () => {
    renderHook(() => useContacts(''), { wrapper })
    expect(api).not.toHaveBeenCalled()
  })
})
