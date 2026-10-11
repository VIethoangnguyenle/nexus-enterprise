import { Link } from '@tanstack/react-router'
import { personStyle } from '../../lib/person-hue'
import { formatCount, formatListTime } from '../../lib/format'
import type { Conversation } from '../../lib/conversations'
import type { Arrival } from '../../hooks/useArrivals'
import { ConversationIcon } from './ConversationIcon'

/**
 * A conversation as Trang chủ and the phone's Tin nhắn tab list it: avatar,
 * title, preview and time, unread in bold with a count. A message that just
 * arrived from someone else washes the row in that person's hue.
 */
export function ConversationRow({ conversation: c, arrival, online }: {
  conversation: Conversation
  arrival?: Arrival
  online: boolean
}) {
  const unread = c.unread > 0
  const byOther = arrival?.source === 'other'
  const metaId = `conv-meta-${c.id}`
  const fallback = c.kind === 'space'
    ? `${c.memberCount} thành viên`
    : c.partner?.role || 'Tin nhắn trực tiếp'
  const preview = c.preview ? `${c.previewAuthor ? `${c.previewAuthor}: ` : ''}${c.preview}` : fallback

  return (
    <Link
      to="/channels/$channelId"
      params={{ channelId: c.id }}
      aria-label={c.title}
      aria-describedby={metaId}
      data-unread={unread || undefined}
      // Re-keying on the change restarts the wash when the same row is hit twice.
      key={byOther ? `rt-${arrival!.at}` : 'still'}
      style={byOther ? personStyle(arrival!.author ?? c.id) : undefined}
      className={`grid grid-cols-[40px_minmax(0,1fr)_auto] gap-3 items-center p-2.5 rounded-surface no-underline
        text-ink focus-ring transition-colors duration-quick hover:bg-raised ${byOther ? 'rt-wash' : ''}`}
    >
      <ConversationIcon conversation={c} size={40} online={online} halo={byOther} />
      <span className="grid gap-0.5 min-w-0">
        <span className={`flex items-center gap-1.5 min-w-0 max-md:text-base md:text-md ${unread ? 'font-semibold' : 'font-medium'}`}>
          <span className="truncate">{c.title}</span>
          {byOther && <span className="rt-tag shrink-0 text-xs font-semibold">vừa nhắn</span>}
        </span>
        <span className={`truncate text-sm ${unread ? 'font-semibold text-ink' : 'text-ink-muted'}`}>{preview}</span>
      </span>
      <span id={metaId} className="grid justify-items-end gap-1 text-xs text-ink-muted tnum">
        <span>{c.lastActivity ? formatListTime(c.lastActivity / 1000) : ''}</span>
        {unread && (
          <span
            className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
              text-on-accent text-2xs font-semibold"
            aria-label={`${c.unread} tin chưa đọc`}
          >
            {formatCount(c.unread)}
          </span>
        )}
      </span>
    </Link>
  )
}
