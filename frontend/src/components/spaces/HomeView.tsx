import { useMemo, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { MessagesSquare, CircleAlert } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { usePeople } from '../../hooks/usePeople'
import { useArrivals } from '../../hooks/useArrivals'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { collapseKey, useNavCollapseStore } from '../../stores/nav-collapse.store'
import { useMotionPresets } from '../../lib/motion'
import { filterConversations, type ConversationFilter } from '../../lib/conversations'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { workspaceDisplayName } from '../../lib/workspace'
import { Button, FilterChip, Heading } from '../primitives'
import { ConversationRow } from './ConversationRow'
import { PhoneChatList } from './PhoneChatList'
import { useSectionOverlays } from './SectionOverlays'
import { EmptyState } from './EmptyState'
import { CreateSpaceDialog } from './CreateSpaceDialog'
import { NewChatMenu } from './NewChatMenu'
import { UnreadThreadsSection } from './UnreadThreadsSection'

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
  const navigate = useNavigate()
  const { workspaceId, workspaceName } = useActiveWorkspace()
  const nav = useChatNavigation(workspaceId)
  const { conversations, isLoading, isError, refetch } = nav
  // Below 1024px the navigator column is not on screen, so this view is the
  // Tin nhắn tab of the phone design: the user's sections, then the default lists.
  const phone = useBottomBarLayout()
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const overlays = useSectionOverlays(nav.sectionsApi, true, (id) => setCollapsed(collapseKey(workspaceId, id), false))
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
    // A muted conversation is quiet: what it receives is neither washed nor counted into the burst line.
    isHistory: (c) => c.level === 'off',
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
            onNewSection={phone ? (el) => overlays.openCreate(el, undefined, 'bottom-end') : undefined}
            placement="bottom-end"
            className="press inline-flex items-center gap-2 h-10 pl-3 pr-4 rounded-xl bg-accent-wash text-sm
              font-semibold text-ink hover:bg-hover"
          />
        </div>
      </header>

      {!phone && <div role="group" aria-label="Lọc" className="flex flex-wrap gap-1.5 px-5 pt-1 pb-2.5">
        {FILTERS.map((f) => (
          <FilterChip key={f.id} pressed={filter === f.id} onClick={() => setFilter(f.id)}>
            {f.label}
          </FilterChip>
        ))}
      </div>}

      {burst && (
        <div role="status" className="mx-5 mb-1 text-small text-ink-muted">
          {burstLine(burst.authors, burst.count, people)}
        </div>
      )}

      <div className="flex-1 min-h-0 overflow-y-auto px-3 pb-4">
        {/* The phone's Tin nhắn tab mounts its own copy inside PhoneChatList. */}
        {!phone && !isError && !isLoading && (
          <UnreadThreadsSection
            conversations={conversations}
            people={people}
            onOpenThread={(channelId, rootId) =>
              void navigate({ to: '/channels/$channelId', params: { channelId }, search: { thread: rootId } })}
          />
        )}
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
        ) : phone && conversations.length > 0 ? (
          <PhoneChatList nav={nav} workspaceId={workspaceId} fresh={fresh} overlays={overlays} />
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
                  layout={c.level === 'off' ? false : m.layoutProp}
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

      {overlays.node}
      <CreateSpaceDialog open={creating} onClose={() => setCreating(false)} />
    </div>
  )
}

function burstLine(authors: string[], count: number, people: PeopleDirectory): string {
  const first = displayName(people, authors[0])
  const others = Math.max(authors.length - 1, 0)
  if (others === 0) return `${first} vừa gửi ${count} tin mới`
  return `${first} và ${others} người khác vừa cập nhật`
}
