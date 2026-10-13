import { useMemo, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Link, useNavigate } from '@tanstack/react-router'
import { motion } from 'motion/react'
import { ArrowLeft, CircleAlert, Clock, SearchX, X } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { useCreateDM } from '../../hooks/useMessaging'
import { useMessageSearch } from '../../hooks/useMessageSearch'
import { useModalFocus } from '../../hooks/useModalFocus'
import { feedSearch } from '../../hooks/useShortcutScreen'
import { explain } from '../../lib/errors'
import { formatFeedTime, toMillis } from '../../lib/format'
import { messageText } from '../../lib/message-model'
import { useMotionPresets } from '../../lib/motion'
import { displayName, type PeopleDirectory, type Person } from '../../lib/people'
import { addRecent, readRecent, removeRecent } from '../../lib/recent-searches'
import { fold, highlight, matchesAll, snippet } from '../../lib/search-text'
import type { Conversation } from '../../lib/conversations'
import { useAuthStore } from '../../stores/auth.store'
import { Avatar, Button, IconButton, Pressable, SearchField, toast } from '../primitives'
import { ConversationIcon } from './ConversationIcon'
import { EmptyState } from './EmptyState'

type Filter = 'all' | 'people' | 'groups' | 'messages' | 'files'

const CHIPS: { id: Filter; label: string }[] = [
  { id: 'all', label: 'Tất cả' },
  { id: 'people', label: 'Người' },
  { id: 'groups', label: 'Nhóm' },
  { id: 'messages', label: 'Tin nhắn' },
  { id: 'files', label: 'Có tệp' },
]

/** Groups and people are matched from the lists already loaded; this many of each is plenty to scan. */
const CLIENT_LIMIT = 10

const ROW = 'grid grid-cols-[36px_minmax(0,1fr)] gap-3.5 items-center w-full min-h-14 px-4 py-1.5 text-ink no-underline focus-ring active:bg-hover'

/** `text` with the words of `query` marked as in the mockup (`--color-warning-wash`). */
function Marked({ text, query }: { text: string; query: string }) {
  return (
    <>
      {highlight(text, query).map((p, i) =>
        p.hit ? <mark key={i} className="bg-warning-wash text-inherit rounded-xs px-px">{p.text}</mark> : p.text)}
    </>
  )
}

const Label = ({ children }: { children: ReactNode }) => (
  <h2 className="m-0 px-4 pt-3.5 pb-1 text-small font-semibold text-ink-muted">{children}</h2>
)

/**
 * Tìm trong chat on a phone (DESIGN.md §6, mockup chat-mobile-home §6): a full screen over the Chat
 * list. Groups and people come from the loaded lists (case and accent insensitive); messages from the
 * server, after a pause in typing. Before anything is typed it lists the person's recent searches.
 * A modal layer: focus goes to the box and returns to what opened it, Esc and Quay lại close.
 */
export function ChatSearchScreen({ conversations, people, onClose }: {
  conversations: Conversation[]
  people: PeopleDirectory
  onClose: () => void
}) {
  const m = useMotionPresets()
  const navigate = useNavigate()
  const me = useAuthStore((s) => s.user?.id ?? '')
  const createDM = useCreateDM()
  const surfaceRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [recent, setRecent] = useState(() => readRecent(me))
  useModalFocus({ active: true, surfaceRef, onClose, initialFocusRef: inputRef })

  const typed = query.trim()
  const showClient = filter === 'all' || filter === 'people' || filter === 'groups'
  const showMessages = filter === 'all' || filter === 'messages' || filter === 'files'
  const search = useMessageSearch(query, filter === 'files', showMessages)

  const byId = useMemo(() => new Map(conversations.map((c) => [c.id, c])), [conversations])
  const groups = useMemo(
    () => typed && (filter === 'all' || filter === 'groups')
      ? conversations.filter((c) => c.kind === 'space' && matchesAll(c.title, typed)).slice(0, CLIENT_LIMIT)
      : [],
    [conversations, typed, filter],
  )
  const persons = useMemo(
    () => typed && (filter === 'all' || filter === 'people')
      ? people.list.filter((p) => p.userId !== me && (matchesAll(p.name, typed) || matchesAll(p.username, typed))).slice(0, CLIENT_LIMIT)
      : [],
    [people.list, me, typed, filter],
  )

  const remember = () => setRecent(addRecent(me, typed))
  const reuse = (q: string) => {
    setQuery(q)
    inputRef.current?.focus()
  }

  const openPerson = (p: Person) => {
    remember()
    const existing = conversations.find((c) => c.kind === 'dm' && c.partner?.userId === p.userId)
    if (existing) {
      void navigate({ to: '/channels/$channelId', params: { channelId: existing.id } })
      return
    }
    createDM.mutate(
      { userId: p.userId, ngacNodeId: p.nodeId },
      {
        onSuccess: (ch) => void navigate({ to: '/channels/$channelId', params: { channelId: ch.id } }),
        onError: (err) => toast.error(explain(err, `mở cuộc trò chuyện với ${p.name}`)),
      },
    )
  }

  const noWords = !typed
  const nothingFound = `Không tìm thấy “${typed}” — thử từ khác.`
  // The server matches words as written, so a search typed without accents may miss "đối soát".
  const noAccents = /[a-z]/i.test(typed) && fold(typed) === typed.toLowerCase()
  const clientEmpty = groups.length + persons.length === 0

  return createPortal(
    <motion.div
      ref={surfaceRef}
      role="dialog"
      aria-modal="true"
      aria-label="Tìm trong chat"
      {...m.scrim}
      className="fixed inset-x-0 top-0 h-app z-modal flex flex-col min-h-0 bg-base pt-safe"
    >
      <form
        role="search"
        onSubmit={(e) => {
          e.preventDefault()
          remember()
        }}
        className="shrink-0 mx-3 mt-2.5"
      >
        <SearchField
          pill
          ref={inputRef}
          label="Tìm trong chat"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          enterKeyHint="search"
          autoComplete="off"
          leading={
            <IconButton aria-label="Đóng tìm kiếm" title="Quay lại" shape="round" size="lg" onClick={onClose}>
              <ArrowLeft size={20} strokeWidth={1.75} />
            </IconButton>
          }
          trailing={query ? (
            <IconButton
              aria-label="Xoá"
              title="Xoá"
              shape="round"
              size="lg"
              onClick={() => {
                setQuery('')
                inputRef.current?.focus()
              }}
            >
              <X size={18} strokeWidth={1.75} />
            </IconButton>
          ) : undefined}
        />
      </form>

      <div role="radiogroup" aria-label="Lọc kết quả" className="shrink-0 flex gap-1.5 px-3 pt-2.5 pb-1.5 overflow-x-auto scrollbar-none">
        {CHIPS.map((c) => {
          const on = c.id === filter
          return (
            <Pressable
              key={c.id}
              role="radio"
              aria-checked={on}
              onClick={() => setFilter(c.id)}
              className={`inline-flex items-center shrink-0 h-9 px-3.5 rounded-full text-small-ui font-semibold transition-colors
                duration-quick motion-reduce:transition-none ${on ? 'bg-accent-wash text-ink ring-1 ring-inset ring-accent' : 'bg-sunk text-ink'}`}
            >
              {c.label}
            </Pressable>
          )
        })}
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto overscroll-y-contain pb-[calc(env(safe-area-inset-bottom,0px)+1rem)]">
        {noWords ? (
          recent.length > 0 && (
            <section>
              <Label>Tìm gần đây</Label>
              <ul className="list-none m-0 p-0">
                {recent.map((q) => (
                  <li key={q} className="flex items-center pr-2">
                    <Pressable
                      aria-label={`Tìm lại ${q}`}
                      onClick={() => reuse(q)}
                      className="flex-1 min-w-0 flex items-center gap-3.5 min-h-14 pl-4 pr-2 text-ink active:bg-hover"
                    >
                      <span className="grid place-items-center w-9 h-9 shrink-0 text-ink-muted" aria-hidden="true">
                        <Clock size={18} strokeWidth={1.75} />
                      </span>
                      <span className="truncate text-md">{q}</span>
                    </Pressable>
                    <IconButton
                      aria-label={`Xoá “${q}” khỏi tìm gần đây`}
                      title="Xoá"
                      shape="round"
                      size="lg"
                      onClick={() => setRecent(removeRecent(me, q))}
                    >
                      <X size={16} strokeWidth={1.75} />
                    </IconButton>
                  </li>
                ))}
              </ul>
            </section>
          )
        ) : (
          <>
            {showClient && !clientEmpty && (
              <section>
                <Label>Nhóm và người</Label>
                <ul className="list-none m-0 p-0">
                  {groups.map((c) => (
                    <li key={c.id}>
                      <Link to="/channels/$channelId" params={{ channelId: c.id }} onClick={remember} className={ROW}>
                        <ConversationIcon conversation={c} size={36} />
                        <span className="min-w-0 grid">
                          <b className="font-medium text-md truncate"><Marked text={c.title} query={typed} /></b>
                          <small className="text-small text-ink-muted truncate">Nhóm · {c.memberCount} thành viên</small>
                        </span>
                      </Link>
                    </li>
                  ))}
                  {persons.map((p) => (
                    <li key={p.userId}>
                      <Pressable onClick={() => openPerson(p)} disabled={createDM.isPending} className={ROW}>
                        <Avatar name={p.name} hueKey={p.userId} src={p.avatarUrl} size={36} />
                        <span className="min-w-0 grid">
                          <b className="font-medium text-md truncate"><Marked text={p.name} query={typed} /></b>
                          <small className="text-small text-ink-muted truncate">{p.role || 'Nhắn tin trực tiếp'}</small>
                        </span>
                      </Pressable>
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {showClient && !showMessages && clientEmpty && (
              <EmptyState compact icon={<SearchX size={24} strokeWidth={1.75} />} text={nothingFound} />
            )}

            {showMessages && (
              <section>
                {(filter === 'all' || search.messages.length > 0) && <Label>Tin nhắn</Label>}
                {!search.active ? (
                  <p className="m-0 px-4 py-3 text-small text-ink-muted">Gõ ít nhất 2 ký tự để tìm tin nhắn.</p>
                ) : search.waiting ? (
                  <div role="status" aria-label="Đang tìm tin nhắn" aria-busy="true" className="grid gap-4 px-4 py-3">
                    {[0, 1, 2].map((i) => (
                      <div key={i} className="grid grid-cols-[36px_minmax(0,1fr)] gap-3.5 items-center">
                        <div className="skeleton w-9 h-9 rounded-full" />
                        <div className="grid gap-1.5">
                          <div className="skeleton h-3.5 w-2/5 rounded-sm" />
                          <div className="skeleton h-3.5 w-4/5 rounded-sm" />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : search.isError ? (
                  <EmptyState
                    compact
                    icon={<CircleAlert size={24} strokeWidth={1.75} className="text-danger" />}
                    text="Chưa tìm được tin nhắn."
                    action={<Button variant="soft" size="sm" onClick={search.retry}>Thử lại</Button>}
                  />
                ) : search.messages.length === 0 ? (
                  <EmptyState
                    compact
                    icon={<SearchX size={24} strokeWidth={1.75} />}
                    text={nothingFound}
                    action={noAccents ? <span className="text-small">Thử gõ có dấu.</span> : undefined}
                  />
                ) : (
                  <>
                    <ul className="list-none m-0 p-0">
                      {search.messages.map((msg) => (
                        <li key={msg.id}>
                          <MessageHit
                            message={msg}
                            conversation={byId.get(msg.channel_id)}
                            people={people}
                            query={typed}
                            onOpen={remember}
                          />
                        </li>
                      ))}
                    </ul>
                    {search.hasMore && (
                      <div className="grid justify-items-center py-3">
                        <Button variant="soft" size="sm" loading={search.loadingMore} onClick={() => void search.loadMore()}>
                          Tải thêm
                        </Button>
                      </div>
                    )}
                  </>
                )}
              </section>
            )}
          </>
        )}
      </div>
    </motion.div>,
    document.body,
  )
}

function MessageHit({ message: msg, conversation, people, query, onOpen }: {
  message: Message
  conversation: Conversation | undefined
  people: PeopleDirectory
  query: string
  onOpen: () => void
}) {
  const sender = displayName(people, msg.sender_id, msg.sender_name)
  // A direct message is named by the person alone; a group adds its name.
  const where = conversation?.kind === 'space' ? `${sender} · ${conversation.title}` : sender
  const text = snippet(messageText(msg.content), query, 160)
  return (
    <Link
      to="/channels/$channelId"
      params={{ channelId: msg.channel_id }}
      search={feedSearch(msg)}
      onClick={onOpen}
      className={`${ROW} items-start`}
    >
      <Avatar name={sender} hueKey={msg.sender_id} src={people.byUserId.get(msg.sender_id)?.avatarUrl} size={36} />
      <span className="min-w-0 grid gap-0.5">
        <b className="font-medium text-md truncate">{where}</b>
        <small className="text-small text-ink-muted line-clamp-2 wrap-anywhere"><Marked text={text} query={query} /></small>
        <small className="text-small text-ink-muted tnum">{formatFeedTime(toMillis(msg.created_at))}</small>
      </span>
    </Link>
  )
}
