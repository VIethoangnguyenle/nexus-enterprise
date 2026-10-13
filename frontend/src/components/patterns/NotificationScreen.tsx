import { useId, useRef } from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, motion } from 'motion/react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { PanelContent } from './NotificationPanel'

/**
 * The same panel as a screen of its own on a phone (DESIGN.md §5, Điều hướng
 * di động): a modal layer over the app with Quay lại in place of Đóng. Focus
 * rules are `useModalFocus`'s: trap, Esc, and focus back to what opened it.
 */
export function NotificationScreen({ open, onClose }: { open: boolean; onClose: () => void }) {
  const m = useMotionPresets()
  const titleId = useId()
  const surfaceRef = useRef<HTMLDivElement>(null)
  useModalFocus({ active: open, surfaceRef, onClose })

  return createPortal(
    <AnimatePresence>
      {open && (
        <motion.div
          ref={surfaceRef}
          role="dialog"
          aria-modal="true"
          aria-labelledby={titleId}
          {...m.panel}
          className="fixed inset-0 z-modal lg:hidden grid grid-rows-[auto_1fr] min-h-0 overflow-hidden bg-base text-ink
            pt-[env(safe-area-inset-top,0px)] pb-[env(safe-area-inset-bottom,0px)]"
        >
          <PanelContent titleId={titleId} onClose={onClose} back />
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  )
}
