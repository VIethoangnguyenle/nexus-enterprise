import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { buildConversations, type Conversation } from '../lib/conversations'
import { recentlyMessaged } from '../lib/suggestions/contacts'
import { useAuthStore } from '../stores/auth.store'
import type { Contact } from './useContacts'
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

/** How many people Thường liên lạc shows at most (DESIGN.md "Danh bạ" › Điện thoại). */
export const FREQUENT_CONTACTS = 8

/**
 * Thường liên lạc: the people of the most recent direct messages, latest first. Only people of `directory`, the
 * list the viewer's scope lets them see (the API already left out everyone it hides), so a partner the directory
 * hides, or one of another workspace, is never offered; nor is the viewer.
 */
export function frequentContacts(directs: Conversation[], directory: Contact[], meId: string | undefined): Contact[] {
  return recentlyMessaged(directs, directory, meId, FREQUENT_CONTACTS)
}

/** Thường liên lạc for a screen: asks for the direct messages once `enabled`, then narrows them as `frequentContacts`. */
export function useFrequentContacts(workspaceId: string, directory: Contact[], enabled: boolean): Contact[] {
  const { directs } = useRecentDirects(workspaceId, enabled)
  const meId = useAuthStore((s) => s.user?.id)
  return useMemo(() => frequentContacts(directs, directory, meId), [directs, directory, meId])
}
