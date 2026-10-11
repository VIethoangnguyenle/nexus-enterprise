import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Message } from '../api/messaging'
import { shortcutsApi, type FeedMessage } from '../api/shortcuts'
import { queryClient } from '../lib/query-client'
import { useToastStore } from '../components/primitives'
import { keys } from './keys'
import { useStarConversation, useStarMessage } from './useShortcuts'

vi.mock('../api/shortcuts', async (orig) => {
  const real = await orig<typeof import('../api/shortcuts')>()
  return {
    ...real,
    shortcutsApi: { starConversation: vi.fn(), unstarConversation: vi.fn(), starMessage: vi.fn(), unstarMessage: vi.fn() },
  }
})
const api = vi.mocked(shortcutsApi)
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const msg = (id: string, over: Partial<FeedMessage> = {}): FeedMessage => ({
  id, channel_id: 'c1', sender_id: 'u2', content: 'x', created_at: { seconds: 1760000000, nanos: 0 }, ...over,
})
type List = { messages: FeedMessage[]; has_more: boolean }

beforeEach(() => {
  queryClient.clear()
  useToastStore.getState().clear()
  vi.clearAllMocks()
})

describe('a refused star undoes only itself', () => {
  it('keeps the message the socket delivered while the request was out', async () => {
    queryClient.setQueryData<List>(keys.messaging.messages('c1'), { messages: [msg('m1')], has_more: false })
    let refuse!: (e: Error) => void
    api.starMessage.mockReturnValueOnce(new Promise((_, rej) => { refuse = rej }))
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    let done!: Promise<void>
    act(() => { done = result.current({ message: { id: 'm1', channel_id: 'c1' } as Message, starred: true }) })

    // A new message and an edit of another arrive over the socket.
    queryClient.setQueryData<List>(keys.messaging.messages('c1'), (old) => ({
      ...old!, messages: [msg('m2', { content: 'mới đến' }), ...old!.messages],
    }))
    refuse(new Error('offline'))
    await act(async () => { await done })

    const list = queryClient.getQueryData<List>(keys.messaging.messages('c1'))!
    expect(list.messages.map((x) => x.id)).toEqual(['m2', 'm1'])
    expect(list.messages.find((x) => x.id === 'm1')!.starred).toBeUndefined()
  })

  it('cancels a refetch already in flight before it patches, so the old flag cannot land after it', async () => {
    queryClient.setQueryData<List>(keys.messaging.messages('c1'), { messages: [msg('m1')], has_more: false })
    void queryClient.fetchQuery({
      queryKey: keys.messaging.messages('c1'), staleTime: 0,
      queryFn: () => new Promise<List>((res) => setTimeout(() => res({ messages: [msg('m1')], has_more: false }), 20)),
    })
    expect(queryClient.isFetching({ queryKey: keys.messaging.messages('c1') })).toBe(1)
    api.starMessage.mockResolvedValueOnce({})
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    await act(async () => { await result.current({ message: { id: 'm1', channel_id: 'c1' } as Message, starred: true }) })
    await new Promise((r) => setTimeout(r, 40))
    expect(queryClient.getQueryData<List>(keys.messaging.messages('c1'))!.messages[0]!.starred).toBe(true)
  })

  it('a refused conversation star keeps what arrived in the unread list meanwhile', async () => {
    const row = (id: string, over = {}) => ({ channel_id: id, unread_count: 0, last_read_message_id: '', ...over })
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [row('c1'), row('c2')] })
    let refuse!: (e: Error) => void
    api.starConversation.mockReturnValueOnce(new Promise((_, rej) => { refuse = rej }))
    const { result } = renderHook(() => useStarConversation(), { wrapper })
    let done!: Promise<void>
    act(() => { done = result.current({ channelId: 'c1', name: 'Lan', starred: true }) })
    queryClient.setQueryData(keys.messaging.unreadCounts(), (old: { channels: unknown[] }) => ({
      channels: (old.channels as { channel_id: string }[]).map((c) => (c.channel_id === 'c2' ? { ...c, unread_count: 4 } : c)),
    }))
    refuse(new Error('offline'))
    await act(async () => { await done })
    const rows = queryClient.getQueryData<{ channels: { channel_id: string; unread_count: number; starred?: boolean }[] }>(keys.messaging.unreadCounts())!.channels
    expect(rows.find((c) => c.channel_id === 'c2')!.unread_count).toBe(4)
    expect(rows.find((c) => c.channel_id === 'c1')!.starred).toBeUndefined()
  })

  it('refreshes the unread list once a conversation star settles', async () => {
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [{ channel_id: 'c1', unread_count: 0, last_read_message_id: '' }] })
    api.starConversation.mockResolvedValueOnce({})
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useStarConversation(), { wrapper })
    await act(async () => { await result.current({ channelId: 'c1', name: 'Lan', starred: true }) })
    expect(spy).toHaveBeenCalledWith({ queryKey: keys.messaging.unreadCounts() })
  })
})

describe('the confirmation toast waits for the server', () => {
  const toasts = () => useToastStore.getState().toasts
  const target = { message: { id: 'm1', channel_id: 'c1' } as Message }

  it('is not shown while the star is on its way, and is shown once it is stored', async () => {
    let accept!: () => void
    api.starMessage.mockReturnValueOnce(new Promise((res) => { accept = () => res({}) }))
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    let done!: Promise<void>
    act(() => { done = result.current({ ...target, starred: true }) })
    expect(toasts()).toHaveLength(0)
    accept()
    await act(async () => { await done })
    expect(toasts().map((t) => t.message)).toEqual(['Đã gắn sao tin'])
  })

  it('never says "Đã gắn sao" before the error when the server refuses', async () => {
    api.starMessage.mockRejectedValueOnce(new Error('offline'))
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    await act(async () => { await result.current({ ...target, starred: true }) })
    expect(toasts().map((t) => t.tone)).toEqual(['error'])
  })

  it('holds for a conversation star as well', async () => {
    let accept!: () => void
    api.starConversation.mockReturnValueOnce(new Promise((res) => { accept = () => res({}) }))
    const { result } = renderHook(() => useStarConversation(), { wrapper })
    let done!: Promise<void>
    act(() => { done = result.current({ channelId: 'c1', name: 'Lan', starred: true }) })
    expect(toasts()).toHaveLength(0)
    accept()
    await act(async () => { await done })
    expect(toasts()).toHaveLength(1)
  })
})
