import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { notificationLevelApi } from '../api/notification-levels'
import type { ChannelUnread } from '../api/messaging'
import { useToastStore } from '../components/primitives'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import { changeNotificationLevel, useNotificationLevel } from './useNotificationLevel'

vi.mock('../api/notification-levels', async (orig) => {
  const real = await orig<typeof import('../api/notification-levels')>()
  return { ...real, notificationLevelApi: { getNotificationLevel: vi.fn(), setNotificationLevel: vi.fn(), unreadThreads: vi.fn() } }
})
const api = vi.mocked(notificationLevelApi)

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const unread = (over: Partial<ChannelUnread> = {}): ChannelUnread =>
  ({ channel_id: 'c1', unread_count: 4, last_read_message_id: 'm', thread_unread_count: 2, mention_count: 1, notification_level: 'main', ...over })
const cached = () =>
  queryClient.getQueryData<{ channels: ChannelUnread[] }>(keys.messaging.unreadCounts())!.channels.find((c) => c.channel_id === 'c1')!
const toasts = () => useToastStore.getState().toasts

beforeEach(() => {
  queryClient.clear()
  useToastStore.getState().clear()
  vi.clearAllMocks()
  api.getNotificationLevel.mockResolvedValue({ level: 'main' })
  queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [unread(), unread({ channel_id: 'c2' })] })
})

async function ready() {
  const hook = renderHook(() => useNotificationLevel('c1', 'Đối soát giao dịch'), { wrapper })
  await waitFor(() => expect(hook.result.current.isLoading).toBe(false))
  return hook
}

describe('useNotificationLevel', () => {
  it('reads the level, defaulting to main', async () => {
    api.getNotificationLevel.mockResolvedValue({ level: 'all' })
    const { result } = await ready()
    expect(result.current.level).toBe('all')
  })

  it('shows off at once everywhere, zeroing the counts but keeping the mentions', async () => {
    let finish!: (v: { level: 'off' }) => void
    api.setNotificationLevel.mockReturnValue(new Promise((r) => { finish = r }))
    const { result } = await ready()
    act(() => result.current.setLevel('off'))
    await waitFor(() => expect(result.current.level).toBe('off'))
    expect(cached()).toMatchObject({ notification_level: 'off', unread_count: 0, mention_count: 1 })
    expect(cached().thread_unread_count ?? 0).toBe(0)
    expect(queryClient.getQueryData<{ channels: ChannelUnread[] }>(keys.messaging.unreadCounts())!.channels[1]!.unread_count).toBe(4)
    expect(result.current.isSaving).toBe(true)
    await act(async () => finish({ level: 'off' }))
    await waitFor(() => expect(result.current.isSaving).toBe(false))
    expect(api.setNotificationLevel).toHaveBeenCalledWith('c1', 'off')
  })

  it('says what happened, with Hoàn tác', async () => {
    api.setNotificationLevel.mockResolvedValue({ level: 'off' })
    const { result } = await ready()
    act(() => result.current.setLevel('off'))
    await waitFor(() => expect(toasts()).toHaveLength(1))
    expect(toasts()[0]).toMatchObject({ message: 'Đã tắt thông báo của “Đối soát giao dịch”', tone: 'success' })
    expect(toasts()[0]!.action!.label).toBe('Hoàn tác')
  })

  it('Hoàn tác puts the previous level back without a second toast', async () => {
    api.setNotificationLevel.mockImplementation(async (_c, level) => ({ level }))
    const { result } = await ready()
    act(() => result.current.setLevel('all'))
    await waitFor(() => expect(toasts()).toHaveLength(1))
    expect(toasts()[0]!.message).toBe('Thông báo mọi tin của “Đối soát giao dịch”')
    act(() => toasts()[0]!.action!.onClick())
    await waitFor(() => expect(api.setNotificationLevel).toHaveBeenLastCalledWith('c1', 'main'))
    await waitFor(() => expect(result.current.level).toBe('main'))
    expect(toasts()).toHaveLength(1)
  })

  it('turning back on from off says it is back on', async () => {
    api.getNotificationLevel.mockResolvedValue({ level: 'off' })
    api.setNotificationLevel.mockResolvedValue({ level: 'main' })
    // The server reports the same level in both answers.
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [unread({ notification_level: 'off', unread_count: 0 })] })
    const { result } = await ready()
    act(() => result.current.setLevel('main'))
    await waitFor(() => expect(toasts()).toHaveLength(1))
    expect(toasts()[0]!.message).toBe('Đã bật lại thông báo của “Đối soát giao dịch”')
  })

  it('rolls everything back and offers Thử lại when the server refuses', async () => {
    api.setNotificationLevel.mockRejectedValueOnce(new Error('500'))
    const { result } = await ready()
    act(() => result.current.setLevel('off'))
    await waitFor(() => expect(toasts()).toHaveLength(1))
    expect(result.current.level).toBe('main')
    expect(cached()).toMatchObject({ notification_level: 'main', unread_count: 4, thread_unread_count: 2 })
    expect(toasts()[0]).toMatchObject({
      message: 'Chưa đổi được thông báo của “Đối soát giao dịch”. Kiểm tra kết nối rồi thử lại.',
      tone: 'error',
      duration: 8000,
    })
    api.setNotificationLevel.mockResolvedValue({ level: 'off' })
    act(() => toasts()[0]!.action!.onClick())
    await waitFor(() => expect(api.setNotificationLevel).toHaveBeenCalledTimes(2))
    expect(api.setNotificationLevel).toHaveBeenLastCalledWith('c1', 'off')
  })

  it('asks the server for the level only when it is not already known from the unread list', async () => {
    const { result } = renderHook(() => useNotificationLevel('c1', 'x', { fetchLevel: false }), { wrapper })
    await act(async () => {})
    expect(api.getNotificationLevel).not.toHaveBeenCalled()
    expect(result.current.level).toBe('main')
  })

  it('does nothing when the level is already the chosen one', async () => {
    const { result } = await ready()
    act(() => result.current.setLevel('main'))
    expect(api.setNotificationLevel).not.toHaveBeenCalled()
  })

  it('settles the counts and Trang chủ threads from the server afterwards', async () => {
    api.setNotificationLevel.mockResolvedValue({ level: 'off' })
    queryClient.setQueryData(keys.notificationLevels.unreadThreads(), { messages: [] })
    const { result } = await ready()
    act(() => result.current.setLevel('off'))
    await waitFor(() => expect(queryClient.getQueryState(keys.notificationLevels.unreadThreads())!.isInvalidated).toBe(true))
    expect(queryClient.getQueryState(keys.messaging.unreadCounts())!.isInvalidated).toBe(true)
  })
})

describe('changeNotificationLevel (a row menu, no component bound to the conversation)', () => {
  it('mutes a conversation that is not open: its count goes, its mentions stay, the others are untouched', async () => {
    api.setNotificationLevel.mockResolvedValue({ level: 'off' })
    await changeNotificationLevel({ channelId: 'c2', name: 'Hợp đồng đối tác', level: 'off' })
    const rows = queryClient.getQueryData<{ channels: ChannelUnread[] }>(keys.messaging.unreadCounts())!.channels
    expect(rows[1]).toMatchObject({ notification_level: 'off', unread_count: 0, mention_count: 1 })
    expect(rows[0]).toMatchObject({ notification_level: 'main', unread_count: 4 })
    expect(toasts()[0]!.message).toBe('Đã tắt thông báo của “Hợp đồng đối tác”')
  })

  it('puts the row back when the server refuses', async () => {
    api.setNotificationLevel.mockRejectedValueOnce(new Error('500'))
    await changeNotificationLevel({ channelId: 'c2', name: 'x', level: 'off' })
    const rows = queryClient.getQueryData<{ channels: ChannelUnread[] }>(keys.messaging.unreadCounts())!.channels
    expect(rows[1]).toMatchObject({ notification_level: 'main', unread_count: 4 })
    expect(toasts()[0]!.tone).toBe('error')
  })
})

describe('a level changed on another device', () => {
  it('is respected: the unread list outranks a level cached by an earlier optimistic change', async () => {
    api.setNotificationLevel.mockResolvedValue({ level: 'all' })
    // An earlier choice of "all" left this behind; another device has since made it "off".
    queryClient.setQueryData(keys.notificationLevels.level('c1'), { level: 'all' })
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [unread({ notification_level: 'off' })] })
    await act(async () => { await changeNotificationLevel({ channelId: 'c1', name: 'Lan', level: 'all' }) })
    expect(api.setNotificationLevel).toHaveBeenCalledWith('c1', 'all')
    expect(toasts().find((t) => t.action?.label === 'Hoàn tác')).toBeTruthy()
  })

  it('is fetched again when the menu opens, whatever the cache holds', async () => {
    queryClient.setQueryData(keys.notificationLevels.level('c1'), { level: 'all' })
    api.getNotificationLevel.mockResolvedValue({ level: 'off' })
    const { result } = renderHook(() => useNotificationLevel('c1', 'Lan'), { wrapper })
    await waitFor(() => expect(result.current.level).toBe('off'))
    expect(api.getNotificationLevel).toHaveBeenCalledTimes(1)
  })

  it('leaves no stale level behind once a change settles', async () => {
    api.setNotificationLevel.mockResolvedValue({ level: 'off' })
    await act(async () => { await changeNotificationLevel({ channelId: 'c1', name: 'Lan', level: 'off' }) })
    expect(queryClient.getQueryState(keys.notificationLevels.level('c1'))!.isInvalidated).toBe(true)
  })
})
