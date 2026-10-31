import { useEffect, type RefObject } from 'react'
import { useKeyboardOpen } from './useKeyboardOpen'

/** Dragging the list down this far with the keyboard up closes the keyboard (DESIGN.md §6 "Vuốt"). */
export const DISMISS_DISTANCE = 48

const TYPING = 'input, textarea, [contenteditable="true"]'

/**
 * Drag a message list down to put the keyboard away (DESIGN.md §6 "Vuốt", mockup §9). iOS web cannot move the
 * keyboard with the finger, so once a downward drag on `ref`'s element passes 48px while the keyboard is up, the
 * field being typed in loses focus and the keyboard goes; the list scrolls as it always does. A sideways drag (a
 * message swipe) is not this. Tapping outside the composer stays the button-free way to do the same.
 */
export function useKeyboardDismissDrag(ref: RefObject<HTMLElement | null>, enabled = true): void {
  const keyboardOpen = useKeyboardOpen()
  const active = enabled && keyboardOpen

  useEffect(() => {
    const el = ref.current
    if (!active || !el) return
    let from: { x: number; y: number } | null = null

    const onStart = (e: TouchEvent) => {
      const t = e.touches[0]
      from = e.touches.length === 1 && t ? { x: t.clientX, y: t.clientY } : null
    }
    const onMove = (e: TouchEvent) => {
      const t = e.touches[0]
      if (!from || !t) return
      const dy = t.clientY - from.y
      if (Math.abs(t.clientX - from.x) > Math.max(dy, 0) + 10) {
        from = null
        return
      }
      if (dy <= DISMISS_DISTANCE) return
      from = null
      const typing = document.activeElement as HTMLElement | null
      if (typing?.matches?.(TYPING)) typing.blur()
    }
    const onEnd = () => {
      from = null
    }

    el.addEventListener('touchstart', onStart, { passive: true })
    el.addEventListener('touchmove', onMove, { passive: true })
    el.addEventListener('touchend', onEnd)
    el.addEventListener('touchcancel', onEnd)
    return () => {
      el.removeEventListener('touchstart', onStart)
      el.removeEventListener('touchmove', onMove)
      el.removeEventListener('touchend', onEnd)
      el.removeEventListener('touchcancel', onEnd)
    }
  }, [active, ref])
}
