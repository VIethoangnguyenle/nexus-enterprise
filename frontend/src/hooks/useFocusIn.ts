import { useEffect, type RefObject } from 'react'

/**
 * Moves focus into `surfaceRef` when it appears (to the element matching
 * `target`, else the surface itself) and puts it back where it was when it
 * goes: what a ringing call that appears out of nowhere owes the keyboard.
 */
export function useFocusIn(surfaceRef: RefObject<HTMLElement | null>, target = '[data-primary]'): void {
  useEffect(() => {
    const surface = surfaceRef.current
    if (!surface) return
    const before = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const el = surface.querySelector<HTMLElement>(target) ?? surface
    el.focus()
    return () => { if (before?.isConnected) before.focus() }
    // The surface keeps its mode for as long as it is mounted; a new mode is a new mount (keyed by the caller).
  }, [surfaceRef, target])
}
