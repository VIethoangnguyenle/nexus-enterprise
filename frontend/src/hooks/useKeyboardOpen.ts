import { useSyncExternalStore } from 'react'

/** How much shorter than the window the visible area must be before it counts as the on-screen keyboard. */
export const KEYBOARD_MIN_PX = 120

/** Below this width the composer docks to the keyboard (DESIGN.md "Ô soạn", phone). */
const PHONE_MAX_PX = 767

function subscribe(onChange: () => void): () => void {
  const vv = window.visualViewport
  vv?.addEventListener('resize', onChange)
  window.addEventListener('resize', onChange)
  return () => {
    vv?.removeEventListener('resize', onChange)
    window.removeEventListener('resize', onChange)
  }
}

function snapshot(): boolean {
  const vv = window.visualViewport
  if (!vv || window.innerWidth > PHONE_MAX_PX) return false
  return window.innerHeight - vv.height > KEYBOARD_MIN_PX
}

/**
 * Whether the on-screen keyboard is up on a phone: the visible viewport is more
 * than KEYBOARD_MIN_PX shorter than the window. iOS Safari and installed apps
 * shrink only the visual viewport for the keyboard, so this is the signal that
 * works there; desktops and tablets always answer false.
 */
export function useKeyboardOpen(): boolean {
  return useSyncExternalStore(subscribe, snapshot, () => false)
}
