import { useEffect, useRef, useState } from 'react'
import { useWebSocketStore } from '../stores/websocket.store'

/** How long the connection must stay down before it is worth telling the user (mockup §2). */
const GRACE_MS = 3000
/** The app-wide bar is quicker to speak (push mockup §5). */
export const BAR_GRACE_MS = 2000
/** How long "Đã kết nối lại" stays up before the bar goes away. */
const RESTORED_MS = 2000

/**
 * True once the live connection has been down for more than `graceMs` (3 seconds): the
 * browser reports no network, or the websocket dropped and is retrying. A
 * socket that has never connected (not signed in yet) is not "lost".
 */
export function useConnectionLost(graceMs: number = GRACE_MS): boolean {
  const retrying = useWebSocketStore((s) => !s.connected && s.reconnectAttempt > 0)
  const [offline, setOffline] = useState(() => typeof navigator !== 'undefined' && !navigator.onLine)
  const [lost, setLost] = useState(false)

  useEffect(() => {
    const down = () => setOffline(true)
    const up = () => setOffline(false)
    window.addEventListener('offline', down)
    window.addEventListener('online', up)
    return () => {
      window.removeEventListener('offline', down)
      window.removeEventListener('online', up)
    }
  }, [])

  const down = retrying || offline
  useEffect(() => {
    if (!down) {
      setLost(false)
      return
    }
    const t = setTimeout(() => setLost(true), graceMs)
    return () => clearTimeout(t)
  }, [down, graceMs])

  return lost
}

export type ConnectionStatus = 'online' | 'lost' | 'restored'

/**
 * `lost` after the bar's grace period, then `restored` for two seconds once the
 * connection is back (only if it was lost long enough to be shown), then `online`.
 */
export function useConnectionStatus(): ConnectionStatus {
  const lost = useConnectionLost(BAR_GRACE_MS)
  const [restored, setRestored] = useState(false)
  const wasLost = useRef(false)

  useEffect(() => {
    if (lost) {
      wasLost.current = true
      setRestored(false)
      return
    }
    if (!wasLost.current) return
    wasLost.current = false
    setRestored(true)
    const t = setTimeout(() => setRestored(false), RESTORED_MS)
    return () => clearTimeout(t)
  }, [lost])

  if (lost) return 'lost'
  return restored ? 'restored' : 'online'
}
