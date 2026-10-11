import { useCallback, useEffect, useRef, type PointerEvent, type MouseEvent } from 'react'

/** How long a finger rests on a message before its action sheet opens (DESIGN.md "Chat trên điện thoại"). */
export const LONG_PRESS_MS = 400
/** A finger that travels further than this is scrolling, not pressing. */
const MOVE_TOLERANCE = 10

const INTERACTIVE = 'a, button, input, textarea, select, [role="button"], [contenteditable="true"]'

/**
 * Long press for touch and pen (a mouse has the hover bar). Spread the result
 * on the element; `onLongPress` fires once after 400ms of rest, with a short
 * vibration where the device has one. A press that starts on a control inside
 * the element (a reaction, the thread summary) is left to that control, and the
 * browser's own long-press menu is suppressed for touch.
 */
export function useLongPress(onLongPress: () => void, enabled = true) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const origin = useRef<{ x: number; y: number } | null>(null)
  const touching = useRef(false)
  const handler = useRef(onLongPress)
  useEffect(() => {
    handler.current = onLongPress
  })

  const cancel = useCallback(() => {
    if (timer.current) clearTimeout(timer.current)
    timer.current = null
    origin.current = null
  }, [])
  useEffect(() => cancel, [cancel])

  return {
    onPointerDown: (e: PointerEvent<HTMLElement>) => {
      touching.current = e.pointerType !== 'mouse'
      if (!enabled || !touching.current) return
      if ((e.target as HTMLElement).closest(INTERACTIVE)) return
      origin.current = { x: e.clientX, y: e.clientY }
      timer.current = setTimeout(() => {
        timer.current = null
        navigator.vibrate?.(10)
        handler.current()
      }, LONG_PRESS_MS)
    },
    onPointerMove: (e: PointerEvent<HTMLElement>) => {
      const o = origin.current
      if (o && Math.hypot(e.clientX - o.x, e.clientY - o.y) > MOVE_TOLERANCE) cancel()
    },
    onPointerUp: cancel,
    onPointerCancel: cancel,
    onPointerLeave: cancel,
    onContextMenu: (e: MouseEvent<HTMLElement>) => {
      if (enabled && touching.current) e.preventDefault()
    },
  }
}
