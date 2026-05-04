import { SmilePlus, Reply, Pin, PinOff, MoreHorizontal } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

interface HoverActionBarProps {
  onReply: () => void
  onReact: () => void
  onPin: () => void
  isPinned?: boolean
  onMore?: () => void
}

/** Floating message action bar matching Stitch nexus-chat.html:
 *  absolute -top-3 right-4 bg-surface-container-lowest border border-outline-variant/30
 *  rounded-lg shadow-sm, opacity-0 group-hover:opacity-100.
 *  Buttons: p-1.5 text-on-surface-variant hover:bg-surface-container.
 *  Dividers: border-l border-outline-variant/20 between buttons. */
export function HoverActionBar({ onReply, onReact, onPin, isPinned, onMore }: HoverActionBarProps) {
  const actions: { icon: LucideIcon; label: string; onClick: () => void }[] = [
    { icon: SmilePlus, label: 'React', onClick: onReact },
    { icon: Reply, label: 'Reply', onClick: onReply },
    { icon: isPinned ? PinOff : Pin, label: isPinned ? 'Unpin' : 'Pin', onClick: onPin },
    ...(onMore ? [{ icon: MoreHorizontal, label: 'More', onClick: onMore }] : []),
  ]

  return (
    <div className="absolute -top-3 right-4 flex items-center
      bg-surface-container-lowest border border-outline-variant/30 rounded-lg shadow-sm
      opacity-0 group-hover:opacity-100 transition-opacity z-10">
      {actions.map((a, i) => {
        const Icon = a.icon
        return (
          <button
            key={a.label}
            onClick={a.onClick}
            title={a.label}
            className={`p-1.5 text-on-surface-variant hover:bg-surface-container hover:text-on-surface
              border-none bg-transparent cursor-pointer transition-colors
              ${i === 0 ? 'rounded-l-lg' : ''}
              ${i === actions.length - 1 ? 'rounded-r-lg' : ''}
              ${i > 0 ? 'border-l border-outline-variant/20' : ''}`}
            style={i > 0 ? { borderLeft: '1px solid rgba(195,198,215,0.2)' } : undefined}
          >
            <Icon size={18} />
          </button>
        )
      })}
    </div>
  )
}
