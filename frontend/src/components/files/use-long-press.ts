import { useCallback, useEffect, useRef, type PointerEvent as ReactPointerEvent } from 'react'

export interface MenuAnchor {
  /** Viewport coordinates the menu opens at. */
  x: number
  y: number
}

const HOLD_MS = 500
const SLOP_PX = 10

/**
 * Press-and-hold on a touch screen. A mouse never long-presses (it has the
 * right button and the ⋯), and a finger that moves is scrolling, not holding.
 * After a hold fires, the click the lift would produce is swallowed with
 * `takeFired()`, and so is the context-menu event some browsers add.
 */
export function useLongPress(onHold: (anchor: MenuAnchor) => void) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const origin = useRef({ x: 0, y: 0 })
  const fired = useRef(false)
  const onHoldRef = useRef(onHold)
  useEffect(() => { onHoldRef.current = onHold })

  const cancel = useCallback(() => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = null
  }, [])
  useEffect(() => cancel, [cancel])

  const onPointerDown = useCallback((e: ReactPointerEvent) => {
    if (e.pointerType === 'mouse') return
    fired.current = false
    origin.current = { x: e.clientX, y: e.clientY }
    cancel()
    timer.current = setTimeout(() => {
      timer.current = null
      fired.current = true
      onHoldRef.current(origin.current)
    }, HOLD_MS)
  }, [cancel])

  const onPointerMove = useCallback((e: ReactPointerEvent) => {
    if (!timer.current) return
    if (Math.hypot(e.clientX - origin.current.x, e.clientY - origin.current.y) > SLOP_PX) cancel()
  }, [cancel])

  /** True once after a hold fired: the caller then ignores the click that follows. */
  const takeFired = useCallback(() => {
    const was = fired.current
    fired.current = false
    return was
  }, [])

  return {
    handlers: { onPointerDown, onPointerMove, onPointerUp: cancel, onPointerCancel: cancel, onPointerLeave: cancel },
    /** The context-menu event after a hold: true when it should be dropped. */
    isHoldContextMenu: () => fired.current,
    takeFired,
  }
}
