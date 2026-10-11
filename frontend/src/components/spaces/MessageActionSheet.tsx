import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion, useDragControls, type PanInfo } from 'motion/react'
import { Bell, BellOff, Copy, MessageSquareReply, Pin, PinOff, Plus } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { previewText } from '../../lib/chat-cache'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Pressable, toast } from '../primitives'

/**
 * Copies a message as plain text, line breaks kept: text selection is off on
 * touch screens (the long press opens this sheet), so this is how a phone copies.
 */
function copyText(content: string) {
  const doc = new DOMParser().parseFromString(content.replace(/<br\s*\/?>/gi, '\n').replace(/<\/(p|div|li)>/gi, '$&\n'), 'text/html')
  const text = (doc.body.textContent ?? '').replace(/\n{3,}/g, '\n\n').trim()
  navigator.clipboard?.writeText(text).then(
    () => toast('Đã sao chép'),
    () => toast('Không sao chép được'),
  )
}

/** The six quick reactions of the sheet; the sixth button opens the full picker. */
export const QUICK_REACTIONS = ['👍', '❤️', '😂', '🙏', '✅'] as const

const CLOSE_DISTANCE = 0.25
const CLOSE_VELOCITY = 500

interface MessageActionSheetProps {
  message: Message
  people: PeopleDirectory
  me: string | undefined
  /** Open the thread (a message in the main stream; not inside the thread panel). */
  onReply?: () => void
  /** Follow or unfollow the thread of this message. */
  onToggleFollow?: () => void
  onPin: () => void
  onReact: (emoji: string) => void
  onMoreReactions: () => void
  onClose: () => void
}

/**
 * Long-press menu of a message on a touch screen (DESIGN.md "Nhấn giữ tin"): a
 * bottom sheet with a row of quick reactions, a preview of the message, and the
 * message's actions as 52px rows. Only actions that exist are listed. Focus
 * rules are `useModalFocus`'s; dragging the handle down closes it.
 */
export function MessageActionSheet({
  message: m, people, me, onReply, onToggleFollow, onPin, onReact, onMoreReactions, onClose,
}: MessageActionSheetProps) {
  const motionPresets = useMotionPresets()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const dragControls = useDragControls()
  useModalFocus({ active: true, surfaceRef, onClose })

  const author = m.sender_id === me ? 'Bạn' : displayName(people, m.sender_id, m.sender_name)
  const following = !!m.following
  const done = (fn: () => void) => () => {
    onClose()
    fn()
  }

  const onDragEnd = (_: unknown, info: PanInfo) => {
    const height = surfaceRef.current?.offsetHeight || 1
    if (info.offset.y > height * CLOSE_DISTANCE || info.velocity.y > CLOSE_VELOCITY) onClose()
  }

  return createPortal(
    <div className="fixed inset-0 z-modal">
      <motion.div {...motionPresets.scrim} className="absolute inset-0 z-backdrop bg-scrim" onClick={onClose} aria-hidden="true" />
      <motion.div
        ref={surfaceRef}
        {...motionPresets.sheet}
        role="dialog"
        aria-modal="true"
        aria-label="Thao tác với tin"
        drag="y"
        dragControls={dragControls}
        dragListener={false}
        dragConstraints={{ top: 0, bottom: 0 }}
        dragElastic={{ top: 0, bottom: 0.5 }}
        onDragEnd={onDragEnd}
        className="absolute inset-x-0 bottom-0 z-modal flex flex-col max-h-sheet overflow-hidden rounded-t-overlay
          bg-overlay shadow-overlay px-2 pb-sheet"
      >
        <div onPointerDown={(e) => dragControls.start(e)} className="grid place-items-center h-5 pt-2 shrink-0 touch-none">
          <span aria-hidden="true" className="w-9 h-1 rounded-full bg-line" />
        </div>
        <div className="min-h-0 overflow-y-auto grid gap-1">
          <div role="group" aria-label="Cảm xúc nhanh" className="flex justify-between px-2.5 pt-1 pb-2">
            {QUICK_REACTIONS.map((emoji) => (
              <Pressable
                key={emoji}
                aria-label={emoji}
                onClick={done(() => onReact(emoji))}
                className="grid place-items-center w-12 h-12 rounded-full bg-sunk text-xl hover:bg-hover"
              >
                {emoji}
              </Pressable>
            ))}
            <Pressable
              aria-label="Thêm cảm xúc"
              onClick={done(onMoreReactions)}
              className="grid place-items-center w-12 h-12 rounded-full bg-sunk text-ink-muted hover:bg-hover"
            >
              <Plus size={20} strokeWidth={1.75} aria-hidden="true" />
            </Pressable>
          </div>

          <div className="mx-3.5 mb-1.5 px-3 py-2.5 rounded-lg bg-sunk text-sm">
            <span className="block mb-0.5 text-xs font-semibold text-ink-muted">{author}</span>
            <span className="line-clamp-2 wrap-anywhere">{previewText(m.content)}</span>
          </div>

          {onReply && (
            <SheetItem icon={<MessageSquareReply size={18} strokeWidth={1.75} />} onClick={done(onReply)}>
              Trả lời trong thread
            </SheetItem>
          )}
          {onToggleFollow && (
            <SheetItem
              icon={following ? <BellOff size={18} strokeWidth={1.75} /> : <Bell size={18} strokeWidth={1.75} />}
              onClick={done(onToggleFollow)}
            >
              {following ? 'Bỏ theo dõi thread' : 'Theo dõi thread'}
            </SheetItem>
          )}
          <SheetItem icon={<Copy size={18} strokeWidth={1.75} />} onClick={done(() => copyText(m.content))}>
            Sao chép
          </SheetItem>
          <SheetItem
            icon={m.is_pinned ? <PinOff size={18} strokeWidth={1.75} /> : <Pin size={18} strokeWidth={1.75} />}
            onClick={done(onPin)}
          >
            {m.is_pinned ? 'Bỏ ghim' : 'Ghim'}
          </SheetItem>
        </div>
      </motion.div>
    </div>,
    document.body,
  )
}

function SheetItem({ icon, onClick, children }: { icon: ReactNode; onClick: () => void; children: ReactNode }) {
  return (
    <Pressable
      onClick={onClick}
      className="flex items-center gap-3.5 w-full min-h-13 px-3.5 rounded-lg text-base font-medium text-ink
        hover:bg-hover active:bg-hover transition-colors duration-quick"
    >
      <span aria-hidden="true" className="text-ink-muted">{icon}</span>
      {children}
    </Pressable>
  )
}
