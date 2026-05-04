import type { ReactNode } from 'react'
import { X } from 'lucide-react'
import { IconButton } from '../primitives'

interface PeekPanelProps {
  title: string
  onClose: () => void
  children: ReactNode
  width?: number
}

/** Right-side slide-in panel — full-screen on mobile, side panel on desktop. */
export function PeekPanel({ title, onClose, children, width = 360 }: PeekPanelProps) {
  return (
    <div
      className="fixed inset-0 z-40
        lg:relative lg:inset-auto lg:z-auto lg:border-l lg:border-outline-variant
        flex-shrink-0 flex flex-col bg-surface-container-low
        animate-panel-slide overflow-hidden"
      style={{ ['--peek-w' as string]: `${width}px` }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-outline-variant flex-shrink-0">
        <span className="text-body-strong text-on-surface truncate">{title}</span>
        <IconButton aria-label="Close panel" size="sm" onClick={onClose}>
          <X size={14} />
        </IconButton>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {children}
      </div>
    </div>
  )
}
