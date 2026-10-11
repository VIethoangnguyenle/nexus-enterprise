import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../api/messaging'
import { batchCheckAccess } from '../api/access'
import { queryClient } from '../lib/query-client'
import { useCanManageChannel } from './useCanManageChannel'

vi.mock('../api/messaging', async (orig) => {
  const real = await orig<typeof import('../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, getChannel: vi.fn() } }
})
vi.mock('../api/access', async (orig) => ({ ...(await orig<typeof import('../api/access')>()), batchCheckAccess: vi.fn() }))

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const channel = { id: 'c1', name: 'Nhóm', channel_type: 'workspace', workspace_id: 'w', ngac_oa_id: 'oa-1' }

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
})

describe('useCanManageChannel', () => {
  it("asks the policy about the channel's content object and answers its manage flag", async () => {
    vi.mocked(messagingApi.getChannel).mockResolvedValue(channel)
    vi.mocked(batchCheckAccess).mockResolvedValue({ results: { 'oa-1': { manage: true, write: true } } })
    const { result } = renderHook(() => useCanManageChannel('c1'), { wrapper })
    await waitFor(() => expect(result.current).toBe(true))
    expect(vi.mocked(batchCheckAccess).mock.calls[0]![0]).toEqual(['oa-1'])
  })

  it('is false when the policy says no', async () => {
    vi.mocked(messagingApi.getChannel).mockResolvedValue(channel)
    vi.mocked(batchCheckAccess).mockResolvedValue({ results: { 'oa-1': { manage: false, write: true } } })
    const { result } = renderHook(() => useCanManageChannel('c1'), { wrapper })
    await waitFor(() => expect(batchCheckAccess).toHaveBeenCalled())
    expect(result.current).toBe(false)
  })

  it('is false, and asks nothing, while the channel has no content object to ask about', async () => {
    vi.mocked(messagingApi.getChannel).mockResolvedValue({ ...channel, ngac_oa_id: undefined })
    const { result } = renderHook(() => useCanManageChannel('c1'), { wrapper })
    await waitFor(() => expect(messagingApi.getChannel).toHaveBeenCalled())
    expect(result.current).toBe(false)
    expect(batchCheckAccess).not.toHaveBeenCalled()
  })
})
