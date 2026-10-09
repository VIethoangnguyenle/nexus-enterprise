import { Fragment, useEffect, useLayoutEffect, useMemo, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { AnimatePresence, motion } from 'motion/react'
import { MessageSquareReply, MessagesSquare, ListChecks } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { threadQueryOptions } from '../../hooks/useMessaging'
import { useArrivals, type Arrival } from '../../hooks/useArrivals'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { formatDate, formatRelative, formatTime, toDate, toMillis } from '../../lib/format'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { Avatar, Pressable } from '../primitives'
import { MessageBlock } from './MessageBlock'
import { EmptyState } from './EmptyState'

type Msg = Message & { _optimistic?: boolean; _clientKey?: string }

export interface TopicStreamProps {
  channelId: string
  /** Top-level messages, oldest first. */
  messages: Msg[]
  /** False while the first page loads: nothing present at that moment is "new". */
  ready: boolean
  me: string | undefined
  people: PeopleDirectory
  activeThreadId?: string | null
  onOpenThread: (id: string) => void
  onReact: (id: string) => void
  onToggleReaction: (id: string, emoji: string, hasReacted: boolean) => void
  onPin: (m: Message) => void
}

const keyOf = (m: Msg) => m._clientKey ?? m.id

/**
 * The space's conversation as topics (design/mockups/spaces.html §2): each
 * top-level message is a raised card, its replies folded into one summary
 * row that opens the thread panel. New topics from others arrive washed in
 * the author's hue; your own only get the insert.
 */
export function TopicStream(props: TopicStreamProps) {
  const { channelId, messages, ready, me, people, onOpenThread } = props
  const m = useMotionPresets()
  const scroller = useRef<HTMLDivElement>(null)
  const lastReplies = useWebSocketStore((s) => s.lastReplies)

  const topics = useMemo(() => messages.filter((x) => !x.parent_message_id), [messages])

  const { fresh, burst } = useArrivals(topics, {
    keyOf,
    authorOf: (x) => x.sender_id,
    me,
    ready,
    scope: channelId,
  })
  const replyArrivals = useArrivals(topics, {
    keyOf: (x) => `${keyOf(x)}:${x.reply_count ?? 0}`,
    authorOf: (x) => lastReplies[x.id]?.senderId,
    me,
    ready,
    scope: channelId,
  })

  // Stick to the bottom: on first load, and when a new topic lands while the
  // reader is already near the end (or wrote it).
  const atEnd = useRef(true)
  useLayoutEffect(() => {
    const el = scroller.current
    if (el && ready) el.scrollTop = el.scrollHeight
  }, [ready, channelId])
  const lastKey = topics.length ? keyOf(topics[topics.length - 1]!) : ''
  const lastMine = topics.length ? topics[topics.length - 1]!.sender_id === me : false
  useEffect(() => {
    const el = scroller.current
    if (!el || !ready) return
    if (atEnd.current || lastMine) {
      if (typeof el.scrollTo === 'function') el.scrollTo({ top: el.scrollHeight, behavior: m.reduced ? 'auto' : 'smooth' })
      else el.scrollTop = el.scrollHeight
    }
  }, [lastKey, lastMine, ready, m.reduced])

  if (ready && topics.length === 0) {
    return (
      <div className="flex-1 grid place-items-center">
        <EmptyState
          icon={<MessagesSquare size={24} strokeWidth={1.75} />}
          text="Chưa có chủ đề nào. Gửi tin đầu tiên để mở chủ đề."
        />
      </div>
    )
  }

  return (
    <div
      ref={scroller}
      onScroll={(e) => {
        const el = e.currentTarget
        atEnd.current = el.scrollHeight - el.scrollTop - el.clientHeight < 120
      }}
      className="flex-1 min-h-0 overflow-y-auto"
    >
      <div className="min-h-full flex flex-col justify-end gap-2.5 px-4 py-2">
        {!ready && <StreamSkeleton />}
        {burst && (
          <div role="status" className="self-center text-small text-ink-muted">
            {displayName(people, burst.authors[0])}
            {burst.authors.length > 1 ? ` và ${burst.authors.length - 1} người khác vừa cập nhật` : ' vừa gửi nhiều tin'}
          </div>
        )}
        <AnimatePresence initial={false}>
          {topics.map((t, i) => {
            const prev = topics[i - 1]
            const day = dayLabel(t.created_at)
            const showDay = !prev || dayLabel(prev.created_at) !== day
            const k = keyOf(t)
            return (
              <Fragment key={k}>
                {showDay && day && (
                  <div className="self-center pt-2 text-xs font-semibold text-ink-muted" role="separator">
                    {day}
                  </div>
                )}
                <motion.div {...m.row} layout={m.layoutProp} transition={m.layout}>
                  {t.linked_entity_type === 'task' ? (
                    <TaskNote message={t} people={people} />
                  ) : (
                    <TopicCard
                      {...props}
                      topic={t}
                      arrival={fresh.get(k)}
                      replyArrival={replyArrivals.fresh.get(`${k}:${t.reply_count ?? 0}`)}
                      onOpenThread={onOpenThread}
                    />
                  )}
                </motion.div>
              </Fragment>
            )
          })}
        </AnimatePresence>
      </div>
    </div>
  )
}

function TopicCard({
  topic: t, arrival, replyArrival, people, me, activeThreadId, onOpenThread, onReact, onToggleReaction, onPin,
}: TopicStreamProps & { topic: Msg; arrival?: Arrival; replyArrival?: Arrival }) {
  const byOther = arrival?.source === 'other'
  const idBase = `topic-${keyOf(t)}`
  const open = activeThreadId === t.id
  return (
    <article
      aria-labelledby={`${idBase}-who ${idBase}-text`}
      data-realtime={arrival?.source}
      style={byOther ? personStyle(arrival!.author ?? t.sender_id) : undefined}
      className={`grid pt-1 pb-1.5 rounded-lg bg-raised ${byOther ? 'rt-wash' : ''}
        ${open ? 'outline-2 outline-accent-wash' : ''}`}
    >
      <MessageBlock
        message={t}
        people={people}
        idBase={idBase}
        halo={byOther}
        tag={byOther ? 'vừa gửi' : undefined}
        me={me}
        onReact={onReact}
        onToggleReaction={onToggleReaction}
        onPin={onPin}
        onReply={onOpenThread}
      />
      <ReplySummary topic={t} people={people} arrival={replyArrival} onOpen={() => onOpenThread(t.id)} />
    </article>
  )
}

/** "2 trả lời · Lần cuối 09:20" with the repliers' faces; "Trả lời" when there are none. */
function ReplySummary({ topic: t, people, arrival, onOpen }: {
  topic: Msg
  people: PeopleDirectory
  arrival?: Arrival
  onOpen: () => void
}) {
  // Read the thread only if it is already cached (opened before, or pushed by
  // the socket). Fetching one thread per topic just for faces is not worth it.
  const { data: thread } = useQuery({ ...threadQueryOptions(t.id), enabled: false })
  const lastReply = useWebSocketStore((s) => s.lastReplies[t.id])
  const count = t.reply_count ?? 0

  if (t._optimistic) return null
  if (count === 0) {
    return (
      <Pressable
        onClick={onOpen}
        className="justify-self-start flex items-center gap-2 ml-14.5 mr-3.5 px-2 py-1 rounded-md text-small
          text-ink-muted hover:bg-hover hover:text-ink transition-colors duration-quick"
      >
        <MessageSquareReply size={16} strokeWidth={1.75} aria-hidden="true" />
        Trả lời
      </Pressable>
    )
  }

  const replies = (thread?.messages ?? []).filter((x) => x.id !== t.id)
  const faces: string[] = []
  for (let i = replies.length - 1; i >= 0 && faces.length < 3; i--) {
    const sid = replies[i]!.sender_id
    if (sid && !faces.includes(sid)) faces.push(sid)
  }
  faces.reverse()
  const nameOf = (sid: string) =>
    displayName(people, sid, replies.find((x) => x.sender_id === sid)?.sender_name)
  const lastAt = toMillis(replies[replies.length - 1]?.created_at) || toMillis(lastReply?.timestamp)
  const fresh = arrival?.source === 'other'

  return (
    <Pressable
      onClick={onOpen}
      style={fresh ? personStyle(arrival!.author ?? t.id) : undefined}
      className={`justify-self-start flex items-center gap-2.5 ml-14.5 mr-3.5 px-2 py-1.5 rounded-md
        text-small font-semibold text-accent hover:bg-hover transition-colors duration-quick
        ${fresh ? 'rt-wash' : ''}`}
    >
      {faces.length > 0 && (
        <span className="flex items-center">
          {faces.map((sid, i) => (
            <Avatar
              key={sid}
              name={nameOf(sid)}
              hueKey={sid}
              size={20}
              className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`}
            />
          ))}
        </span>
      )}
      <span>{count} trả lời</span>
      {lastAt > 0 && (
        <span className="font-normal text-ink-muted tnum" title={formatDate(lastAt / 1000)}>
          · Lần cuối {formatTime(lastAt / 1000)}
        </span>
      )}
      {fresh && (
        <span className="px-2 py-0.5 rounded-full bg-accent-wash text-ink text-xs">mới</span>
      )}
    </Pressable>
  )
}

/** A task created in the space: a quiet line instead of a topic card. */
function TaskNote({ message: t, people }: { message: Msg; people: PeopleDirectory }) {
  const title = t.content.replace(/^📋\s*Task:\s*/i, '').trim()
  return (
    <div className="flex items-center gap-2 px-3.5 py-1 text-small text-ink-muted">
      <ListChecks size={16} strokeWidth={1.75} aria-hidden="true" />
      <span>
        <span className="font-semibold text-ink">{displayName(people, t.sender_id, t.sender_name)}</span>
        {' '}đã tạo công việc “{title}”
      </span>
      <time className="ml-auto text-xs tnum">{formatTime(t.created_at)}</time>
    </div>
  )
}

function dayLabel(ts: unknown): string {
  const d = toDate(ts)
  if (!d) return ''
  const rel = formatRelative(d)
  // Within today the relative form is "x phút trước"; for a day divider say "Hôm nay".
  const today = new Date()
  if (d.toDateString() === today.toDateString()) return 'Hôm nay'
  return rel === 'Hôm qua' ? rel : formatDate(d)
}

function StreamSkeleton() {
  return (
    <div className="grid gap-2.5" aria-busy="true" aria-label="Đang tải tin nhắn">
      {[0, 1, 2].map((i) => (
        <div key={i} className="grid grid-cols-[32px_minmax(0,1fr)] gap-3 px-3.5 py-3 rounded-lg bg-raised">
          <div className="skeleton w-8 h-8 rounded-full" />
          <div className="grid gap-2">
            <div className="skeleton h-3.5 w-1/4 rounded-sm" />
            <div className="skeleton h-3.5 w-3/4 rounded-sm" />
          </div>
        </div>
      ))}
    </div>
  )
}
