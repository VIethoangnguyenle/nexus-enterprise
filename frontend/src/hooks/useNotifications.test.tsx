import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider, type InfiniteData } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { notificationApi, type NotificationPage } from '../api/notifications'
import type { NotificationItem } from '../lib/notification-copy'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import {
  useMarkAllNotificationsRead, useMarkNotificationRead, useNotificationList, useUnreadNotificationCount,
} from './useNotifications'

vi.mock('../api/notifications', async (orig) => {
  const real = await orig<typeof import('../api/notifications')>()
  return { ...real, notificationApi: { list: vi.fn(), unreadCount: vi.fn(), markRead: vi.fn(), markAllRead: vi.fn() } }
})
const api = vi.mocked(notificationApi)

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const n = (id: string, read = false): NotificationItem => ({
  id, type: 'approval_approved', title: 't', body: 'b', read, created_at: '2026-10-10T09:41:00', entity_type: 'approval', entity_id: `r-${id}`,
})
const page = (ids: string[], total: number, unread: number): NotificationPage => ({
  notifications: ids.map((id) => n(id)), total, unread_count: unread,
})
const cachedList = () => queryClient.getQueryData<InfiniteData<NotificationPage>>(keys.notifications.list(25))!
const firstPage = () => cachedList().pages[0] as NotificationPage
const cachedCount = () => queryClient.getQueryData<{ count: number }>(keys.notifications.unreadCount())

beforeEach(() => {
  queryClient.clear()
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: Infinity } })
  vi.clearAllMocks()
})

describe('useUnreadNotificationCount', () => {
  it('reads the count, and is 0 until it arrives', async () => {
    api.unreadCount.mockResolvedValue({ count: 128 })
    const { result } = renderHook(() => useUnreadNotificationCount(), { wrapper })
    expect(result.current).toBe(0)
    await waitFor(() => expect(result.current).toBe(128))
  })
})

describe('useNotificationList', () => {
  it('loads 25 at a time and asks for the next page from where the last ended', async () => {
    api.list.mockResolvedValueOnce(page(['a', 'b'], 3, 2)).mockResolvedValueOnce(page(['c'], 3, 2))
    const { result } = renderHook(() => useNotificationList(true), { wrapper })
    await waitFor(() => expect(result.current.items.map((i) => i.id)).toEqual(['a', 'b']))
    expect(api.list).toHaveBeenLastCalledWith(25, 0)
    expect(result.current.total).toBe(3)
    expect(result.current.hasMore).toBe(true)

    act(() => { void result.current.loadMore() })
    await waitFor(() => expect(result.current.items.map((i) => i.id)).toEqual(['a', 'b', 'c']))
    expect(api.list).toHaveBeenLastCalledWith(25, 2)
    expect(result.current.hasMore).toBe(false)
  })

  it('does not ask while closed', () => {
    renderHook(() => useNotificationList(false), { wrapper })
    expect(api.list).not.toHaveBeenCalled()
  })
})

describe('useMarkNotificationRead', () => {
  async function seeded() {
    api.list.mockResolvedValue(page(['a', 'b'], 2, 2))
    api.unreadCount.mockResolvedValue({ count: 2 })
    const list = renderHook(() => useNotificationList(true), { wrapper })
    renderHook(() => useUnreadNotificationCount(), { wrapper })
    await waitFor(() => expect(list.result.current.items).toHaveLength(2))
    await waitFor(() => expect(cachedCount()).toEqual({ count: 2 }))
    return renderHook(() => useMarkNotificationRead(), { wrapper })
  }

  it('shows it read and the count down at once, before the server answers', async () => {
    api.markRead.mockReturnValue(new Promise(() => {}))
    const { result } = await seeded()
    act(() => { result.current.mutate('a') })
    await waitFor(() => expect(firstPage().notifications.find((x) => x.id === 'a')!.read).toBe(true))
    expect(firstPage().notifications.find((x) => x.id === 'b')!.read).toBe(false)
    expect(firstPage().unread_count).toBe(1)
    expect(cachedCount()).toEqual({ count: 1 })
    expect(api.markRead).toHaveBeenCalledWith('a')
  })

  it('puts it back when the server refuses', async () => {
    api.markRead.mockRejectedValue(new Error('boom'))
    const { result } = await seeded()
    act(() => { result.current.mutate('a') })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(firstPage().notifications.find((x) => x.id === 'a')!.read).toBe(false)
    expect(cachedCount()).toEqual({ count: 2 })
  })

  it('an already-read notification does not lower the count again', async () => {
    api.markRead.mockResolvedValue({ status: 'ok' })
    const { result } = await seeded()
    act(() => { result.current.mutate('a') })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    act(() => { result.current.mutate('a') })
    await waitFor(() => expect(api.markRead).toHaveBeenCalledTimes(2))
    expect(cachedCount()).toEqual({ count: 1 })
  })
})

describe('useMarkAllNotificationsRead', () => {
  it('clears every row and the count together, and restores both on failure', async () => {
    api.list.mockResolvedValue(page(['a', 'b'], 2, 2))
    api.unreadCount.mockResolvedValue({ count: 2 })
    const list = renderHook(() => useNotificationList(true), { wrapper })
    renderHook(() => useUnreadNotificationCount(), { wrapper })
    await waitFor(() => expect(list.result.current.items).toHaveLength(2))
    await waitFor(() => expect(cachedCount()).toEqual({ count: 2 }))
    const { result } = renderHook(() => useMarkAllNotificationsRead(), { wrapper })

    api.markAllRead.mockRejectedValueOnce(new Error('boom'))
    act(() => { result.current.mutate() })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(firstPage().notifications.every((x) => !x.read)).toBe(true)
    expect(cachedCount()).toEqual({ count: 2 })

    api.markAllRead.mockResolvedValueOnce({ status: 'ok' })
    act(() => { result.current.mutate() })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(firstPage().notifications.every((x) => x.read)).toBe(true)
    expect(firstPage().unread_count).toBe(0)
    expect(cachedCount()).toEqual({ count: 0 })
  })
})
