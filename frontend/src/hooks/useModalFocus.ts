import { useEffect, useRef, type RefObject } from 'react'

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

interface ModalFocusOptions {
  /** True while the surface is open (mounted and meant to hold focus). */
  active: boolean
  /** The dialog surface; Tab cycles inside it. */
  surfaceRef: RefObject<HTMLElement | null>
  /** Esc and the like. */
  onClose: () => void
  /** Focused on open; defaults to the first focusable element in the surface. */
  initialFocusRef?: RefObject<HTMLElement | null>
}

/** The surface of the modal layer that opened last: only it answers the keyboard. */
const isTopmost = (surface: HTMLElement | null) => {
  const all = document.querySelectorAll('[aria-modal="true"]')
  return !!surface && all[all.length - 1] === surface
}

/**
 * The focus rules every modal layer follows (DESIGN.md §6): while open the app
 * behind is inert; focus moves in on open (to `initialFocusRef` or the first
 * control); Tab cycles inside; Esc closes and is marked taken so a panel behind
 * does not also close; on close focus returns to whatever opened it. With two
 * layers open (a dialog over a phone sheet) only the top one reacts to keys.
 *
 * Shared by `Dialog` and by `SidePanel` where it is a full-screen sheet.
 */
export function useModalFocus({ active, surfaceRef, onClose, initialFocusRef }: ModalFocusOptions) {
  const returnTo = useRef<HTMLElement | null>(null)

  // While open, the app behind the scrim is inert: no focus, no clicks, and
  // hidden from assistive tech (aria-modal alone is not honoured everywhere).
  useEffect(() => {
    const app = document.getElementById('root')
    if (!active || !app) return
    app.inert = true
    return () => {
      app.inert = false
    }
  }, [active])

  useEffect(() => {
    if (!active) return
    returnTo.current = document.activeElement as HTMLElement | null
    const id = requestAnimationFrame(() => {
      const target = initialFocusRef?.current ?? surfaceRef.current?.querySelector<HTMLElement>(FOCUSABLE)
      target?.focus()
    })
    return () => {
      cancelAnimationFrame(id)
      // Give focus back to the opener so keyboard users land where they were.
      returnTo.current?.focus?.()
    }
  }, [active, initialFocusRef, surfaceRef])

  useEffect(() => {
    if (!active) return
    const onKey = (e: KeyboardEvent) => {
      const surface = surfaceRef.current
      if (!isTopmost(surface)) return
      if (e.key === 'Escape') {
        // Marks the key as taken, so a panel behind this layer does not also close.
        e.preventDefault()
        e.stopPropagation()
        onClose()
        return
      }
      if (e.key !== 'Tab' || !surface) return
      const nodes = Array.from(surface.querySelectorAll<HTMLElement>(FOCUSABLE))
      const first = nodes[0]
      const last = nodes[nodes.length - 1]
      if (!first || !last) return
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      } else if (!surface.contains(document.activeElement)) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [active, onClose, surfaceRef])
}
