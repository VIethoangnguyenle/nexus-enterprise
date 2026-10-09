import { create } from 'zustand'
import { queryClient } from '../lib/query-client'
import { keys } from '../hooks/keys'
import { useAuthStore } from './auth.store'
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

/**
 * Channels the client is subscribed to, with a reference count per channel.
 *
 * Several views subscribe to the same channel (the open space, and the
 * navigator that listens to every conversation). Counting lets each release
 * its own interest without unsubscribing the others; the server only hears
 * about the first subscribe and the last unsubscribe. Re-subscribed on
 * reconnect.
 */
const subscribedChannels = new Map<string, number>()

export interface LastMessageInfo { content: string; timestamp: string; senderName: string; senderId: string }
export interface LastReplyInfo { senderId: string; senderName: string; timestamp: string }

interface WebSocketState {
  ws: WebSocket | null
  connected: boolean
  authenticated: boolean
  typingUsers: Record<string, string[]>
  reconnectAttempt: number
  /** Last message per channel, seen this session: previews and Home ordering. */
  lastMessages: Record<string, LastMessageInfo>
  /** Latest reply per topic (parent message id), seen this session. */
  lastReplies: Record<string, LastReplyInfo>
  /** Online user IDs + usernames — updated via PresenceEvent. */
  onlineUsers: Record<string, string>
  connect: (token: string) => void
  disconnect: () => void
  sendTyping: (channelId: string) => void
  sendSubscribe: (channelId: string) => void
  sendUnsubscribe: (channelId: string) => void
  /** Subscribe to many channels at once; returns the matching release. */
  subscribeMany: (channelIds: string[]) => () => void
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
  for (const channelId of subscribedChannels.keys()) {
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
  lastReplies: {},
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
    const n = subscribedChannels.get(channelId) ?? 0
    subscribedChannels.set(channelId, n + 1)
    if (n > 0) return
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, {
        payload: { oneofKind: 'subscribe', subscribe: { channelId } },
      })
    }
  },

  sendUnsubscribe: (channelId) => {
    const n = subscribedChannels.get(channelId) ?? 0
    if (n > 1) {
      subscribedChannels.set(channelId, n - 1)
      return
    }
    subscribedChannels.delete(channelId)
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, {
        payload: { oneofKind: 'unsubscribe', unsubscribe: { channelId } },
      })
    }
  },

  subscribeMany: (channelIds) => {
    for (const id of channelIds) get().sendSubscribe(id)
    return () => {
      for (const id of channelIds) get().sendUnsubscribe(id)
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
      const timestamp = timestampToIso(msg.createdAt)
      set((s) => ({
        lastMessages: {
          ...s.lastMessages,
          [msg.channelId]: {
            content: previewText(msg.content),
            timestamp,
            senderName: msg.senderName || '',
            senderId: msg.senderId,
          },
        },
      }))
      // The hub broadcasts thread replies as plain chat messages carrying a
      // parent id. Prepending those to the channel list would turn every reply
      // into a new topic, so they are routed to their thread instead.
      if (msg.parentMessageId) {
        injectReply(msg, set)
      } else {
        // Cache injection: append new message directly, skip full refetch
        queryClient.setQueryData(
          keys.messaging.messages(msg.channelId),
          (old: { messages: Message[]; has_more: boolean } | undefined) => {
            if (!old) return old
            // Deduplicate: skip if message already exists (sender's optimistic update)
            if ((old.messages || []).some((m: Message) => m.id === msg.id)) return old
            // Remove any pending optimistic messages and prepend the real one,
            // inheriting the optimistic row's key so it is not remounted.
            const pending = (old.messages || []).find(
              (m: Message & { _optimistic?: boolean }) => m._optimistic && m.sender_id === msg.senderId,
            )
            const cleaned = (old.messages || []).filter((m: Message & { _optimistic?: boolean }) => !m._optimistic)
            const real = { ...convertChatMsgToMessage(msg), _clientKey: pending?.id }
            return { ...old, messages: [real, ...cleaned] }
          },
        )
      }
      // Unread counts still need server aggregation
      queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
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
      queryClient.invalidateQueries({ queryKey: keys.notifications.all() })
      break

    case 'unreadCount':
      queryClient.invalidateQueries({ queryKey: keys.notifications.unreadCount() })
      queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
      break

    case 'threadReply': {
      const reply = envelope.payload.threadReply
      if (reply.message) {
        injectReply({ ...reply.message, parentMessageId: reply.message.parentMessageId || reply.parentMessageId }, set)
      }
      break
    }

    case 'assetUpdated': {
      const asset = envelope.payload.assetUpdated
      // The event names the asset, not its workspace, so lists and summaries
      // are refreshed under every workspace prefix.
      queryClient.invalidateQueries({ queryKey: keys.assets.listsAll() })
      if (asset.assetId) {
        queryClient.invalidateQueries({ queryKey: keys.assets.asset(asset.assetId) })
        queryClient.invalidateQueries({ queryKey: keys.assets.history(asset.assetId) })
        queryClient.invalidateQueries({ queryKey: keys.assets.transitions(asset.assetId) })
      }
      queryClient.invalidateQueries({ queryKey: keys.assets.summaries() })
      break
    }

    case 'reactionEvent': {
      const reaction = envelope.payload.reactionEvent
      // Inject reaction change directly into the messages cache
      queryClient.setQueryData(
        keys.messaging.messages(reaction.channelId),
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
        keys.messaging.messages(pin.channelId),
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
      queryClient.invalidateQueries({ queryKey: keys.messaging.pins(pin.channelId) })
      break
    }

    case 'pollVote': {
      const vote = envelope.payload.pollVote
      // Inject updated vote counts directly into poll cache
      queryClient.setQueryData(
        keys.messaging.poll(vote.pollId),
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
      // Inject task status/assignee change into every cached list of the
      // channel's tasks, whatever status filter each was fetched with.
      queryClient.setQueriesData(
        { queryKey: keys.messaging.tasksOf(task.channelId) },
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
      // Scope invalidation to the folder the item sits in, not all drive queries.
      // An item with no parent sits at the workspace root, which is cached
      // under its own key.
      queryClient.invalidateQueries({ queryKey: keys.drive.folder(event.workspaceId, event.parentId) })
      // A move also empties the folder the item left, and the event names only
      // the destination, so refresh every listing in the workspace.
      if (event.eventType === 'moved') {
        queryClient.invalidateQueries({ queryKey: keys.drive.folders(event.workspaceId) })
      }
      // For deleted/moved items, also invalidate the item detail cache
      if (event.eventType === 'deleted' || event.eventType === 'moved') {
        queryClient.invalidateQueries({ queryKey: keys.drive.item(event.itemId) })
      }
      // Quota may change on create/delete
      if (event.eventType === 'created' || event.eventType === 'deleted') {
        queryClient.invalidateQueries({ queryKey: keys.drive.quota(event.workspaceId) })
      }
      if (WS_DEBUG()) {
        console.log(`[WS] drive object ${event.eventType}: ${event.itemId} in folder ${event.parentId}`)
      }
      break
    }

    case 'drivePerm': {
      const event = envelope.payload.drivePerm
      // Invalidate permission cache for the affected item
      queryClient.invalidateQueries({
        queryKey: keys.permissions.object(useAuthStore.getState().tenantId, event.itemId),
      })
      // Also invalidate shares queries
      queryClient.invalidateQueries({ queryKey: keys.drive.shares(event.itemId) })
      if (WS_DEBUG()) {
        console.log(`[WS] drive perm changed: ${event.itemId}`)
      }
      break
    }

    case 'approvalEvent': {
      // Real-time approval status sync — invalidate all approval queries
      queryClient.invalidateQueries({ queryKey: keys.approval.all() })
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

/** Plain-text preview of a message body: tags stripped, whitespace folded, capped. */
function previewText(content: string): string {
  return content.replace(/<[^>]+>/g, ' ').replace(/&nbsp;/g, ' ').replace(/\s+/g, ' ').trim().slice(0, 120)
}

/** Proto Timestamp (or nothing) to ISO; "now" when the server omitted it. */
function timestampToIso(ts: WSChatMessage['createdAt']): string {
  if (ts && ts.seconds !== undefined) {
    const ms = Number(ts.seconds) * 1000 + Math.floor((ts.nanos ?? 0) / 1e6)
    if (Number.isFinite(ms) && ms > 0) return new Date(ms).toISOString()
  }
  return new Date().toISOString()
}

/**
 * A reply arrived: add it to its thread (if that thread is loaded), move the
 * topic's reply count, and remember who replied last for the topic summary.
 * The sender's own reply was already counted optimistically by useSendReply,
 * so only other people's replies move the count here.
 */
function injectReply(msg: WSChatMessage, set: (fn: (s: WebSocketState) => Partial<WebSocketState>) => void) {
  const parentId = msg.parentMessageId
  if (!parentId) return
  const me = useAuthStore.getState().user?.id
  const fromMe = !!me && msg.senderId === me
  let isNew = true
  queryClient.setQueryData(keys.messaging.thread(parentId), (old: { messages: Message[] } | undefined) => {
    if (!old) return old
    if ((old.messages || []).some((m) => m.id === msg.id)) {
      isNew = false
      return old
    }
    const rest = fromMe
      ? (old.messages || []).filter((m: Message & { _optimistic?: boolean }) => !m._optimistic)
      : old.messages || []
    return { ...old, messages: [...rest, convertChatMsgToMessage(msg)] }
  })
  if (isNew && !fromMe) {
    queryClient.setQueryData(
      keys.messaging.messages(msg.channelId),
      (old: { messages: Message[]; has_more: boolean } | undefined) =>
        old
          ? {
              ...old,
              messages: old.messages.map((m) => (m.id === parentId ? { ...m, reply_count: (m.reply_count || 0) + 1 } : m)),
            }
          : old,
    )
  }
  set((s) => ({
    lastReplies: {
      ...s.lastReplies,
      [parentId]: { senderId: msg.senderId, senderName: msg.senderName || '', timestamp: timestampToIso(msg.createdAt) },
    },
  }))
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
    parent_message_id: chatMsg.parentMessageId || '',
    mentions: chatMsg.mentions || [],
    linked_entity_type: chatMsg.linkedEntityType || '',
    linked_entity_id: chatMsg.linkedEntityId || '',
    reactions: [],
    is_pinned: false,
  }
}

/** Re-sync all active queries after a reconnect to catch missed events. */
function resyncAfterReconnect() {
  const refresh = (queryKey: readonly unknown[]) => queryClient.invalidateQueries({ queryKey })
  refresh(keys.messaging.messagesAll())
  refresh(keys.messaging.unreadCounts())
  refresh(keys.notifications.all())
  refresh(keys.messaging.channelsAll())
  refresh(keys.messaging.pinsAll())
  refresh(keys.messaging.tasksAll())
  refresh(keys.messaging.reactionsAll())
  refresh(keys.messaging.pollsAll())
  refresh(keys.messaging.threadsAll())
  // Drive: every folder, item and share, and every cached permission answer
  refresh(keys.drive.everything())
  refresh(keys.permissions.all())
  // Approval: catch any missed approval state changes
  refresh(keys.approval.all())
}
