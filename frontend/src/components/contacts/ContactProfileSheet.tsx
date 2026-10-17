import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'

/**
 * A person's profile on a tablet (DESIGN.md "Danh bạ"): a sheet from the right, 380 wide, on a scrim. It is a
 * modal layer like any other: it renders in the body, the page behind it is inert, focus moves in and is
 * kept in, Esc, the scrim and the close button give it up, and focus returns to the row that opened it.
 * Mount inside `AnimatePresence` for the exit.
 */
export function ContactProfileSheet({ label, onClose, children }: { label: string; onClose: () => void; children: ReactNode }) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose })
  // From the right edge over 280ms (expo); a reduced-motion reader gets a fade.
  const slide = m.reduced ? m.panel : {
    initial: { x: '100%' },
    animate: { x: 0, transition: { duration: DURATION.layout, ease: EASE.outExpo } },
    exit: { x: '100%', transition: { duration: DURATION.exit, ease: EASE.in } },
  }
  return createPortal(
    <div className="fixed inset-0 z-modal">
      <motion.div {...m.scrim} className="absolute inset-0 bg-scrim" onClick={onClose} aria-hidden="true" />
      <motion.aside
        ref={surfaceRef}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        {...slide}
        className="absolute inset-y-0 right-0 w-95 max-w-[calc(100%-3rem)] p-3"
      >
        <div className="h-full overflow-y-auto rounded-overlay bg-overlay shadow-overlay">{children}</div>
      </motion.aside>
    </div>,
    document.body,
  )
}
