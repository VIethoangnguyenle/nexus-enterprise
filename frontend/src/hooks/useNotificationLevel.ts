import { useCallback, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ChannelUnread } from '../api/messaging'
import { DEFAULT_LEVEL, notificationLevelApi, type NotificationLevel } from '../api/notification-levels'
import { toast } from '../components/primitives/Toast'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

type UnreadAnswer = { channels: ChannelUnread[] }

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

/**
 * The level in force now. The unread list comes first: it is refreshed by the
 * socket, so a change made on another device is in it, whereas the level query
 * keeps whatever an earlier optimistic change wrote (nothing refetches it when
 * the screen reads the list instead).
 */
function currentLevel(channelId: string): NotificationLevel {
  return queryClient.getQueryData<UnreadAnswer>(keys.messaging.unreadCounts())
    ?.channels.find((c) => c.channel_id === channelId)?.notification_level
    ?? queryClient.getQueryData<{ level: NotificationLevel }>(keys.notificationLevels.level(channelId))?.level
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

export interface ChangeLevelInput {
  channelId: string
  /** The name the toasts speak of. */
  name: string
  level: NotificationLevel
  /** No toast on success: this call is itself an undo. */
  quiet?: boolean
}

/**
 * Chooses a level for any conversation, with no component bound to it (a row's
 * menu can mute a conversation that is not open). The level query and the
 * unread counts move at once; a refusal puts everything back and offers
 * Thử lại; success offers Hoàn tác, which restores the level that was replaced.
 * Resolves once the server has answered.
 */
export async function changeNotificationLevel(input: ChangeLevelInput): Promise<void> {
  const { channelId, name, level, quiet } = input
  const prev = currentLevel(channelId)
  if (level === prev) return
  const levelKey = keys.notificationLevels.level(channelId)
  await queryClient.cancelQueries({ queryKey: levelKey })
  const item = queryClient.getQueryData<UnreadAnswer>(keys.messaging.unreadCounts())
    ?.channels.find((c) => c.channel_id === channelId)
  queryClient.setQueryData(levelKey, { level })
  patchUnreadItem(channelId, withLevel(level))
  try {
    await notificationLevelApi.setNotificationLevel(channelId, level)
  } catch {
    queryClient.setQueryData(levelKey, { level: prev })
    if (item) patchUnreadItem(channelId, () => item)
    toast.error(`Chưa đổi được thông báo của “${name}”. Kiểm tra kết nối rồi thử lại.`, {
      duration: ERROR_TOAST_MS,
      action: { label: 'Thử lại', onClick: () => void changeNotificationLevel(input) },
    })
    return
  } finally {
    // The counts and the followed-thread list are the server's numbers; ask again either way.
    void queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
    void queryClient.invalidateQueries({ queryKey: keys.notificationLevels.unreadThreads() })
    // The optimistic value in the level query is a guess; the next reader gets the server's.
    void queryClient.invalidateQueries({ queryKey: levelKey })
  }
  if (!quiet) {
    toast(levelToast(name, level, prev), {
      action: { label: 'Hoàn tác', onClick: () => void changeNotificationLevel({ channelId, name, level: prev, quiet: true }) },
    })
  }
}

/** For menus that act on rows other than the open conversation. */
export function useChangeNotificationLevel(): typeof changeNotificationLevel {
  return changeNotificationLevel
}

/**
 * The signed-in user's notification level for one conversation, and how to
 * change it (see `changeNotificationLevel`). `name` is the conversation's
 * display name. `isSaving` is true while a change is on its way. A screen that
 * already holds the unread list (it carries the level) passes `fetchLevel: false`.
 */
export function useNotificationLevel(channelId: string, name: string, opts: { fetchLevel?: boolean } = {}) {
  const query = useQuery({
    queryKey: keys.notificationLevels.level(channelId),
    queryFn: () => notificationLevelApi.getNotificationLevel(channelId),
    enabled: !!channelId && (opts.fetchLevel ?? true),
    // Opening the menu asks again: the level may have been changed on another device since.
    staleTime: 0,
    refetchOnMount: 'always',
  })
  const [saving, setSaving] = useState(0)

  const setLevel = useCallback(
    (level: NotificationLevel) => {
      if (level === currentLevel(channelId)) return
      setSaving((n) => n + 1)
      void changeNotificationLevel({ channelId, name, level }).finally(() => setSaving((n) => n - 1))
    },
    [channelId, name],
  )

  return {
    level: query.data?.level ?? DEFAULT_LEVEL,
    isLoading: query.isLoading,
    isSaving: saving > 0,
    setLevel,
  }
}
