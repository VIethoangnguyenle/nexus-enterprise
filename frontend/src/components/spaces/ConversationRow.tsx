import { Link } from '@tanstack/react-router'
import { personStyle } from '../../lib/person-hue'
import { formatCount, formatListTime } from '../../lib/format'
import type { Conversation } from '../../lib/conversations'
import type { Arrival } from '../../hooks/useArrivals'
import { ConversationIcon } from './ConversationIcon'
import { MentionPill, MutedMark } from './ConversationBadges'

/**
 * A conversation as Trang chủ and the phone's Tin nhắn tab list it: avatar,
 * title, preview and time, unread in bold with a count. A message that just
 * arrived from someone else washes the row in that person's hue. A muted
 * conversation is drawn quiet (no bold, no count, no wash): a bell-off, and an
 * "@" when someone named the reader.
 */
export function ConversationRow({ conversation: c, arrival, online }: {
  conversation: Conversation
  arrival?: Arrival
  online: boolean
}) {
  const muted = c.level === 'off'
  const unread = !muted && c.unread > 0
  const mentions = muted ? (c.mentions ?? 0) : 0
  const byOther = !muted && arrival?.source === 'other'
  const metaId = `conv-meta-${c.id}`
  const fallback = c.kind === 'space'
    ? `${c.memberCount} thành viên`
    : c.partner?.role || 'Tin nhắn trực tiếp'
  const preview = c.preview ? `${c.previewAuthor ? `${c.previewAuthor}: ` : ''}${c.preview}` : fallback

  return (
    <Link
      to="/channels/$channelId"
      params={{ channelId: c.id }}
      aria-label={muted ? `${c.title}, đã tắt thông báo` : c.title}
      aria-describedby={metaId}
      data-unread={unread || undefined}
      data-muted={muted || undefined}
      // Re-keying on the change restarts the wash when the same row is hit twice.
      key={byOther ? `rt-${arrival!.at}` : 'still'}
      style={byOther ? personStyle(arrival!.author ?? c.id) : undefined}
      className={`grid grid-cols-[40px_minmax(0,1fr)_auto] gap-3 items-center p-2.5 rounded-surface no-underline
        focus-ring transition-colors duration-quick hover:bg-raised ${muted ? 'text-ink-muted' : 'text-ink'} ${byOther ? 'rt-wash' : ''}`}
    >
      <span className={muted ? 'opacity-[0.72] inline-flex' : 'inline-flex'}>
        <ConversationIcon conversation={c} size={40} online={online} halo={byOther} />
      </span>
      <span className="grid gap-0.5 min-w-0">
        <span className={`flex items-center gap-1.5 min-w-0 max-md:text-16 md:text-md ${unread ? 'font-semibold' : 'font-medium'}`}>
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
        {muted && (
          <span className="inline-flex items-center gap-1.5">
            {mentions > 0 && <MentionPill n={mentions} />}
            <MutedMark />
          </span>
        )}
      </span>
    </Link>
  )
}
