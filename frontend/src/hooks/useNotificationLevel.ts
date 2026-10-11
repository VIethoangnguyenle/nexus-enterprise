import { useCallback } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import type { ChannelUnread } from '../api/messaging'
import { DEFAULT_LEVEL, notificationLevelApi, type NotificationLevel } from '../api/notification-levels'
import { toast } from '../components/primitives/Toast'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

type UnreadAnswer = { channels: ChannelUnread[] }
type Variables = { level: NotificationLevel; quiet?: boolean }
type Context = { prev: NotificationLevel; item?: ChannelUnread }

/** A failure stays up longer than a confirmation: it asks for a decision (design: 8s). */
const ERROR_TOAST_MS = 8000

/** What the confirmation says, by the level chosen and the one it replaced. */
export function levelToast(name: string, level: NotificationLevel, prev: NotificationLevel): string {
  if (level === 'off') return `Đã tắt thông báo của “${name}”`
  if (level === 'all') return `Thông báo mọi tin của “${name}”`
  return prev === 'off'
    ? `Đã bật lại thông báo của “${name}”`
    : `Thông báo tin chính và thread theo dõi của “${name}”`
}

function currentLevel(channelId: string): NotificationLevel {
  return queryClient.getQueryData<{ level: NotificationLevel }>(keys.notificationLevels.level(channelId))?.level
    ?? queryClient.getQueryData<UnreadAnswer>(keys.messaging.unreadCounts())
      ?.channels.find((c) => c.channel_id === channelId)?.notification_level
    ?? DEFAULT_LEVEL
}

/** Rewrites one conversation's row in the unread cache; the other rows stay as they are. */
function patchUnreadItem(channelId: string, patch: (item: ChannelUnread) => ChannelUnread) {
  queryClient.setQueryData<UnreadAnswer>(keys.messaging.unreadCounts(), (old) =>
    old && { ...old, channels: old.channels.map((c) => (c.channel_id === channelId ? patch(c) : c)) })
}

/** A muted conversation counts nothing; its mentions still show. */
const withLevel = (level: NotificationLevel) => (item: ChannelUnread): ChannelUnread => {
  if (level !== 'off') return { ...item, notification_level: level }
  const { thread_unread_count: _dropped, ...rest } = item
  return { ...rest, notification_level: level, unread_count: 0 }
}

/**
 * The signed-in user's notification level for one conversation. Choosing a level
 * shows it everywhere at once (the level query and the unread counts that feed the
 * navigator, Trang chủ and the totals), then lets the server settle it. A refusal
 * puts everything back and offers Thử lại; success offers Hoàn tác, which restores
 * the level that was replaced. `name` is the conversation's display name.
 */
export function useNotificationLevel(channelId: string, name: string) {
  const query = useQuery({
    queryKey: keys.notificationLevels.level(channelId),
    queryFn: () => notificationLevelApi.getNotificationLevel(channelId),
    enabled: !!channelId,
  })

  const mutation = useMutation<{ level: NotificationLevel }, Error, Variables, Context>({
    meta: { silentError: true },
    mutationFn: ({ level }) => notificationLevelApi.setNotificationLevel(channelId, level),
    onMutate: async ({ level }) => {
      await queryClient.cancelQueries({ queryKey: keys.notificationLevels.level(channelId) })
      const prev = currentLevel(channelId)
      const item = queryClient.getQueryData<UnreadAnswer>(keys.messaging.unreadCounts())
        ?.channels.find((c) => c.channel_id === channelId)
      queryClient.setQueryData(keys.notificationLevels.level(channelId), { level })
      patchUnreadItem(channelId, withLevel(level))
      return { prev, item }
    },
    onError: (_err, { level, quiet }, ctx) => {
      const prev = ctx?.prev ?? DEFAULT_LEVEL
      queryClient.setQueryData(keys.notificationLevels.level(channelId), { level: prev })
      if (ctx?.item) patchUnreadItem(channelId, () => ctx.item!)
      toast.error(`Chưa đổi được thông báo của “${name}”. Kiểm tra kết nối rồi thử lại.`, {
        duration: ERROR_TOAST_MS,
        action: { label: 'Thử lại', onClick: () => mutation.mutate({ level, quiet }) },
      })
    },
    onSuccess: (_answer, { level, quiet }, ctx) => {
      if (quiet) return
      const prev = ctx?.prev ?? DEFAULT_LEVEL
      toast(levelToast(name, level, prev), {
        action: { label: 'Hoàn tác', onClick: () => mutation.mutate({ level: prev, quiet: true }) },
      })
    },
    // The counts and the followed-thread list are the server's numbers; ask again either way.
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
      void queryClient.invalidateQueries({ queryKey: keys.notificationLevels.unreadThreads() })
    },
  })

  const { mutate } = mutation
  const setLevel = useCallback(
    (level: NotificationLevel) => {
      if (level === currentLevel(channelId)) return
      mutate({ level })
    },
    [channelId, mutate],
  )

  return {
    level: query.data?.level ?? DEFAULT_LEVEL,
    isLoading: query.isLoading,
    isSaving: mutation.isPending,
    setLevel,
  }
}
