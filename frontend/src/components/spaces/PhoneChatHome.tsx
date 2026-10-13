import { useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { CircleAlert } from 'lucide-react'
import type { Arrival } from '../../hooks/useArrivals'
import type { ChatNavigation } from '../../hooks/useChatNavigation'
import { usePeople } from '../../hooks/usePeople'
import { usePullToRefresh } from '../../hooks/usePullToRefresh'
import { useUnreadThreads } from '../../hooks/useUnreadThreads'
import { keys } from '../../hooks/keys'
import { chatTabOf } from '../../lib/nav-destinations'
import { queryClient } from '../../lib/query-client'
import { collapseKey, useNavCollapseStore } from '../../stores/nav-collapse.store'
import { Button, Spinner } from '../primitives'
import { EmptyState } from './EmptyState'
import { PhoneHomeFeed } from './PhoneHomeFeed'
import { PhoneMoreList } from './PhoneMoreList'
import { PhoneSectionsList } from './PhoneSectionsList'
import { PhoneThreadsScreen, type ThreadsChip } from './PhoneThreadsScreen'
import { useSectionOverlays } from './SectionOverlays'

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
 * Chat below 1024px (DESIGN.md "Chat trên điện thoại"): the tab named by
 * `?tab=` — Trang chủ, Trực tiếp, Mục or Thêm — in one scrolling column that
 * reloads when pulled down from the top, and the full-screen Thread list over
 * any of them. The shell around it (search pill, bottom bar, +) is not its job.
 */
export function PhoneChatHome({ nav, workspaceId, fresh }: {
  nav: ChatNavigation
  workspaceId: string
  fresh: Map<string, Arrival>
}) {
  const tab = chatTabOf(useSearch({ strict: false }) as { tab?: unknown })
  const navigate = useNavigate()
  const people = usePeople(workspaceId)
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const overlays = useSectionOverlays(nav.sectionsApi, true, (id) => setCollapsed(collapseKey(workspaceId, id), false))
  const [threads, setThreads] = useState<ThreadsChip | null>(null)
  const threadsWithNews = useUnreadThreads().data?.length ?? 0
  const { bind, pull, refreshing } = usePullToRefresh(() => reloadLists(workspaceId))

  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-base">
      <div {...bind} className="relative flex-1 min-h-0 overflow-y-auto overscroll-y-contain">
        {(pull > 0 || refreshing) && (
          <div
            role="status"
            aria-label="Đang tải lại"
            style={{ height: refreshing ? 40 : pull }}
            className="grid place-items-center overflow-hidden"
          >
            <Spinner size="md" />
          </div>
        )}

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
        ) : tab === 'more' ? (
          <PhoneMoreList
            workspaceId={workspaceId}
            threadsWithNews={threadsWithNews}
            onOpenFollowedThreads={() => setThreads('following')}
          />
        ) : (
          <PhoneHomeFeed
            key={tab}
            scope={tab === 'direct' ? 'direct' : 'home'}
            conversations={tab === 'direct' ? nav.directs : nav.conversations}
            fresh={fresh}
            overlays={overlays}
            threadsWithNews={threadsWithNews}
            onOpenThreads={() => setThreads('unread')}
          />
        )}
      </div>

      {threads && (
        <PhoneThreadsScreen
          initial={threads}
          conversations={nav.conversations}
          people={people}
          onClose={() => setThreads(null)}
          onOpen={(channelId, rootId) =>
            void navigate({ to: '/channels/$channelId', params: { channelId }, search: { thread: rootId } })}
        />
      )}
      {overlays.node}
    </div>
  )
}
