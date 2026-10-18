import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion } from 'motion/react'
import { Check } from 'lucide-react'
import { useDragToClose } from '../../hooks/useDragToClose'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { Pressable } from '../primitives'

/**
 * Bottom sheet of the phone navigator (design/mockups/chat-mobile.html, "sheet"):
 * the menus of a conversation, of a section, and the section-name form, where
 * the desktop has popovers. Focus rules are `useModalFocus`'s; Esc, the scrim
 * and dragging it down close it, from the handle or from anywhere on it whose
 * list is scrolled to the top (`useDragToClose`).
 */
export function NavSheet({ label, onClose, roomy = false, children }: {
  label: string
  onClose: () => void
  /** A sheet that holds a list with a search box (the filter lists of Chat's results): 24px corners and a 32px handle. */
  roomy?: boolean
  children: ReactNode
}) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose })
  const y = useDragToClose({ ref: surfaceRef, reduced: m.reduced, onClose })

  return createPortal(
    <div className="fixed inset-0 z-modal">
      <motion.div {...m.scrim} className="absolute inset-0 z-backdrop bg-scrim" onClick={onClose} aria-hidden="true" />
      <motion.div
        ref={surfaceRef}
        {...m.sheet}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        style={{ y }}
        className={`absolute inset-x-0 bottom-0 z-modal flex flex-col max-h-sheet overflow-hidden
          bg-overlay shadow-overlay px-2 pb-sheet ${roomy ? 'rounded-t-3xl' : 'rounded-t-overlay'}`}
      >
        <div className="grid place-items-center h-5 pt-2 shrink-0">
          <span aria-hidden="true" className={`h-1 rounded-full bg-line ${roomy ? 'w-8' : 'w-9'}`} />
        </div>
        <div className="min-h-0 overflow-y-auto grid gap-1">{children}</div>
      </motion.div>
    </div>,
    document.body,
  )
}

/** A 52px row of a sheet (chat-mobile.html `.sheet .item`). */
export function SheetItem({ icon, onClick, tone = 'default', checked, disabled, children }: {
  icon?: ReactNode
  onClick: () => void
  tone?: 'default' | 'danger'
  /** Set on a radio row: this one is the current choice. */
  checked?: boolean
  disabled?: boolean
  children: ReactNode
}) {
  const radio = checked !== undefined
  return (
    <Pressable
      role={radio ? 'radio' : undefined}
      aria-checked={radio ? checked : undefined}
      disabled={disabled}
      onClick={onClick}
      className={`flex items-center gap-3.5 w-full min-h-13 px-3.5 rounded-lg text-16 font-medium
        hover:bg-hover active:bg-hover transition-colors duration-quick ${tone === 'danger' ? 'text-danger' : 'text-ink'}`}
    >
      {icon && <span aria-hidden="true" className={tone === 'danger' ? '' : 'text-ink-muted'}>{icon}</span>}
      <span className="flex-1 min-w-0 truncate">{children}</span>
      {checked && <Check size={18} strokeWidth={1.75} aria-hidden="true" className="text-accent shrink-0" />}
    </Pressable>
  )
}

export function SheetSeparator() {
  return <hr className="border-0 h-px bg-line mx-3.5 my-1" />
}

/** Small heading above a group of rows ("Chuyển tới mục"). */
export function SheetLabel({ children }: { children: ReactNode }) {
  return <div className="px-3.5 pt-1.5 pb-1 text-xs font-semibold text-ink-muted">{children}</div>
}
