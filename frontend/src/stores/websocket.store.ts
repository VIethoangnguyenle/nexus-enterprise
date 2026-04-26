import { create } from 'zustand'
import { queryClient } from '../lib/query-client'

interface WebSocketState {
  ws: WebSocket | null
  connected: boolean
  typingUsers: Record<string, string>
  connect: (token: string) => void
  disconnect: () => void
  sendTyping: (channelId: string) => void
}

export const useWebSocketStore = create<WebSocketState>()((set, get) => ({
  ws: null,
  connected: false,
  typingUsers: {},

  connect: (token) => {
    const existing = get().ws
    if (existing && existing.readyState === WebSocket.OPEN) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/ws?token=${token}`
    const ws = new WebSocket(wsUrl)

    ws.onopen = () => set({ connected: true })

    ws.onclose = () => {
      set({ connected: false, ws: null })
      // Reconnect after 3s
      setTimeout(() => {
        if (get().ws === null) get().connect(token)
      }, 3000)
    }

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        handleWsMessage(data, set)
      } catch { /* ignore non-JSON */ }
    }

    set({ ws })
  },

  disconnect: () => {
    const ws = get().ws
    if (ws) ws.close()
    set({ ws: null, connected: false })
  },

  sendTyping: (channelId) => {
    const ws = get().ws
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'typing', channel_id: channelId }))
    }
  },
}))

/** Route WebSocket messages to TanStack Query cache invalidation. */
function handleWsMessage(data: any, set: any) {
  switch (data.type) {
    case 'message':
      queryClient.invalidateQueries({ queryKey: ['messages', data.message?.channel_id] })
      break

    case 'typing':
      set((s: WebSocketState) => ({
        typingUsers: { ...s.typingUsers, [data.channel_id]: data.content },
      }))
      setTimeout(() => {
        set((s: WebSocketState) => {
          const t = { ...s.typingUsers }
          delete t[data.channel_id]
          return { typingUsers: t }
        })
      }, 3000)
      break

    case 'notification':
    case 'notification_count':
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
      break

    case 'thread_reply':
      queryClient.invalidateQueries({ queryKey: ['thread', data.message?.parent_message_id] })
      queryClient.invalidateQueries({ queryKey: ['messages'] })
      break

    case 'asset_updated':
      queryClient.invalidateQueries({ queryKey: ['assets'] })
      if (data.asset?.id) {
        queryClient.invalidateQueries({ queryKey: ['asset', data.asset.id] })
      }
      break
  }
}
