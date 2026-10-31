import { useEffect, useRef, type PointerEvent } from 'react'

/** Two taps within this window are a double-tap (DESIGN.md §6 "Vuốt": ≤ 300ms). */
export const DOUBLE_TAP_MS = 300
/** A finger that travels further than this between down and up is a drag, not a tap. */
const TAP_SLOP = 10
/** The second tap lands this close to the first, or it is a tap somewhere else. */
const SAME_SPOT = 30

/** Taps on these are the control's own (a link, a reaction chip, the thread summary). */
const INTERACTIVE = 'a, button, input, textarea, select, [role="button"], [contenteditable="true"]'

export interface DoubleTapHandlers {
  onPointerDown?: (e: PointerEvent<HTMLElement>) => void
  onPointerUp?: (e: PointerEvent<HTMLElement>) => void
}

/**
 * Double-tap on a touch screen: two short taps within 300ms on the same spot (DESIGN.md §6 "Vuốt": "Chạm đúp thả
 * 👍"). Single taps pass through without delay: the callback fires on the second tap, not after a timer, so links
 * and the tap that shows a grouped line's time keep their speed. A press held longer than the window (a long
 * press) or a drag is not a tap and resets the count; taps on controls inside the element are theirs. Mouse
 * input is ignored: a mouse has the hover bar.
 */
export function useDoubleTap(onDoubleTap: () => void, enabled = true): DoubleTapHandlers {
  const handler = useRef(onDoubleTap)
  useEffect(() => {
    handler.current = onDoubleTap
  })
  const last = useRef<{ t: number; x: number; y: number } | null>(null)
  const down = useRef<{ t: number; x: number; y: number } | null>(null)

  if (!enabled) return {}

  return {
    onPointerDown: (e) => {
      if (e.pointerType === 'mouse' || !e.isPrimary || (e.target as HTMLElement).closest(INTERACTIVE)) {
        down.current = null
        return
      }
      down.current = { t: e.timeStamp, x: e.clientX, y: e.clientY }
    },
    onPointerUp: (e) => {
      const d = down.current
      down.current = null
      if (e.pointerType === 'mouse' || !d) return
      const tap = Math.hypot(e.clientX - d.x, e.clientY - d.y) <= TAP_SLOP && e.timeStamp - d.t <= DOUBLE_TAP_MS
      if (!tap) {
        last.current = null
        return
      }
      const prev = last.current
      if (prev && e.timeStamp - prev.t <= DOUBLE_TAP_MS && Math.hypot(e.clientX - prev.x, e.clientY - prev.y) <= SAME_SPOT) {
        last.current = null
        handler.current()
      } else {
        last.current = { t: e.timeStamp, x: e.clientX, y: e.clientY }
      }
    },
  }
}
