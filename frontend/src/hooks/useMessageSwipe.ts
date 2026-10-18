import { useCallback, useRef, type PointerEvent } from 'react'
import { animate, useMotionValue, type MotionValue } from 'motion/react'
import { isInEdgeZone } from './useEdgeBack'
import { useAxisLock } from './useAxisLock'
import { EASE } from '../lib/motion'

/** The swipe distance (px) that acts (DESIGN.md §6 "Vuốt": 64px). */
export const MSG_SWIPE_THRESHOLD = 64
/** The affordance icon past the threshold (DESIGN.md: 1.15, and the accent). */
export const MSG_SWIPE_SCALE = 1.15
/** How far the message follows the finger before it stops. */
const MAX_PULL = MSG_SWIPE_THRESHOLD * 1.5
/** Back to rest: 240ms `--ease-out-expo` (DESIGN.md §6 "Quy tắc chung"). */
const SETTLE_S = 0.24

export interface UseMessageSwipeReturn {
  /** Bind these on the message's swipeable wrapper. */
  bind: {
    onPointerDown: (e: PointerEvent<HTMLElement>) => void
    onPointerMove: (e: PointerEvent<HTMLElement>) => void
    onPointerUp: (e: PointerEvent<HTMLElement>) => void
    onPointerCancel: (e: PointerEvent<HTMLElement>) => void
  }
  /** The x offset, for `style={{ x }}` on the message content. */
  x: MotionValue<number>
  /** 0→1 progress of the right (quote) swipe; 1 is the threshold. */
  rightProgress: MotionValue<number>
  /** 0→1 progress of the left (thread) swipe. */
  leftProgress: MotionValue<number>
  /** Drops the gesture under way (a long press took the finger over). */
  cancel: () => void
}

/** Text is selected: a horizontal move extends the selection, it is not a swipe. */
function selecting(): boolean {
  const sel = typeof window !== 'undefined' ? window.getSelection?.() : null
  return !!sel && !sel.isCollapsed
}

/**
 * Swipe a message right to quote-reply, left to open its thread (DESIGN.md §6 "Vuốt", mockup §9): the same
 * directions for your own messages and others'. Touch and pen only; a swipe that starts in the 20px left edge is
 * always the app's back, and one that starts while text is selected is the selection's. The axis is locked after
 * 10px, so a vertical scroll never drags a message. The message follows the finger 1:1 and settles back in 240ms;
 * released past 64px it acts. Under reduced motion it does not follow; passing the threshold acts at once.
 * A direction without a handler does not move.
 */
export function useMessageSwipe({ onQuote, onThread, enabled = true, reduced = false }: {
  onQuote?: () => void
  onThread?: () => void
  enabled?: boolean
  reduced?: boolean
}): UseMessageSwipeReturn {
  const x = useMotionValue(0)
  const rightProgress = useMotionValue(0)
  const leftProgress = useMotionValue(0)
  const axis = useAxisLock()
  const g = useRef<{ pointerId: number; startX: number; locked: boolean } | null>(null)

  const settle = useCallback(() => {
    for (const v of [x, rightProgress, leftProgress]) {
      if (reduced) v.jump(0)
      else void animate(v, 0, { duration: SETTLE_S, ease: EASE.outExpo })
    }
  }, [reduced, x, rightProgress, leftProgress])

  const cancel = useCallback(() => {
    if (!g.current) return
    g.current = null
    axis.reset()
    settle()
  }, [axis, settle])

  const onPointerDown = useCallback((e: PointerEvent<HTMLElement>) => {
    g.current = null
    if (!enabled || e.pointerType === 'mouse' || !e.isPrimary || isInEdgeZone(e.clientX) || selecting()) return
    g.current = { pointerId: e.pointerId, startX: e.clientX, locked: false }
    axis.start(e.clientX, e.clientY)
  }, [enabled, axis])

  const act = useCallback((dx: number): boolean => {
    if (dx >= MSG_SWIPE_THRESHOLD && onQuote) {
      onQuote()
      return true
    }
    if (dx <= -MSG_SWIPE_THRESHOLD && onThread) {
      onThread()
      return true
    }
    return false
  }, [onQuote, onThread])

  const onPointerMove = useCallback((e: PointerEvent<HTMLElement>) => {
    const s = g.current
    if (!s || e.pointerId !== s.pointerId) return
    const locked = axis.update(e.clientX, e.clientY)
    if (locked === 'y') {
      g.current = null
      axis.reset()
      return
    }
    if (locked !== 'x') return
    if (!s.locked) {
      s.locked = true
      try {
        e.currentTarget.setPointerCapture(e.pointerId)
      } catch {
        // Capture is a nicety (the finger may leave the row); the gesture works without it.
      }
    }
    const raw = e.clientX - s.startX
    const dx = (raw > 0 && !onQuote) || (raw < 0 && !onThread) ? 0 : raw
    if (reduced) {
      if (act(dx)) {
        g.current = null
        axis.reset()
      }
      return
    }
    x.set(Math.max(-MAX_PULL, Math.min(MAX_PULL, dx)))
    rightProgress.set(Math.min(Math.max(dx, 0) / MSG_SWIPE_THRESHOLD, 1))
    leftProgress.set(Math.min(Math.max(-dx, 0) / MSG_SWIPE_THRESHOLD, 1))
  }, [axis, onQuote, onThread, reduced, act, x, rightProgress, leftProgress])

  const finish = useCallback((e: PointerEvent<HTMLElement>, cancelled: boolean) => {
    const s = g.current
    if (!s || e.pointerId !== s.pointerId) return
    g.current = null
    axis.reset()
    settle()
    if (s.locked && !cancelled) act(e.clientX - s.startX)
  }, [axis, settle, act])

  const onPointerUp = useCallback((e: PointerEvent<HTMLElement>) => finish(e, false), [finish])
  const onPointerCancel = useCallback((e: PointerEvent<HTMLElement>) => finish(e, true), [finish])

  return { bind: { onPointerDown, onPointerMove, onPointerUp, onPointerCancel }, x, rightProgress, leftProgress, cancel }
}
