import { create } from 'zustand'
import { queryClient } from '../lib/query-client'
import {
  ClientEnvelope,
  ServerEnvelope,
} from '../generated/proto/messaging/ws'
import type {
  ServerEnvelope as ServerEnvelopeType,
} from '../generated/proto/messaging/ws'

const WS_DEBUG = () => typeof localStorage !== 'undefined' && localStorage.getItem('WS_DEBUG') === '1'

interface WebSocketState {
  ws: WebSocket | null
  connected: boolean
  authenticated: boolean
  typingUsers: Record<string, string>
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

export const useWebSocketStore = create<WebSocketState>()((set, get) => ({
  ws: null,
  connected: false,
  authenticated: false,
  typingUsers: {},

  connect: (token) => {
    const existing = get().ws
    if (existing && existing.readyState === WebSocket.OPEN) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/api/ws`
    const ws = new WebSocket(wsUrl)
    ws.binaryType = 'arraybuffer'

    ws.onopen = () => {
      set({ connected: true })
      // Auth handshake: send token as first message
      sendEnvelope(ws, {
        payload: { oneofKind: 'auth', auth: { token } },
      })
    }

    ws.onclose = () => {
      set({ connected: false, authenticated: false, ws: null })
      // Reconnect after 3s
      setTimeout(() => {
        if (get().ws === null) get().connect(token)
      }, 3000)
    }

    ws.onmessage = (event: MessageEvent) => {
      if (!(event.data instanceof ArrayBuffer)) return

      const envelope = ServerEnvelope.fromBinary(new Uint8Array(event.data))

      if (WS_DEBUG()) {
        console.log('[WS]', envelope.payload.oneofKind, envelope)
      }

      handleServerMessage(envelope, set)
    }

    set({ ws })
  },

  disconnect: () => {
    const ws = get().ws
    if (ws) ws.close()
    set({ ws: null, connected: false, authenticated: false })
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
    const ws = get().ws
    if (ws && get().authenticated) {
      sendEnvelope(ws, {
        payload: { oneofKind: 'subscribe', subscribe: { channelId } },
      })
    }
  },

  sendUnsubscribe: (channelId) => {
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
) {
  switch (envelope.payload.oneofKind) {
    case 'authResponse': {
      const auth = envelope.payload.authResponse
      if (auth.ok) {
        set(() => ({ authenticated: true }))
      } else {
        console.error('[WS] auth failed:', auth.reason)
      }
      break
    }

    case 'chatMessage': {
      const msg = envelope.payload.chatMessage
      queryClient.invalidateQueries({ queryKey: ['messages', msg.channelId] })
      break
    }

    case 'typingEvent': {
      const typing = envelope.payload.typingEvent
      set((s) => ({
        typingUsers: { ...s.typingUsers, [typing.channelId]: typing.username },
      }))
      setTimeout(() => {
        set((s) => {
          const t = { ...s.typingUsers }
          delete t[typing.channelId]
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
      break

    case 'threadReply': {
      const reply = envelope.payload.threadReply
      queryClient.invalidateQueries({ queryKey: ['thread', reply.parentMessageId] })
      queryClient.invalidateQueries({ queryKey: ['messages'] })
      break
    }

    case 'assetUpdated': {
      const asset = envelope.payload.assetUpdated
      queryClient.invalidateQueries({ queryKey: ['assets'] })
      if (asset.assetId) {
        queryClient.invalidateQueries({ queryKey: ['asset', asset.assetId] })
      }
      break
    }

    case 'error': {
      const err = envelope.payload.error
      console.error(`[WS] server error (${err.code}):`, err.message)
      break
    }
  }
}
