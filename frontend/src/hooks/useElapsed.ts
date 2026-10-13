import { useEffect, useState } from 'react'
import { formatElapsed } from '../lib/call-model'

/** `mm:ss` since `startMs`, ticking once a second. */
export function useElapsed(startMs: number | null): string {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (startMs === null) return
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [startMs])
  return startMs === null ? '00:00' : formatElapsed(now - startMs)
}
