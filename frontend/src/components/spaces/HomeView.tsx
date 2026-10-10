import { useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { MessagesSquare, CircleAlert } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useConversations } from '../../hooks/useConversations'
import { usePeople } from '../../hooks/usePeople'
import { useArrivals, type Arrival } from '../../hooks/useArrivals'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useMotionPresets } from '../../lib/motion'
import { personStyle } from '../../lib/person-hue'
import { formatCount, formatListTime } from '../../lib/format'
import { filterConversations, type Conversation, type ConversationFilter } from '../../lib/conversations'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, FilterChip, Heading } from '../primitives'
import { ConversationIcon } from './ConversationIcon'
import { EmptyState } from './EmptyState'
import { CreateSpaceDialog } from './CreateSpaceDialog'
import { NewChatMenu } from './NewChatMenu'

const FILTERS: { id: ConversationFilter; label: string; empty: string }[] = [
  { id: 'all', label: 'Tất cả', empty: 'Chưa có cuộc trò chuyện nào.' },
  { id: 'unread', label: 'Chưa đọc', empty: 'Bạn đã đọc hết mọi cuộc trò chuyện.' },
  { id: 'spaces', label: 'Nhóm', empty: 'Bạn chưa ở nhóm nào.' },
  { id: 'direct', label: 'Trực tiếp', empty: 'Chưa có tin nhắn trực tiếp.' },
]

/**
 * Trang chủ (design/mockups/spaces.html §1): every conversation, newest
 * activity first, unread in bold. When someone writes, the row slides to the
 * top (layout animation) and carries the writer's hue for 2.4s.
 */
export function HomeView() {
  const m = useMotionPresets()
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const { conversations, isLoading, isError, refetch } = useConversations(workspaceId)
  const people = usePeople(workspaceId)
  const me = useAuthStore((s) => s.user?.id)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const [filter, setFilter] = useState<ConversationFilter>('all')
  const [creating, setCreating] = useState(false)

  const { fresh, burst } = useArrivals(conversations, {
    keyOf: (c) => `${c.id}:${c.lastActivity}`,
    authorOf: (c) => c.lastSenderId,
    me,
    ready: !isLoading,
    scope: workspaceId,
  })

  const visible = useMemo(() => filterConversations(conversations, filter), [conversations, filter])
  const current = FILTERS.find((f) => f.id === filter)!

  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-base">
      <header className="flex items-start gap-3 px-5 pt-3.5 pb-1.5">
        <div className="grid gap-px min-w-0 flex-1">
          <Heading as="h1" look="panel">Trang chủ</Heading>
          <span className="text-small text-ink-muted truncate">
            Mọi cuộc trò chuyện của bạn trong {workspaceDisplayName(workspaceName)}
          </span>
        </div>
        {/* The navigator column that holds this menu on desktop is not on a phone. */}
        <div className="lg:hidden shrink-0">
          <NewChatMenu
            placement="bottom-end"
            className="press inline-flex items-center gap-2 h-10 pl-3 pr-4 rounded-xl bg-accent-wash text-sm
              font-semibold text-ink hover:bg-hover"
          />
        </div>
      </header>

      <div role="group" aria-label="Lọc" className="flex flex-wrap gap-1.5 px-5 pt-1 pb-2.5">
        {FILTERS.map((f) => (
          <FilterChip key={f.id} pressed={filter === f.id} onClick={() => setFilter(f.id)}>
            {f.label}
          </FilterChip>
        ))}
      </div>

      {burst && (
        <div role="status" className="mx-5 mb-1 text-small text-ink-muted">
          {burstLine(burst.authors, burst.count, people)}
        </div>
      )}

      <div className="flex-1 min-h-0 overflow-y-auto px-3 pb-4">
        {isError ? (
          <EmptyState
            icon={<CircleAlert size={24} strokeWidth={1.75} />}
            text="Chưa tải được danh sách trò chuyện."
            action={<Button variant="soft" size="sm" onClick={refetch}>Thử lại</Button>}
          />
        ) : isLoading ? (
          <div className="grid gap-0.5" aria-busy="true" aria-label="Đang tải">
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="grid grid-cols-[40px_minmax(0,1fr)] gap-3 items-center p-2.5">
                <div className="skeleton w-10 h-10 rounded-surface" />
                <div className="grid gap-1.5">
                  <div className="skeleton h-3.5 w-2/5 rounded-sm" />
                  <div className="skeleton h-3.5 w-4/5 rounded-sm" />
                </div>
              </div>
            ))}
          </div>
        ) : visible.length === 0 ? (
          <EmptyState
            icon={<MessagesSquare size={24} strokeWidth={1.75} />}
            text={current.empty}
            action={
              filter === 'all' || filter === 'spaces' ? (
                <Button variant="soft" size="sm" onClick={() => setCreating(true)}>Tạo nhóm</Button>
              ) : (
                <Button variant="soft" size="sm" onClick={() => setFilter('all')}>Xem tất cả</Button>
              )
            }
          />
        ) : (
          <ul aria-label="Cuộc trò chuyện" className="grid gap-0.5 list-none m-0 p-0">
            <AnimatePresence initial={false}>
              {visible.map((c) => (
                <motion.li
                  key={c.id}
                  layout={m.layoutProp}
                  transition={m.layout}
                  {...m.row}
                >
                  <ConversationRow
                    conversation={c}
                    arrival={fresh.get(`${c.id}:${c.lastActivity}`)}
                    online={!!c.partner && !!onlineUsers[c.partner.userId]}
                  />
                </motion.li>
              ))}
            </AnimatePresence>
          </ul>
        )}
      </div>

      <CreateSpaceDialog open={creating} onClose={() => setCreating(false)} />
    </div>
  )
}

function ConversationRow({ conversation: c, arrival, online }: {
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
        <span className={`flex items-center gap-1.5 min-w-0 ${unread ? 'font-semibold' : 'font-medium'}`}>
          <span className="truncate">{c.title}</span>
          {byOther && <span className="rt-tag shrink-0 text-xs font-semibold">vừa nhắn</span>}
        </span>
        <span className={`truncate ${unread ? 'font-semibold text-ink' : 'text-ink-muted'}`}>{preview}</span>
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

function burstLine(authors: string[], count: number, people: PeopleDirectory): string {
  const first = displayName(people, authors[0])
  const others = Math.max(authors.length - 1, 0)
  if (others === 0) return `${first} vừa gửi ${count} tin mới`
  return `${first} và ${others} người khác vừa cập nhật`
}
