import type { ReactNode } from 'react'
import { motion } from 'motion/react'
import { X } from 'lucide-react'
import { useMotionPresets } from '../../lib/motion'
import { Heading, IconButton } from '../primitives'

interface SidePanelProps {
  /** Landmark name, e.g. "Chủ đề", "Thành viên". */
  label: string
  title: string
  sub?: ReactNode
  /** Extra header controls, before the close button. */
  actions?: ReactNode
  /** Object of "Đóng …" for the close button's name. */
  closeLabel: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}

/**
 * Right detail panel (DESIGN.md §5–§7): raised, radius 10, 12px off the
 * edges, 360px. Enters with translateX(16px) scale(.985) over 280ms and leaves
 * in 210ms; under 1280px it floats over the content, under 768px it is a
 * full-screen sheet. Mount inside `AnimatePresence` for the exit.
 */
export function SidePanel({ label, title, sub, actions, closeLabel, onClose, children, footer }: SidePanelProps) {
  const m = useMotionPresets()
  return (
    <motion.aside
      role="complementary"
      aria-label={label}
      {...m.panel}
      className="flex flex-col min-h-0 min-w-0 bg-raised
        max-md:fixed max-md:inset-0 max-md:z-modal
        md:absolute md:right-3 md:inset-y-3 md:w-90 md:rounded-surface md:shadow-overlay md:z-sticky
        xl:static xl:my-3 xl:mr-3 xl:shadow-none xl:shrink-0"
    >
      <div className="flex items-center gap-2 pt-3.5 pr-3 pb-2.5 pl-4">
        <div className="grid min-w-0">
          <Heading as="h2" look="section" className="truncate">{title}</Heading>
          {sub && <span className="text-xs text-ink-muted truncate">{sub}</span>}
        </div>
        <div className="ml-auto flex items-center gap-1 shrink-0">
          {actions}
          <IconButton aria-label={`Đóng ${closeLabel}`} title="Đóng" onClick={onClose}>
            <X size={18} strokeWidth={1.75} />
          </IconButton>
        </div>
      </div>
      <div className="flex-1 min-h-0 overflow-y-auto px-1 pb-2">{children}</div>
      {footer}
    </motion.aside>
  )
}
