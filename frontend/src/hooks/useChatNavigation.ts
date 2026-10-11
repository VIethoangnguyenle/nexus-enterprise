import { useMemo } from 'react'
import { arrange, type Arranged } from '../lib/chat-sections'
import { useChatSections } from './useChatSections'
import { useConversations, type ConversationsState } from './useConversations'

export interface ChatNavigation extends ConversationsState {
  /** The conversations split between the user's sections and the two default lists. */
  arranged: Arranged
  sectionsApi: ReturnType<typeof useChatSections>
}

/**
 * What the navigator column and the phone's Tin nhắn tab list: conversations
 * (with unread and activity) arranged into the user's own sections. A section
 * list that fails to load leaves the conversations in their default lists
 * rather than hiding them.
 */
export function useChatNavigation(workspaceId: string): ChatNavigation {
  const conversations = useConversations(workspaceId)
  const sectionsApi = useChatSections(workspaceId)
  const arranged = useMemo(
    () => arrange(conversations.conversations, sectionsApi.sections),
    [conversations.conversations, sectionsApi.sections],
  )
  return {
    ...conversations,
    isLoading: conversations.isLoading || sectionsApi.isLoading,
    arranged,
    sectionsApi,
  }
}
