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
import type { Message } from '../api/messaging'
import {
  applyPin, applyReaction, applyTaskUpdate, convertChatMsgToMessage, previewText, timestampToIso,
} from '../lib/chat-cache'
import {
  createBatcher,
  emptySeq,
  observeSeq,
  planFor,
  resetSeq,
  workspaceResyncKeys,
  type SeqState,
} from '../lib/realtime'

const WS_DEBUG = () => typeof localStorage !== 'undefined' && localStorage.getItem('WS_DEBUG') === '1'

/** Max reconnect backoff in ms. */
const MAX_RECONNECT_DELAY = 30000

/** Presence changes within this window are applied together. */
const PRESENCE_BATCH_MS = 100

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

/** The workspace whose live changes this tab follows; re-subscribed on reconnect. */
let followedWorkspace: string | null = null
/** Where each followed workspace's event stream stands. */
let seqState: SeqState = emptySeq()
/**
 * True from the moment the connection is made until the workspace
 * subscription is acknowledged: whatever happened while it was down has to be
 * fetched again, and only after the subscription is live, or the refetch could
 * predate it and an event in between would be lost.
 */
let resyncPending = true

/**
 * Invalidations from events are held for one frame and run together, so a burst
 * of changes costs one refetch of each affected query.
 */
const invalidations = createBatcher((queryKey) => {
  void queryClient.invalidateQueries({ queryKey })
})

/** Pending reconnect after a dropped socket; cleared by a disconnect. */
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
/** Pending second refresh after a permission event. */
const permissionRetries = new Set<ReturnType<typeof setTimeout>>()
/** Pending retry of a refused workspace subscription, and how many were made. */
let subscribeRetryTimer: ReturnType<typeof setTimeout> | null = null
let subscribeRetries = 0
const SUBSCRIBE_RETRY_MS = [5_000, 15_000, 45_000]
/**
 * A permission event is repeated once after this delay: policy read replicas
 * learn of a change from their own feed, and the first refetch can reach one
 * that has not yet applied it.
 */
const PERMISSION_RETRY_MS = 2_000

function clearTimers() {
  if (reconnectTimer) clearTimeout(reconnectTimer)
  reconnectTimer = null
  if (subscribeRetryTimer) clearTimeout(subscribeRetryTimer)
  subscribeRetryTimer = null
  for (const t of permissionRetries) clearTimeout(t)
  permissionRetries.clear()
}

/** Who last changed an entity, for the wash on its row; see `ChangeInfo`. */
const CHANGE_TTL_MS = 10_000

export interface LastMessageInfo { content: string; timestamp: string; senderName: string; senderId: string }
export interface LastReplyInfo { senderId: string; senderName: string; timestamp: string }
/** Who last acted on an approval request this session, for attributing realtime changes. */
export interface ApprovalActivityInfo { actorNodeId: string; action: string; at: number }
/** Who else just changed an entity (by id), seen this session. Never rendered; it picks a hue. */
export interface ChangeInfo { actorUserId: string; at: number }

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
  /** Latest action per approval request (by request id), seen this session. */
  approvalActivity: Record<string, ApprovalActivityInfo>
  /** Entities somebody else changed in the last few seconds, by entity id. */
  recentChanges: Record<string, ChangeInfo>
  connect: (token: string) => void
  disconnect: () => void
  sendTyping: (channelId: string) => void
  sendSubscribe: (channelId: string) => void
  sendUnsubscribe: (channelId: string) => void
  /** Subscribe to many channels at once; returns the matching release. */
  subscribeMany: (channelIds: string[]) => () => void
  /** Follow a workspace's live changes; returns the matching release. */
  followWorkspace: (workspaceId: string) => () => void
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
      payload: { oneofKind: 'subscribe', subscribe: { channelId, workspaceId: '' } },
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
  approvalActivity: {},
  recentChanges: {},

  connect: (token) => {
    const existing = get().ws
    if (existing && existing.readyState === WebSocket.OPEN) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/ws`
    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => {
      resyncPending = true
      set({ connected: true, reconnectAttempt: 0 })
      // Auth handshake: send token as first message
      sendEnvelope(ws, {
        payload: { oneofKind: 'auth', auth: { token } },
      })
    }

    ws.onclose = (event) => {
      // A connection replaced since (token change, workspace switch) must not
      // wipe the state of the one that replaced it.
      const current = get().ws
      if (current && current !== ws) return
      const attempt = get().reconnectAttempt
      set({ connected: false, authenticated: false, ws: null, reconnectAttempt: attempt + 1 })

      // Don't reconnect on intentional close (code 1000)
      if (event.code === 1000) return

      const delay = reconnectDelay(attempt)
      if (WS_DEBUG()) {
        console.log(`[WS] reconnecting in ${Math.round(delay)}ms (attempt ${attempt + 1})`)
      }

      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        // The session may have refreshed its token while the socket was down,
        // and may have ended altogether: then there is nobody to reconnect.
        const current = useAuthStore.getState().accessToken
        if (!current) return
        if (get().ws === null) get().connect(current)
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
    // followedWorkspace stays: a disconnect is how a token refresh swaps the
    // socket, and the new one must resume following. The follower's release
    // (unmount, sign-out, switching workspace) is what ends it.
    seqState = emptySeq()
    invalidations.cancel()
    clearTimers()
    subscribeRetries = 0
    // What the session learned about who acted belongs to the session: it must not
    // be read as news by whoever signs in next in this tab.
    set({ ws: null, connected: false, authenticated: false, reconnectAttempt: 0, approvalActivity: {}, recentChanges: {} })
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
        payload: { oneofKind: 'subscribe', subscribe: { channelId, workspaceId: '' } },
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
        payload: { oneofKind: 'unsubscribe', unsubscribe: { channelId, workspaceId: '' } },
      })
    }
  },

  followWorkspace: (workspaceId) => {
    followedWorkspace = workspaceId
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, { payload: { oneofKind: 'subscribe', subscribe: { channelId: '', workspaceId } } })
    }
    return () => {
      if (followedWorkspace !== workspaceId) return
      followedWorkspace = null
      seqState = emptySeq()
      if (subscribeRetryTimer) clearTimeout(subscribeRetryTimer)
      subscribeRetryTimer = null
      subscribeRetries = 0
      const open = get().ws
      if (open && get().authenticated) {
        sendEnvelope(open, { payload: { oneofKind: 'unsubscribe', unsubscribe: { channelId: '', workspaceId } } })
      }
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
        // On reconnect: re-subscribe, then refetch what may have changed while
        // the socket was down. With a workspace followed the refetch waits for
        // its acknowledgement (see 'workspaceSubscribed').
        resubscribeChannels(ws)
        if (followedWorkspace) {
          sendEnvelope(ws, {
            payload: { oneofKind: 'subscribe', subscribe: { channelId: '', workspaceId: followedWorkspace } },
          })
        } else {
          resyncAfterReconnect()
          resyncPending = false
        }
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

    // The notification keys are kept for the notifications UI that is still to
    // be decided; no screen reads them yet.
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

    case 'reactionEvent':
      applyReaction(envelope.payload.reactionEvent)
      break

    case 'pinEvent':
      applyPin(envelope.payload.pinEvent)
      break

    case 'pollVote':
      // No screen renders polls, so there is no cache to patch.
      break

    case 'taskUpdate':
      applyTaskUpdate(envelope.payload.taskUpdate)
      break

    case 'workspaceSubscribed': {
      const ack = envelope.payload.workspaceSubscribed
      if (ack.workspaceId !== followedWorkspace) break
      if (ack.denied) {
        // Not following, but the attempt is decided: fetch what was missed
        // (chat and notifications do not depend on the workspace stream), and
        // ask again later, since a refusal can be a policy replica a step behind.
        if (resyncPending) {
          resyncPending = false
          resyncAfterReconnect()
        }
        if (subscribeRetries < SUBSCRIBE_RETRY_MS.length && !subscribeRetryTimer) {
          const wait = SUBSCRIBE_RETRY_MS[subscribeRetries++]
          subscribeRetryTimer = setTimeout(() => {
            subscribeRetryTimer = null
            const { ws: open, authenticated } = useWebSocketStore.getState()
            if (open && authenticated && followedWorkspace === ack.workspaceId) {
              sendEnvelope(open, {
                payload: { oneofKind: 'subscribe', subscribe: { channelId: '', workspaceId: ack.workspaceId } },
              })
            }
          }, wait)
        }
        break
      }
      subscribeRetries = 0
      if (subscribeRetryTimer) clearTimeout(subscribeRetryTimer)
      subscribeRetryTimer = null
      seqState = resetSeq(seqState, ack.workspaceId, Number(ack.seq))
      if (resyncPending) {
        resyncPending = false
        resyncAfterReconnect()
      }
      break
    }

    case 'domainEvent': {
      const d = envelope.payload.domainEvent
      const seq = Number(d.seq)
      if (seq && d.workspaceId) {
        const seen = observeSeq(seqState, d.workspaceId, seq)
        seqState = seen.state
        if (seen.verdict === 'duplicate') break
        if (seen.verdict === 'resync') {
          // Events were missed: nothing this tab holds for the workspace can be trusted.
          if (WS_DEBUG()) console.warn(`[WS] sequence hole in ${d.workspaceId}, resynchronising`)
          resyncAfterReconnect()
          break
        }
      }
      const plan = planFor({
        domain: d.domain,
        kind: d.kind,
        workspaceId: d.workspaceId,
        ids: d.ids,
        parentId: d.parentId,
        oldParentId: d.oldParentId,
        channelId: d.channelId,
        actorUserId: d.actorUserId,
      })
      invalidations.add(plan.invalidate)
      if (d.domain === 'permission') {
        const retry = setTimeout(() => {
          permissionRetries.delete(retry)
          invalidations.add(plan.invalidate)
        }, PERMISSION_RETRY_MS)
        permissionRetries.add(retry)
      }
      noteChanges(plan.touched, d.actorUserId, set)
      if (WS_DEBUG()) console.log(`[WS] ${d.domain} ${d.kind}`, d.ids)
      break
    }

    case 'approvalEvent': {
      // Remember who acted before the refetch lands, so the list can attribute
      // the change it is about to show.
      const activity = envelope.payload.approvalEvent
      set((s) => ({
        approvalActivity: {
          ...s.approvalActivity,
          [activity.requestId]: { actorNodeId: activity.actorNodeId, action: activity.action, at: Date.now() },
        },
      }))
      // Real-time approval status sync — invalidate all approval queries
      invalidations.add([keys.approval.all()])
      if (WS_DEBUG()) {
        const evt = envelope.payload.approvalEvent
        console.log(`[WS] approval ${evt.action}: ${evt.requestId}`)
      }
      break
    }

    case 'presenceEvent': {
      const presence = envelope.payload.presenceEvent
      queuePresence(presence.userId, presence.status === 'online' ? presence.username : null, set)
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

/**
 * Presence changes are applied once per frame: a reconnect storm or a morning
 * rush of people coming online would otherwise re-render everything that reads
 * the roster once per person.
 */
const pendingPresence = new Map<string, string | null>()
let presenceTimer: ReturnType<typeof setTimeout> | null = null

function queuePresence(
  userId: string,
  username: string | null,
  set: (fn: (s: WebSocketState) => Partial<WebSocketState>) => void,
) {
  pendingPresence.set(userId, username)
  if (presenceTimer) return
  presenceTimer = setTimeout(() => flushPresence(set), PRESENCE_BATCH_MS)
}

function flushPresence(set: (fn: (s: WebSocketState) => Partial<WebSocketState>) => void) {
  presenceTimer = null
  const batch = [...pendingPresence]
  pendingPresence.clear()
  set((s) => {
    const onlineUsers = { ...s.onlineUsers }
    for (const [userId, username] of batch) {
      if (username === null) delete onlineUsers[userId]
      else onlineUsers[userId] = username
    }
    return { onlineUsers }
  })
}

/** Remember who else just changed these entities, so their rows can be washed in that person's hue. */
function noteChanges(
  ids: string[],
  actorUserId: string,
  set: (fn: (s: WebSocketState) => Partial<WebSocketState>) => void,
) {
  if (ids.length === 0 || !actorUserId) return
  if (actorUserId === useAuthStore.getState().user?.id) return
  const now = Date.now()
  set((s) => {
    const next: Record<string, ChangeInfo> = {}
    for (const [id, info] of Object.entries(s.recentChanges)) {
      if (now - info.at < CHANGE_TTL_MS) next[id] = info
    }
    for (const id of ids) next[id] = { actorUserId, at: now }
    return { recentChanges: next }
  })
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

/**
 * Re-sync everything after a reconnect or a hole in the event stream: whatever
 * happened while this tab was not listening is unknown, so every query family
 * is refreshed, immediately rather than in the next batch.
 */
function resyncAfterReconnect() {
  invalidations.add(workspaceResyncKeys(followedWorkspace ?? ''))
  invalidations.flush()
}
