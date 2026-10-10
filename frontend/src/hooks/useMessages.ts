import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { messagingApi, type SendMessageInput, type Message, type ReactionGroup } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { keys } from './keys'

// --- Messages ---

export const messagesQueryOptions = (channelId: string) =>
  queryOptions({ queryKey: keys.messaging.messages(channelId), queryFn: () => messagingApi.listMessages(channelId), enabled: !!channelId })

export const threadQueryOptions = (messageId: string) =>
  queryOptions({ queryKey: keys.messaging.thread(messageId), queryFn: () => messagingApi.getThread(messageId), enabled: !!messageId })

export function useMessages(channelId: string) { return useQuery(messagesQueryOptions(channelId)) }
export function useThread(messageId: string) { return useQuery(threadQueryOptions(messageId)) }

/** Sends a message with optimistic UI: message appears immediately, no refetch needed. */
export function useSendMessage(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (params: string | { content: string; linkedEntity?: { type: string; id: string } }) => {
      if (typeof params === 'string') {
        return messagingApi.sendMessage(channelId, params)
      }
      const input: SendMessageInput = {
        content: params.content,
        content_format: 'html',
        linked_entity_type: params.linkedEntity?.type || '',
        linked_entity_id: params.linkedEntity?.id || '',
      }
      return messagingApi.sendMessage(channelId, input)
    },

    // Optimistic: insert temp message BEFORE server responds
    onMutate: async (params) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))
      const user = useAuthStore.getState().user

      const content = typeof params === 'string' ? params : params.content
      const tempMsg: Message & { _optimistic: true } = {
        id: `temp-${Date.now()}`,
        channel_id: channelId,
        sender_id: user?.id || '',
        sender_name: user?.username || '',
        content,
        content_format: 'html',
        created_at: new Date().toISOString(),
        _optimistic: true,
      }

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
        (old) => old ? { ...old, messages: [tempMsg as Message, ...(old.messages || [])] } : old,
      )

      return { previous }
    },

    // Replace temp message with server response (WS event will dedup via ID match)
    onSuccess: (serverMsg) => {
      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
        (old) => {
          if (!old) return old
          return {
            ...old,
            // Keep the optimistic row's key so the topic is not remounted (no
            // flash, no second insert animation) when the server copy lands.
            messages: (old.messages || []).some((m) => m.id === serverMsg.id)
              ? (old.messages || []).filter((m: Message & { _optimistic?: boolean }) => !m._optimistic)
              : (old.messages || []).map((m: Message & { _optimistic?: boolean }) =>
                  m._optimistic ? { ...serverMsg, _clientKey: m.id } : m,
                ),
          }
        },
      )
    },

    // Rollback on error
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
      }
    },
  })
}

type MessageList = { messages: Message[]; has_more: boolean }
type Thread = { messages: Message[] }
type Optimistic = Message & { _optimistic?: boolean }

/**
 * Replies inside a topic, with optimistic UI: the reply shows in the thread
 * at once and the topic's reply count moves with it. The WebSocket echo of
 * the same reply is de-duplicated by id in websocket.store.
 */
export function useSendReply(channelId: string, parentId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (content: string) =>
      messagingApi.sendMessage(channelId, { content, content_format: 'html', parent_message_id: parentId }),
    onMutate: async (content) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.thread(parentId) })
      const prevThread = queryClient.getQueryData<Thread>(keys.messaging.thread(parentId))
      const prevList = queryClient.getQueryData<MessageList>(keys.messaging.messages(channelId))
      const user = useAuthStore.getState().user
      const temp: Optimistic = {
        id: `temp-${Date.now()}`,
        channel_id: channelId,
        sender_id: user?.id || '',
        sender_name: user?.username || '',
        content,
        content_format: 'html',
        parent_message_id: parentId,
        created_at: new Date().toISOString(),
        _optimistic: true,
      }
      queryClient.setQueryData<Thread>(keys.messaging.thread(parentId), (old) =>
        old ? { ...old, messages: [...(old.messages || []), temp] } : old,
      )
      queryClient.setQueryData<MessageList>(keys.messaging.messages(channelId), (old) =>
        old
          ? { ...old, messages: old.messages.map((m) => (m.id === parentId ? { ...m, reply_count: (m.reply_count || 0) + 1 } : m)) }
          : old,
      )
      return { prevThread, prevList }
    },
    onSuccess: (serverMsg) => {
      queryClient.setQueryData<Thread>(keys.messaging.thread(parentId), (old) => {
        if (!old) return old
        const already = old.messages.some((m) => m.id === serverMsg.id)
        const rest = old.messages.filter((m: Optimistic) => !m._optimistic)
        return { ...old, messages: already ? rest : [...rest, serverMsg] }
      })
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevThread) queryClient.setQueryData(keys.messaging.thread(parentId), ctx.prevThread)
      if (ctx?.prevList) queryClient.setQueryData(keys.messaging.messages(channelId), ctx.prevList)
    },
  })
}

// --- Reactions ---

/** Toggles a reaction with optimistic UI — instant feedback, WS event deduplication. */
export function useToggleReaction(channelId: string) {
  return useMutation({
    mutationFn: ({ messageId, emoji, hasReacted }: { messageId: string; emoji: string; hasReacted: boolean }) =>
      hasReacted
        ? messagingApi.removeReaction(messageId, emoji)
        : messagingApi.addReaction(messageId, emoji),
    onMutate: async ({ messageId, emoji, hasReacted }) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))
      const userId = useAuthStore.getState().user?.id || ''

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
        (old) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m) => {
              if (m.id !== messageId) return m
              const reactions = [...(m.reactions || [])]
              const idx = reactions.findIndex((r) => r.emoji === emoji)
              if (hasReacted) {
                // Removing reaction
                const current = reactions[idx]
                if (current) {
                  const group: ReactionGroup = { ...current }
                  group.user_ids = group.user_ids.filter((id) => id !== userId)
                  group.count = group.user_ids.length
                  if (group.count <= 0) reactions.splice(idx, 1)
                  else reactions[idx] = group
                }
              } else {
                // Adding reaction
                const current = reactions[idx]
                if (current) {
                  const group: ReactionGroup = { ...current }
                  if (!group.user_ids.includes(userId)) {
                    group.count += 1
                    group.user_ids = [...group.user_ids, userId]
                  }
                  reactions[idx] = group
                } else {
                  reactions.push({ emoji, count: 1, user_ids: [userId] })
                }
              }
              return { ...m, reactions }
            }),
          }
        },
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
    },
  })
}

// --- Pins ---

export function usePins(channelId: string) {
  return useQuery({
    queryKey: keys.messaging.pins(channelId),
    queryFn: () => messagingApi.listPins(channelId),
    enabled: !!channelId,
  })
}

/** Toggles pin status with optimistic UI. */
export function useTogglePin(channelId: string) {
  return useMutation({
    mutationFn: ({ messageId, isPinned }: { messageId: string; isPinned: boolean }) =>
      isPinned
        ? messagingApi.unpinMessage(channelId, messageId)
        : messagingApi.pinMessage(channelId, messageId),
    onMutate: async ({ messageId, isPinned }) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
        (old) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m) =>
              m.id === messageId ? { ...m, is_pinned: !isPinned } : m,
            ),
          }
        },
      )
      return { previous }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: keys.messaging.pins(channelId) })
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
    },
  })
}
