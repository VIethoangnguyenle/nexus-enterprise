import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { ArrowDown, ArrowUp, ListChecks, MessagesSquare, Trash2 } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { useArrivals } from '../../hooks/useArrivals'
import { DURATION, useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { formatTime, toMillis } from '../../lib/format'
import { displayName, type PeopleDirectory } from '../../lib/people'
import {
  buildStream, keyOfMessage, unreadStartId, type StreamItem, type StreamMessage,
} from '../../lib/stream-model'
import { Pressable, Spinner } from '../primitives'
import { usePhone } from '../../hooks/usePhone'
import type { MessageLifecycle } from '../../hooks/useMessageActions'
import { toEditorHtml } from '../../lib/message-model'
import { cardKindOf } from '../chat/MessageCard'
import { MessageBlock } from './MessageBlock'
import { MessageActionSheet } from './MessageActionSheet'
import { MessageMenu, type MessageMenuTarget } from './MessageMenu'
import { MessageEditBox } from './MessageEditBox'
import { DeleteMessageDialog } from './DeleteMessageDialog'
import { ThreadSummary } from './ThreadSummary'
import { EmptyState } from './EmptyState'

export interface ConversationStreamProps {
  channelId: string
  /** Accessible name of the log, e.g. "Tin nhắn trong Đối soát giao dịch". */
  label: string
  /** Root messages of the conversation, oldest first. */
  messages: StreamMessage[]
  /** False while the first page loads: nothing present at that moment is "new". */
  ready: boolean
  hasMore: boolean
  loadingOlder: boolean
  olderFailed?: boolean
  onLoadOlder: () => void
  me: string | undefined
  people: PeopleDirectory
  /** Where the reader stopped last time; the stream waits for it (`ready`) before placing "Tin mới". */
  unread: { ready: boolean; lastReadId: string; count: number }
  activeThreadId?: string | null
  onOpenThread: (id: string) => void
  onReact: (id: string) => void
  onToggleReaction: (id: string, emoji: string, hasReacted: boolean) => void
  onPin: (m: Message) => void
  onFollow: (m: Message, follow: boolean) => void
  /** The newest message is on screen (or within a few lines of it). */
  onAtEndChange?: (atEnd: boolean) => void
  /** Editing, deleting and un-reading; without it the stream offers none of them. */
  lifecycle?: MessageLifecycle
}

/** Distance from the end that still counts as "at the end". */
const END_SLACK = 120
/** Where "Tin mới" lands below the top of the stream when it is placed there. */
const DIVIDER_OFFSET = 48
/** The stream asks for older messages when the reader is this close to the top. */
const TOP_SLACK = 160

interface OpenState {
  scope: string
  resolved: boolean
  /** The first unread message, fixed for this visit. */
  unreadId: string | null
  /** Time of the newest message when the conversation was opened; later is news, earlier is history. */
  baselineMs: number
  /** Time of the oldest message then: anything older joined later as an older page. */
  windowStartMs: number
}

interface ViewState { end: boolean; far: boolean; dividerAbove: boolean }
const INITIAL_VIEW: ViewState = { end: true, far: false, dividerAbove: false }

/**
 * A group or direct message as an in-line stream (DESIGN.md "Chat: luồng
 * in-line"): oldest first, two-sided bubbles, same-sender grouping, day and
 * "Tin mới" dividers, a jump pill, older pages on scroll up, and a summary row
 * under every message that has a thread. New messages from others arrive washed
 * in their hue; yours only get the insert.
 */
export function ConversationStream(props: ConversationStreamProps) {
  const {
    channelId, label, messages, ready, hasMore, loadingOlder, olderFailed, onLoadOlder, me, people, unread,
    activeThreadId, onOpenThread, onReact, onToggleReaction, onPin, onFollow, onAtEndChange, lifecycle,
  } = props
  const phone = usePhone()
  const motionPresets = useMotionPresets()
  const scroller = useRef<HTMLDivElement>(null)

  // What this visit is anchored to is decided once, when both the messages and
  // the read position are known, and kept: reading moves the position, but the
  // divider stays where the visit began.
  const [open, setOpen] = useState<OpenState>({ scope: channelId, resolved: false, unreadId: null, baselineMs: 0, windowStartMs: 0 })
  if (open.scope !== channelId) {
    setOpen({ scope: channelId, resolved: false, unreadId: null, baselineMs: 0, windowStartMs: 0 })
  } else if (!open.resolved && ready && unread.ready) {
    const newest = messages[messages.length - 1]
    const oldest = messages[0]
    setOpen({
      scope: channelId,
      resolved: true,
      unreadId: unreadStartId(messages, { lastReadId: unread.lastReadId, unreadCount: unread.count, me }),
      baselineMs: newest ? toMillis(newest.created_at) : 0,
      windowStartMs: oldest ? toMillis(oldest.created_at) : 0,
    })
  }
  // Sending a message reads everything above it: the divider goes.
  const sentSince = open.resolved && messages.some((x) => x.sender_id === me && toMillis(x.created_at) > open.baselineMs)
  const unreadId = sentSince ? null : open.unreadId

  const items = useMemo(() => buildStream(messages, { me, unreadFromId: unreadId }), [messages, me, unreadId])
  const isOlderPage = useCallback((x: StreamMessage) => toMillis(x.created_at) < open.windowStartMs, [open.windowStartMs])
  const isNews = (x: StreamMessage) => open.baselineMs > 0 && toMillis(x.created_at) > open.baselineMs

  const { fresh, burst } = useArrivals(messages, {
    keyOf: keyOfMessage, authorOf: (x) => x.sender_id, me, ready, scope: channelId, isHistory: isOlderPage,
  })
  const replyArrivals = useArrivals(messages, {
    keyOf: (x) => `${keyOfMessage(x)}:${x.reply_count ?? 0}`,
    authorOf: (x) => x.reply_participant_ids?.[0],
    me, ready, scope: channelId, isHistory: isOlderPage,
  })

  // --- scrolling -----------------------------------------------------------
  const atEnd = useRef(true)
  const lastTop = useRef(0)
  /** A jump to the latest is under way: passing the "Tin mới" divider on the way is not reading it. */
  const jumping = useRef(false)
  const reportedEnd = useRef<boolean | null>(null)
  const [view, setView] = useState<ViewState>(INITIAL_VIEW)
  const [newBelow, setNewBelow] = useState(0)
  const [skipped, setSkipped] = useState(false)

  const divider = () => scroller.current?.querySelector<HTMLElement>('[data-unread-divider]') ?? null
  const scrollTo = (top: number, smooth: boolean) => {
    const el = scroller.current
    if (!el) return
    if (typeof el.scrollTo === 'function') el.scrollTo({ top, behavior: smooth && !motionPresets.reduced ? 'smooth' : 'auto' })
    else el.scrollTop = top
  }

  const measure = (allowLoad = true) => {
    const el = scroller.current
    if (!el) return
    const dist = el.scrollHeight - el.scrollTop - el.clientHeight
    const end = dist < END_SLACK
    atEnd.current = end
    const d = divider()
    const above = !!d && d.offsetTop + d.offsetHeight <= el.scrollTop
    const inView = !!d && !above && d.offsetTop < el.scrollTop + el.clientHeight
    const next: ViewState = { end, far: dist > el.clientHeight / 2, dividerAbove: above }
    setView((v) => (v.end === next.end && v.far === next.far && v.dividerAbove === next.dividerAbove ? v : next))
    if (end) setNewBelow(0)
    if (end) jumping.current = false
    if (inView && !jumping.current) setSkipped(false)
    if (reportedEnd.current !== end) {
      reportedEnd.current = end
      onAtEndChange?.(end)
    }
    // Older messages are asked for while the reader is at the top and not moving away from it: a jump
    // to the latest message starts at the top too, and a page landing mid-flight would cut it short.
    const movingDown = el.scrollTop > lastTop.current
    lastTop.current = el.scrollTop
    if (allowLoad && !movingDown && el.scrollTop < TOP_SLACK && hasMore && !loadingOlder && !olderFailed) onLoadOlder()
  }

  // Older messages land above the reader: move the scroll by what was added so
  // the line being read stays put.
  const prevOldest = useRef('')
  const prevHeight = useRef(0)
  const oldestKey = items.find((i) => i.kind === 'message')?.key ?? ''
  useLayoutEffect(() => {
    const el = scroller.current
    if (!el) return
    const stillThere = prevOldest.current && messages.some((x) => keyOfMessage(x) === prevOldest.current)
    if (stillThere && oldestKey && prevOldest.current !== oldestKey) el.scrollTop += el.scrollHeight - prevHeight.current
    prevOldest.current = oldestKey
    prevHeight.current = el.scrollHeight
  })

  // Opening: at the end, or at "Tin mới" when the unread run is longer than a screen.
  useLayoutEffect(() => {
    const el = scroller.current
    if (!el || !open.resolved) return
    const d = divider()
    if (d && el.scrollHeight - d.offsetTop > el.clientHeight) el.scrollTop = Math.max(0, d.offsetTop - DIVIDER_OFFSET)
    else el.scrollTop = el.scrollHeight
    reportedEnd.current = null
    setSkipped(false)
    setNewBelow(0)
    lastTop.current = el.scrollTop
    // Opened near the top (a long unread run): fetch the page above, so "Tin mới" has context over it.
    measure()
    // Only a new visit repositions.
  }, [open.scope, open.resolved])

  // A new message at the tail: follow it when the reader is at the end or wrote it, otherwise count it.
  const last = messages[messages.length - 1]
  const lastKey = last ? keyOfMessage(last) : ''
  const lastMine = !!last && last.sender_id === me
  const prevLast = useRef('')
  useLayoutEffect(() => {
    const el = scroller.current
    const changed = !!prevLast.current && prevLast.current !== lastKey
    prevLast.current = lastKey
    if (!el || !open.resolved || !changed) return
    if (atEnd.current || lastMine) scrollTo(el.scrollHeight, true)
    else setNewBelow((n) => n + 1)
  }, [lastKey, lastMine, open.resolved])

  const jumpLatest = () => {
    const el = scroller.current
    const d = divider()
    if (el && d && d.offsetTop >= el.scrollTop) {
      setSkipped(true)
      jumping.current = true
      setTimeout(() => { jumping.current = false }, 1500)
    }
    setNewBelow(0)
    scrollTo(el?.scrollHeight ?? 0, true)
  }
  const jumpUnread = () => {
    const d = divider()
    setSkipped(false)
    if (d) scrollTo(Math.max(0, d.offsetTop - DIVIDER_OFFSET), true)
  }

  const showLatest = ready && (view.far || (newBelow > 0 && !view.end))
  const showUnread = ready && !showLatest && skipped && !!unreadId && view.dividerAbove

  // --- menus ---------------------------------------------------------------
  const [sheetFor, setSheetFor] = useState<StreamMessage | null>(null)
  const [menu, setMenu] = useState<MessageMenuTarget | null>(null)
  const anchorRef = useRef<HTMLElement | null>(null)
  const openMenu = (m: Message, anchor: HTMLElement) => {
    anchorRef.current = anchor
    const room = (scroller.current?.getBoundingClientRect().bottom ?? window.innerHeight) - anchor.getBoundingClientRect().bottom
    setMenu({ message: m as StreamMessage, up: room < 140 })
  }
  const loadedIds = useMemo(() => new Set(messages.map((x) => x.id)), [messages])
  const openQuote = (id: string) => {
    const el = scroller.current?.querySelector<HTMLElement>(`[data-message-id="${id}"]`)
    el?.scrollIntoView?.({ block: 'center', behavior: motionPresets.reduced ? 'auto' : 'smooth' })
  }
  const hasReacted = (m: Message, emoji: string) =>
    !!m.reactions?.some((r) => r.emoji === emoji && r.user_ids?.includes(me ?? ''))

  // --- render --------------------------------------------------------------
  const row = (item: Extract<StreamItem, { kind: 'message' }>) => {
    const t = item.message
    const a = fresh.get(item.key)
    const byOther = a?.source === 'other'
    const ra = replyArrivals.fresh.get(`${item.key}:${t.reply_count ?? 0}`)
    const idBase = `msg-${item.key}`
    // A poll or task is posted as a system message, but it is a card of its sender, not a note.
    if (item.system && !cardKindOf(t)) return <SystemNote message={t} people={people} />
    const can = lifecycle?.abilities(t)
    const editingHere = lifecycle?.editing?.message.id === t.id && !phone
    return (
      <article
        aria-labelledby={`${idBase}-who ${idBase}-text`}
        data-realtime={a?.source}
        data-message-id={t.id}
        style={byOther ? personStyle(a!.author ?? t.sender_id) : undefined}
        className={`rounded-surface ${byOther ? 'rt-wash' : ''}`}
      >
        <MessageBlock
          message={t}
          people={people}
          idBase={idBase}
          mine={item.mine}
          grouped={item.grouped}
          pressed={sheetFor?.id === t.id}
          halo={byOther}
          tag={byOther ? 'vừa gửi' : undefined}
          me={me}
          onReact={onReact}
          onToggleReaction={onToggleReaction}
          onPin={onPin}
          onReply={onOpenThread}
          onMore={openMenu}
          onEdit={can?.edit ? (m) => lifecycle!.startEdit(m) : undefined}
          onOpenQuote={t.quoted && loadedIds.has(t.quoted.id) ? openQuote : undefined}
          editor={editingHere ? (
            <MessageEditBox
              initialHtml={lifecycle!.editing!.draft ?? toEditorHtml(t)}
              onSave={lifecycle!.saveEdit}
              onCancel={lifecycle!.cancelEdit}
            />
          ) : undefined}
          onOpenSheet={(m) => setSheetFor(m as StreamMessage)}
        >
          {!t._optimistic && (
            <ThreadSummary
              message={t}
              people={people}
              mine={item.mine}
              open={activeThreadId === t.id}
              onOpen={() => onOpenThread(t.id)}
              washStyle={ra?.source === 'other' && t.following ? personStyle(ra.author ?? t.id) : undefined}
            />
          )}
        </MessageBlock>
      </article>
    )
  }

  return (
    <div className="relative flex-1 flex flex-col min-h-0">
      <div
        ref={scroller}
        role="log"
        aria-live="polite"
        aria-relevant="additions"
        aria-label={label}
        aria-busy={!ready}
        onScroll={() => measure()}
        // Positions are managed above; the browser's own anchoring would move the scroll a second time.
        style={{ overflowAnchor: 'none' }}
        className="relative flex-1 min-h-0 overflow-y-auto overscroll-contain"
      >
        <div className="min-h-full flex flex-col justify-end px-2.5 md:px-2 pt-2 pb-5">
          {!ready && <StreamSkeleton />}
          {burst && (
            <div role="status" className="self-center py-1 text-small text-ink-muted">
              {displayName(people, burst.authors[0])}
              {burst.authors.length > 1 ? ` và ${burst.authors.length - 1} người khác vừa cập nhật` : ' vừa gửi nhiều tin'}
            </div>
          )}
          {ready && messages.length === 0 && (
            <EmptyState
              icon={<MessagesSquare size={24} strokeWidth={1.75} />}
              text="Chưa có tin nhắn nào. Gửi tin đầu tiên để bắt đầu."
            />
          )}
          <AnimatePresence initial={false}>
            {items.map((item) => {
              if (item.kind === 'day') {
                return (
                  // Shown with its messages: an older page brings separators too, and a height animation
                  // above the reader would move the line being read.
                  <motion.div key={item.key} initial={false}>
                    <div
                      role="separator"
                      aria-label={item.label}
                      className="flex items-center gap-3 px-3.5 pt-3.5 pb-1.5 text-xs font-semibold text-ink-muted
                        before:content-[''] before:h-px before:flex-1 before:bg-line
                        after:content-[''] after:h-px after:flex-1 after:bg-line"
                    >
                      {item.label}
                    </div>
                  </motion.div>
                )
              }
              if (item.kind === 'unread') {
                return (
                  <motion.div
                    key={item.key}
                    data-unread-divider
                    initial={false}
                    exit={{ opacity: 0, transition: { duration: DURATION.base } }}
                  >
                    <div
                      role="separator"
                      aria-label="Tin mới"
                      className="flex items-center gap-3 px-3.5 pt-3.5 pb-1.5 text-xs font-semibold text-accent
                        before:content-[''] before:h-px before:flex-1 before:bg-accent"
                    >
                      Tin mới
                    </div>
                  </motion.div>
                )
              }
              return (
                <motion.div
                  key={item.key}
                  {...(isNews(item.message) ? motionPresets.row : { initial: false as const })}
                >
                  {row(item)}
                </motion.div>
              )
            })}
          </AnimatePresence>
        </div>
      </div>

      {/* Over the top edge, not in the flow: adding or removing it must not move the line being read. */}
      {(loadingOlder || olderFailed) && (
        <div className="absolute inset-x-0 top-2 z-sticky flex justify-center pointer-events-none">
          {loadingOlder ? (
            <div role="status" className="inline-flex items-center gap-2 h-8 px-3 rounded-full bg-overlay shadow-overlay text-small text-ink-muted">
              <Spinner size="sm" /> Đang tải tin cũ
            </div>
          ) : (
            <Pressable onClick={onLoadOlder} className="pointer-events-auto h-8 px-3 rounded-full bg-overlay shadow-overlay text-small text-accent">
              Chưa tải được tin cũ. Thử lại
            </Pressable>
          )}
        </div>
      )}

      {(showLatest || showUnread) && (
        // The pill is placed by this wrapper: on touch screens every button gets `position: relative`
        // from an unlayered rule in index.css, which would beat `absolute` on the button itself.
        <div className="absolute inset-x-0 bottom-3 z-sticky flex justify-center pointer-events-none">
          <Pressable
            onClick={showLatest ? jumpLatest : jumpUnread}
            className="pointer-events-auto inline-flex items-center gap-2 h-10 md:h-9 px-3.5 md:px-3 rounded-full
              bg-overlay shadow-overlay text-small-ui font-semibold text-ink whitespace-nowrap"
          >
            {showLatest
              ? <ArrowDown size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent" />
              : <ArrowUp size={16} strokeWidth={1.75} aria-hidden="true" className="text-accent" />}
            {showLatest ? 'Tin mới nhất' : 'Nhảy tới tin chưa đọc'}
            {showLatest && newBelow > 0 && (
              <span
                aria-label={`${newBelow} tin mới`}
                className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum"
              >
                {newBelow}
              </span>
            )}
          </Pressable>
        </div>
      )}

      <MessageMenu target={menu} anchorRef={anchorRef} onClose={() => setMenu(null)} lifecycle={lifecycle} onFollow={onFollow} />

      <AnimatePresence>
        {sheetFor && (
          <MessageActionSheet
            key={sheetFor.id}
            message={sheetFor}
            people={people}
            me={me}
            onReply={() => onOpenThread(sheetFor.id)}
            onToggleFollow={() => onFollow(sheetFor, !sheetFor.following)}
            onPin={() => onPin(sheetFor)}
            onReact={(emoji) => onToggleReaction(sheetFor.id, emoji, hasReacted(sheetFor, emoji))}
            onMoreReactions={() => onReact(sheetFor.id)}
            onEdit={lifecycle?.abilities(sheetFor).edit ? () => lifecycle.startEdit(sheetFor) : undefined}
            onMarkUnread={lifecycle?.abilities(sheetFor).markUnread ? () => lifecycle.markUnread(sheetFor) : undefined}
            onDelete={lifecycle?.abilities(sheetFor).remove ? () => lifecycle.askDelete(sheetFor) : undefined}
            onClose={() => setSheetFor(null)}
          />
        )}
      </AnimatePresence>

      {lifecycle && (
        <DeleteMessageDialog
          message={lifecycle.deleting}
          me={me}
          people={people}
          busy={lifecycle.deleteBusy}
          onConfirm={lifecycle.confirmDelete}
          onCancel={lifecycle.cancelDelete}
        />
      )}
    </div>
  )
}

/** A line the product wrote into the conversation, not a bubble: a task created, or a manager's deletion logged. */
function SystemNote({ message: t, people }: { message: StreamMessage; people: PeopleDirectory }) {
  const deletion = t.linked_entity_type === 'message_deleted'
  const title = t.content.replace(/^📋\s*Task:\s*/i, '').trim()
  return (
    <div className="flex items-center gap-2 px-3.5 py-1.5 text-small text-ink-muted">
      {deletion ? <Trash2 size={16} strokeWidth={1.75} aria-hidden="true" /> : <ListChecks size={16} strokeWidth={1.75} aria-hidden="true" />}
      {deletion ? (
        <span>{t.content}</span>
      ) : (
        <span>
          <span className="font-semibold text-ink">{displayName(people, t.sender_id, t.sender_name)}</span>
          {' '}đã tạo công việc “{title}”
        </span>
      )}
      <time className="ml-auto text-xs tnum">{formatTime(t.created_at)}</time>
    </div>
  )
}

function StreamSkeleton() {
  return (
    <div className="grid gap-3 py-2" aria-label="Đang tải tin nhắn">
      {[0, 1, 2].map((i) => (
        <div key={i} className={`flex gap-2 px-3.5 ${i === 1 ? 'justify-end' : ''}`}>
          {i !== 1 && <div className="skeleton w-8 h-8 rounded-full shrink-0" />}
          <div className="grid gap-1.5 w-3/5">
            <div className={`skeleton h-3 w-1/4 rounded-sm ${i === 1 ? 'justify-self-end' : ''}`} />
            <div className="skeleton h-9 rounded-xl" />
          </div>
        </div>
      ))}
    </div>
  )
}
