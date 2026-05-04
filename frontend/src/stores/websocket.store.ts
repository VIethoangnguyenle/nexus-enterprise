import { create } from 'zustand'
import { queryClient } from '../lib/query-client'
import { usePermissionStore } from './permission.store'
import {
  ClientEnvelope,
  ServerEnvelope,
} from '../generated/proto/messaging/ws'
import type {
  ServerEnvelope as ServerEnvelopeType,
  ChatMessage as WSChatMessage,
} from '../generated/proto/messaging/ws'
import type { Message, ReactionGroup, Poll, ChatTask } from '../api/messaging'

const WS_DEBUG = () => typeof localStorage !== 'undefined' && localStorage.getItem('WS_DEBUG') === '1'

/** Max reconnect backoff in ms. */
const MAX_RECONNECT_DELAY = 30000

/** Tracks channels the client has subscribed to so we can re-subscribe on reconnect. */
const subscribedChannels = new Set<string>()

interface WebSocketState {
  ws: WebSocket | null
  connected: boolean
  authenticated: boolean
  typingUsers: Record<string, string[]>
  reconnectAttempt: number
  /** Last message per channel — used by ChatList for preview text. */
  lastMessages: Record<string, { content: string; timestamp: string; senderName: string }>
  /** Online user IDs + usernames — updated via PresenceEvent. */
  onlineUsers: Record<string, string>
  connect: (token: string) => void
  disconnect: () => void
  sendTyping: (channelId: string) => void
  sendSubscribe: (channelId: string) => void
  sendUnsubscribe: (channelId: string) => void
}

/** Send a binary-encoded ClientEnvelope over WebSocket. */
function sendEnvelope(ws: WebSocket, envelope: Parameters<typeof ClientEnvelope.toBinary>[0]) {
  if (ws.readyState !== WebSocket.OPEN) return
  ws.send(ClientEnvelope.toBinary(envelope))
}

/** Calculate exponential backoff delay with jitter. */
function reconnectDelay(attempt: number): number {
  const base = Math.min(1000 * Math.pow(2, attempt), MAX_RECONNECT_DELAY)
  const jitter = Math.random() * 500
  return base + jitter
}

/** Re-subscribe to all previously subscribed channels after reconnect. */
function resubscribeChannels(ws: WebSocket) {
  for (const channelId of subscribedChannels) {
    sendEnvelope(ws, {
      payload: { oneofKind: 'subscribe', subscribe: { channelId } },
    })
  }
  if (WS_DEBUG() && subscribedChannels.size > 0) {
    console.log(`[WS] re-subscribed to ${subscribedChannels.size} channel(s)`)
  }
}

export const useWebSocketStore = create<WebSocketState>()((set, get) => ({
  ws: null,
  connected: false,
  authenticated: false,
  typingUsers: {},
  reconnectAttempt: 0,
  lastMessages: {},
  onlineUsers: {},

  connect: (token) => {
    const existing = get().ws
    if (existing && existing.readyState === WebSocket.OPEN) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/ws`
    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => {
      set({ connected: true, reconnectAttempt: 0 })
      // Auth handshake: send token as first message
      sendEnvelope(ws, {
        payload: { oneofKind: 'auth', auth: { token } },
      })
    }

    ws.onclose = (event) => {
      const attempt = get().reconnectAttempt
      set({ connected: false, authenticated: false, ws: null, reconnectAttempt: attempt + 1 })

      // Don't reconnect on intentional close (code 1000)
      if (event.code === 1000) return

      const delay = reconnectDelay(attempt)
      if (WS_DEBUG()) {
        console.log(`[WS] reconnecting in ${Math.round(delay)}ms (attempt ${attempt + 1})`)
      }

      setTimeout(() => {
        if (get().ws === null) get().connect(token)
      }, delay)
    }

    ws.onerror = () => {
      // onerror is always followed by onclose, so reconnect happens there
      if (WS_DEBUG()) console.warn('[WS] connection error')
    }

    ws.onmessage = (event: MessageEvent) => {
      if (!(event.data instanceof ArrayBuffer)) return

      const envelope = ServerEnvelope.fromBinary(new Uint8Array(event.data))

      if (WS_DEBUG()) {
        console.log('[WS]', envelope.payload.oneofKind, envelope)
      }

      handleServerMessage(envelope, set, ws)
    }

    set({ ws })
  },

  disconnect: () => {
    const ws = get().ws
    if (ws) ws.close(1000, 'user disconnect')
    subscribedChannels.clear()
    set({ ws: null, connected: false, authenticated: false, reconnectAttempt: 0 })
  },

  sendTyping: (channelId) => {
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, {
        payload: { oneofKind: 'typing', typing: { channelId } },
      })
    }
  },

  sendSubscribe: (channelId) => {
    subscribedChannels.add(channelId)
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, {
        payload: { oneofKind: 'subscribe', subscribe: { channelId } },
      })
    }
  },

  sendUnsubscribe: (channelId) => {
    subscribedChannels.delete(channelId)
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, {
        payload: { oneofKind: 'unsubscribe', unsubscribe: { channelId } },
      })
    }
  },
}))

/** Route decoded ServerEnvelope payloads to TanStack Query cache invalidation. */
function handleServerMessage(
  envelope: ServerEnvelopeType,
  set: (fn: (s: WebSocketState) => Partial<WebSocketState>) => void,
  ws: WebSocket,
) {
  switch (envelope.payload.oneofKind) {
    case 'authResponse': {
      const auth = envelope.payload.authResponse
      if (auth.ok) {
        set(() => ({ authenticated: true }))
        // On reconnect: re-subscribe channels and refetch stale data
        resubscribeChannels(ws)
        resyncAfterReconnect()
      } else {
        console.error('[WS] auth failed:', auth.reason)
      }
      break
    }

    case 'chatMessage': {
      const msg = envelope.payload.chatMessage
      // Update last message cache for ChatList preview
      set((s) => ({
        lastMessages: {
          ...s.lastMessages,
          [msg.channelId]: {
            content: msg.content.replace(/<[^>]+>/g, '').slice(0, 80),
            timestamp: msg.createdAt || new Date().toISOString(),
            senderName: msg.senderName || '',
          },
        },
      }))
      // Cache injection: append new message directly, skip full refetch
      queryClient.setQueryData(
        ['messages', msg.channelId],
        (old: { messages: Message[]; has_more: boolean } | undefined) => {
          if (!old) return old
          // Deduplicate: skip if message already exists (sender's optimistic update)
          if ((old.messages || []).some((m: Message) => m.id === msg.id)) return old
          // Remove any pending optimistic messages and prepend the real one
          const cleaned = (old.messages || []).filter((m: Message & { _optimistic?: boolean }) => !m._optimistic)
          return { ...old, messages: [convertChatMsgToMessage(msg), ...cleaned] }
        },
      )
      // Unread counts still need server aggregation
      queryClient.invalidateQueries({ queryKey: ['unreadCounts'] })
      break
    }

    case 'typingEvent': {
      const typing = envelope.payload.typingEvent
      set((s) => {
        const existing = s.typingUsers[typing.channelId] || []
        const updated = existing.includes(typing.username) ? existing : [...existing, typing.username]
        return { typingUsers: { ...s.typingUsers, [typing.channelId]: updated } }
      })
      // Remove this specific user after 3s of no typing
      setTimeout(() => {
        set((s) => {
          const current = s.typingUsers[typing.channelId] || []
          const filtered = current.filter((u) => u !== typing.username)
          const t = { ...s.typingUsers }
          if (filtered.length === 0) {
            delete t[typing.channelId]
          } else {
            t[typing.channelId] = filtered
          }
          return { typingUsers: t }
        })
      }, 3000)
      break
    }

    case 'notification':
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
      break

    case 'unreadCount':
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
      queryClient.invalidateQueries({ queryKey: ['unreadCounts'] })
      break

    case 'threadReply': {
      const reply = envelope.payload.threadReply
      if (reply.message) {
        // Inject reply directly into thread cache
        queryClient.setQueryData(
          ['thread', reply.parentMessageId],
          (old: { messages: Message[] } | undefined) => {
            if (!old) return old
            if ((old.messages || []).some((m: Message) => m.id === reply.message!.id)) return old
            return { ...old, messages: [...(old.messages || []), convertChatMsgToMessage(reply.message!)] }
          },
        )
      }
      // Invalidate channel messages so reply_count refreshes
      queryClient.invalidateQueries({ queryKey: ['messages'] })
      break
    }

    case 'assetUpdated': {
      const asset = envelope.payload.assetUpdated
      queryClient.invalidateQueries({ queryKey: ['assets'] })
      if (asset.assetId) {
        queryClient.invalidateQueries({ queryKey: ['asset', asset.assetId] })
        queryClient.invalidateQueries({ queryKey: ['asset-history', asset.assetId] })
        queryClient.invalidateQueries({ queryKey: ['asset-transitions', asset.assetId] })
      }
      queryClient.invalidateQueries({ queryKey: ['asset-summary'] })
      break
    }

    case 'reactionEvent': {
      const reaction = envelope.payload.reactionEvent
      // Inject reaction change directly into the messages cache
      queryClient.setQueryData(
        ['messages', reaction.channelId],
        (old: { messages: Message[]; has_more: boolean } | undefined) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m: Message) => {
              if (m.id !== reaction.messageId) return m
              const reactions = [...(m.reactions || [])]
              const idx = reactions.findIndex((r: ReactionGroup) => r.emoji === reaction.emoji)
              if (reaction.action === 'add') {
                if (idx >= 0) {
                  const group = { ...reactions[idx] }
                  if (!group.user_ids.includes(reaction.userId)) {
                    group.count += 1
                    group.user_ids = [...group.user_ids, reaction.userId]
                  }
                  reactions[idx] = group
                } else {
                  reactions.push({ emoji: reaction.emoji, count: 1, user_ids: [reaction.userId] })
                }
              } else {
                if (idx >= 0) {
                  const group = { ...reactions[idx] }
                  group.user_ids = group.user_ids.filter((id: string) => id !== reaction.userId)
                  group.count = group.user_ids.length
                  if (group.count <= 0) {
                    reactions.splice(idx, 1)
                  } else {
                    reactions[idx] = group
                  }
                }
              }
              return { ...m, reactions }
            }),
          }
        },
      )
      break
    }

    case 'pinEvent': {
      const pin = envelope.payload.pinEvent
      const isPinned = pin.action === 'pin'
      // Update is_pinned flag in messages cache
      queryClient.setQueryData(
        ['messages', pin.channelId],
        (old: { messages: Message[]; has_more: boolean } | undefined) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m: Message) =>
              m.id === pin.messageId ? { ...m, is_pinned: isPinned } : m,
            ),
          }
        },
      )
      // Invalidate pins list (need full pin metadata from server)
      queryClient.invalidateQueries({ queryKey: ['pins', pin.channelId] })
      break
    }

    case 'pollVote': {
      const vote = envelope.payload.pollVote
      // Inject updated vote counts directly into poll cache
      queryClient.setQueryData(
        ['poll', vote.pollId],
        (old: Poll | undefined) => {
          if (!old) return old
          return {
            ...old,
            total_votes: vote.totalVotes,
            options: old.options.map((opt) =>
              opt.id === vote.optionId ? { ...opt, vote_count: vote.voteCount } : opt,
            ),
          }
        },
      )
      break
    }

    case 'taskUpdate': {
      const task = envelope.payload.taskUpdate
      // Inject task status/assignee change into tasks cache
      queryClient.setQueryData(
        ['tasks', task.channelId],
        (old: { tasks: ChatTask[] } | undefined) => {
          if (!old) return old
          return {
            ...old,
            tasks: old.tasks.map((t: ChatTask) =>
              t.id === task.taskId
                ? { ...t, status: task.status || t.status, assignee_id: task.assigneeId || t.assignee_id, title: task.title || t.title }
                : t,
            ),
          }
        },
      )
      break
    }

    case 'driveObject': {
      const event = envelope.payload.driveObject
      // Scope invalidation to the specific parent folder, not all drive queries
      if (event.parentId) {
        queryClient.invalidateQueries({ queryKey: ['drive', event.workspaceId, 'folder', event.parentId] })
      }
      // For deleted/moved items, also invalidate the item detail cache
      if (event.eventType === 'deleted' || event.eventType === 'moved') {
        queryClient.invalidateQueries({ queryKey: ['drive', 'item', event.itemId] })
      }
      // Quota may change on create/delete
      if (event.eventType === 'created' || event.eventType === 'deleted') {
        queryClient.invalidateQueries({ queryKey: ['drive', event.workspaceId, 'quota'] })
      }
      if (WS_DEBUG()) {
        console.log(`[WS] drive object ${event.eventType}: ${event.itemId} in folder ${event.parentId}`)
      }
      break
    }

    case 'drivePerm': {
      const event = envelope.payload.drivePerm
      // Invalidate permission cache for the affected item
      usePermissionStore.getState().invalidate(event.itemId)
      // Also invalidate shares queries
      queryClient.invalidateQueries({ queryKey: ['drive', 'shares', event.itemId] })
      if (WS_DEBUG()) {
        console.log(`[WS] drive perm changed: ${event.itemId}`)
      }
      break
    }

    case 'approvalEvent': {
      // Real-time approval status sync — invalidate all approval queries
      queryClient.invalidateQueries({ queryKey: ['approval'] })
      if (WS_DEBUG()) {
        const evt = envelope.payload.approvalEvent
        console.log(`[WS] approval ${evt.action}: ${evt.requestId}`)
      }
      break
    }

    case 'presenceEvent': {
      const presence = envelope.payload.presenceEvent
      set((s) => {
        const onlineUsers = { ...s.onlineUsers }
        if (presence.status === 'online') {
          onlineUsers[presence.userId] = presence.username
        } else {
          delete onlineUsers[presence.userId]
        }
        return { onlineUsers }
      })
      if (WS_DEBUG()) {
        console.log(`[WS] presence: ${presence.username} is ${presence.status}`)
      }
      break
    }

    case 'error': {
      const err = envelope.payload.error
      console.error(`[WS] server error (${err.code}):`, err.message)
      break
    }

    default:
      // All known event types have explicit handlers above.
      // Log unhandled types in debug mode — no broad invalidation.
      if (envelope.payload.oneofKind && WS_DEBUG()) {
        console.warn('[WS] unhandled event type:', envelope.payload.oneofKind)
      }
      break
  }
}

/** Convert a WebSocket ChatMessage (proto camelCase) to API Message (snake_case). */
function convertChatMsgToMessage(chatMsg: WSChatMessage): Message {
  return {
    id: chatMsg.id,
    channel_id: chatMsg.channelId,
    sender_id: chatMsg.senderId,
    sender_name: chatMsg.senderName,
    content: chatMsg.content,
    content_format: chatMsg.contentFormat || 'markdown',
    created_at: chatMsg.createdAt ?? new Date().toISOString(),
    reply_count: chatMsg.replyCount || 0,
    mentions: chatMsg.mentions || [],
    linked_entity_type: chatMsg.linkedEntityType || '',
    linked_entity_id: chatMsg.linkedEntityId || '',
    reactions: [],
    is_pinned: false,
  }
}

/** Re-sync all active queries after a reconnect to catch missed events. */
function resyncAfterReconnect() {
  queryClient.invalidateQueries({ queryKey: ['messages'] })
  queryClient.invalidateQueries({ queryKey: ['unreadCounts'] })
  queryClient.invalidateQueries({ queryKey: ['notifications'] })
  queryClient.invalidateQueries({ queryKey: ['unread-count'] })
  queryClient.invalidateQueries({ queryKey: ['channels'] })
  queryClient.invalidateQueries({ queryKey: ['pins'] })
  queryClient.invalidateQueries({ queryKey: ['tasks'] })
  queryClient.invalidateQueries({ queryKey: ['reactions'] })
  queryClient.invalidateQueries({ queryKey: ['polls'] })
  queryClient.invalidateQueries({ queryKey: ['thread'] })
  // Drive: invalidate all folder/item queries + clear permission cache
  queryClient.invalidateQueries({ queryKey: ['drive'] })
  usePermissionStore.getState().clear()
  // Approval: catch any missed approval state changes
  queryClient.invalidateQueries({ queryKey: ['approval'] })
}
