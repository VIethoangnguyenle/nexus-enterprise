import { useCallback, useRef, type PointerEvent } from 'react'
import { animate, useMotionValue, type MotionValue } from 'motion/react'
import { EASE } from '../lib/motion'

/** Dragged past this share of the screen height, the composer opens full screen (or closes back). */
export const GRIP_FRACTION = 0.25
/** Or flicked faster than this (px/ms), whatever the distance. */
export const GRIP_VELOCITY = 0.5
/** Less than this between press and release is a tap on the grip, which toggles. */
const TAP_SLOP = 10
const SETTLE_S = 0.24

/** Whether a drag of `dy` px (negative is up) at `velocity` px/ms changes the composer, given its state. */
export function gripChanges(expanded: boolean, dy: number, velocity: number, height: number): boolean {
  const toward = expanded ? dy : -dy
  const speed = expanded ? velocity : -velocity
  return toward > height * GRIP_FRACTION || speed > GRIP_VELOCITY
}

/**
 * The grip above the phone composer (DESIGN.md §6 "Vuốt", mockup §9): drag it up past 25% of the screen, or flick
 * it, to write full screen; drag it down to come back. `pull` is how far the finger has dragged in the direction
 * that counts (negative up while collapsed, positive down while open), for the composer to follow; it settles back
 * in 240ms. Under reduced motion it does not follow; the release decides at once. The grip is a button: a tap
 * toggles, so the gesture has its button and a screen-reader action.
 */
export function useComposerGrip({ expanded, onExpand, onCollapse, reduced }: {
  expanded: boolean
  onExpand: () => void
  onCollapse: () => void
  reduced: boolean
}): {
  bind: {
    onPointerDown: (e: PointerEvent<HTMLElement>) => void
    onPointerMove: (e: PointerEvent<HTMLElement>) => void
    onPointerUp: (e: PointerEvent<HTMLElement>) => void
    onPointerCancel: () => void
    onClick: () => void
  }
  pull: MotionValue<number>
} {
  const pull = useMotionValue(0)
  const g = useRef<{ id: number; y0: number; lastY: number; lastT: number; velocity: number } | null>(null)
  const dragged = useRef(false)

  const settle = useCallback(() => {
    if (reduced) pull.jump(0)
    else void animate(pull, 0, { duration: SETTLE_S, ease: EASE.outExpo })
  }, [reduced, pull])

  const onPointerDown = (e: PointerEvent<HTMLElement>) => {
    if (!e.isPrimary) return
    g.current = { id: e.pointerId, y0: e.clientY, lastY: e.clientY, lastT: e.timeStamp, velocity: 0 }
    dragged.current = false
    try {
      e.currentTarget.setPointerCapture(e.pointerId)
    } catch {
      // Without capture the drag still works while the finger stays over the grip.
    }
  }
  const onPointerMove = (e: PointerEvent<HTMLElement>) => {
    const s = g.current
    if (!s || e.pointerId !== s.id) return
    const dy = e.clientY - s.y0
    if (Math.abs(dy) > TAP_SLOP) dragged.current = true
    const dt = e.timeStamp - s.lastT
    if (dt > 0) s.velocity = (e.clientY - s.lastY) / dt
    s.lastY = e.clientY
    s.lastT = e.timeStamp
    if (!reduced) pull.set(expanded ? Math.max(0, dy) : Math.min(0, dy))
  }
  const onPointerUp = (e: PointerEvent<HTMLElement>) => {
    const s = g.current
    if (!s || e.pointerId !== s.id) return
    g.current = null
    const dy = e.clientY - s.y0
    if (Math.abs(dy) > TAP_SLOP) dragged.current = true
    settle()
    if (!dragged.current) return
    if (gripChanges(expanded, dy, s.velocity, window.innerHeight || 1)) {
      if (expanded) onCollapse()
      else onExpand()
    }
  }
  const onPointerCancel = () => {
    g.current = null
    settle()
  }
  // The click that ends a drag is the drag's; a plain tap toggles.
  const onClick = () => {
    if (dragged.current) {
      dragged.current = false
      return
    }
    if (expanded) onCollapse()
    else onExpand()
  }

  return { bind: { onPointerDown, onPointerMove, onPointerUp, onPointerCancel, onClick }, pull }
}
