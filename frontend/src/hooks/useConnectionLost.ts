import { useEffect, useState } from 'react'
import { useWebSocketStore } from '../stores/websocket.store'

/** How long the connection must stay down before it is worth telling the user (mockup §2). */
const GRACE_MS = 3000

/**
 * True once the live connection has been down for more than 3 seconds: the
 * browser reports no network, or the websocket dropped and is retrying. A
 * socket that has never connected (not signed in yet) is not "lost".
 */
export function useConnectionLost(): boolean {
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
    const t = setTimeout(() => setLost(true), GRACE_MS)
    return () => clearTimeout(t)
  }, [down])

  return lost
}
