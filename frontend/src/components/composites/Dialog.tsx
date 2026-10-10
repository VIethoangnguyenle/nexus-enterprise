import { useId, useRef, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, motion } from 'motion/react'
import { useModalFocus } from '../../hooks/useModalFocus'
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

  useModalFocus({ active: open, surfaceRef, onClose, initialFocusRef })

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
