import { useQuery } from '@tanstack/react-query'
import { notificationLevelApi } from '../api/notification-levels'
import { keys } from './keys'
import { startupPart } from './useChatState'

/** Enough for "Xem thêm N thread" to be honest without paging. */
export const UNREAD_THREADS_LIMIT = 20

/**
 * Followed threads with unread replies, newest reply first, across the
 * conversations the person has not muted. The server computes it; live frames
 * only mark it stale (`invalidateUnreadThreadsFor`).
 */
export function useUnreadThreads(enabled = true) {
  return useQuery({
    queryKey: keys.notificationLevels.unreadThreads(),
    queryFn: () =>
      startupPart(keys.notificationLevels.unreadThreads(), undefined, (s) => ({ messages: s.unread_threads }), () =>
        notificationLevelApi.unreadThreads(UNREAD_THREADS_LIMIT)),
    select: (answer) => answer.messages,
    enabled,
  })
}
