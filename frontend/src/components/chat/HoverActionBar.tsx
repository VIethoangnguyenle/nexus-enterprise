import { SmilePlus, MessageSquareReply, Pin, PinOff, MoreHorizontal } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { IconButton } from '../primitives'

interface HoverActionBarProps {
  onReply?: () => void
  onReact: () => void
  onPin: () => void
  isPinned?: boolean
  onMore?: () => void
}

/**
 * Message actions that float over the top-right corner on hover or keyboard
 * focus. An overlay, so it is the one place in a message with a shadow.
 */
export function HoverActionBar({ onReply, onReact, onPin, isPinned, onMore }: HoverActionBarProps) {
  const actions: { icon: LucideIcon; label: string; onClick: () => void }[] = [
    { icon: SmilePlus, label: 'Bày tỏ cảm xúc', onClick: onReact },
    ...(onReply ? [{ icon: MessageSquareReply, label: 'Trả lời trong chủ đề', onClick: onReply }] : []),
    { icon: isPinned ? PinOff : Pin, label: isPinned ? 'Bỏ ghim' : 'Ghim', onClick: onPin },
    ...(onMore ? [{ icon: MoreHorizontal, label: 'Thêm', onClick: onMore }] : []),
  ]

  return (
    <div
      className="absolute -top-3.5 right-3 z-dropdown flex items-center gap-0.5 p-0.5 rounded-md bg-overlay shadow-overlay
        opacity-0 pointer-events-none group-hover:opacity-100 group-hover:pointer-events-auto
        group-focus-within:opacity-100 group-focus-within:pointer-events-auto
        transition-opacity duration-quick"
    >
      {actions.map((a) => (
        <IconButton key={a.label} size="sm" aria-label={a.label} title={a.label} onClick={a.onClick}>
          <a.icon size={16} strokeWidth={1.75} />
        </IconButton>
      ))}
    </div>
  )
}
