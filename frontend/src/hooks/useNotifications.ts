import { useInfiniteQuery, useMutation, useQuery, type InfiniteData } from '@tanstack/react-query'
import { notificationsApi, type AppNotification, type NotificationPage } from '../api/notifications'
import { queryClient } from '../lib/query-client'
import { toast } from '../components/primitives/Toast'
import { keys } from './keys'

type Pages = InfiniteData<NotificationPage, number>

/** Shown when marking fails; the cache has already been put back. */
export const MARK_FAILED = 'Chưa đánh dấu được. Kiểm tra kết nối rồi thử lại.'

/**
 * The notifications of the workspace in the session, 25 to a page. The list is
 * only asked for when a surface that shows it is open.
 */
export function useNotificationList(enabled = true) {
  return useInfiniteQuery({
    queryKey: keys.notifications.list(),
    queryFn: ({ pageParam }) => notificationsApi.list(pageParam),
    initialPageParam: 0,
    getNextPageParam: (last, all) => {
      const loaded = all.reduce((n, p) => n + (p.notifications?.length ?? 0), 0)
      return loaded < last.total && (last.notifications?.length ?? 0) > 0 ? loaded : undefined
    },
    enabled,
    retry: false,
  })
}

/**
 * How many are unread. It has its own cheap request so the entry points show a
 * number before the list has loaded (or without it ever being opened).
 */
export function useUnreadCount(enabled = true) {
  return useQuery({
    queryKey: keys.notifications.unreadCount(),
    queryFn: () => notificationsApi.unreadCount(),
    select: (d) => d.count,
    enabled,
    retry: false,
  })
}

/** What the optimistic update changed, to put back if the request fails. */
interface Snapshot {
  list?: Pages
  count?: { count: number }
}

async function snapshotAndMark(ids: string[] | 'all'): Promise<Snapshot> {
  await queryClient.cancelQueries({ queryKey: keys.notifications.all() })
  const snap: Snapshot = {
    list: queryClient.getQueryData<Pages>(keys.notifications.list()),
    count: queryClient.getQueryData<{ count: number }>(keys.notifications.unreadCount()),
  }
  const flips = (n: AppNotification) => !n.read && (ids === 'all' || ids.includes(n.id))
  let flipped = 0
  queryClient.setQueryData<Pages>(keys.notifications.list(), (old) => {
    if (!old) return old
    return {
      ...old,
      pages: old.pages.map((p) => ({
        ...p,
        notifications: (p.notifications ?? []).map((n) => {
          if (!flips(n)) return n
          flipped++
          return { ...n, read: true }
        }),
      })),
    }
  })
  queryClient.setQueryData<{ count: number }>(keys.notifications.unreadCount(), (old) => {
    if (!old) return old
    if (ids === 'all') return { count: 0 }
    // A notification not in the loaded pages still counts down by the one marked.
    return { count: Math.max(0, old.count - (flipped || ids.length)) }
  })
  return snap
}

function restore(snap: Snapshot | undefined) {
  if (snap?.list) queryClient.setQueryData(keys.notifications.list(), snap.list)
  if (snap?.count) queryClient.setQueryData(keys.notifications.unreadCount(), snap.count)
}

const refetch = () => queryClient.invalidateQueries({ queryKey: keys.notifications.all() })

/**
 * Marks one notification read: the row and the count change at once, and a
 * failure puts both back and says so with a way to try again.
 */
export function useMarkRead() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (id: string) => notificationsApi.markRead(id),
    onMutate: (id) => snapshotAndMark([id]),
    onError: (_e, id, snap) => {
      restore(snap)
      toast.error(MARK_FAILED, { action: { label: 'Thử lại', onClick: () => markReadNow(id) } })
    },
    onSettled: refetch,
  })
}

/** Marks every unread notification read. There is no undo: the API cannot mark unread. */
export function useMarkAllRead() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => notificationsApi.markAllRead(),
    onMutate: async () => {
      const before = queryClient.getQueryData<{ count: number }>(keys.notifications.unreadCount())?.count ?? 0
      return { snap: await snapshotAndMark('all'), before }
    },
    onSuccess: (_d, _v, ctx) => {
      if (ctx.before > 0) toast(`Đã đánh dấu ${ctx.before} thông báo là đã đọc`)
    },
    onError: (_e, _v, ctx) => {
      restore(ctx?.snap)
      toast.error(MARK_FAILED, { action: { label: 'Thử lại', onClick: () => markAllReadNow() } })
    },
    onSettled: refetch,
  })
}

/**
 * The same optimistic mark for code outside a component: a row opened from a
 * toast, an arrival for something already on screen.
 */
export function markReadNow(id: string) {
  const run = async () => {
    let snap: Snapshot | undefined
    try {
      snap = await snapshotAndMark([id])
      await notificationsApi.markRead(id)
    } catch {
      restore(snap)
      toast.error(MARK_FAILED, { action: { label: 'Thử lại', onClick: () => markReadNow(id) } })
    } finally {
      void refetch()
    }
  }
  return run()
}

function markAllReadNow() {
  return (async () => {
    const before = queryClient.getQueryData<{ count: number }>(keys.notifications.unreadCount())?.count ?? 0
    let snap: Snapshot | undefined
    try {
      snap = await snapshotAndMark('all')
      await notificationsApi.markAllRead()
      if (before > 0) toast(`Đã đánh dấu ${before} thông báo là đã đọc`)
    } catch {
      restore(snap)
      toast.error(MARK_FAILED, { action: { label: 'Thử lại', onClick: () => void markAllReadNow() } })
    } finally {
      void refetch()
    }
  })()
}

/** Reads the notices about an invitation the person has just answered. Best effort. */
export async function markInvitationRead(invitationId: string) {
  try {
    await notificationsApi.markAboutRead('workspace_invitation', invitationId)
  } catch {
    // The notice stays unread; answering the invitation itself already worked.
  }
  void refetch()
}
