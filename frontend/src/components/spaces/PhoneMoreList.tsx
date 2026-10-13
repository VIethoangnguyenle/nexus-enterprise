import type { ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { AtSign, Bell, Star } from 'lucide-react'
import { useMentionsUnreadCount } from '../../hooks/useShortcuts'
import { formatCount } from '../../lib/format'
import { Pressable } from '../primitives'
import { ThreadGlyph } from './PhoneGlyphs'
import { PhoneListTitle } from './PhoneListControls'

const ROW = `flex items-center gap-3.5 w-full min-h-15 px-3.5 rounded-full text-left text-md font-medium text-ink
  no-underline focus-ring hover:bg-hover active:bg-hover transition-colors duration-quick motion-reduce:transition-none`

function Row({ icon, label, hint, count }: {
  icon: ReactNode
  label: string
  hint?: string
  count?: number
}) {
  return (
    <>
      <span aria-hidden="true" className="text-ink-muted shrink-0">{icon}</span>
      <span className="flex-1 min-w-0 truncate">{label}</span>
      {hint && <span className="text-xs font-medium text-ink-muted shrink-0">{hint}</span>}
      {!!count && count > 0 && (
        <span
          className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent
            text-2xs font-semibold tnum shrink-0"
        >
          {formatCount(count)}
        </span>
      )}
    </>
  )
}

/**
 * Thêm on a phone (DESIGN.md "Chat trên điện thoại", mockup §4): what is left of
 * Chat's shortcuts. The other modules of Nexus are in the ☰ menu, not here.
 */
export function PhoneMoreList({ workspaceId, threadsWithNews, onOpenFollowedThreads }: {
  workspaceId: string
  /** Followed threads with unread replies. */
  threadsWithNews: number
  onOpenFollowedThreads: () => void
}) {
  const mentionsUnread = useMentionsUnreadCount(workspaceId).data ?? 0

  return (
    <div className="grid">
      <PhoneListTitle title="Thêm" />
      <nav aria-label="Lối tắt của Chat" className="grid gap-0.5 px-2">
        <Link
          to="/channels/mentions"
          aria-label={mentionsUnread > 0 ? `Được nhắc tới, ${mentionsUnread} tin chưa đọc` : 'Được nhắc tới'}
          className={ROW}
        >
          <Row icon={<AtSign size={20} strokeWidth={1.75} />} label="Được nhắc tới" count={mentionsUnread} />
        </Link>
        <Link to="/channels/starred" className={ROW}>
          <Row icon={<Star size={20} strokeWidth={1.75} />} label="Tin đã gắn sao" />
        </Link>
        <Pressable
          onClick={onOpenFollowedThreads}
          aria-haspopup="dialog"
          className={ROW}
        >
          <Row
            icon={<ThreadGlyph size={20} />}
            label="Thread đang theo dõi"
            hint={threadsWithNews > 0 ? `${threadsWithNews} có tin mới` : undefined}
          />
        </Pressable>
        <hr className="border-0 h-px bg-line mx-3.5 my-1.5" />
        <Link to="/settings" search={{ tab: 'thong-bao' }} className={ROW}>
          <Row
            icon={<Bell size={20} strokeWidth={1.75} />}
            label="Thông báo của Chat"
            hint="Mức mặc định, giờ yên lặng"
          />
        </Link>
      </nav>
    </div>
  )
}
