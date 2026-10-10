import { renderHook, waitFor, cleanup } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, ApiError } from '../api/client'
import { queryClient } from '../lib/query-client'
import { useAcceptInvitation, useDeclineInvitation } from './useInvitations'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const INVITATION = 'aaaaaaaa-aaaa-4bbb-8ccc-000000000009'
const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>

beforeEach(() => {
  api.mockReset()
  api.mockResolvedValue({ status: 'ok' })
})
afterEach(() => {
  cleanup()
  queryClient.clear()
})

describe('answering an invitation reads its notification', () => {
  const readAbout = { method: 'POST', body: JSON.stringify({ type: 'workspace_invitation', id: INVITATION }) }

  it('on accepting', async () => {
    const { result } = renderHook(() => useAcceptInvitation(), { wrapper })
    result.current.mutate(INVITATION)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api).toHaveBeenCalledWith(`/invitations/${INVITATION}/accept`, { method: 'POST' })
    expect(api).toHaveBeenCalledWith('/notifications/read-about', readAbout)
  })

  it('on declining', async () => {
    const { result } = renderHook(() => useDeclineInvitation(), { wrapper })
    result.current.mutate(INVITATION)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api).toHaveBeenCalledWith('/notifications/read-about', readAbout)
  })

  it('but a failed answer reads nothing', async () => {
    api.mockRejectedValue(new ApiError('gone', 404))
    const { result } = renderHook(() => useAcceptInvitation(), { wrapper })
    result.current.mutate(INVITATION)
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(api).not.toHaveBeenCalledWith('/notifications/read-about', expect.anything())
  })

  it('and a notification that cannot be marked does not undo the answer', async () => {
    api.mockImplementation((path: string) =>
      path === '/notifications/read-about' ? Promise.reject(new ApiError('down', 500)) : Promise.resolve({ status: 'ok' }))
    const { result } = renderHook(() => useAcceptInvitation(), { wrapper })
    result.current.mutate(INVITATION)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
  })
})
