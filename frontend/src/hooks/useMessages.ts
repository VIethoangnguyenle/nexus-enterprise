import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { messagingApi, type SendMessageInput, type Message, type MessageAttachment, type ReactionGroup } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { keepOlderPages } from '../lib/stream-model'
import { QUOTED_MESSAGE_UNAVAILABLE, reasonOf, statusOf } from '../lib/errors'
import { useAuthStore } from '../stores/auth.store'
import { threadScope, useDraftsStore, type PendingQuote } from '../stores/drafts.store'
import { keys } from './keys'

// --- Messages ---

export const messagesQueryOptions = (channelId: string) =>
  queryOptions({
    queryKey: keys.messaging.messages(channelId),
    // The newest page; older pages the reader loaded by scrolling up survive a refetch.
    queryFn: async () =>
      keepOlderPages(
        await messagingApi.listMessages(channelId),
        queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId)),
      ),
    enabled: !!channelId,
  })

export const threadQueryOptions = (messageId: string) =>
  queryOptions({ queryKey: keys.messaging.thread(messageId), queryFn: () => messagingApi.getThread(messageId), enabled: !!messageId })

export function useMessages(channelId: string) { return useQuery(messagesQueryOptions(channelId)) }
export function useThread(messageId: string) { return useQuery(threadQueryOptions(messageId)) }

export type SendMessageParams =
  | string
  | {
      content: string
      mentions?: string[]
      linkedEntity?: { type: string; id: string }
      attachments?: { drive_file_id: string; width?: number; height?: number }[]
      optimisticAttachments?: MessageAttachment[]
    }


/**
 * The quote waiting in a composer, for a message that is about to leave it. A
 * card link (a document or an approval posted as a message) never carries it,
 * and neither does a quote whose message was deleted meanwhile.
 */
function liveQuote(scope: string, linked: unknown): PendingQuote | undefined {
  if (linked) return undefined
  const q = useDraftsStore.getState().drafts[scope]?.quote
  return q && !q.deleted ? q : undefined
}

/** What the optimistic copy of a message shows of the message it quotes. */
const quotedOf = (q: PendingQuote | undefined): Message['quoted'] =>
  q ? { id: q.id, sender_id: q.sender_id, sender_name: q.sender_name, text: q.text || q.summary?.label } : undefined

/** Quotes a send in flight took from their composers, by the params object of that send. */
const taken = new WeakMap<object, PendingQuote | undefined>()

/**
 * A send takes the quote off the composer's strip at once, so a second message
 * sent before the first is answered does not carry it again.
 */
function takeQuote(scope: string, linked: unknown, send: object): PendingQuote | undefined {
  const q = liveQuote(scope, linked)
  if (q) useDraftsStore.getState().setQuote(scope, null)
  taken.set(send, q)
  return q
}

/**
 * A failed send gives the quote back, unless another was chosen meanwhile. It
 * shows as deleted only when the server says the quoted message is unavailable:
 * any other refusal leaves a quote that is still good.
 */
function returnQuote(scope: string, sent: PendingQuote | undefined, err: unknown) {
  if (!sent) return
  const store = useDraftsStore.getState()
  if (store.drafts[scope]?.quote) return
  const gone = statusOf(err) === 400 && reasonOf(err) === QUOTED_MESSAGE_UNAVAILABLE
  store.setQuote(scope, gone ? { ...sent, deleted: true } : sent)
}

/** Sends a message with optimistic UI: message appears immediately, no refetch needed. */
export function useSendMessage(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (params: SendMessageParams) => {
      if (typeof params === 'string') {
        return messagingApi.sendMessage(channelId, params)
      }
      const quote = taken.get(params)
      const input: SendMessageInput = {
        content: params.content,
        content_format: 'html',
        linked_entity_type: params.linkedEntity?.type || '',
        linked_entity_id: params.linkedEntity?.id || '',
        ...(params.mentions?.length ? { mentions: params.mentions } : {}),
        ...(params.attachments?.length ? { attachments: params.attachments } : {}),
        ...(quote ? { quoted_message_id: quote.id } : {}),
      }
      return messagingApi.sendMessage(channelId, input)
    },

    // Optimistic: insert temp message BEFORE server responds
    onMutate: async (params) => {
      const quote = typeof params === 'string' ? undefined : takeQuote(channelId, params.linkedEntity, params)
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))
      const user = useAuthStore.getState().user

      const content = typeof params === 'string' ? params : params.content
      const attachments = typeof params === 'string'
        ? undefined
        : params.optimisticAttachments ?? params.attachments?.map((a) => ({
            drive_file_id: a.drive_file_id,
            file_name: '',
            mime_type: '',
            size_bytes: 0,
            width: a.width,
            height: a.height,
          }))
      const tempMsg: Message & { _optimistic: true } = {
        id: `temp-${Date.now()}`,
        channel_id: channelId,
        sender_id: user?.id || '',
        sender_name: user?.username || '',
        content,
        content_format: 'html',
        created_at: new Date().toISOString(),
        attachments,
        quoted: quotedOf(quote),
        _optimistic: true,
      }

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
        (old) => old ? { ...old, messages: [tempMsg as Message, ...(old.messages || [])] } : old,
      )

      return { previous, quote }
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
    onError: (err, _vars, context) => {
      returnQuote(channelId, context?.quote, err)
      if (context?.previous) {
        queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
      }
    },
    onSettled: (_data, _err, vars) => {
      if (typeof vars !== 'string' && vars?.optimisticAttachments) {
        vars.optimisticAttachments.forEach((att) => {
          if (att.localUrl) URL.revokeObjectURL(att.localUrl)
        })
      }
    },
  })
}

type MessageList = { messages: Message[]; has_more: boolean }
type Thread = { messages: Message[] }
type Optimistic = Message & { _optimistic?: boolean }

export type SendReplyParams = {
  content: string
  mentions?: string[]
  /** A card posted as the reply (a document, an approval request). */
  linkedEntity?: { type: string; id: string }
  attachments?: { drive_file_id: string; width?: number; height?: number }[]
  optimisticAttachments?: MessageAttachment[]
}

/**
 * Replies inside a topic, with optimistic UI: the reply shows in the thread
 * at once and the topic's reply count moves with it. The WebSocket echo of
 * the same reply is de-duplicated by id in websocket.store.
 */
export function useSendReply(channelId: string, parentId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (params: SendReplyParams) => {
      const { content, mentions, attachments, linkedEntity } = params
      const quote = taken.get(params)
      return messagingApi.sendMessage(channelId, {
        content,
        content_format: 'html',
        parent_message_id: parentId,
        ...(linkedEntity ? { linked_entity_type: linkedEntity.type, linked_entity_id: linkedEntity.id } : {}),
        ...(mentions?.length ? { mentions } : {}),
        ...(attachments?.length ? { attachments } : {}),
        ...(quote ? { quoted_message_id: quote.id } : {}),
      })
    },
    onMutate: async (params) => {
      const { content, attachments, optimisticAttachments, linkedEntity } = params
      const quote = takeQuote(threadScope(parentId), linkedEntity, params)
      await queryClient.cancelQueries({ queryKey: keys.messaging.thread(parentId) })
      const prevThread = queryClient.getQueryData<Thread>(keys.messaging.thread(parentId))
      const prevList = queryClient.getQueryData<MessageList>(keys.messaging.messages(channelId))
      const user = useAuthStore.getState().user
      const tempAttachments = optimisticAttachments ?? attachments?.map((a) => ({
        drive_file_id: a.drive_file_id,
        file_name: '',
        mime_type: '',
        size_bytes: 0,
        width: a.width,
        height: a.height,
      }))
      const temp: Optimistic = {
        id: `temp-${Date.now()}`,
        channel_id: channelId,
        sender_id: user?.id || '',
        sender_name: user?.username || '',
        content,
        content_format: 'html',
        parent_message_id: parentId,
        created_at: new Date().toISOString(),
        attachments: tempAttachments,
        ...(linkedEntity ? { linked_entity_type: linkedEntity.type, linked_entity_id: linkedEntity.id } : {}),
        quoted: quotedOf(quote),
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
      return { prevThread, prevList, quote }
    },
    onSuccess: (serverMsg) => {
      queryClient.setQueryData<Thread>(keys.messaging.thread(parentId), (old) => {
        if (!old) return old
        const already = old.messages.some((m) => m.id === serverMsg.id)
        const rest = old.messages.filter((m: Optimistic) => !m._optimistic)
        return { ...old, messages: already ? rest : [...rest, serverMsg] }
      })
    },
    onError: (err, _vars, ctx) => {
      returnQuote(threadScope(parentId), ctx?.quote, err)
      if (ctx?.prevThread) queryClient.setQueryData(keys.messaging.thread(parentId), ctx.prevThread)
      if (ctx?.prevList) queryClient.setQueryData(keys.messaging.messages(channelId), ctx.prevList)
    },
    onSettled: (_data, _err, vars) => {
      if (vars?.optimisticAttachments) {
        vars.optimisticAttachments.forEach((att) => {
          if (att.localUrl) URL.revokeObjectURL(att.localUrl)
        })
      }
    },
  })
}

/**
 * Posts a card (a new document, an approval request) as a message of its own:
 * in the conversation, or as a reply when a thread root is given.
 */
export function useSendCardMessage(channelId: string, parentMessageId?: string) {
  const main = useSendMessage(channelId)
  const reply = useSendReply(channelId, parentMessageId ?? '')
  return {
    isPending: parentMessageId ? reply.isPending : main.isPending,
    send: (linkedEntity: { type: string; id: string }) =>
      parentMessageId
        ? reply.mutateAsync({ content: '', linkedEntity })
        : main.mutateAsync({ content: '', linkedEntity }),
  }
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
