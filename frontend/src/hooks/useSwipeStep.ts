import { useRef, type TouchEvent } from 'react'
import { animate, useMotionValue, type MotionValue } from 'motion/react'
import { EASE, useMotionPresets } from '../lib/motion'
import { LOCK_THRESHOLD } from './useAxisLock'
import { isInEdgeZone } from './useEdgeBack'

/** A horizontal swipe this long changes the period. */
export const STEP_DISTANCE = 48
/** Or one flicked faster than this (px/ms). */
export const STEP_VELOCITY = 0.5
const SETTLE_S = 0.24

/**
 * A horizontal swipe that steps a period: the week strip and the month grid of Lịch (DESIGN.md §6 "Vuốt": "vuốt
 * ngang dải tuần / lưới tháng"; mockup §7, with ‹ › as the buttons that do the same). Left is the next period,
 * right the one before. The axis is decided after 10px, so a vertical scroll over the grid stays a scroll; a touch
 * that starts in the 20px edge zone is the app's back, never a step.
 *
 * Spread `bind` on the element and bind `x` as its `style.x`: it follows the finger 1:1 and settles back in 240ms
 * when the swipe falls short. Under reduced motion it does not follow; a swipe past the threshold steps at once.
 * Give the element `touch-pan-y` so the browser leaves horizontal moves to it.
 */
export function useSwipeStep(onStep: (step: -1 | 1) => void, enabled = true): {
  bind: {
    onTouchStart: (e: TouchEvent) => void
    onTouchMove: (e: TouchEvent) => void
    onTouchEnd: (e: TouchEvent) => void
    onTouchCancel: () => void
  }
  x: MotionValue<number>
} {
  const { reduced } = useMotionPresets()
  const x = useMotionValue(0)
  const g = useRef<{ x0: number; y0: number; lastX: number; lastT: number; velocity: number; axis: 'x' | 'y' | null } | null>(null)

  const settle = () => {
    if (reduced) x.jump(0)
    else void animate(x, 0, { duration: SETTLE_S, ease: EASE.outExpo })
  }

  const bind = {
    onTouchStart: (e: TouchEvent) => {
      const t = e.touches[0]
      g.current = enabled && t && e.touches.length === 1 && !isInEdgeZone(t.clientX)
        ? { x0: t.clientX, y0: t.clientY, lastX: t.clientX, lastT: e.timeStamp, velocity: 0, axis: null }
        : null
    },
    onTouchMove: (e: TouchEvent) => {
      const s = g.current
      const t = e.touches[0]
      if (!s || !t) return
      if (!s.axis) {
        const dx = Math.abs(t.clientX - s.x0)
        const dy = Math.abs(t.clientY - s.y0)
        if (dx < LOCK_THRESHOLD && dy < LOCK_THRESHOLD) return
        s.axis = dx >= dy ? 'x' : 'y'
      }
      if (s.axis === 'y') {
        g.current = null
        return
      }
      const dt = e.timeStamp - s.lastT
      if (dt > 0) s.velocity = (t.clientX - s.lastX) / dt
      s.lastX = t.clientX
      s.lastT = e.timeStamp
      if (!reduced) x.set(t.clientX - s.x0)
    },
    onTouchEnd: (e: TouchEvent) => {
      const s = g.current
      g.current = null
      const t = e.changedTouches[0]
      if (!s) return
      const end = t?.clientX ?? s.lastX
      const dx = end - s.x0
      // No move reported between the two (a quick flick on some devices): the axis is read off the release.
      if (!s.axis && t && Math.abs(dx) >= LOCK_THRESHOLD && Math.abs(dx) >= Math.abs(t.clientY - s.y0)) s.axis = 'x'
      if (s.axis !== 'x') return
      if (Math.abs(dx) >= STEP_DISTANCE || Math.abs(s.velocity) > STEP_VELOCITY) {
        x.jump(0)
        onStep(dx < 0 ? 1 : -1)
      } else {
        settle()
      }
    },
    onTouchCancel: () => {
      if (g.current?.axis === 'x') settle()
      g.current = null
    },
  }
  return { bind, x }
}
