import { useQuery } from '@tanstack/react-query'
import { loadChatState, type ChatState } from '../api/chat-state'
import { queryClient } from '../lib/query-client'
import { tenantIdFromToken, useAuthStore } from '../stores/auth.store'
import { keys } from './keys'

/** Whose chat state a request can ask for: the person, and the workspace the session speaks for. */
function sessionScope(): { owner: string; tenant: string } | null {
  const { accessToken, user } = useAuthStore.getState()
  const tenant = tenantIdFromToken(accessToken)
  if (!tenant || !user?.id) return null
  return { owner: `${user.id}:${tenant}`, tenant }
}

/**
 * The first answer of a query that the chat state also carries (channels,
 * direct messages, unread, previews, sections, mention and thread counts).
 *
 * While the query holds nothing yet, it takes its part of the one shared
 * chat-state request instead of making its own: opening the app is one request
 * however many screens ask, and in whatever order they mount. Once it holds
 * data, or when the chat state cannot be had (another workspace than the
 * session's, an older server), it asks its own endpoint, exactly as before.
 */
export async function startupPart<T>(
  queryKey: readonly unknown[],
  workspaceId: string | undefined,
  pick: (state: ChatState) => T,
  ownEndpoint: () => Promise<T>,
): Promise<T> {
  const scope = sessionScope()
  if (!scope || (workspaceId !== undefined && workspaceId !== scope.tenant) || queryClient.getQueryData(queryKey) !== undefined) {
    return ownEndpoint()
  }
  try {
    const { state } = await loadChatState(scope.owner, scope.tenant)
    return pick(state)
  } catch {
    return ownEndpoint()
  }
}

/** Puts each part of a fresh chat state under the query key its own hook reads. */
export function seedFromChatState(wsId: string, state: ChatState) {
  queryClient.setQueryData(keys.messaging.channels(wsId), { channels: state.channels })
  queryClient.setQueryData(keys.messaging.dms(), { channels: state.dms })
  queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: state.unread })
  queryClient.setQueryData(keys.messaging.conversationActivity(), state.activity)
  queryClient.setQueryData(keys.messaging.sections(wsId), [...state.sections].sort((a, b) => a.position - b.position))
  queryClient.setQueryData(keys.shortcuts.mentionsUnread(wsId), state.mention_unread_count)
  queryClient.setQueryData(keys.notificationLevels.unreadThreads(), { messages: state.unread_threads })
}

/**
 * The chat navigator's state, revalidated when the app opens or comes back. A
 * state that changed since the copy held is put under the keys the rest of the
 * app reads; one that did not (304) leaves them as they are.
 */
export function useChatState(wsId: string) {
  return useQuery({
    queryKey: keys.messaging.chatState(wsId),
    queryFn: async () => {
      const scope = sessionScope()
      if (!scope || scope.tenant !== wsId) return null
      const { state, changed } = await loadChatState(scope.owner, scope.tenant)
      if (changed) seedFromChatState(wsId, state)
      return state
    },
    enabled: !!wsId,
    // Failing here costs nothing: each part has its own endpoint to fall back on.
    retry: false,
  })
}
