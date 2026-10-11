import { useInfiniteQuery, useMutation, useQuery, type InfiniteData } from '@tanstack/react-query'
import { NOTIFICATION_PAGE_SIZE, notificationApi, type NotificationPage } from '../api/notifications'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

type ListData = InfiniteData<NotificationPage, number>
type Snapshot = { list?: ListData; count?: { count: number } }

/** How many notifications the signed-in person has not read. The socket's frames refresh it. */
export function useUnreadNotificationCount(): number {
  const { data } = useQuery({
    queryKey: keys.notifications.unreadCount(),
    queryFn: notificationApi.unreadCount,
    staleTime: 60_000,
  })
  return data?.count ?? 0
}

/** The notification list, 25 a page, newest first; fetched only while the panel is open. */
export function useNotificationList(enabled: boolean) {
  const q = useInfiniteQuery({
    queryKey: keys.notifications.list(NOTIFICATION_PAGE_SIZE),
    queryFn: ({ pageParam }) => notificationApi.list(NOTIFICATION_PAGE_SIZE, pageParam),
    initialPageParam: 0,
    getNextPageParam: (last, all) => {
      const loaded = all.reduce((sum, p) => sum + p.notifications.length, 0)
      return last.notifications.length > 0 && loaded < last.total ? loaded : undefined
    },
    enabled,
    staleTime: 0,
  })
  const items = q.data?.pages.flatMap((p) => p.notifications) ?? []
  return {
    items,
    total: q.data?.pages[0]?.total ?? 0,
    /** The list's own figure: it moves with the rows, whereas the badge query may be a minute old. */
    unread: q.data?.pages[0]?.unread_count,
    isLoading: q.isPending && q.fetchStatus !== 'idle',
    isError: q.isError,
    hasMore: !!q.hasNextPage,
    loadingMore: q.isFetchingNextPage,
    loadMore: q.fetchNextPage,
    refetch: q.refetch,
  }
}

const snapshot = (): Snapshot => ({
  list: queryClient.getQueryData<ListData>(keys.notifications.list(NOTIFICATION_PAGE_SIZE)),
  count: queryClient.getQueryData<{ count: number }>(keys.notifications.unreadCount()),
})
function restore(s: Snapshot) {
  queryClient.setQueryData(keys.notifications.list(NOTIFICATION_PAGE_SIZE), s.list)
  queryClient.setQueryData(keys.notifications.unreadCount(), s.count)
}
const withUnread = (page: NotificationPage, unread: number): NotificationPage => ({ ...page, unread_count: unread })

/** Marks one read: the row, the panel's count and the rail's count change before the server answers; a refusal puts them back. */
export function useMarkNotificationRead() {
  return useMutation({
    meta: { action: 'đánh dấu thông báo đã đọc' },
    mutationFn: (id: string) => notificationApi.markRead(id),
    onMutate: async (id): Promise<Snapshot> => {
      await queryClient.cancelQueries({ queryKey: keys.notifications.all() })
      const before = snapshot()
      const wasUnread = before.list?.pages.some((p) => p.notifications.some((n) => n.id === id && !n.read)) ?? false
      queryClient.setQueryData<ListData>(keys.notifications.list(NOTIFICATION_PAGE_SIZE), (old) => old && {
        ...old,
        pages: old.pages.map((p) => withUnread(
          { ...p, notifications: p.notifications.map((n) => (n.id === id ? { ...n, read: true } : n)) },
          wasUnread ? Math.max(0, p.unread_count - 1) : p.unread_count,
        )),
      })
      if (wasUnread) {
        queryClient.setQueryData<{ count: number }>(keys.notifications.unreadCount(), (old) => old && { count: Math.max(0, old.count - 1) })
      }
      return before
    },
    onError: (_e, _id, before) => { if (before) restore(before) },
  })
}

/** Marks everything read, with the same optimistic change and rollback. */
export function useMarkAllNotificationsRead() {
  return useMutation({
    meta: { action: 'đánh dấu tất cả thông báo đã đọc' },
    mutationFn: () => notificationApi.markAllRead(),
    onMutate: async (): Promise<Snapshot> => {
      await queryClient.cancelQueries({ queryKey: keys.notifications.all() })
      const before = snapshot()
      queryClient.setQueryData<ListData>(keys.notifications.list(NOTIFICATION_PAGE_SIZE), (old) => old && {
        ...old,
        pages: old.pages.map((p) => withUnread({ ...p, notifications: p.notifications.map((n) => ({ ...n, read: true })) }, 0)),
      })
      queryClient.setQueryData<{ count: number }>(keys.notifications.unreadCount(), (old) => old && { count: 0 })
      return before
    },
    onError: (_e, _v, before) => { if (before) restore(before) },
  })
}
