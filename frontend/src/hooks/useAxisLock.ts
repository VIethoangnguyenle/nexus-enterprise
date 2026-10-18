import { useRef, useCallback } from 'react'

/** Pixels of movement before the axis is decided. */
export const LOCK_THRESHOLD = 10

export type Axis = 'x' | 'y' | null

/**
 * After 10px of movement the dominant axis is locked and stays locked until
 * `reset()`. DESIGN.md §6 "Vuốt": "khoá trục sau 10px đầu". The lock prevents
 * a horizontal swipe from scrolling vertically or a vertical scroll from
 * opening a row's actions.
 */
export function useAxisLock() {
  const axis = useRef<Axis>(null)
  const origin = useRef<{ x: number; y: number } | null>(null)

  const start = useCallback((x: number, y: number) => {
    origin.current = { x, y }
    axis.current = null
  }, [])

  /** Returns the locked axis, or `null` while undecided. */
  const update = useCallback((x: number, y: number): Axis => {
    if (axis.current) return axis.current
    const o = origin.current
    if (!o) return null
    const dx = Math.abs(x - o.x)
    const dy = Math.abs(y - o.y)
    if (dx < LOCK_THRESHOLD && dy < LOCK_THRESHOLD) return null
    axis.current = dx >= dy ? 'x' : 'y'
    return axis.current
  }, [])

  const reset = useCallback(() => {
    axis.current = null
    origin.current = null
  }, [])

  /** The current locked axis (read-only). */
  const current = useCallback((): Axis => axis.current, [])

  return { start, update, reset, current }
}
