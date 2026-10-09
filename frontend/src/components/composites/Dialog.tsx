import { useEffect, useId, useRef, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, motion } from 'motion/react'
import { useMotionPresets } from '../../lib/motion'
import { Heading } from '../primitives'

interface DialogProps {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  /** Right-aligned action row. */
  footer?: ReactNode
  /** Focused on open; defaults to the first focusable element. */
  initialFocusRef?: RefObject<HTMLElement | null>
  /** Wrap the body in a form so Enter submits. */
  onSubmit?: () => void
}

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

/**
 * Modal dialog (DESIGN.md §6): overlay surface, radius 12, max 520px, focus
 * trapped inside while open, focus returned to whatever opened it, Esc closes.
 * Enter/exit follow §7: scrim fades, the surface rises 8px and settles.
 *
 * Modals are a last resort; prefer a detail panel or an in-place action + Undo.
 */
export function Dialog({ open, onClose, title, children, footer, initialFocusRef, onSubmit }: DialogProps) {
  const m = useMotionPresets()
  const titleId = useId()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const returnTo = useRef<HTMLElement | null>(null)

  // While open, the app behind the scrim is inert: no focus, no clicks, and
  // hidden from assistive tech (aria-modal alone is not honoured everywhere).
  useEffect(() => {
    const app = document.getElementById('root')
    if (!open || !app) return
    app.inert = true
    return () => {
      app.inert = false
    }
  }, [open])

  useEffect(() => {
    if (!open) return
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
  }, [open, initialFocusRef])

  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose()
        return
      }
      if (e.key !== 'Tab' || !surfaceRef.current) return
      const nodes = Array.from(surfaceRef.current.querySelectorAll<HTMLElement>(FOCUSABLE))
      const first = nodes[0]
      const last = nodes[nodes.length - 1]
      if (!first || !last) return
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first.focus()
      } else if (!surfaceRef.current.contains(document.activeElement)) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  const body = (
    <>
      <Heading as="h2" look="panel" id={titleId}>{title}</Heading>
      {children}
      {footer && <div className="flex justify-end gap-2">{footer}</div>}
    </>
  )

  return createPortal(
    <AnimatePresence>
      {open && (
        <div className="fixed inset-0 z-modal grid place-items-center p-4">
          <motion.div
            {...m.scrim}
            className="absolute inset-0 bg-scrim"
            onClick={onClose}
            aria-hidden="true"
          />
          <motion.div
            ref={surfaceRef}
            {...m.modal}
            role="dialog"
            aria-modal="true"
            aria-labelledby={titleId}
            className="relative w-full max-w-130 max-h-full overflow-y-auto rounded-overlay bg-overlay
              shadow-overlay p-5"
          >
            {onSubmit ? (
              <form
                className="grid gap-4.5"
                onSubmit={(e) => {
                  e.preventDefault()
                  onSubmit()
                }}
              >
                {body}
              </form>
            ) : (
              <div className="grid gap-4.5">{body}</div>
            )}
          </motion.div>
        </div>
      )}
    </AnimatePresence>,
    document.body,
  )
}
