import { useMemo, type ReactNode } from 'react'
import { motion } from 'motion/react'
import { AtSign, ChevronLeft, CircleAlert, Paperclip, Star, CornerDownRight } from 'lucide-react'
import type { FeedMessage } from '../../api/shortcuts'
import type { Message } from '../../api/messaging'
import type { Conversation } from '../../lib/conversations'
import { cardTime, messageSegments, type Segment } from '../../lib/shortcut-feed'
import { formatTime } from '../../lib/format'
import { feedItemMotion } from '../../lib/feed-motion'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import type { Arrival } from '../../hooks/useArrivals'
import { Avatar, Button, Pressable } from '../primitives'
import { ConversationIcon } from './ConversationIcon'
import { EmptyState } from './EmptyState'

/** Pieces the two shortcut screens share: a message card, the loading skeleton, and the states around a list. */

/** Phones get 44px targets (DESIGN.md §5); the desktop keeps the denser size. */
const TOUCH = 'max-lg:min-h-11'

export function BackBar({ title, onBack }: { title: string; onBack: () => void }) {
  return (
    <div className="flex items-center gap-1 px-2 pt-2 lg:hidden">
      <Pressable
        onClick={onBack}
        className="inline-flex items-center gap-1 min-h-11 pl-1 pr-3 rounded-md text-sm font-semibold text-ink
          hover:bg-hover"
      >
        <ChevronLeft size={20} strokeWidth={1.75} aria-hidden="true" />
        Quay lại<span className="sr-only"> {title}</span>
      </Pressable>
    </div>
  )
}

function Runs({ segments }: { segments: Segment[] }) {
  return (
    <>
      {segments.map((s, i) =>
        s.mention === 'self' ? (
          <span key={i} className="rounded-sm bg-accent-wash px-1 font-semibold text-accent">{s.text}</span>
        ) : s.mention === 'other' ? (
          <span key={i} className="font-semibold text-accent">{s.text}</span>
        ) : (
          <span key={i}>{s.text}</span>
        ),
      )}
    </>
  )
}

const preview = (m: Message, me: string | undefined, max: number): string => {
  const text = messageSegments(m.content, m.content_format, me).map((s) => s.text).join('')
  return text.length > max ? `${text.slice(0, max - 1)}…` : text
}

/**
 * One message of a feed as a link: the conversation it is in, who wrote it and
 * when, the opening of the text with the reader's own @name washed. An unread
 * mention is bold with a dot; a starred card is always plain and carries its
 * star button outside the link, so no button sits inside a link. Inside an
 * `AnimatePresence`, a card that leaves collapses (220ms) rather than vanishing.
 */
export function FeedCard({ item, kind, conversation, me, now, root, href, arrival, onOpen, onUnstar }: {
  item: FeedMessage
  kind: 'mention' | 'star'
  conversation: Conversation | undefined
  me: string | undefined
  now: Date
  /** The root of the thread this reply belongs to, once fetched. */
  root: Message | undefined
  href?: string
  /** A mention that arrived while the list was open: it grows in and is washed in its sender's colour. */
  arrival?: Arrival
  onOpen: (m: FeedMessage) => void
  onUnstar?: (m: FeedMessage) => void
}) {
  const m = useMotionPresets()
  const byOther = arrival?.source === 'other'
  const unread = kind === 'mention' && !!item.unread
  const segments = useMemo(() => messageSegments(item.content, item.content_format, me), [item.content, item.content_format, me])
  const author = item.sender_name || 'Người dùng'
  const reply = !!item.parent_message_id
  const files = item.attachments?.length ?? 0
  const ink = unread || kind === 'star' ? 'text-ink' : 'text-ink-muted'
  const rootLine = root
    ? `Trả lời trong thread của ${root.sender_id === me ? 'bạn' : (root.sender_name || 'người dùng')}${root.deleted ? '' : `: “${preview(root, me, 60)}”`}`
    : 'Trả lời trong thread'

  return (
    <motion.li
      {...feedItemMotion(m.reduced, !!arrival)}
      // Clip the collapse, with room left for the focus ring.
      className="relative list-none overflow-clip [overflow-clip-margin:4px]"
    >
      <a
        href={href ?? '#'}
        data-realtime={arrival?.source}
        style={byOther ? personStyle(arrival!.author ?? item.sender_id) : undefined}
        onClick={(e) => {
          if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
          e.preventDefault()
          onOpen(item)
        }}
        data-unread={unread || undefined}
        className={`grid grid-cols-[32px_minmax(0,1fr)_auto] gap-x-3 gap-y-1.5 items-start no-underline rounded-surface
          px-4 py-3 lg:px-3 hover:bg-raised focus-ring ${ink} ${onUnstar ? 'pr-14 lg:pr-12' : ''} ${byOther ? 'rt-wash' : ''}`}
      >
        <span className="col-span-3 flex items-center gap-2 min-w-0">
          {conversation && <ConversationIcon conversation={conversation} size={24} />}
          <span className="truncate text-small-ui">{conversation?.title}</span>
          {unread && (
            <>
              <span className="w-2 h-2 rounded-full bg-accent shrink-0" aria-hidden="true" />
              <span className="sr-only">Chưa đọc</span>
            </>
          )}
        </span>
        {reply && (
          <span className="col-span-3 flex items-center gap-1.5 min-w-0 text-xs text-ink-muted">
            <CornerDownRight size={14} strokeWidth={1.75} aria-hidden="true" className="shrink-0" />
            <span className="truncate">{rootLine}</span>
          </span>
        )}
        <Avatar name={author} hueKey={item.sender_id} size={32} />
        <span className="col-span-2 grid gap-0.5 min-w-0">
          <span className="flex items-baseline gap-2 min-w-0">
            <span className={`truncate text-sm ${unread ? 'font-semibold' : 'font-medium'}`}>
              {item.sender_id === me ? 'Bạn' : author}
            </span>
            <time className="shrink-0 text-xs text-ink-muted tnum">
              {kind === 'star' ? cardTime(item.created_at, now) : formatTime(item.created_at)}
            </time>
          </span>
          <span className={`line-clamp-3 break-words text-sm leading-[1.55] ${unread ? 'font-semibold' : ''}`}>
            <Runs segments={segments} />
          </span>
          {files > 0 && (
            <span className="inline-flex items-center gap-1 text-xs text-ink-muted">
              <Paperclip size={12} strokeWidth={1.75} aria-hidden="true" />
              {files === 1 ? '1 tệp' : `${files} tệp`}
            </span>
          )}
        </span>
      </a>
      {onUnstar && (
        <Pressable
          aria-pressed="true"
          aria-label={`Gắn sao tin của ${item.sender_id === me ? 'bạn' : author}`}
          title="Bỏ gắn sao"
          onClick={() => onUnstar(item)}
          className="absolute top-2 right-2 inline-flex items-center justify-center w-11 h-11 lg:w-8 lg:h-8 rounded-md
            text-accent hover:bg-hover"
        >
          <Star size={16} strokeWidth={1.75} fill="currentColor" aria-hidden="true" />
        </Pressable>
      )}
    </motion.li>
  )
}

export function FeedSkeleton({ rows }: { rows: number }) {
  return (
    <div className="grid gap-1" aria-hidden="true">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="grid grid-cols-[32px_minmax(0,1fr)] gap-x-3 gap-y-2 px-4 py-3 lg:px-3">
          <span className="skeleton col-span-2 h-4 w-2/5 rounded-sm" />
          <span className="skeleton w-8 h-8 rounded-full" />
          <span className="grid gap-1.5">
            <span className="skeleton h-3.5 rounded-sm" style={{ width: `${30 + ((i * 13) % 25)}%` }} />
            <span className="skeleton h-9 w-4/5 rounded-surface" />
          </span>
        </div>
      ))}
    </div>
  )
}

/** The block under a feed: "Tải thêm", the two skeletons, a failed page, or the closing line. */
export function FeedFooter({ hasMore, loadingMore, moreFailed, onMore, doneText }: {
  hasMore: boolean
  loadingMore: boolean
  moreFailed: boolean
  onMore: () => void
  doneText: string
}) {
  if (loadingMore) return <FeedSkeleton rows={2} />
  if (moreFailed) {
    return (
      <div role="alert" className="flex items-center justify-center gap-3 py-4 text-small text-ink-muted">
        Không tải thêm được.
        <Button variant="soft" size="sm" className={TOUCH} onClick={onMore}>Thử lại</Button>
      </div>
    )
  }
  if (hasMore) {
    return (
      <div className="flex justify-center py-4">
        <Button variant="soft" size="sm" className={TOUCH} onClick={onMore}>Tải thêm</Button>
      </div>
    )
  }
  return <p className="py-4 text-center text-small text-ink-muted">{doneText}</p>
}

export function FeedError({ text, onRetry }: { text: string; onRetry: () => void }) {
  return (
    <EmptyState
      icon={<CircleAlert size={24} strokeWidth={1.75} />}
      text={text}
      action={<Button variant="soft" size="sm" className={TOUCH} onClick={onRetry}>Thử lại</Button>}
    />
  )
}

export function FeedEmpty({ kind, text, homeLabel, onHome }: {
  kind: 'mention' | 'star'
  text: string
  homeLabel: string
  onHome: () => void
}): ReactNode {
  return (
    <EmptyState
      icon={kind === 'mention' ? <AtSign size={24} strokeWidth={1.75} /> : <Star size={24} strokeWidth={1.75} />}
      text={text}
      action={<Button variant="soft" size="sm" className={TOUCH} onClick={onHome}>{homeLabel}</Button>}
    />
  )
}
