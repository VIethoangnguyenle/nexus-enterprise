import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Link } from '@tanstack/react-router'
import { motion } from 'motion/react'
import { AtSign, Star, User } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useChatChip } from '../../hooks/useChatChip'
import { useChatOverlay } from '../../hooks/useChatOverlay'
import { useConversations } from '../../hooks/useConversations'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMentionsUnreadCount } from '../../hooks/useShortcuts'
import { useUnreadThreads } from '../../hooks/useUnreadThreads'
import { totalUnread } from '../../lib/conversations'
import { useMotionPresets } from '../../lib/motion'
import { Pressable } from '../primitives'
import { ThreadGlyph } from '../spaces/PhoneGlyphs'
import { CountPill } from './CountPill'

const ITEM = `flex items-center gap-3.5 w-full min-h-12 px-3.5 rounded-full text-left text-16 font-medium text-ink no-underline
  hover:bg-hover active:bg-hover focus-ring transition-colors duration-quick motion-reduce:transition-none`

/**
 * Đi nhanh (DESIGN.md §5, Điều hướng di động): the menu a long press on the Chat slot of the bottom bar
 * opens, above the bar: Trực tiếp, Được nhắc, Thread theo dõi and Đã gắn sao, each with what is new in it.
 * The first two are chips of the Chat list, Thread theo dõi is its full-screen layer, and Đã gắn sao keeps its
 * own screen. Focus rules are `useModalFocus`'s. Render it only while open.
 */
export function ChatQuickJump({ onClose }: { onClose: () => void }) {
  const m = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  useModalFocus({ active: true, surfaceRef, onClose })
  const { workspaceId } = useActiveWorkspace()
  const { directs } = useConversations(workspaceId)
  const mentions = useMentionsUnreadCount(workspaceId).data ?? 0
  const threads = useUnreadThreads().data?.length ?? 0
  const { select } = useChatChip()
  const { openThreads } = useChatOverlay()
  const directUnread = totalUnread(directs)

  const go = (act: () => void) => () => { onClose(); act() }
  const item = (icon: ReactNode, label: string, count: number, unit: string, onClick: () => void) => (
    <Pressable
      role="menuitem"
      aria-label={count > 0 ? `${label}, ${count} ${unit}` : label}
      onClick={go(onClick)}
      className={ITEM}
    >
      <span aria-hidden="true" className="text-ink-muted shrink-0">{icon}</span>
      <span className="flex-1 min-w-0 truncate">{label}</span>
      {count > 0 && <CountPill n={count} />}
    </Pressable>
  )

  return createPortal(
    <div className="fixed inset-0 z-modal lg:hidden">
      <motion.div {...m.scrim} className="absolute inset-0 z-backdrop bg-scrim" onClick={onClose} aria-hidden="true" />
      <motion.div
        ref={surfaceRef}
        {...m.popover}
        role="menu"
        aria-modal="true"
        aria-label="Đi nhanh trong Chat"
        className="absolute left-3.5 bottom-[calc(7.125rem+env(safe-area-inset-bottom,0px))] z-modal grid w-60 p-1.5 rounded-2xl
          bg-overlay shadow-overlay ring-1 ring-inset ring-line"
      >
        {item(<User size={20} strokeWidth={1.75} />, 'Trực tiếp', directUnread, 'chưa đọc', () => select('direct'))}
        {item(<AtSign size={20} strokeWidth={1.75} />, 'Được nhắc', mentions, 'chưa đọc', () => select('mentions'))}
        {item(<ThreadGlyph size={20} />, 'Thread theo dõi', threads, 'có tin mới', () => openThreads('following'))}
        <Link to="/channels/starred" role="menuitem" onClick={onClose} className={ITEM}>
          <span aria-hidden="true" className="text-ink-muted shrink-0"><Star size={20} strokeWidth={1.75} /></span>
          <span className="flex-1 min-w-0 truncate">Đã gắn sao</span>
        </Link>
      </motion.div>
    </div>,
    document.body,
  )
}
