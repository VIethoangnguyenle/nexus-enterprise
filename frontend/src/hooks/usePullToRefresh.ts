import { useRef, useState, type TouchEvent } from 'react'

/** How far the finger must travel, after damping, to count as a pull. */
const THRESHOLD = 56
/** The indicator never follows the finger further than this. */
const MAX_PULL = 80
/** The list gives way at half the finger's speed. */
const DAMPING = 0.5

/**
 * Pull down from the top of a list to reload it. An installed app has no
 * browser pull-to-refresh, so lists draw their own. Spread `bind` on the
 * scrolling element; `pull` is how far to draw the indicator, `refreshing`
 * holds it open until `onRefresh` settles (a failure just ends it).
 */
export function usePullToRefresh(onRefresh: () => Promise<unknown>) {
  const [pull, setPull] = useState(0)
  const [refreshing, setRefreshing] = useState(false)
  const startY = useRef<number | null>(null)
  const latest = useRef(0)

  const reset = () => {
    startY.current = null
    latest.current = 0
    setPull(0)
  }

  const bind = {
    onTouchStart: (e: TouchEvent<HTMLElement>) => {
      // Only a pull that starts with the list at its top is a pull.
      startY.current = !refreshing && e.currentTarget.scrollTop <= 0 ? e.touches[0]!.clientY : null
    },
    onTouchMove: (e: TouchEvent<HTMLElement>) => {
      if (startY.current === null) return
      const dy = (e.touches[0]!.clientY - startY.current) * DAMPING
      if (dy <= 0) {
        if (latest.current !== 0) reset()
        return
      }
      latest.current = Math.min(dy, MAX_PULL)
      setPull(latest.current)
    },
    onTouchEnd: () => {
      const pulled = latest.current >= THRESHOLD
      reset()
      if (!pulled || refreshing) return
      setRefreshing(true)
      void onRefresh().catch(() => {}).finally(() => setRefreshing(false))
    },
    onTouchCancel: reset,
  }

  return { bind, pull, refreshing }
}
