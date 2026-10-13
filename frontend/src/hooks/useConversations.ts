import { useEffect, useMemo } from 'react'
import { useChannels, useDirectMessages, useUnreadCounts } from './useMessaging'
import { useConversationActivity } from './useConversationActivity'
import { usePeople } from './usePeople'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { timestampToRfc3339 } from '../lib/stream-model'
import {
  buildConversations, sortConversations, totalUnread, type Conversation, type ConversationMeta,
} from '../lib/conversations'

export interface ConversationsState {
  conversations: Conversation[]
  spaces: Conversation[]
  directs: Conversation[]
  unreadTotal: number
  isLoading: boolean
  isError: boolean
  refetch: () => void
}

/**
 * Spaces of the workspace plus the user's DMs, with unread counts, what each
 * last said (the activity list, then live events when newer), sorted for Home.
 * Four requests in total, never one per conversation.
 *
 * While mounted it also subscribes to every listed conversation, so Home and
 * the navigator move when someone writes anywhere, not only in the open one.
 */
export function useConversations(workspaceId: string): ConversationsState {
  const spacesQ = useChannels(workspaceId)
  const dmsQ = useDirectMessages()
  const unreadQ = useUnreadCounts()
  const activityQ = useConversationActivity()
  const people = usePeople(workspaceId)
  const lastMessages = useWebSocketStore((s) => s.lastMessages)
  const subscribeMany = useWebSocketStore((s) => s.subscribeMany)
  const user = useAuthStore((s) => s.user)

  const unread = useMemo(() => {
    const map: Record<string, number> = {}
    for (const u of unreadQ.data?.channels ?? []) map[u.channel_id] = u.unread_count
    return map
  }, [unreadQ.data])

  // Level, @mentions and star come with the same unread list: one request for all three.
  const meta = useMemo(() => {
    const map: Record<string, ConversationMeta> = {}
    for (const u of unreadQ.data?.channels ?? []) {
      map[u.channel_id] = {
        level: u.notification_level, mentions: u.mention_count, starred: u.starred,
        starredAt: Date.parse(timestampToRfc3339(u.starred_at)) || undefined,
      }
    }
    return map
  }, [unreadQ.data])

  const conversations = useMemo(
    () =>
      sortConversations(
        buildConversations({
          spaces: spacesQ.data?.channels ?? [],
          dms: dmsQ.data?.channels ?? [],
          unread,
          meta,
          activity: activityQ.data,
          lastMessages,
          me: { id: user?.id, username: user?.username },
          dir: people,
        }),
      ),
    [spacesQ.data, dmsQ.data, unread, meta, activityQ.data, lastMessages, user, people],
  )

  const ids = useMemo(() => conversations.map((c) => c.id).sort().join(','), [conversations])
  useEffect(() => {
    if (!ids) return
    return subscribeMany(ids.split(','))
  }, [ids, subscribeMany])

  return {
    conversations,
    spaces: conversations.filter((c) => c.kind === 'space'),
    directs: conversations.filter((c) => c.kind === 'dm'),
    unreadTotal: totalUnread(conversations),
    isLoading: spacesQ.isLoading || dmsQ.isLoading,
    isError: spacesQ.isError && dmsQ.isError,
    refetch: () => {
      void spacesQ.refetch()
      void dmsQ.refetch()
      void activityQ.refetch()
    },
  }
}
