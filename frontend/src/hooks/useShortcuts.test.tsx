import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider, type InfiniteData } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Channel, Message } from '../api/messaging'
import { shortcutsApi, type FeedMessage, type FeedPage } from '../api/shortcuts'
import { queryClient } from '../lib/query-client'
import { useToastStore } from '../components/primitives'
import { keys } from './keys'
import { shortcutKeys } from './keys/shortcuts'
import {
  dropFromFeed, markFeedRead, setChannelRecordStarred, setMessageStarred, setUnreadListStarred,
  useMentionsFeed, useMentionsUnreadCount, useOpenMention, useStarConversation, useStarMessage, useStarredFeed,
} from './useShortcuts'

vi.mock('../api/shortcuts', async (orig) => {
  const real = await orig<typeof import('../api/shortcuts')>()
  return {
    ...real,
    shortcutsApi: {
      starConversation: vi.fn(), unstarConversation: vi.fn(), starMessage: vi.fn(), unstarMessage: vi.fn(),
      listStarred: vi.fn(), listMentions: vi.fn(), mentionsUnreadCount: vi.fn(),
    },
  }
})
const api = vi.mocked(shortcutsApi)
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)

const m = (id: string, over: Partial<FeedMessage> = {}): FeedMessage => ({
  id, channel_id: 'c1', sender_id: 'u2', content: 'x', created_at: { seconds: 1760000000 + Number(id.replace(/\D/g, '') || 0), nanos: 0 }, ...over,
})
const infinite = (...pages: FeedPage[]): InfiniteData<FeedPage, unknown> => ({ pages, pageParams: pages.map(() => undefined) })
const toasts = () => useToastStore.getState().toasts

beforeEach(() => {
  queryClient.clear()
  useToastStore.getState().clear()
  vi.clearAllMocks()
  api.starMessage.mockResolvedValue({})
  api.unstarMessage.mockResolvedValue({})
  api.starConversation.mockResolvedValue({})
  api.unstarConversation.mockResolvedValue({})
})
afterEach(() => useToastStore.getState().clear())

describe('cache patches', () => {
  it('flags one message of a list and leaves a list without it alone', () => {
    const list = { messages: [m('a'), m('b')] as Message[], has_more: false }
    const out = setMessageStarred(list, 'b', true)!
    expect(out.messages.map((x) => (x as FeedMessage).starred)).toEqual([undefined, true])
    expect(setMessageStarred(list, 'zz', true)).toBe(list)
    expect(setMessageStarred(undefined, 'a', true)).toBeUndefined()
  })

  it('drops the star flag rather than storing false', () => {
    const list = { messages: [m('a', { starred: true })] as Message[], has_more: false }
    expect('starred' in setMessageStarred(list, 'a', false)!.messages[0]!).toBe(false)
  })

  it('flags a conversation in the unread list and in its own record', () => {
    const unread = { channels: [{ channel_id: 'c1', unread_count: 2, last_read_message_id: '' }, { channel_id: 'c2', unread_count: 0, last_read_message_id: '' }] }
    expect(setUnreadListStarred(unread, 'c2', true)!.channels.map((c) => (c as { starred?: boolean }).starred)).toEqual([undefined, true])
    expect(setUnreadListStarred(unread, 'nope', true)).toBe(unread)
    // Starring stamps the time at once, so the section orders it first before the server answers; unstarring drops it.
    const starred = setUnreadListStarred(unread, 'c2', true)!
    expect(Date.parse(String((starred.channels[1] as { starred_at?: unknown }).starred_at))).toBeGreaterThan(Date.now() - 5000)
    expect('starred_at' in setUnreadListStarred(starred, 'c2', false)!.channels[1]!).toBe(false)
    const ch: Channel = { id: 'c1', name: 'Vận hành chung', channel_type: 'workspace', workspace_id: 'w' }
    expect(setChannelRecordStarred(ch, true)).toMatchObject({ starred: true })
    expect(setChannelRecordStarred(undefined, true)).toBeUndefined()
  })

  it('removes a message from every page of a feed', () => {
    const feed = infinite({ messages: [m('a'), m('b')], has_more: true }, { messages: [m('c')], has_more: false })
    const out = dropFromFeed(feed, 'c')!
    expect(out.pages.map((p) => p.messages.map((x) => x.id))).toEqual([['a', 'b'], []])
    expect(dropFromFeed(feed, 'zz')).toBe(feed)
  })

  it('marks one feed message read, once', () => {
    const feed = infinite({ messages: [m('a', { unread: true }), m('b', { unread: true })], has_more: false })
    const out = markFeedRead(feed, 'a')!
    expect(out.pages[0]!.messages.map((x) => x.unread)).toEqual([undefined, true])
    expect(markFeedRead(out, 'a')).toBe(out)
  })
})

describe('useStarMessage', () => {
  const root = { id: 'm1', channel_id: 'c1' }
  const seed = () => {
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [m('m1')], has_more: false })
    queryClient.setQueryData(keys.messaging.thread('m1'), { messages: [m('m1')] })
  }
  const starredOf = () => (queryClient.getQueryData<{ messages: FeedMessage[] }>(keys.messaging.messages('c1'))!.messages[0]!).starred

  it('stars at once in the stream and its thread, then offers Xem', async () => {
    seed()
    const onView = vi.fn()
    const { result } = renderHook(() => useStarMessage({ onView }), { wrapper })
    let done!: Promise<void>
    act(() => { done = result.current({ message: root, starred: true }) })
    expect(starredOf()).toBe(true)
    expect((queryClient.getQueryData<{ messages: FeedMessage[] }>(keys.messaging.thread('m1')))!.messages[0]!.starred).toBe(true)
    await act(async () => { await done })
    expect(api.starMessage).toHaveBeenCalledWith('m1')
    expect(toasts()[0]).toMatchObject({ message: 'Đã gắn sao tin', action: { label: 'Xem' } })
    toasts()[0]!.action!.onClick()
    expect(onView).toHaveBeenCalled()
  })

  it('stars a reply only in its thread', async () => {
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [m('m1')], has_more: false })
    queryClient.setQueryData(keys.messaging.thread('m1'), { messages: [m('m1'), m('r1', { parent_message_id: 'm1' })] })
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    await act(async () => { await result.current({ message: { id: 'r1', channel_id: 'c1', parent_message_id: 'm1' }, starred: true }) })
    expect(starredOf()).toBeUndefined()
    expect(queryClient.getQueryData<{ messages: FeedMessage[] }>(keys.messaging.thread('m1'))!.messages[1]!.starred).toBe(true)
  })

  it('rolls back and offers Thử lại when the server refuses', async () => {
    seed()
    api.starMessage.mockRejectedValueOnce(new Error('offline'))
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    await act(async () => { await result.current({ message: root, starred: true }) })
    expect(starredOf()).toBeUndefined()
    const t = toasts().find((x) => x.tone === 'error')!
    expect(t.message).toBe('Không gắn sao được. Kiểm tra kết nối rồi thử lại.')
    expect(t.action!.label).toBe('Thử lại')
    await act(async () => { t.action!.onClick() })
    await waitFor(() => expect(api.starMessage).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(starredOf()).toBe(true))
  })

  it('unstarring from the feed removes the card and Hoàn tác puts it back where it was', async () => {
    seed()
    const feedKey = shortcutKeys.starredMessages('w1')
    queryClient.setQueryData(feedKey, infinite({ messages: [m('m0'), m('m1'), m('m2')], has_more: false }))
    const { result } = renderHook(() => useStarMessage(), { wrapper })
    await act(async () => { await result.current({ message: root, starred: false }) })
    expect(queryClient.getQueryData<InfiniteData<FeedPage>>(feedKey)!.pages[0]!.messages.map((x) => x.id)).toEqual(['m0', 'm2'])
    const undo = toasts().find((x) => x.message === 'Đã bỏ gắn sao tin')!
    expect(undo.action!.label).toBe('Hoàn tác')
    await act(async () => { undo.action!.onClick() })
    expect(queryClient.getQueryData<InfiniteData<FeedPage>>(feedKey)!.pages[0]!.messages.map((x) => x.id)).toEqual(['m0', 'm1', 'm2'])
    await waitFor(() => expect(api.starMessage).toHaveBeenCalledWith('m1'))
    // The undo is quiet: no second toast about it.
    expect(toasts().filter((x) => x.message === 'Đã gắn sao tin')).toHaveLength(0)
  })
})

describe('useStarConversation', () => {
  const seed = () => {
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [{ channel_id: 'c1', unread_count: 0, last_read_message_id: '' }] })
    queryClient.setQueryData(keys.messaging.channel('c1'), { id: 'c1', name: 'x', channel_type: 'workspace', workspace_id: 'w' })
  }
  const flagInList = () => (queryClient.getQueryData<{ channels: { starred?: boolean }[] }>(keys.messaging.unreadCounts())!.channels[0]!).starred

  it('stars at once in the unread list and the record, with Hoàn tác naming the conversation', async () => {
    seed()
    const { result } = renderHook(() => useStarConversation(), { wrapper })
    let done!: Promise<void>
    act(() => { done = result.current({ channelId: 'c1', name: 'Nguyễn Thu Lan', starred: true }) })
    expect(flagInList()).toBe(true)
    expect(queryClient.getQueryData<{ starred?: boolean }>(keys.messaging.channel('c1'))!.starred).toBe(true)
    await act(async () => { await done })
    expect(api.starConversation).toHaveBeenCalledWith('c1')
    expect(toasts()[0]).toMatchObject({ message: 'Đã gắn sao “Nguyễn Thu Lan”', action: { label: 'Hoàn tác' } })
    await act(async () => { toasts()[0]!.action!.onClick() })
    await waitFor(() => expect(api.unstarConversation).toHaveBeenCalledWith('c1'))
    expect(flagInList()).toBeUndefined()
  })

  it('rolls back with an error toast on failure', async () => {
    seed()
    api.unstarConversation.mockRejectedValueOnce(new Error('403'))
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [{ channel_id: 'c1', unread_count: 0, last_read_message_id: '', starred: true }] })
    const { result } = renderHook(() => useStarConversation(), { wrapper })
    await act(async () => { await result.current({ channelId: 'c1', name: 'Lan', starred: false }) })
    expect(flagInList()).toBe(true)
    expect(toasts().find((x) => x.tone === 'error')!.message).toBe('Không bỏ gắn sao được. Kiểm tra kết nối rồi thử lại.')
  })
})

describe('feeds', () => {
  it('pages the mentions feed 25 at a time by the last message time and id', async () => {
    const first = Array.from({ length: 25 }, (_, i) => m(`m${i}`, { created_at: { seconds: 1760000100 - i, nanos: 7 } }))
    api.listMentions.mockResolvedValueOnce({ messages: first, has_more: true }).mockResolvedValueOnce({ messages: [m('z9')], has_more: false })
    const { result } = renderHook(() => useMentionsFeed('w1'), { wrapper })
    await waitFor(() => expect(result.current.data?.pages).toHaveLength(1))
    expect(api.listMentions).toHaveBeenCalledWith({ cursor: undefined, limit: 25 })
    expect(result.current.hasNextPage).toBe(true)
    await act(async () => { await result.current.fetchNextPage() })
    expect(api.listMentions).toHaveBeenLastCalledWith({ cursor: { before: '2025-10-09T08:54:36.000000007Z', beforeId: 'm24' }, limit: 25 })
    await waitFor(() => expect(result.current.hasNextPage).toBe(false))
  })

  it('pages the starred feed by the star time', async () => {
    const starred_at = { seconds: 1760000000, nanos: 0 }
    api.listStarred.mockResolvedValueOnce({ messages: [m('a', { starred_at })], has_more: true })
    const { result } = renderHook(() => useStarredFeed('w1'), { wrapper })
    await waitFor(() => expect(result.current.data?.pages).toHaveLength(1))
    api.listStarred.mockResolvedValueOnce({ messages: [], has_more: false })
    await act(async () => { await result.current.fetchNextPage() })
    expect(api.listStarred).toHaveBeenLastCalledWith({ cursor: { before: '2025-10-09T08:53:20Z', beforeId: 'a' }, limit: 25 })
  })

  it('reads the mentions badge', async () => {
    api.mentionsUnreadCount.mockResolvedValue(7)
    const { result } = renderHook(() => useMentionsUnreadCount('w1'), { wrapper })
    await waitFor(() => expect(result.current.data).toBe(7))
  })

  it('does not ask without a workspace', () => {
    renderHook(() => useMentionsFeed(''), { wrapper })
    expect(api.listMentions).not.toHaveBeenCalled()
  })
})

describe('useOpenMention', () => {
  it('turns the dot off and lowers the badge, without a request', () => {
    queryClient.setQueryData(shortcutKeys.mentions('w1'), infinite({ messages: [m('a', { unread: true })], has_more: false }))
    queryClient.setQueryData(shortcutKeys.mentionsUnread('w1'), 3)
    const { result } = renderHook(() => useOpenMention(), { wrapper })
    act(() => result.current(m('a', { unread: true })))
    expect(queryClient.getQueryData<InfiniteData<FeedPage>>(shortcutKeys.mentions('w1'))!.pages[0]!.messages[0]!.unread).toBeUndefined()
    expect(queryClient.getQueryData(shortcutKeys.mentionsUnread('w1'))).toBe(2)
    act(() => result.current(m('a')))
    expect(queryClient.getQueryData(shortcutKeys.mentionsUnread('w1'))).toBe(2)
  })
})
