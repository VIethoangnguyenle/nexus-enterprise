import { Fragment } from 'react'
import { Link } from '@tanstack/react-router'
import { personStyle } from '../../lib/person-hue'
import { formatCount, formatFeedTime } from '../../lib/format'
import { splitMentions } from '../../lib/feed-mentions'
import type { Conversation, FeedLine } from '../../lib/conversations'
import type { Arrival } from '../../hooks/useArrivals'
import { ConversationIcon } from './ConversationIcon'
import { MentionPill } from './ConversationBadges'
import { ThreadGlyph } from './PhoneGlyphs'

const PILL = `inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent
  text-2xs font-semibold tnum shrink-0`

/** "Người gửi: đoạn đầu", with the people it names drawn as chips. */
function Line({ line, strong }: { line: FeedLine; strong: boolean }) {
  return (
    <span className={strong ? 'font-semibold text-ink' : undefined}>
      {line.author}:{' '}
      {splitMentions(line.text, line.mentionNames).map((part, i) =>
        part.mention ? (
          <span
            key={i}
            className="px-1.5 py-px rounded-full bg-accent-wash font-semibold text-ink whitespace-nowrap first-letter:text-accent"
          >
            {part.text}
          </span>
        ) : (
          <Fragment key={i}>{part.text}</Fragment>
        ),
      )}
    </span>
  )
}

/**
 * A conversation as a Trang chủ or Trực tiếp row on a phone (DESIGN.md "Chat
 * trên điện thoại"): avatar, name and time, "Người gửi: đoạn đầu" of the newest
 * message, and below it, when a thread reply is newer, an indented line with its
 * own link that opens that thread. The row is a stretched link, so the thread
 * line is the one place that opens something else.
 *
 * Unread is bold with a count; a muted conversation is faded, with an "@" only
 * when someone named the reader. A message that just arrived from someone else
 * washes the row in that person's hue.
 */
export function FeedRow({ conversation: c, arrival, online }: {
  conversation: Conversation
  arrival?: Arrival
  online: boolean
}) {
  const muted = c.level === 'off'
  const unread = !muted && c.unread > 0
  const mentions = muted ? (c.mentions ?? 0) : 0
  const byOther = !muted && arrival?.source === 'other'
  const { main, reply } = c
  const metaId = `feed-meta-${c.id}`
  const fallback = c.kind === 'space' ? `${c.memberCount} thành viên` : c.partner?.role || 'Tin nhắn trực tiếp'

  // What ends a line: the count, or "@" when the newest message names the reader; a muted row has no count and only the "@".
  const mark = unread
    ? (reply?.mentionsMe
        ? <span className={PILL} aria-label="nhắc tên bạn">@</span>
        : <span className={PILL} aria-label={`${c.unread} tin chưa đọc`}>{formatCount(c.unread)}</span>)
    : mentions > 0 && <MentionPill n={mentions} />

  return (
    <div
      // Re-keying on the change restarts the wash when the same row is hit twice.
      key={byOther ? `rt-${arrival!.at}` : 'still'}
      style={byOther ? personStyle(arrival!.author ?? c.id) : undefined}
      data-unread={unread || undefined}
      data-muted={muted || undefined}
      className={`relative grid grid-cols-[44px_minmax(0,1fr)] gap-3.5 px-4 py-2.5 active:bg-hover
        transition-colors duration-quick motion-reduce:transition-none ${muted ? 'opacity-[0.66]' : ''} ${byOther ? 'rt-wash' : ''}`}
    >
      <span className="relative mt-0.5 w-11 h-11">
        <ConversationIcon conversation={c} size={44} online={online} halo={byOther} />
        {reply && (
          <span
            aria-hidden="true"
            className="absolute -right-1.5 -bottom-1.5 grid place-items-center w-5.5 h-5.5 rounded-full bg-accent-wash
              text-accent ring-2 ring-base"
          >
            <ThreadGlyph size={12} />
          </span>
        )}
      </span>

      <div className="grid gap-px min-w-0">
        <div className="flex items-baseline gap-2">
          <Link
            to="/channels/$channelId"
            params={{ channelId: c.id }}
            aria-label={muted ? `${c.title}, đã tắt thông báo` : c.title}
            aria-describedby={metaId}
            // The pseudo-element stretches this link over the whole row.
            className={`flex-1 min-w-0 flex items-baseline gap-1.5 text-base no-underline text-ink focus-ring
              after:absolute after:inset-0 ${unread ? 'font-bold' : 'font-medium'}`}
          >
            <span className="truncate">{c.title}</span>
            {byOther && <span className="rt-tag shrink-0 text-xs font-semibold">vừa nhắn</span>}
          </Link>
          <time
            className={`shrink-0 text-xs tnum ${unread ? 'font-bold text-ink' : 'text-ink-muted'}`}
            dateTime={c.lastActivity ? new Date(c.lastActivity).toISOString() : undefined}
          >
            {formatFeedTime(c.lastActivity)}
          </time>
        </div>

        <div id={metaId} className="grid gap-1 text-sm text-ink-muted">
          {main ? (
            <span className="flex items-start gap-2">
              <span className={`flex-1 min-w-0 wrap-anywhere ${reply ? 'line-clamp-1' : 'line-clamp-2'}`}>
                <Line line={main} strong={unread && !reply} />
              </span>
              {!reply && mark}
            </span>
          ) : !reply && (
            <span className="flex items-start gap-2">
              <span className="flex-1 min-w-0 truncate">{fallback}</span>
              {mark}
            </span>
          )}
          {reply && (
            <Link
              to="/channels/$channelId"
              params={{ channelId: c.id }}
              search={{ thread: reply.parentId }}
              aria-label={`Mở thread: ${reply.author}: ${reply.text}`}
              className="relative z-10 grid grid-cols-[22px_minmax(0,1fr)_auto] gap-1 items-start text-ink-muted
                no-underline focus-ring rounded-md"
            >
              <span aria-hidden="true" className="w-3 h-3 mt-0.5 ml-1 border-l-[1.5px] border-b-[1.5px] border-line rounded-bl-lg" />
              <span className="line-clamp-2 wrap-anywhere leading-[1.55]">
                <Line line={reply} strong={unread} />
              </span>
              {mark}
            </Link>
          )}
        </div>
      </div>
    </div>
  )
}
