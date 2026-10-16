import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'

/**
 * The folder tree of Tài liệu below 1024px, where it has no column of its own
 * (DESIGN.md "Tài liệu: lưới, danh sách, panel xem trước"): a drawer from the
 * left edge over a scrim. It is a modal layer, so focus rules are
 * `useModalFocus`'s: focus moves in, Esc and the scrim close it. Mount inside
 * `AnimatePresence` for the exit.
 */
export function DriveTreeDrawer({ onClose, children }: { onClose: () => void; children: ReactNode }) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose })

  // The drawer slides in from its own edge; reduced motion only fades.
  const slide = m.reduced
    ? m.scrim
    : {
        initial: { x: '-100%' },
        animate: { x: 0, transition: m.sheet.animate.transition },
        exit: { x: '-100%', transition: m.sheet.exit.transition },
      }

  return createPortal(
    <div className="fixed inset-0 z-modal">
      <motion.div {...m.scrim} className="absolute inset-0 z-backdrop bg-scrim" onClick={onClose} aria-hidden="true" />
      <motion.aside
        ref={surfaceRef}
        {...slide}
        role="dialog"
        aria-modal="true"
        aria-label="Cây thư mục"
        className="absolute inset-y-0 left-0 z-modal flex flex-col w-80 max-w-[86vw] bg-base shadow-overlay pt-safe"
      >
        {children}
      </motion.aside>
    </div>,
    document.body,
  )
}
