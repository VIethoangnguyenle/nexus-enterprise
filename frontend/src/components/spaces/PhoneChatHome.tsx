import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { AnimatePresence } from 'motion/react'
import { CircleAlert } from 'lucide-react'
import type { Arrival } from '../../hooks/useArrivals'
import type { ChatNavigation } from '../../hooks/useChatNavigation'
import { useChatChip } from '../../hooks/useChatChip'
import { useChatOverlay } from '../../hooks/useChatOverlay'
import { useModuleSettings } from '../../hooks/useModuleSettings'
import { usePeople } from '../../hooks/usePeople'
import { useShortcutScreen } from '../../hooks/useShortcutScreen'
import { useMentionsUnreadCount } from '../../hooks/useShortcuts'
import { useUnreadThreads } from '../../hooks/useUnreadThreads'
import { keys } from '../../hooks/keys'
import { filterConversations } from '../../lib/conversations'
import { queryClient } from '../../lib/query-client'
import { collapseKey, useNavCollapseStore } from '../../stores/nav-collapse.store'
import { Button } from '../primitives'
import { ModuleMenu } from '../patterns/ModuleMenu'
import { PullToRefresh } from '../patterns/PullToRefresh'
import { EmptyState } from './EmptyState'
import { PhoneHomeFeed } from './PhoneHomeFeed'
import { ChatChips } from './ChatChips'
import { MentionsView } from './MentionsView'
import { PhoneListTitle, ThreadButton } from './PhoneListControls'
import { PhoneSectionsList } from './PhoneSectionsList'
import { PhoneThreadsScreen } from './PhoneThreadsScreen'
import { ChatSearchScreen } from './ChatSearchScreen'
import { useSectionOverlays, type RowTarget } from './SectionOverlays'
import { LeaveGroupDialog } from './LeaveGroupDialog'

/** Everything the conversation lists show, asked for again: the pull-down reload. */
function reloadLists(workspaceId: string): Promise<unknown> {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.messaging.channels(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: keys.messaging.dms() }),
    queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() }),
    queryClient.invalidateQueries({ queryKey: keys.messaging.conversationActivity() }),
    queryClient.invalidateQueries({ queryKey: keys.messaging.followedThreads() }),
    queryClient.invalidateQueries({ queryKey: keys.notificationLevels.unreadThreads() }),
  ])
}

/**
 * Chat below 1024px (DESIGN.md "Chat trên điện thoại"): the title with Thread, the chips (Tất cả · Mục ·
 * Chưa đọc · Được nhắc · Trực tiếp · Nhóm) and the list the chosen chip names, in one scrolling column that
 * reloads when pulled down from the top, and the full-screen Thread list and search over any of them (both
 * live in the address, so the system back gesture closes them). The chip is the address's `?tab=` or the one
 * this device remembers. The shell around it (search pill, bottom bar, +) is not its job.
 */
export function PhoneChatHome({ nav, workspaceId, fresh }: {
  nav: ChatNavigation
  workspaceId: string
  fresh: Map<string, Arrival>
}) {
  const { chip: tab, select } = useChatChip()
  const navigate = useNavigate()
  const people = usePeople(workspaceId)
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const [leaveTarget, setLeaveTarget] = useState<RowTarget | null>(null)
  const overlays = useSectionOverlays(nav.sectionsApi, true, (id) => setCollapsed(collapseKey(workspaceId, id), false), (c) => setLeaveTarget(c))
  const { overlay, openThreads, close } = useChatOverlay()
  const threadsWithNews = useUnreadThreads().data?.length ?? 0
  const mentionsUnread = useMentionsUnreadCount(workspaceId).data ?? 0
  const { sort } = useModuleSettings('chat', workspaceId)

  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-base">
      <PullToRefresh onRefresh={() => reloadLists(workspaceId)} className="relative flex-1 min-h-0 overflow-y-auto overscroll-y-contain">
        <PhoneListTitle title="Chat">
          <ThreadButton news={threadsWithNews} onClick={() => openThreads('unread')} />
          <ModuleMenu section="chat" label="Chat" />
        </PhoneListTitle>
        <ChatChips
          current={tab}
          onSelect={select}
          counts={{ unread: filterConversations(nav.conversations, 'unread').length, mentions: mentionsUnread }}
        />

        {nav.isError ? (
          <EmptyState
            icon={<CircleAlert size={24} strokeWidth={1.75} />}
            text="Chưa tải được danh sách trò chuyện."
            action={<Button variant="soft" size="sm" onClick={nav.refetch}>Thử lại</Button>}
          />
        ) : nav.isLoading ? (
          <div className="grid gap-3 p-4" aria-busy="true" aria-label="Đang tải">
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="grid grid-cols-[44px_minmax(0,1fr)] gap-3.5 items-center">
                <div className="skeleton w-11 h-11 rounded-surface" />
                <div className="grid gap-1.5">
                  <div className="skeleton h-3.5 w-2/5 rounded-sm" />
                  <div className="skeleton h-3.5 w-4/5 rounded-sm" />
                </div>
              </div>
            ))}
          </div>
        ) : tab === 'sections' ? (
          <PhoneSectionsList nav={nav} workspaceId={workspaceId} overlays={overlays} />
        ) : tab === 'mentions' ? (
          <ChipMentions onAll={() => select('all')} />
        ) : (
          <PhoneHomeFeed key={tab} scope={tab} sort={sort} conversations={nav.conversations} fresh={fresh} overlays={overlays} />
        )}
      </PullToRefresh>

      {/* Closing runs the opening backwards (☰ back, ring and layer fading), so the layer is drawn until it has. */}
      <AnimatePresence>
        {overlay?.kind === 'search' && (
          <ChatSearchScreen
            key="search"
            conversations={nav.conversations}
            people={people}
            onClose={close}
          />
        )}
      </AnimatePresence>
      {overlay?.kind === 'threads' && (
        <PhoneThreadsScreen
          initial={overlay.chip}
          conversations={nav.conversations}
          people={people}
          onClose={close}
          onOpen={(channelId, rootId) =>
            void navigate({ to: '/channels/$channelId', params: { channelId }, search: { thread: rootId } })}
        />
      )}
      {overlays.node}
      {leaveTarget && (
        <LeaveGroupDialog
          open
          channelId={leaveTarget.id}
          workspaceId={workspaceId}
          groupName={leaveTarget.title}
          onClose={() => setLeaveTarget(null)}
        />
      )}
    </div>
  )
}

/** Được nhắc: the mentions feed of `/channels/mentions`, inside the list under the chips. */
function ChipMentions({ onAll }: { onAll: () => void }) {
  const s = useShortcutScreen()
  return (
    <MentionsView
      embedded
      phone
      workspaceId={s.workspaceId}
      conversationOf={s.conversationOf}
      onOpen={s.open}
      onHome={onAll}
      hrefOf={s.hrefOf}
    />
  )
}
