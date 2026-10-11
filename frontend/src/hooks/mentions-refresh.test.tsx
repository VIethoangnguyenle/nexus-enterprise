import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import { useMarkRead } from './useMessaging'
import { useMarkThreadRead } from './useThreads'

vi.mock('../api/messaging', async (orig) => {
  const real = await orig<typeof import('../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, markRead: vi.fn(), markThreadRead: vi.fn() } }
})
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const stale = (key: readonly unknown[]) => queryClient.getQueryState(key)?.isInvalidated === true

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
  queryClient.setQueryData(keys.shortcuts.mentionsUnread('w1'), 3)
  queryClient.setQueryData(keys.shortcuts.mentions('w1'), { pages: [], pageParams: [] })
})

describe('reading refreshes the "Được nhắc tới" count', () => {
  it('when a conversation is read', async () => {
    vi.mocked(messagingApi.markRead).mockResolvedValue({} as never)
    const { result } = renderHook(() => useMarkRead('c1'), { wrapper })
    act(() => result.current.mutate('m9'))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(stale(keys.shortcuts.mentionsUnread('w1'))).toBe(true)
    expect(stale(keys.shortcuts.mentions('w1'))).toBe(true)
  })

  it('when a thread is read', async () => {
    vi.mocked(messagingApi.markThreadRead).mockResolvedValue({ rootMessageId: 'r', channelId: 'c1', following: true, unreadCount: 0 } as never)
    const { result } = renderHook(() => useMarkThreadRead('c1'), { wrapper })
    act(() => result.current.mutate('r'))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(stale(keys.shortcuts.mentionsUnread('w1'))).toBe(true)
  })
})
