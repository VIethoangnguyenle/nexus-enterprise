import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { messagingApi, type ConversationActivity } from '../api/messaging'
import { timestampToRfc3339 } from '../lib/stream-model'
import { keys } from './keys'
import { startupPart } from './useChatState'

const FOLLOWED_PAGE = 20

/**
 * The newest message and thread reply of every conversation the person reads:
 * the previews of the conversation list, there from the moment the app opens.
 * Without it the list falls back to what live events carried this session.
 */
export function useConversationActivity(enabled = true) {
  return useQuery({
    enabled,
    queryKey: keys.messaging.conversationActivity(),
    queryFn: () => startupPart(keys.messaging.conversationActivity(), undefined, (s) => s.activity, () => messagingApi.conversationActivity()),
    select: (rows): Record<string, ConversationActivity> => Object.fromEntries(rows.map((r) => [r.channel_id, r])),
  })
}

/** Threads the person follows, across every conversation they read, most recently active first. */
export function useFollowedThreads(enabled = true) {
  return useInfiniteQuery({
    queryKey: keys.messaging.followedThreads(),
    initialPageParam: undefined as { before: string; beforeId: string } | undefined,
    queryFn: ({ pageParam }) =>
      messagingApi.followedThreads({ limit: FOLLOWED_PAGE, before: pageParam?.before, beforeId: pageParam?.beforeId }),
    getNextPageParam: (page) => {
      const last = page.messages[page.messages.length - 1]
      if (!page.has_more || !last) return undefined
      const before = timestampToRfc3339(last.last_reply_at)
      return before ? { before, beforeId: last.id } : undefined
    },
    enabled,
  })
}
