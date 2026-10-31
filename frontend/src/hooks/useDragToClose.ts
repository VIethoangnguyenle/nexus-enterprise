import { useEffect, useRef, type RefObject } from 'react'
import { animate, useMotionValue, type MotionValue } from 'motion/react'
import { EASE } from '../lib/motion'
import { LOCK_THRESHOLD } from './useAxisLock'

/** Dragged down past this share of its height, the surface closes (mockup §7: > 25%). */
export const CLOSE_FRACTION = 0.25
/** Flicked down faster than this (px/ms), it closes whatever the distance. */
export const CLOSE_VELOCITY = 0.5
/** Back to rest: 240ms `--ease-out-expo`. */
const SETTLE_S = 0.24

/** Elements whose touches are their own: typing, and anything that marks itself (a zoomed image pans). */
const OWN_TOUCH = 'input, textarea, select, [contenteditable="true"], [data-own-touch]'

/** Whether a release `dy` px down at `velocity` px/ms closes a surface `height` px tall. */
export function shouldClose(dy: number, height: number, velocity: number): boolean {
  return dy > height * CLOSE_FRACTION || velocity > CLOSE_VELOCITY
}

/** A scrolled element between the touch and the surface: then a pull down scrolls it, it does not close. */
function scrolledBetween(target: Element | null, surface: HTMLElement): boolean {
  for (let el = target; el && el !== surface.parentElement; el = el.parentElement) {
    if (el.scrollTop > 0) return true
    if (el === surface) break
  }
  return false
}

/**
 * Drag a sheet, page or viewer down to close it (DESIGN.md §6 "Vuốt": "kéo sheet, trang Tôi, trình xem ảnh/tệp
 * xuống để đóng"). Anywhere on the surface, not only on its handle, as long as what is under the finger is scrolled
 * to its top: a list that has been scrolled scrolls back first. The axis is decided after 10px; a sideways touch is
 * left alone. `y` follows the finger 1:1 (bind it as the surface's `style.y`); past 25% of the height or a fast
 * flick it closes, otherwise it settles back in 240ms. Under reduced motion it does not follow; a release past the
 * threshold closes at once.
 *
 * Touch events, so the content keeps its native scroll and a drag is claimed with `preventDefault` only once it is
 * known to be a downward pull from the top.
 */
export function useDragToClose({ ref, enabled = true, reduced, onClose }: {
  ref: RefObject<HTMLElement | null>
  enabled?: boolean
  reduced: boolean
  onClose: () => void
}): MotionValue<number> {
  const y = useMotionValue(0)
  const latest = useRef({ reduced, onClose })
  useEffect(() => {
    latest.current = { reduced, onClose }
  })

  useEffect(() => {
    const el = ref.current
    if (!enabled || !el) return
    let g: { id: number; x0: number; y0: number; lastY: number; lastT: number; velocity: number; pulling: boolean } | null = null
    const touchOf = (e: TouchEvent) => (g ? Array.from(e.changedTouches).find((t) => t.identifier === g!.id) : undefined)

    const onStart = (e: TouchEvent) => {
      const t = e.touches[0]
      const target = e.target as Element | null
      g = e.touches.length === 1 && t && !target?.closest(OWN_TOUCH) && !scrolledBetween(target, el)
        ? { id: t.identifier, x0: t.clientX, y0: t.clientY, lastY: t.clientY, lastT: e.timeStamp, velocity: 0, pulling: false }
        : null
    }
    const onMove = (e: TouchEvent) => {
      const t = touchOf(e)
      if (!g || !t) return
      const dy = t.clientY - g.y0
      if (!g.pulling) {
        const dx = Math.abs(t.clientX - g.x0)
        if (dx < LOCK_THRESHOLD && Math.abs(dy) < LOCK_THRESHOLD) return
        // Sideways, or up (a scroll): not ours.
        if (dx > Math.abs(dy) || dy < 0) {
          g = null
          return
        }
        g.pulling = true
      }
      if (e.cancelable) e.preventDefault()
      const dt = e.timeStamp - g.lastT
      if (dt > 0) g.velocity = (t.clientY - g.lastY) / dt
      g.lastY = t.clientY
      g.lastT = e.timeStamp
      if (!latest.current.reduced) y.set(Math.max(0, dy))
    }
    const onEnd = (e: TouchEvent) => {
      const t = touchOf(e)
      if (!g || !t) return
      const done = g
      g = null
      if (!done.pulling) return
      const height = el.offsetHeight || window.innerHeight || 1
      if (e.type === 'touchend' && shouldClose(t.clientY - done.y0, height, done.velocity)) {
        latest.current.onClose()
        return
      }
      if (latest.current.reduced) y.jump(0)
      else void animate(y, 0, { duration: SETTLE_S, ease: EASE.outExpo })
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
  }, [enabled, ref, y])

  return y
}
