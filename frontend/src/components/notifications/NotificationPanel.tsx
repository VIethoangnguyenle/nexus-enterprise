import { useEffect, useRef, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, motion } from 'motion/react'
import { X } from 'lucide-react'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { Heading, IconButton } from '../primitives'
import { NotificationsView } from './NotificationsView'

interface NotificationPanelProps {
  open: boolean
  onClose: () => void
  /** The sidebar row that opened it: clicking it is not "outside", and focus returns to it. */
  anchorRef: RefObject<HTMLElement | null>
  /** The sidebar is the 64px rail rather than the full 232px. */
  rail: boolean
}

/**
 * The desktop notifications panel (DESIGN.md §6): 380px beside the sidebar, the
 * height of the frame, `--color-overlay`, no scrim. It grows out of the sidebar
 * (translateX(-16px)). It follows the shared modal focus rules (`useModalFocus`):
 * Esc closes only the topmost layer, focus returns to the sidebar row; a click
 * anywhere outside it also closes it.
 */
export function NotificationPanel({ open, onClose, anchorRef, rail }: NotificationPanelProps) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const closeRef = useRef<HTMLButtonElement>(null)
  useModalFocus({ active: open, surfaceRef, onClose, initialFocusRef: closeRef })

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node
      if (surfaceRef.current?.contains(t) || anchorRef.current?.contains(t)) return
      // A dialog or a toast above the panel is not "outside" it.
      if (t instanceof Element && t.closest('[aria-modal="true"], [role="status"], [role="alert"]')) return
      onClose()
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open, onClose, anchorRef])

  return createPortal(
    <AnimatePresence>
      {open && (
        <motion.div
          ref={surfaceRef}
          {...m.panelLeft}
          role="dialog"
          aria-modal="true"
          aria-labelledby="notification-panel-title"
          style={{ left: rail ? '4.75rem' : '15.25rem' }}
          className="fixed top-3 bottom-3 z-dropdown hidden lg:flex flex-col w-95 max-w-[calc(100vw-1.5rem)] pt-4
            rounded-overlay bg-overlay shadow-overlay overflow-hidden"
        >
          <div className="flex items-center justify-between gap-2 px-4 pb-2 shrink-0">
            <Heading as="h2" look="panel" id="notification-panel-title">Thông báo</Heading>
            <IconButton ref={closeRef} aria-label="Đóng thông báo" onClick={onClose}>
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          </div>
          <NotificationsView onNavigated={onClose} />
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  )
}
