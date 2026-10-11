import { useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { motion, useDragControls, type PanInfo } from 'motion/react'
import { Bell, BellOff, Copy, MailOpen, MessageSquareReply, Pen, Pin, PinOff, Plus, Trash2 } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { useModalFocus } from '../../hooks/useModalFocus'
import { useMotionPresets } from '../../lib/motion'
import { previewText } from '../../lib/chat-cache'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Pressable } from '../primitives'
import { copyText } from './copy-message'

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
  /** Pin or unpin; absent where pins are not offered (the thread panel). */
  onPin?: () => void
  /** Quick reactions; absent where reactions are not offered (the thread panel). */
  onReact?: (emoji: string) => void
  onMoreReactions?: () => void
  /** Your own text message: open it for editing. */
  onEdit?: () => void
  /** Someone else's message: move the read position back to it. */
  onMarkUnread?: () => void
  /** Your own message, or any message when you manage the channel. */
  onDelete?: () => void
  onClose: () => void
}

/**
 * Long-press menu of a message on a touch screen (DESIGN.md "Nhấn giữ tin"): a
 * bottom sheet with a row of quick reactions, a preview of the message, and the
 * message's actions as 52px rows. Only actions that exist are listed. Focus
 * rules are `useModalFocus`'s; dragging the handle down closes it.
 */
export function MessageActionSheet({
  message: m, people, me, onReply, onToggleFollow, onPin, onReact, onMoreReactions, onEdit, onMarkUnread, onDelete, onClose,
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
          {onReact && (
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
              {onMoreReactions && (
                <Pressable
                  aria-label="Thêm cảm xúc"
                  onClick={done(onMoreReactions)}
                  className="grid place-items-center w-12 h-12 rounded-full bg-sunk text-ink-muted hover:bg-hover"
                >
                  <Plus size={20} strokeWidth={1.75} aria-hidden="true" />
                </Pressable>
              )}
            </div>
          )}

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
          {onPin && (
            <SheetItem
              icon={m.is_pinned ? <PinOff size={18} strokeWidth={1.75} /> : <Pin size={18} strokeWidth={1.75} />}
              onClick={done(onPin)}
            >
              {m.is_pinned ? 'Bỏ ghim' : 'Ghim'}
            </SheetItem>
          )}
          {onEdit && (
            <SheetItem icon={<Pen size={18} strokeWidth={1.75} />} onClick={done(onEdit)}>Sửa tin</SheetItem>
          )}
          {onMarkUnread && (
            <SheetItem icon={<MailOpen size={18} strokeWidth={1.75} />} onClick={done(onMarkUnread)}>
              Đánh dấu chưa đọc
            </SheetItem>
          )}
          {onDelete && (
            <>
              <div role="separator" className="h-px mx-3.5 my-1 bg-line" />
              <SheetItem icon={<Trash2 size={18} strokeWidth={1.75} />} tone="danger" onClick={done(onDelete)}>
                Xoá tin
              </SheetItem>
            </>
          )}
        </div>
      </motion.div>
    </div>,
    document.body,
  )
}

function SheetItem({
  icon, onClick, children, tone = 'default',
}: { icon: ReactNode; onClick: () => void; children: ReactNode; tone?: 'default' | 'danger' }) {
  return (
    <Pressable
      onClick={onClick}
      className={`flex items-center gap-3.5 w-full min-h-13 px-3.5 rounded-lg text-base font-medium
        hover:bg-hover active:bg-hover transition-colors duration-quick ${tone === 'danger' ? 'text-danger' : 'text-ink'}`}
    >
      <span aria-hidden="true" className={tone === 'danger' ? 'text-danger' : 'text-ink-muted'}>{icon}</span>
      {children}
    </Pressable>
  )
}
