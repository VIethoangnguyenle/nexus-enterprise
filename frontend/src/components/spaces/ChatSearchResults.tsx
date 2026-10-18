import { useEffect, useMemo, useRef, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { motion } from 'motion/react'
import { ArrowLeft, CircleAlert, Link2, Paperclip, SearchX } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { SEARCH_PAGE, useMessageSearch } from '../../hooks/useMessageSearch'
import { useOpenPerson } from '../../hooks/useOpenPerson'
import { useRecentSearches } from '../../hooks/useRecentSearches'
import { feedSearch } from '../../hooks/useShortcutScreen'
import { activeFilterCount, type ChatSearchFilters } from '../../lib/chat-search'
import type { Conversation } from '../../lib/conversations'
import { formatMessageTime } from '../../lib/format'
import { messageText } from '../../lib/message-model'
import { DURATION, EASE, staggerDelay, useMotionPresets, type MotionPresets } from '../../lib/motion'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { snippet } from '../../lib/search-text'
import { messageSegments } from '../../lib/shortcut-feed'
import { conversationSub, conversationsMatching, peopleMatching } from '../../lib/suggestions/chat'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, IconButton, Pressable } from '../primitives'
import { Marked } from '../patterns/Marked'
import { ChatSearchFilterBar } from './ChatSearchFilters'
import { ConversationIcon } from './ConversationIcon'
import { EmptyState } from './EmptyState'

interface Props {
  /** A desktop's results fill the content column under a header; a phone's sit under the search pill. */
  variant: 'desktop' | 'phone'
  q: string
  filters: ChatSearchFilters
  onFilters: (next: ChatSearchFilters) => void
  conversations: Conversation[]
  people: PeopleDirectory
  /** Desktop header ←: back to the conversation the search was opened from. */
  onClose?: () => void
}

/** A link in a result: the host it leads to, never the address. */
const URL_RE = /https?:\/\/[^\s<>"')]+/gi
function hostsOf(text: string): string[] {
  const hosts = new Set<string>()
  for (const raw of text.match(URL_RE) ?? []) {
    try {
      hosts.add(new URL(raw).hostname.replace(/^www\./, ''))
    } catch {
      // Not a link after all.
    }
  }
  return [...hosts]
}

/**
 * Chat's results (DESIGN.md "Gợi ý trong ô tìm", Trang kết quả): the filter chips, then the messages that match,
 * newest first, 25 a page and more as the list is scrolled. The words and the filters are the page's props (the
 * address holds them); every change of either asks the server again at once and the old list stays, dimmed,
 * until the new one is in. Only messages of conversations the person may read ever come back.
 */
export function ChatSearchResults({ variant, q, filters, onFilters, conversations, people, onClose }: Props) {
  const m = useMotionPresets()
  const phone = variant === 'phone'
  const me = useAuthStore((s) => s.user?.id ?? '')
  const recent = useRecentSearches('chat')
  const words = q.trim()
  const narrowed = activeFilterCount(filters) > 0
  const search = useMessageSearch(words, filters, { debounce: false })
  const byId = useMemo(() => new Map(conversations.map((c) => [c.id, c])), [conversations])

  // Is this the list that first appeared, or one that replaced it after a filter changed? Pages and swaps move differently.
  const shownOnce = useRef(false)
  useEffect(() => {
    if (search.messages.length > 0 && !search.refreshing) shownOnce.current = true
  }, [search.messages.length, search.refreshing])

  const count = search.messages.length
  const countText = !search.active || search.waiting ? '' : count === 0 ? '' : search.hasMore ? `Hơn ${count} tin nhắn` : `${count} tin nhắn`
  // The count announces itself (it is a live region of its own); the other states are told here.
  const status = !search.active ? '' : search.waiting ? 'Đang tìm…' : search.isError ? 'Chưa tìm được' : count === 0 ? 'Không có kết quả' : ''

  return (
    <section
      role="region"
      aria-label="Kết quả tìm kiếm"
      aria-busy={search.waiting || search.refreshing || undefined}
      className={`flex-1 flex flex-col min-h-0 min-w-0 ${phone ? 'bg-base' : 'bg-raised'}`}
    >
      {!phone && (
        <header className="shrink-0 flex items-center gap-2 h-14 pl-3 pr-4">
          <IconButton aria-label="Đóng tìm kiếm" title="Đóng tìm kiếm" shape="round" size="lg" onClick={onClose}>
            <ArrowLeft size={20} strokeWidth={1.75} aria-hidden="true" />
          </IconButton>
          <h1 className="m-0 font-display text-[1.1875rem] font-semibold">Kết quả tìm kiếm</h1>
          <span aria-live="polite" className="ml-auto"><CountText text={countText} m={m} className="text-sm" /></span>
        </header>
      )}

      <ChatSearchFilterBar filters={filters} onChange={onFilters} conversations={conversations} people={people} meId={me} phone={phone} />
      {phone && <div aria-live="polite" className="shrink-0 px-4 empty:hidden pb-1"><CountText text={countText} m={m} className="text-[0.8125rem]" /></div>}
      <p aria-live="polite" className="sr-only">{status}</p>

      <div className={`flex-1 min-h-0 overflow-y-auto overscroll-y-contain pb-4 ${phone ? '' : 'px-3'}`}>
        {!search.active ? (
          <p className="m-0 px-4 py-6 text-small text-ink-muted">
            {words ? 'Gõ ít nhất 2 ký tự để tìm tin nhắn.' : 'Gõ từ cần tìm vào ô tìm, rồi nhấn Enter.'}
          </p>
        ) : search.waiting ? (
          <ResultSkeleton phone={phone} />
        ) : search.isError ? (
          <EmptyState
            icon={<CircleAlert size={24} strokeWidth={1.75} className="text-danger" />}
            text="Chưa tìm được. Kiểm tra kết nối rồi thử lại."
            action={<Button variant="soft" size="sm" onClick={search.retry}>Thử lại</Button>}
          />
        ) : count === 0 ? (
          <EmptyState
            icon={<SearchX size={24} strokeWidth={1.75} />}
            text={narrowed
              ? `Không có tin nhắn nào khớp “${words}” với các bộ lọc này.`
              : `Không có tin nhắn nào khớp “${words}”. Thử từ khác hoặc viết ngắn hơn.`}
            action={narrowed ? <Button variant="soft" size="sm" onClick={() => onFilters({})}>Xoá bộ lọc</Button> : undefined}
          />
        ) : (
          <>
            {!phone && !narrowed && <PeopleAndGroups words={words} conversations={conversations} people={people} me={me} onOpened={() => recent.add(words)} />}
            {!phone && !narrowed && <h2 className="m-0 px-3 pt-3 pb-1 text-xs font-semibold text-ink-muted">Tin nhắn</h2>}
            <ul
              aria-busy={search.refreshing || undefined}
              className={`list-none m-0 p-0 transition-opacity duration-quick ease-out motion-reduce:transition-none ${search.refreshing ? 'opacity-50' : ''}`}
            >
              {search.messages.map((msg, i) => (
                <ResultCard
                  key={msg.id}
                  message={msg}
                  index={i}
                  swap={shownOnce.current && i < SEARCH_PAGE}
                  conversation={byId.get(msg.channel_id)}
                  people={people}
                  words={words}
                  me={me}
                  phone={phone}
                  m={m}
                  onOpen={() => recent.add(words)}
                />
              ))}
            </ul>
            {search.hasMore && <LoadMore loading={search.loadingMore} onMore={() => void search.loadMore()} />}
          </>
        )}
      </div>
    </section>
  )
}

/** "N tin nhắn": a new number fades in (160ms) instead of jumping. */
function CountText({ text, m, className }: { text: string; m: MotionPresets; className: string }) {
  if (!text) return null
  return (
    <motion.span
      key={text}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: m.reduced ? DURATION.reducedFade : DURATION.quick, ease: EASE.out }}
      className={`text-ink-muted tnum ${className}`}
    >
      {text}
    </motion.span>
  )
}

function ResultSkeleton({ phone }: { phone: boolean }) {
  return (
    <div role="status" aria-label="Đang tìm tin nhắn" className="grid gap-1">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className={`grid gap-2.5 p-3 ${phone ? 'px-4' : ''}`}>
          <div className="skeleton h-3.5 w-1/3 rounded-sm" />
          <div className="grid grid-cols-[32px_minmax(0,1fr)] gap-3">
            <div className="skeleton w-8 h-8 rounded-full" />
            <div className="grid gap-1.5">
              <div className="skeleton h-3.5 w-2/5 rounded-sm" />
              <div className="skeleton h-3.5 w-full rounded-sm" />
              <div className="skeleton h-3.5 w-4/5 rounded-sm" />
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

/**
 * Further pages come as the list is scrolled to its end; where the browser cannot tell (no IntersectionObserver)
 * a button asks for them.
 */
function LoadMore({ loading, onMore }: { loading: boolean; onMore: () => void }) {
  const end = useRef<HTMLDivElement>(null)
  const observed = typeof IntersectionObserver !== 'undefined'
  const latest = useRef(onMore)
  useEffect(() => {
    latest.current = onMore
  })
  useEffect(() => {
    const el = end.current
    if (!observed || !el || loading) return
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) latest.current()
    })
    io.observe(el)
    return () => io.disconnect()
  }, [observed, loading])
  return (
    <div ref={end} className="grid justify-items-center py-3">
      {!observed && <Button variant="soft" size="sm" loading={loading} onClick={onMore}>Tải thêm</Button>}
      {observed && loading && <span className="skeleton h-3.5 w-24 rounded-sm" aria-hidden="true" />}
    </div>
  )
}

/** Up to three people and three groups the words match, above the messages, while no filter narrows the search (desktop). */
function PeopleAndGroups({ words, conversations, people, me, onOpened }: {
  words: string; conversations: Conversation[]; people: PeopleDirectory; me: string; onOpened: () => void
}) {
  const opener = useOpenPerson(conversations)
  const persons = useMemo(() => peopleMatching(people.list, words, me), [people.list, words, me])
  const groups = useMemo(
    () => conversationsMatching(conversations, words, new Set(persons.map((p) => p.userId))),
    [conversations, words, persons],
  )
  if (persons.length + groups.length === 0) return null
  const row = 'grid grid-cols-[32px_minmax(0,1fr)] items-center gap-3 w-full min-h-11 px-3 rounded-md text-sm text-ink no-underline hover:bg-hover transition-colors duration-quick focus-ring'
  return (
    <section aria-label="Nhóm và người">
      <h2 className="m-0 px-3 pt-3 pb-1 text-xs font-semibold text-ink-muted">Nhóm và người</h2>
      <ul className="list-none m-0 p-0 grid gap-0.5">
        {persons.map((p) => (
          <li key={p.userId}>
            <Pressable
              onClick={() => {
                onOpened()
                opener.open(p)
              }}
              disabled={opener.pending}
              className={row}
            >
              <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl} size={32} />
              <span className="min-w-0 grid">
                <span className="truncate font-medium"><Marked text={p.name} query={words} /></span>
                {p.role && <span className="truncate text-xs text-ink-muted">{p.role}</span>}
              </span>
            </Pressable>
          </li>
        ))}
        {groups.map((c) => (
          <li key={c.id}>
            <Link to="/channels/$channelId" params={{ channelId: c.id }} onClick={onOpened} className={row}>
              <ConversationIcon conversation={c} size={32} />
              <span className="min-w-0 grid">
                <span className="truncate font-medium"><Marked text={c.title} query={words} /></span>
                <span className="truncate text-xs text-ink-muted">{conversationSub(c)}</span>
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

/**
 * One hit: where it was said, who said it and when, and the words around the match with the match marked.
 * First results and later pages rise 8px as they fade in, 30ms apart; a list that replaces another (a filter
 * changed) only fades.
 */
function ResultCard({ message: msg, index, swap, conversation, people, words, me, phone, m, onOpen }: {
  message: Message
  index: number
  swap: boolean
  conversation: Conversation | undefined
  people: PeopleDirectory
  words: string
  me: string
  phone: boolean
  m: MotionPresets
  onOpen: () => void
}) {
  const sender = displayName(people, msg.sender_id, msg.sender_name)
  const segments = useMemo(() => messageSegments(msg.content, msg.content_format, me), [msg.content, msg.content_format, me])
  const plain = segments.map((s) => s.text).join('')
  const around = useMemo(() => snippet(plain, words, 320), [plain, words])
  const hosts = useMemo(() => hostsOf(messageText(msg.content)), [msg.content])
  const files = msg.attachments ?? []
  const pageIndex = index % SEARCH_PAGE
  const motionProps = m.reduced || swap
    ? { initial: { opacity: 0 }, animate: { opacity: 1, transition: { duration: m.reduced ? DURATION.reducedFade : DURATION.quick, ease: EASE.out } } }
    : {
        initial: { opacity: 0, y: 8 },
        animate: { opacity: 1, y: 0, transition: { duration: DURATION.base, ease: EASE.out, delay: staggerDelay(pageIndex, false) } },
      }
  const when = formatMessageTime(msg.created_at)
  // The match is marked in the plain words; @names keep the colour the rest of the app gives a mention.
  const body: ReactNode = segments.length > 0 && around === plain && segments.some((s) => s.mention)
    ? segments.map((s, i) => (
      <span key={i} className={s.mention === 'self' ? 'rounded-sm bg-accent-wash px-1 font-semibold text-accent' : s.mention ? 'font-semibold text-accent' : undefined}>
        <Marked text={s.text} query={words} />
      </span>
    ))
    : <Marked text={around} query={words} />

  return (
    <motion.li {...motionProps} className="list-none relative">
      {index > 0 && <span aria-hidden="true" className={`absolute top-0 h-px bg-line ${phone ? 'inset-x-4' : 'inset-x-3'}`} />}
      <Link
        to="/channels/$channelId"
        params={{ channelId: msg.channel_id }}
        search={feedSearch(msg)}
        onClick={onOpen}
        className={`grid gap-1.5 p-3 rounded-surface text-ink no-underline hover:bg-hover transition-colors duration-quick focus-ring
          ${phone ? 'rounded-none px-4' : ''}`}
      >
        {conversation && (
          <span className="flex items-center gap-2 min-w-0 text-[0.8125rem] font-semibold text-ink-muted">
            <ConversationIcon conversation={conversation} size={20} />
            <span className="truncate">{conversation.title}</span>
          </span>
        )}
        <span className="grid grid-cols-[32px_minmax(0,1fr)] gap-3">
          <Avatar name={sender} hueKey={msg.sender_id} src={people.byUserId.get(msg.sender_id)?.avatarUrl} size={32} />
          <span className="min-w-0 grid gap-0.5">
            <span className="flex items-baseline gap-2">
              <b className={`font-semibold truncate ${phone ? 'text-16' : 'text-[0.9375rem]'}`}>{sender}</b>
              <time className="shrink-0 text-xs text-ink-muted tnum">{when}</time>
            </span>
            <span className={`line-clamp-3 wrap-anywhere ${phone ? 'text-16 leading-6' : 'text-[0.9375rem] leading-[1.4375rem]'}`}>{body}</span>
            {(files.length > 0 || hosts.length > 0) && (
              <span className="flex flex-wrap gap-2 pt-1">
                {files.map((f) => <Chip key={f.drive_file_id} icon={<Paperclip size={16} strokeWidth={1.75} className="text-info" />}>{f.file_name}</Chip>)}
                {hosts.map((h) => <Chip key={h} icon={<Link2 size={16} strokeWidth={1.75} className="text-accent" />}>{h}</Chip>)}
              </span>
            )}
          </span>
        </span>
      </Link>
    </motion.li>
  )
}

function Chip({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-2 h-9 max-w-full pl-2 pr-3 rounded-md bg-sunk text-sm font-medium">
      <span aria-hidden="true" className="inline-flex shrink-0">{icon}</span>
      <span className="truncate">{children}</span>
    </span>
  )
}
