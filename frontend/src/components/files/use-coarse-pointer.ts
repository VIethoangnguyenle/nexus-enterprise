import { useEffect, useState } from 'react'

const QUERY = '(pointer: coarse)'

/** True while the main pointer is a finger: hover does not exist, so what hover reveals is always shown. */
export function useCoarsePointer(): boolean {
  const supported = typeof window !== 'undefined' && typeof window.matchMedia === 'function'
  const [coarse, setCoarse] = useState(() => (supported ? window.matchMedia(QUERY).matches : false))
  useEffect(() => {
    if (!supported) return
    const mq = window.matchMedia(QUERY)
    const update = () => setCoarse(mq.matches)
    update()
    mq.addEventListener?.('change', update)
    return () => mq.removeEventListener?.('change', update)
  }, [supported])
  return coarse
}
