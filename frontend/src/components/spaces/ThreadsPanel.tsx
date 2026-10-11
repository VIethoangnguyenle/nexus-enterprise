import { useState } from 'react'
import { BellRing, CircleAlert, FileText, MessagesSquare } from 'lucide-react'
import type { Message, ThreadFilter } from '../../api/messaging'
import { useThreadsList } from '../../hooks/useThreads'
import { previewText } from '../../lib/chat-cache'
import { formatListTime } from '../../lib/format'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { threadSummary } from '../../lib/stream-model'
import { Avatar, Button, Pressable, Spinner } from '../primitives'
import { SidePanel } from './SidePanel'
import { EmptyState } from './EmptyState'

const FILTERS: { id: ThreadFilter; label: string }[] = [
  { id: 'all', label: 'Tất cả' },
  { id: 'following', label: 'Đang theo dõi' },
  { id: 'unread', label: 'Chưa đọc' },
]

const EMPTY_TEXT: Record<ThreadFilter, string> = {
  all: 'Chưa có thread nào. Trả lời một tin để bắt đầu thread.',
  following: 'Bạn chưa theo dõi thread nào.',
  unread: 'Không có thread nào chưa đọc.',
}

interface ThreadsPanelProps {
  channelId: string
  spaceName: string
  people: PeopleDirectory
  /** The thread open beside the list, if any. */
  activeId?: string | null
  onOpenThread: (id: string) => void
  onClose: () => void
}

/**
 * The threads of a conversation (DESIGN.md "Panel Thread của nhóm"): most
 * recently active first, filtered by Tất cả / Đang theo dõi / Chưa đọc, a page
 * at a time. Choosing one opens it. Below 1024 it is a full-screen sheet.
 */
export function ThreadsPanel({ channelId, spaceName, people, activeId, onOpenThread, onClose }: ThreadsPanelProps) {
  const [filter, setFilter] = useState<ThreadFilter>('all')
  const list = useThreadsList(channelId, filter)
  const unread = useThreadsList(channelId, 'unread')
  const threads = list.data?.pages.flatMap((p) => p.messages) ?? []
  const unreadCount = unread.data?.pages.flatMap((p) => p.messages).length ?? 0

  return (
    <SidePanel
      label="Thread của nhóm"
      title="Thread"
      sub={spaceName}
      closeLabel="danh sách thread"
      sheetBelow="lg"
      backOnSheet
      onClose={onClose}
    >
      <div role="radiogroup" aria-label="Lọc thread" className="flex gap-1.5 px-3 pb-2.5 overflow-x-auto scrollbar-none">
        {FILTERS.map((f) => {
          const on = f.id === filter
          return (
            <Pressable
              key={f.id}
              role="radio"
              aria-checked={on}
              onClick={() => setFilter(f.id)}
              className={`inline-flex items-center gap-1.5 shrink-0 h-9 md:h-8 px-3.5 md:px-3 rounded-full md:rounded-md text-small-ui
                transition-colors duration-quick
                ${on ? 'bg-accent-wash text-ink font-semibold' : 'bg-sunk text-ink hover:bg-hover'}`}
            >
              {f.label}
              {f.id === 'unread' && unreadCount > 0 && ' '}
              {f.id === 'unread' && unreadCount > 0 && (
                <span className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum">
                  {unreadCount}
                </span>
              )}
            </Pressable>
          )
        })}
      </div>

      {list.isPending ? (
        <ThreadsSkeleton />
      ) : list.isError ? (
        <EmptyState
          compact
          icon={<CircleAlert size={24} strokeWidth={1.75} className="text-danger" />}
          text="Không tải được danh sách thread."
          action={<Button variant="soft" size="sm" onClick={() => void list.refetch()}>Thử lại</Button>}
        />
      ) : threads.length === 0 ? (
        <EmptyState
          compact
          icon={<MessagesSquare size={24} strokeWidth={1.75} />}
          text={EMPTY_TEXT[filter]}
          action={filter !== 'all' ? <Button variant="soft" size="sm" onClick={() => setFilter('all')}>Xem tất cả</Button> : undefined}
        />
      ) : (
        <ul aria-label="Thread" className="grid gap-0.5 m-0 p-0 px-1.5">
          {threads.map((t) => (
            <li key={t.id} className="list-none">
              <ThreadItem message={t} people={people} active={t.id === activeId} onOpen={() => onOpenThread(t.id)} />
            </li>
          ))}
        </ul>
      )}

      {list.hasNextPage && (
        <div className="grid justify-items-center py-3">
          <Button variant="soft" size="sm" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>
            {list.isFetchingNextPage && <Spinner size="sm" />}
            Xem thêm thread
          </Button>
        </div>
      )}
    </SidePanel>
  )
}

function ThreadItem({ message: t, people, active, onOpen }: {
  message: Message
  people: PeopleDirectory
  active: boolean
  onOpen: () => void
}) {
  const nameOf = (id: string) => displayName(people, id)
  const s = threadSummary(t, nameOf)
  const author = displayName(people, t.sender_id, t.sender_name)
  const file = t.linked_entity_type === 'drive_file'
  const text = previewText(t.content).replace(/^📎\s*/, '')
  return (
    <Pressable
      onClick={onOpen}
      aria-current={active || undefined}
      className={`grid gap-1.5 w-full p-3 rounded-surface transition-colors duration-quick
        ${active ? 'bg-accent-wash' : 'hover:bg-hover'}`}
    >
      <span className="flex items-center gap-2 min-w-0">
        <Avatar name={author} hueKey={t.sender_id} src={people.byUserId.get(t.sender_id)?.avatarUrl} size={24} />
        <span className="font-semibold text-ink truncate">{author}</span>
        <span className="text-xs text-ink-muted tnum shrink-0">{formatListTime(t.created_at)}</span>
        {t.following && (
          <BellRing size={14} strokeWidth={1.75} role="img" aria-label="Đang theo dõi" className="ml-auto shrink-0 text-ink-muted" />
        )}
      </span>
      <span className="line-clamp-2 text-ink wrap-anywhere">
        {text}
        {file && (
          <span className="text-ink-muted">
            {' '}<FileText size={14} strokeWidth={1.75} aria-hidden="true" className="inline align-text-bottom" /> 1 tệp
          </span>
        )}
      </span>
      {s && (
        <span className="flex items-center gap-2 min-w-0 text-small text-ink-muted">
          <span className="flex items-center shrink-0" aria-hidden="true">
            {s.faces.map((f, i) => (
              <Avatar
                key={f.id}
                name={f.name}
                hueKey={f.id}
                src={people.byUserId.get(f.id)?.avatarUrl}
                size={20}
                className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`}
              />
            ))}
          </span>
          <span className="font-semibold text-accent whitespace-nowrap">{s.count} trả lời</span>
          {s.last && (
            <span className="truncate tnum">
              <span className="max-sm:hidden">Lần cuối </span>
              {s.last}
            </span>
          )}
          {s.unread > 0 && (
            <span className="ml-auto inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum shrink-0">
              {s.unread} mới
            </span>
          )}
        </span>
      )}
    </Pressable>
  )
}

function ThreadsSkeleton() {
  return (
    <div className="grid gap-4 p-3" aria-busy="true" aria-label="Đang tải danh sách thread">
      {[0, 1, 2, 3].map((i) => (
        <div key={i} className="grid grid-cols-[1.5rem_minmax(0,1fr)] gap-x-2.5 gap-y-2">
          <div className="skeleton w-6 h-6 rounded-full row-span-3" />
          <div className="skeleton h-3.5 w-2/5 rounded-sm" />
          <div className="skeleton h-3.5 w-11/12 rounded-sm" />
          <div className="skeleton h-3.5 w-1/2 rounded-sm" />
        </div>
      ))}
    </div>
  )
}
