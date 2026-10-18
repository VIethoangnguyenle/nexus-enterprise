import { useCallback } from 'react'
import { Bell, BellOff, CheckCheck, Mail, MoreHorizontal, Star, StarOff } from 'lucide-react'
import { messagingApi } from '../../api/messaging'
import { keys } from '../../hooks/keys'
import { changeNotificationLevel } from '../../hooks/useNotificationLevel'
import { useStarConversation } from '../../hooks/useShortcuts'
import type { Conversation } from '../../lib/conversations'
import { queryClient } from '../../lib/query-client'
import { toast } from '../primitives/Toast'
import type { SwipeAction } from '../patterns/SwipeRow'
import { rowTargetOf, type SectionOverlays } from './SectionOverlays'

const ICON = { size: 22, strokeWidth: 1.75 } as const

function refreshUnread() {
  void queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsAll() })
  void queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsUnreadAll() })
  return queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
}

/**
 * Đã đọc / Chưa đọc of a conversation row, with Hoàn tác (DESIGN.md §6 "Vuốt": a full swipe acts at once, so it
 * must be undoable). Read moves the read position to the newest message; unread puts it just before that message,
 * so the conversation shows one unread. Undoing a read therefore leaves one message unread, not the earlier count:
 * the server keeps a position, not a history of positions.
 */
export async function setConversationRead(input: {
  channelId: string
  name: string
  /** The newest main-stream message, the one the position moves around. */
  messageId: string
  read: boolean
  /** No toast: this call is itself an undo. */
  quiet?: boolean
}): Promise<void> {
  const { channelId, name, messageId, read, quiet } = input
  try {
    await (read ? messagingApi.markRead(channelId, messageId) : messagingApi.markMessageUnread(messageId))
    if (!quiet) {
      toast(read ? `Đã đánh dấu đã đọc “${name}”` : `Đã đánh dấu chưa đọc “${name}”`, {
        action: { label: 'Hoàn tác', onClick: () => void setConversationRead({ ...input, read: !read, quiet: true }) },
      })
    }
  } catch {
    toast.error(read ? `Chưa đánh dấu đã đọc được “${name}”. Kiểm tra kết nối rồi thử lại.` : `Chưa đánh dấu chưa đọc được “${name}”. Kiểm tra kết nối rồi thử lại.`, {
      action: { label: 'Thử lại', onClick: () => void setConversationRead({ ...input, quiet: true }) },
    })
  }
  await refreshUnread()
}

/**
 * The swipe buttons of a Chat row (DESIGN.md §6 "Vuốt", mockup §7): left reveals Tắt thông báo · Gắn sao · Thêm,
 * Thêm being the row's long-press sheet (with Chuyển tới mục); right reveals Đã đọc or Chưa đọc, which a full swipe
 * runs at once. Each one is also in the long-press sheet, so the gesture is never the only way.
 */
export function useChatRowSwipe(overlays: SectionOverlays) {
  const star = useStarConversation()
  return useCallback((c: Conversation): { start: SwipeAction[]; end: SwipeAction[] } => {
    const muted = c.level === 'off'
    const end: SwipeAction[] = [
      {
        key: 'mute',
        label: muted ? 'Bật thông báo' : 'Tắt thông báo',
        icon: muted ? <Bell {...ICON} /> : <BellOff {...ICON} />,
        tone: 'muted',
        onAction: () => void changeNotificationLevel({ channelId: c.id, name: c.title, level: muted ? 'main' : 'off' }),
      },
      {
        key: 'star',
        label: c.starred ? 'Bỏ gắn sao' : 'Gắn sao',
        icon: c.starred ? <StarOff {...ICON} /> : <Star {...ICON} />,
        tone: 'warning',
        onAction: () => void star({ channelId: c.id, name: c.title, starred: !c.starred }),
      },
      {
        key: 'more',
        label: 'Thêm',
        icon: <MoreHorizontal {...ICON} />,
        tone: 'ink',
        onAction: () => overlays.openRowMenu(rowTargetOf(c), null),
      },
    ]
    const newest = c.main?.id
    const unread = c.unread > 0
    const start: SwipeAction[] = newest
      ? [{
          key: 'read',
          label: unread ? 'Đã đọc' : 'Chưa đọc',
          icon: unread ? <CheckCheck {...ICON} /> : <Mail {...ICON} />,
          tone: 'accent',
          onAction: () => void setConversationRead({ channelId: c.id, name: c.title, messageId: newest, read: unread }),
        }]
      : []
    return { start, end }
  }, [overlays, star])
}
