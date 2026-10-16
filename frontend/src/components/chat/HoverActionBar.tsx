import { SmilePlus, MessageSquareReply, Pen, Pin, PinOff, MoreHorizontal } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { IconButton } from '../primitives'

interface HoverActionBarProps {
  onReply?: () => void
  onReact?: () => void
  onPin?: () => void
  isPinned?: boolean
  /** Your own text message: the pencil that opens it for editing, ahead of "Thêm". */
  onEdit?: () => void
  /** Opens the "Thêm" menu hung from the button that was pressed. */
  onMore?: (anchor: HTMLElement) => void
  /** Your own messages sit on the right, so the bar sits over the corner of the bubble. */
  mine?: boolean
}

/**
 * Message actions that float over the top-right corner on hover or keyboard
 * focus (DESIGN.md "Hover actions"). An overlay, so it is the one place in a
 * message with a shadow. Only the actions the place offers are drawn: the
 * thread panel has no "Trả lời trong thread", and a message that is not
 * yours has no pencil. Touch screens have no hover: the long-press sheet
 * carries the same actions.
 */
export function HoverActionBar({ onReply, onReact, onPin, isPinned, onEdit, onMore, mine }: HoverActionBarProps) {
  const actions: { icon: LucideIcon; label: string; onClick: (anchor: HTMLElement) => void }[] = [
    ...(onReact ? [{ icon: SmilePlus, label: 'Bày tỏ cảm xúc', onClick: onReact }] : []),
    ...(onReply ? [{ icon: MessageSquareReply, label: 'Trả lời trong thread', onClick: onReply }] : []),
    ...(onPin ? [{ icon: isPinned ? PinOff : Pin, label: isPinned ? 'Bỏ ghim' : 'Ghim', onClick: onPin }] : []),
    ...(onEdit ? [{ icon: Pen, label: 'Sửa tin', onClick: onEdit }] : []),
    ...(onMore ? [{ icon: MoreHorizontal, label: 'Thêm', onClick: onMore }] : []),
  ]
  if (actions.length === 0) return null

  return (
    <div
      className={`absolute -top-3.5 ${mine ? 'right-3 md:right-13' : 'right-3'} z-dropdown flex items-center gap-0.5 p-0.5 rounded-md
        bg-overlay shadow-overlay opacity-0 pointer-events-none
        group-hover:opacity-100 group-hover:pointer-events-auto
        group-focus-within:opacity-100 group-focus-within:pointer-events-auto
        transition-opacity duration-quick`}
    >
      {actions.map((a) => (
        <IconButton key={a.label} size="sm" aria-label={a.label} title={a.label} onClick={(e) => a.onClick(e.currentTarget)}>
          <a.icon size={16} strokeWidth={1.75} />
        </IconButton>
      ))}
    </div>
  )
}
