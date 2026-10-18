import { useRef, useState, type TouchEvent } from 'react'
import { useMotionPresets } from '../lib/motion'
import { LOCK_THRESHOLD } from './useAxisLock'

/** How far the finger must travel, after damping, to count as a pull. */
export const PULL_THRESHOLD = 56
/** The indicator never follows the finger further than this. */
export const MAX_PULL = 80
/** The list gives way at half the finger's speed. */
const DAMPING = 0.5

/**
 * Pull down from the top of a list to reload it (DESIGN.md §6 "Vuốt": "kéo xuống ở đầu mọi danh sách để tải lại").
 * An installed app has no browser pull-to-refresh, so lists draw their own; `PullToRefresh` is the ready-made
 * scrolling element with its indicator.
 *
 * Spread `bind` on the scrolling element; `pull` is how far to draw the indicator, `refreshing` holds it open until
 * `onRefresh` settles (a failure just ends it). A pull only starts with the element at its very top, and a touch
 * that turns out horizontal (a row swipe, the edge back) is never one: the axis is decided after 10px. Under reduced
 * motion the indicator does not follow the finger: it shows once the pull has passed the threshold.
 */
export function usePullToRefresh(onRefresh: () => Promise<unknown>, enabled = true) {
  const { reduced } = useMotionPresets()
  const [pull, setPull] = useState(0)
  const [refreshing, setRefreshing] = useState(false)
  const start = useRef<{ x: number; y: number; axis: 'x' | 'y' | null } | null>(null)
  const latest = useRef(0)

  const reset = () => {
    start.current = null
    latest.current = 0
    setPull(0)
  }

  const bind = {
    onTouchStart: (e: TouchEvent<HTMLElement>) => {
      const t = e.touches[0]
      // Only a pull that starts with the list at its top is a pull.
      start.current = enabled && t && !refreshing && e.touches.length === 1 && e.currentTarget.scrollTop <= 0
        ? { x: t.clientX, y: t.clientY, axis: null }
        : null
    },
    onTouchMove: (e: TouchEvent<HTMLElement>) => {
      const s = start.current
      const t = e.touches[0]
      if (!s || !t) return
      if (!s.axis) {
        const dx = Math.abs(t.clientX - s.x)
        const dy = Math.abs(t.clientY - s.y)
        if (dx < LOCK_THRESHOLD && dy < LOCK_THRESHOLD) return
        s.axis = dx > dy ? 'x' : 'y'
      }
      if (s.axis === 'x') {
        if (latest.current !== 0) reset()
        start.current = null
        return
      }
      const dy = (t.clientY - s.y) * DAMPING
      if (dy <= 0) {
        if (latest.current !== 0) {
          latest.current = 0
          setPull(0)
        }
        return
      }
      latest.current = Math.min(dy, MAX_PULL)
      const shown = reduced ? (latest.current >= PULL_THRESHOLD ? PULL_THRESHOLD : 0) : latest.current
      setPull(shown)
    },
    onTouchEnd: () => {
      const pulled = latest.current >= PULL_THRESHOLD
      reset()
      if (!pulled || refreshing) return
      setRefreshing(true)
      void onRefresh().catch(() => {}).finally(() => setRefreshing(false))
    },
    onTouchCancel: reset,
  }

  return { bind, pull, refreshing }
}
