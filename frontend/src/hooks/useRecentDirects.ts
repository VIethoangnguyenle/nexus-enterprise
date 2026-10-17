import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { buildConversations, type Conversation } from '../lib/conversations'
import { useAuthStore } from '../stores/auth.store'
import { useConversationActivity } from './useConversationActivity'
import { directMessagesQueryOptions } from './useMessaging'
import { usePeople } from './usePeople'

const NO_LIVE = {}

/**
 * The person's direct messages with when each last said something, for a suggestion outside Chat (Danh bạ's
 * "Nhắn gần đây"). Only the two lists it needs, asked for once `enabled` (the search box was focused), and no live
 * subscription: unlike `useConversations` it does not keep a screen re-rendering with every message anywhere.
 */
export function useRecentDirects(workspaceId: string, enabled: boolean): { directs: Conversation[]; isLoading: boolean } {
  const dmsQ = useQuery({ ...directMessagesQueryOptions(), enabled })
  const activityQ = useConversationActivity(enabled)
  const people = usePeople(workspaceId)
  const user = useAuthStore((s) => s.user)

  const directs = useMemo(
    () => buildConversations({
      spaces: [],
      dms: dmsQ.data?.channels ?? [],
      unread: {},
      activity: activityQ.data,
      lastMessages: NO_LIVE,
      me: { id: user?.id, username: user?.username },
      dir: people,
    }),
    [dmsQ.data, activityQ.data, user, people],
  )
  return { directs, isLoading: enabled && (dmsQ.isLoading || activityQ.isLoading) }
}
