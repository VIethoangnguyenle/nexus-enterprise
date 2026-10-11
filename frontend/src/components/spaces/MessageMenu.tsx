import type { RefObject } from 'react'
import { Bell, BellOff, Copy, MailOpen, Pen, Trash2 } from 'lucide-react'
import type { Message } from '../../api/messaging'
import type { MessageLifecycle } from '../../hooks/useMessageActions'
import { MenuItem, MenuSeparator, Popover } from '../primitives'
import { copyText } from './copy-message'

export interface MessageMenuTarget { message: Message; up: boolean }

interface MessageMenuProps {
  target: MessageMenuTarget | null
  anchorRef: RefObject<HTMLElement | null>
  onClose: () => void
  /** Editing, deleting and un-reading; without it the menu offers only what needs no server. */
  lifecycle?: MessageLifecycle
  /** Follow or unfollow the thread of a root message; absent inside the thread panel. */
  onFollow?: (m: Message, follow: boolean) => void
}

/**
 * The "Thêm" menu of a message (DESIGN.md "Hover actions"), in the order the
 * design gives: follow the thread, copy the text, mark unread (someone else's
 * message), edit (yours), a divider, delete. An action the reader cannot take
 * is not listed.
 */
export function MessageMenu({ target, anchorRef, onClose, lifecycle, onFollow }: MessageMenuProps) {
  const m = target?.message
  const can = m && lifecycle ? lifecycle.abilities(m) : undefined
  return (
    <Popover
      open={!!target}
      onClose={onClose}
      anchorRef={anchorRef}
      role="menu"
      label="Thêm thao tác cho tin"
      placement={target?.up ? 'top-start' : 'bottom-end'}
    >
      {m && onFollow && (
        <MenuItem
          icon={m.following ? <BellOff size={16} strokeWidth={1.75} /> : <Bell size={16} strokeWidth={1.75} />}
          onClick={() => onFollow(m, !m.following)}
        >
          {m.following ? 'Bỏ theo dõi thread' : 'Theo dõi thread'}
        </MenuItem>
      )}
      {m && !m.deleted && (
        <MenuItem icon={<Copy size={16} strokeWidth={1.75} />} onClick={() => copyText(m.content)}>
          Sao chép văn bản
        </MenuItem>
      )}
      {m && can?.markUnread && (
        <MenuItem icon={<MailOpen size={16} strokeWidth={1.75} />} onClick={() => lifecycle!.markUnread(m)}>
          Đánh dấu chưa đọc
        </MenuItem>
      )}
      {m && can?.edit && (
        <MenuItem icon={<Pen size={16} strokeWidth={1.75} />} onClick={() => lifecycle!.startEdit(m)}>
          Sửa tin
        </MenuItem>
      )}
      {m && can?.remove && (
        <>
          <MenuSeparator />
          <MenuItem tone="danger" icon={<Trash2 size={16} strokeWidth={1.75} />} onClick={() => lifecycle!.askDelete(m)}>
            Xoá tin
          </MenuItem>
        </>
      )}
    </Popover>
  )
}
