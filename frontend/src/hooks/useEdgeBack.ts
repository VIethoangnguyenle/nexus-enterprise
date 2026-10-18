import { useCallback, useEffect, useRef, useState, type RefObject } from 'react'
import { useCanGoBack, useNavigate, useRouter } from '@tanstack/react-router'
import { animate, useMotionValue, type MotionValue } from 'motion/react'
import { footNavItems, isModuleBarPath, mainNavItems, moduleOfPath } from '../lib/nav-destinations'
import { EASE } from '../lib/motion'
import { LOCK_THRESHOLD } from './useAxisLock'

/** The strip at the left edge where a swipe is the app's back (DESIGN.md §6 "Vuốt"). */
export const EDGE_ZONE = 20
/** Released past this share of the width, the swipe goes back. */
export const RELEASE_THRESHOLD = 0.35
/** Released faster than this (px/ms), it goes back whatever the distance. */
export const VELOCITY_THRESHOLD = 0.5
/** Release to rest or off screen: 240ms `--ease-out-expo`. */
export const RELEASE_S = 0.24

/** Whether a touch at this x starts in the edge zone, where it is a back and never a row or message swipe. */
export function isInEdgeZone(clientX: number): boolean {
  return clientX <= EDGE_ZONE
}

/** Whether a release at `dx` px from where it started, moving at `velocity` px/ms, goes back. */
export function shouldGoBack(dx: number, width: number, velocity: number): boolean {
  return dx > width * RELEASE_THRESHOLD || velocity > VELOCITY_THRESHOLD
}

/** Screens with a way back, where the edge swipe works: every screen without the bottom bar (details, settings). */
export function isEdgeBackPath(pathname: string): boolean {
  return !isModuleBarPath(pathname)
}

/**
 * Where back goes with no earlier screen in the app (opened from a push notification): the root list of the
 * module the screen belongs to, or Chat when the screen is that root already.
 */
export function edgeBackRoot(pathname: string): string {
  const module = moduleOfPath(pathname)
  const root = [...mainNavItems, ...footNavItems].find((d) => d.id === module)?.to
  return root && root !== pathname ? root : '/channels'
}

/**
 * The step an edge swipe takes: the app's history back, or the module's root when there is none. No `markOwnBack`:
 * the finger has already drawn the move, so the screen it lands on appears at once, as after the device's own back.
 */
export function useEdgeBackAction(pathname: string): () => void {
  const router = useRouter()
  const navigate = useNavigate()
  const canGoBack = useCanGoBack()
  return useCallback(() => {
    if (canGoBack) router.history.back()
    else void navigate({ to: edgeBackRoot(pathname), replace: true })
  }, [canGoBack, router, navigate, pathname])
}

export type EdgePhase = 'idle' | 'dragging' | 'leaving'

/**
 * The edge-back gesture on `ref`'s element. Touch events, not pointer events: the screen keeps its native vertical
 * scroll (a touch that turns out vertical is left alone) and a horizontal one is claimed with `preventDefault` once
 * the axis is decided after 10px. `x` is how far the screen has moved, written straight to the motion value so a
 * drag re-renders nothing; `phase` changes only at the start and end of a gesture.
 *
 * Under reduced motion the screen does not follow the finger; a release past the threshold goes back at once.
 */
export function useEdgeBack({ ref, enabled, reduced, onBack }: {
  ref: RefObject<HTMLElement | null>
  enabled: boolean
  reduced: boolean
  onBack: () => void
}): { x: MotionValue<number>; phase: EdgePhase; reset: () => void } {
  const x = useMotionValue(0)
  const [phase, setPhase] = useState<EdgePhase>('idle')
  const latest = useRef({ reduced, onBack })
  useEffect(() => {
    latest.current = { reduced, onBack }
  })

  const reset = useCallback(() => {
    x.jump(0)
    setPhase('idle')
  }, [x])

  useEffect(() => {
    const el = ref.current
    if (!enabled || !el) return
    let g: { id: number; x0: number; y0: number; lastX: number; lastT: number; velocity: number; axis: 'x' | 'y' | null } | null = null

    const width = () => el.clientWidth || window.innerWidth || 1
    const touchOf = (e: TouchEvent) => (g ? Array.from(e.changedTouches).find((t) => t.identifier === g!.id) : undefined)

    const onStart = (e: TouchEvent) => {
      const t = e.touches[0]
      if (e.touches.length !== 1 || !t || !isInEdgeZone(t.clientX)) {
        g = null
        return
      }
      g = { id: t.identifier, x0: t.clientX, y0: t.clientY, lastX: t.clientX, lastT: e.timeStamp, velocity: 0, axis: null }
    }
    const onMove = (e: TouchEvent) => {
      const t = touchOf(e)
      if (!g || !t) return
      if (!g.axis) {
        const dx = Math.abs(t.clientX - g.x0)
        const dy = Math.abs(t.clientY - g.y0)
        if (dx < LOCK_THRESHOLD && dy < LOCK_THRESHOLD) return
        g.axis = dx >= dy ? 'x' : 'y'
        if (g.axis === 'y') {
          g = null
          return
        }
        setPhase('dragging')
      }
      if (e.cancelable) e.preventDefault()
      const dt = e.timeStamp - g.lastT
      if (dt > 0) g.velocity = (t.clientX - g.lastX) / dt
      g.lastX = t.clientX
      g.lastT = e.timeStamp
      if (!latest.current.reduced) x.set(Math.max(0, t.clientX - g.x0))
    }
    const onEnd = (e: TouchEvent) => {
      const t = touchOf(e)
      if (!g || !t) return
      const done = g
      g = null
      if (done.axis !== 'x') return
      const w = width()
      const back = e.type === 'touchend' && shouldGoBack(t.clientX - done.x0, w, done.velocity)
      const { reduced: still, onBack: goBack } = latest.current
      if (!back) {
        if (still) reset()
        else void animate(x, 0, { duration: RELEASE_S, ease: EASE.outExpo, onComplete: () => setPhase('idle') })
        return
      }
      setPhase('leaving')
      if (still) goBack()
      else void animate(x, w, { duration: RELEASE_S, ease: EASE.outExpo, onComplete: goBack })
    }

    el.addEventListener('touchstart', onStart, { passive: true })
    el.addEventListener('touchmove', onMove, { passive: false })
    el.addEventListener('touchend', onEnd)
    el.addEventListener('touchcancel', onEnd)
    return () => {
      el.removeEventListener('touchstart', onStart)
      el.removeEventListener('touchmove', onMove)
      el.removeEventListener('touchend', onEnd)
      el.removeEventListener('touchcancel', onEnd)
    }
  }, [enabled, ref, x, reset])

  return { x, phase, reset }
}
