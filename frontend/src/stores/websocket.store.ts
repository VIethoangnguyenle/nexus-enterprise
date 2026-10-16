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
  applyMessageDeleted, applyMessageEdited, applyPin, applyReaction, applyTaskUpdate, convertChatMsgToMessage,
  lastMessageOf, previewText, timestampToIso, withEditedLiveMessage, withoutLiveMessage,
} from '../lib/chat-cache'
import type { LastMessage } from '../lib/conversations'
import { applyThreadReplySummary, applyThreadState } from '../lib/thread-cache'
import { invalidateUnreadThreadsFor } from '../lib/unread-threads-cache'
import { mentionsMe } from '../lib/shortcut-feed'
import { onSessionEnd } from '../lib/session-end'
import { invalidatePollOnVote } from '../hooks/usePolls'
import { useIncomingCallStore } from './incoming-call.store'
import { isLive, useCallStore } from './call.store'
import { asCallKind } from '../api/calls'
import { callKeys } from '../hooks/keys/calls'
import {
  adoptOwner,
  createBatcher,
  dropWorkspaceSeq,
  emptySeq,
  observeSeq,
  parseControl,
  planFor,
  resetSeq,
  restoreSeq,
  setUserSeq,
  snapshotSeq,
  syncFrame,
  userSyncFrame,
  workspaceResyncKeys,
  type ControlMessage,
  type SeqState,
  type SyncSnapshot,
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
/**
 * Where each followed workspace's event stream stands, and the user's own
 * stream. Held in memory; `getSyncSnapshot` / `restoreSyncSnapshot` let a
 * device cache carry it across a reload.
 */
let seqState: SeqState = emptySeq()
/**
 * Per workspace, how a gap in the stream is being repaired:
 *  - 'syncing': a sync was asked for and has not been answered; frames out of
 *    order meanwhile are dropped, the replay brings them back;
 *  - 'answered': the sync was answered; a gap before the next good frame means
 *    replay cannot be trusted and everything is refreshed.
 */
const holeRepair = new Map<string, 'syncing' | 'answered'>()
/** The same repair policy for the user's own stream. */
let userRepair: 'syncing' | 'answered' | undefined

/** The cursors, as plain data for a device cache. */
export function getSyncSnapshot(): SyncSnapshot {
  return snapshotSeq(seqState)
}

/** Loads cursors saved earlier. Takes effect at the next connection; a different owner's cursors are dropped then. */
export function restoreSyncSnapshot(raw: unknown) {
  seqState = restoreSeq(raw)
  holeRepair.clear()
  userRepair = undefined
}

/** Forgets every cursor: the next connection subscribes plainly and refreshes everything. */
function clearCursors() {
  seqState = emptySeq()
  holeRepair.clear()
  userRepair = undefined
}
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
 * How long a sync request waits for any answer. A server that does not know
 * the request (during a deploy or rollback) ignores it silently.
 */
const SYNC_ACK_MS = 5_000
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

export type LastMessageInfo = LastMessage
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
  /** Newest main-stream message per channel, seen this session: previews and Home ordering. */
  lastMessages: Record<string, LastMessageInfo>
  /** Newest thread reply per channel, seen this session; kept apart so it never hides a newer main message. */
  lastThreadMessages: Record<string, LastMessageInfo>
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
  /**
   * The conversation on screen; returns the matching release. The server holds
   * back push for it while the page is visible. Subscribing is not viewing.
   */
  viewChannel: (channelId: string) => () => void
}

/** Send a binary-encoded ClientEnvelope over WebSocket. */
function sendEnvelope(ws: WebSocket, envelope: Parameters<typeof ClientEnvelope.toBinary>[0]) {
  if (ws.readyState !== WebSocket.OPEN) return
  ws.send(ClientEnvelope.toBinary(envelope))
}

/** Send a JSON control frame (sync requests). */
function sendControl(ws: WebSocket, text: string) {
  if (ws.readyState !== WebSocket.OPEN) return
  ws.send(text)
}

/** The conversation the screen shows, if any. Re-announced on reconnect. */
let viewedChannel: string | null = null
/** What the server was last told this socket views, so nothing is repeated. */
let announcedView: { ws: WebSocket; channelId: string } | null = null

/**
 * Tells the server what this session is looking at: the viewed conversation
 * while the page is visible and the channel is subscribed (the server refuses
 * a view of anything else), otherwise nothing.
 */
function announceView(ws: WebSocket | null, authenticated: boolean) {
  if (!ws || !authenticated) return
  const visible = typeof document === 'undefined' || document.visibilityState !== 'hidden'
  const channelId = viewedChannel && visible && subscribedChannels.has(viewedChannel) ? viewedChannel : ''
  if (announcedView?.ws === ws ? announcedView.channelId === channelId : channelId === '') return
  announcedView = { ws, channelId }
  sendEnvelope(ws, { payload: { oneofKind: 'view', view: { channelId } } })
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
  lastThreadMessages: {},
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
      if (typeof event.data === 'string') {
        const control = parseControl(event.data)
        if (control) handleControl(control)
        return
      }
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
    // (unmount, sign-out, switching workspace) is what ends it. The cursors
    // stay for the same reason: the next connection resumes from them. Only
    // the repairs in flight die with the socket.
    holeRepair.clear()
    userRepair = undefined
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
      if (channelId === viewedChannel) announceView(ws, true)
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
      // The server stops counting a view with the subscription.
      if (announcedView?.ws === ws && announcedView.channelId === channelId) announcedView = { ws, channelId: '' }
    }
  },

  followWorkspace: (workspaceId) => {
    followedWorkspace = workspaceId
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, { payload: { oneofKind: 'subscribe', subscribe: { channelId: '', workspaceId } } })
    }
    return () => {
      // Nothing is heard for this workspace from now on, so its cursor goes
      // stale, even when another workspace is already being followed.
      seqState = dropWorkspaceSeq(seqState, workspaceId)
      holeRepair.delete(workspaceId)
      if (followedWorkspace !== workspaceId) return
      followedWorkspace = null
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

  viewChannel: (channelId) => {
    viewedChannel = channelId
    announceView(get().ws, get().authenticated)
    return () => {
      if (viewedChannel !== channelId) return
      viewedChannel = null
      announceView(get().ws, get().authenticated)
    }
  },
}))

// A hidden page views nothing: push resumes for the conversation it shows.
if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    const { ws, authenticated } = useWebSocketStore.getState()
    announceView(ws, authenticated)
  })
}

/**
 * A message that names me is a new item of "Được nhắc tới" and of its pill. The
 * server decides what counts (a direct @you, muted conversations included); the
 * client only asks for the feed again.
 */
function invalidateMentionsFor(msg: Pick<WSChatMessage, 'mentions' | 'senderId'>) {
  if (!mentionsMe(msg.mentions, msg.senderId, useAuthStore.getState().user?.id)) return
  invalidations.add([keys.shortcuts.mentionsAll(), keys.shortcuts.mentionsUnreadAll()])
}

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
        // Cursors belong to one user in one tenant.
        seqState = adoptOwner(seqState, `${auth.userId}|${useAuthStore.getState().tenantId ?? ''}`)
        // On reconnect: re-subscribe, then refetch what may have changed while
        // the socket was down. With a workspace followed the refetch waits for
        // its acknowledgement (see 'workspaceSubscribed').
        resubscribeChannels(ws)
        announceView(ws, true)
        const wsCursor = followedWorkspace ? seqState.last[followedWorkspace] : undefined
        const userCursor = seqState.user
        if (userCursor !== undefined && (!followedWorkspace || wsCursor !== undefined)) {
          // Both streams have a known position: ask for what came after it
          // instead of refreshing everything. resyncPending stays set until
          // the workspace answers (or is refused), so a refusal still refreshes.
          userRepair = undefined
          sendControl(ws, userSyncFrame(userCursor))
          if (followedWorkspace && wsCursor !== undefined) {
            sendControl(ws, syncFrame(followedWorkspace, wsCursor))
            holeRepair.delete(followedWorkspace)
            armSyncFallback(ws, followedWorkspace)
          } else {
            resyncPending = false
          }
        } else if (followedWorkspace) {
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
      set((s) => liveMessageSlot(s, lastMessageOf(msg, msg.parentMessageId), msg.channelId))
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
      invalidateMentionsFor(msg)
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
      // Read here or on another device: what the mention pill counts may have gone down.
      queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsAll() })
      queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsUnreadAll() })
      break

    case 'threadReply': {
      const reply = envelope.payload.threadReply
      if (reply.message) {
        const m = reply.message
        // A reply still moves its conversation: preview and order, as before replies had their own frame.
        set((s) => liveMessageSlot(s, lastMessageOf(m, m.parentMessageId || reply.parentMessageId), m.channelId))
        injectReply({ ...m, parentMessageId: m.parentMessageId || reply.parentMessageId }, set)
        invalidateMentionsFor(m)
      }
      // The server's summary replaces the local reply-count guess.
      applyThreadReplySummary(reply)
      invalidateUnreadThreadsFor(envelope.payload.oneofKind)
      break
    }

    case 'threadState':
      applyThreadState(envelope.payload.threadState)
      // A thread read elsewhere settles the unread mentions inside it.
      queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsAll() })
      queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsUnreadAll() })
      invalidateUnreadThreadsFor(envelope.payload.oneofKind)
      break

    case 'messageEdited': {
      const ev = envelope.payload.messageEdited
      // The list previews quote the text: the entry showing this message takes the new text in place.
      applyMessageEdited(ev)
      const edited = ev.message
      if (edited?.id) {
        const edit = { content: previewText(edited.content), mentions: edited.mentions?.length ? edited.mentions : undefined }
        set((s) => {
          const lastMessages = withEditedLiveMessage(s.lastMessages, edited.channelId, edited.id, edit)
          const lastThreadMessages = withEditedLiveMessage(s.lastThreadMessages, edited.channelId, edited.id, edit)
          return lastMessages === s.lastMessages && lastThreadMessages === s.lastThreadMessages
            ? s
            : { lastMessages, lastThreadMessages }
        })
      }
      break
    }

    case 'messageDeleted': {
      const ev = envelope.payload.messageDeleted
      // The cache drops or refreshes the server previews; a live preview of the deleted message goes at once.
      applyMessageDeleted(ev)
      set((s) => {
        const lastMessages = withoutLiveMessage(s.lastMessages, ev.channelId, ev.messageId)
        const lastThreadMessages = withoutLiveMessage(s.lastThreadMessages, ev.channelId, ev.messageId)
        return lastMessages === s.lastMessages && lastThreadMessages === s.lastThreadMessages
          ? s
          : { lastMessages, lastThreadMessages }
      })
      break
    }

    case 'reactionEvent':
      applyReaction(envelope.payload.reactionEvent)
      break

    case 'pinEvent':
      applyPin(envelope.payload.pinEvent)
      break

    case 'pollVote':
      void invalidatePollOnVote(queryClient, envelope.payload.pollVote)
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
        // The refusal is an answer: it replaces a pending sync deadline.
        if (subscribeRetryTimer) clearTimeout(subscribeRetryTimer)
        subscribeRetryTimer = null
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
        const beforeHole = seqState
        const seen = observeSeq(seqState, d.workspaceId, seq)
        seqState = seen.state
        if (seen.verdict === 'duplicate') break
        if (seen.verdict === 'resync') {
          // Events were missed. Ask for them once from the last good position;
          // if that was already tried, nothing this tab holds can be trusted.
          if (WS_DEBUG()) console.warn(`[WS] sequence hole in ${d.workspaceId}, resynchronising`)
          const repair = holeRepair.get(d.workspaceId)
          if (repair === 'syncing') {
            // Frames sent before the request was handled: the replay returns them.
            seqState = beforeHole
            break
          }
          if (repair === undefined) {
            seqState = beforeHole
            holeRepair.set(d.workspaceId, 'syncing')
            sendControl(ws, syncFrame(d.workspaceId, beforeHole.last[d.workspaceId] ?? 0))
            break
          }
          holeRepair.delete(d.workspaceId)
          resyncAfterReconnect()
          break
        }
        if (holeRepair.get(d.workspaceId) === 'answered') holeRepair.delete(d.workspaceId)
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
    case 'callEvent': {
      const ev = envelope.payload.callEvent
      const myId = useAuthStore.getState().user?.id
      const incoming = useIncomingCallStore.getState()
      // A kind that is neither audio nor video is not a call this client knows how to ring or join.
      const kind = asCallKind(ev.kind)
      const who = { callId: ev.callId, channelId: ev.channelId, callerName: ev.callerName, callerAvatarUrl: ev.callerAvatarUrl }

      switch (ev.type) {
        case 'ring':
          // Ring all devices of the callee, not the caller.
          if (myId && ev.callerId !== myId && kind) incoming.ring({ ...who, kind })
          break
        case 'invited':
          if (kind) incoming.invite({ ...who, kind })
          break
        case 'cancel':
        case 'missed':
        case 'declined':
          incoming.clear(ev.callId)
          break
        case 'answered': {
          // Answered on this device (it is in that call, live): just stop ringing. Anywhere else: say so for 3 s.
          const mine = useCallStore.getState()
          if (mine.session?.callId === ev.callId && isLive(mine.phase)) incoming.clear(ev.callId)
          else incoming.answeredElsewhere(ev.callId)
          break
        }
        // 'state' and others: no incoming-call action, just invalidate.
      }

      // The call and the conversation's open call change with every event. The card in the stream arrives as an
      // ordinary message edit, so the message list and the activity feed are left alone.
      invalidations.add([callKeys.detail(ev.callId)])
      if (ev.channelId) invalidations.add([callKeys.open(ev.channelId)])

      if (WS_DEBUG()) {
        console.log(`[WS] callEvent: ${ev.type} call=${ev.callId} kind=${ev.kind}`)
      }
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
/** Files a live message under its channel's main slot, or its thread slot when it is a reply. */
function liveMessageSlot(s: WebSocketState, line: LastMessageInfo, channelId: string): Partial<WebSocketState> {
  return line.parentId
    ? { lastThreadMessages: { ...s.lastThreadMessages, [channelId]: line } }
    : { lastMessages: { ...s.lastMessages, [channelId]: line } }
}

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
 * A user-addressed frame and its position arrive together or not at all, so a
 * position that is not the next one means frames were lost. Same policy as a
 * workspace hole: ask once from the last good position, and when that was
 * already tried without a good frame since, refresh everything.
 */
function observeUserSeq(seq: number) {
  const last = seqState.user
  if (last === undefined || seq === last + 1) {
    seqState = setUserSeq(seqState, seq)
    if (userRepair === 'answered') userRepair = undefined
    return
  }
  if (seq <= last) return // a repeat
  if (userRepair === 'syncing') return // in flight before the request was handled; the replay returns them
  if (userRepair === undefined) {
    userRepair = 'syncing'
    const ws = useWebSocketStore.getState().ws
    if (ws) sendControl(ws, userSyncFrame(last))
    return
  }
  userRepair = undefined
  seqState = setUserSeq(seqState, seq)
  resyncAfterReconnect()
}

/**
 * If nothing answers a sync (a server that predates the request ignores it),
 * subscribe the plain way and refresh everything once that is acknowledged.
 */
function armSyncFallback(ws: WebSocket, workspaceId: string) {
  if (subscribeRetryTimer) clearTimeout(subscribeRetryTimer)
  subscribeRetryTimer = setTimeout(() => {
    subscribeRetryTimer = null
    const state = useWebSocketStore.getState()
    if (state.ws !== ws || !state.authenticated || followedWorkspace !== workspaceId || !resyncPending) return
    sendEnvelope(ws, { payload: { oneofKind: 'subscribe', subscribe: { channelId: '', workspaceId } } })
  }, SYNC_ACK_MS)
}

/** The server's answers to a sync request, and the user stream's position. */
function handleControl(msg: ControlMessage) {
  if (msg.type === 'user_seq') {
    observeUserSeq(msg.seq)
    return
  }
  const tooFar = msg.type === 'too_far'
  if (msg.scope === 'user') {
    seqState = setUserSeq(seqState, msg.seq)
    if (tooFar) {
      userRepair = undefined
      resyncAfterReconnect()
    } else {
      if (userRepair) userRepair = 'answered'
      if (followedWorkspace === null) resyncPending = false
    }
    return
  }
  if (msg.workspaceId !== followedWorkspace) return
  subscribeRetries = 0
  if (subscribeRetryTimer) clearTimeout(subscribeRetryTimer)
  subscribeRetryTimer = null
  // After a complete replay the cursor equals the answer; a difference means a
  // frame was lost on the way.
  const lost = !tooFar && seqState.last[msg.workspaceId] !== msg.seq
  const repair = holeRepair.get(msg.workspaceId)
  seqState = resetSeq(seqState, msg.workspaceId, msg.seq)
  resyncPending = false
  if (tooFar || lost) {
    holeRepair.delete(msg.workspaceId)
    resyncAfterReconnect()
    return
  }
  if (repair) holeRepair.set(msg.workspaceId, 'answered')
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

// Previews, replies, typing and presence are what this user's session saw: the
// next person to sign in on this tab must start without them.
onSessionEnd(() => {
  clearCursors()
  useWebSocketStore.setState({
    lastMessages: {}, lastThreadMessages: {}, lastReplies: {}, typingUsers: {}, onlineUsers: {}, approvalActivity: {}, recentChanges: {},
  })
})
