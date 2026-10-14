import { useEffect, useMemo, useRef } from 'react'
import { AnimatePresence } from 'motion/react'
import { useAuthStore } from '../../stores/auth.store'
import type { FeedMessage } from '../../api/shortcuts'
import type { Conversation } from '../../lib/conversations'
import { toMillis } from '../../lib/format'
import { groupByDay } from '../../lib/shortcut-feed'
import { useArrivals } from '../../hooks/useArrivals'
import { usePeople } from '../../hooks/usePeople'
import { avatarOf } from '../../lib/people'
import { useMentionsFeed, useMentionsUnreadCount, useOpenMention, useThreadRoots } from '../../hooks/useShortcuts'
import { Heading } from '../primitives'
import { BackBar, FeedCard, FeedEmpty, FeedError, FeedFooter, FeedSkeleton } from './FeedParts'

/**
 * Được nhắc tới (design/mockups/shortcuts.html §1): every message that mentions
 * the reader, main messages and thread replies, newest first under day
 * dividers. Looking at the list marks nothing read; opening an item does.
 */
export function MentionsView({ workspaceId, conversationOf, onOpen, onHome, onBack, hrefOf, phone = false }: {
  workspaceId: string
  /** The conversation a channel id stands for; its name and icon head each card. */
  conversationOf: (channelId: string) => Conversation | undefined
  /** Open the message (and, for a reply, its thread). The caller routes. */
  onOpen: (item: FeedMessage) => void
  /** "Về Trang chủ" (phone: "Về Tin nhắn") of the empty state. */
  onHome: () => void
  /** Phone full screen: the Quay lại bar. Omit on desktop. */
  onBack?: () => void
  /** A real link target for each card, so it can open in a new tab. */
  hrefOf?: (item: FeedMessage) => string
  phone?: boolean
}) {
  const me = useAuthStore((s) => s.user?.id)
  const feed = useMentionsFeed(workspaceId)
  const people = usePeople(workspaceId)
  const unread = useMentionsUnreadCount(workspaceId).data
  const opened = useOpenMention()
  const items = useMemo(() => feed.data?.pages.flatMap((p) => p.messages) ?? [], [feed.data])
  const roots = useThreadRoots(items)
  // A mention that lands while the list is open grows in at the top, washed in its sender's colour.
  // What the first page held, and the older pages "Tải thêm" adds below it, are history, not news.
  const newest = useRef<number | null>(null)
  const { fresh } = useArrivals(items, {
    keyOf: (m) => m.id,
    authorOf: (m) => m.sender_id,
    me,
    ready: feed.isSuccess,
    scope: workspaceId,
    isHistory: (m) => newest.current !== null && toMillis(m.created_at) <= newest.current,
  })
  useEffect(() => {
    if (!feed.isSuccess) return
    newest.current = Math.max(newest.current ?? 0, ...items.map((m) => toMillis(m.created_at)))
  }, [feed.isSuccess, items])
  // Day labels are relative to now; recompute whenever the feed does.
  const now = useMemo(() => new Date(), [feed.dataUpdatedAt])
  const groups = useMemo(() => groupByDay(items, now), [items, now])
  const sub = 'tin nhắc tên bạn trong mọi cuộc trò chuyện bạn đọc được'

  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-base">
      {onBack && <BackBar title="Được nhắc tới" onBack={onBack} />}
      <header className="grid gap-px px-5 pt-3.5 pb-2">
        <Heading as="h1" look="panel">Được nhắc tới</Heading>
        <span className="text-small text-ink-muted">{typeof unread === 'number' ? `${unread} chưa đọc · ${sub}` : sub}</span>
      </header>
      <div className="flex-1 min-h-0 overflow-y-auto px-2 lg:px-4 pb-6">
        {feed.isPending ? (
          <div aria-busy="true"><span className="sr-only">Đang tải</span><FeedSkeleton rows={4} /></div>
        ) : feed.isError && !feed.data ? (
          <FeedError text="Không tải được tin nhắc tên bạn." onRetry={() => void feed.refetch()} />
        ) : items.length === 0 ? (
          <FeedEmpty
            kind="mention"
            text="Chưa ai nhắc tên bạn. Khi có người gõ @tên bạn, tin sẽ hiện ở đây."
            homeLabel={phone ? 'Về Tin nhắn' : 'Về Trang chủ'}
            onHome={onHome}
          />
        ) : (
          <>
            {groups.map((g, gi) => (
              <div key={`${g.label}-${gi}`}>
                <h4
                  id={`mention-day-${gi}`}
                  className="flex items-center gap-3 px-3 pt-4 pb-1 text-xs font-semibold text-ink-muted
                    before:flex-1 before:h-px before:bg-line after:flex-1 after:h-px after:bg-line"
                >
                  {g.label}
                </h4>
                <ul aria-labelledby={`mention-day-${gi}`} className="m-0 p-0 grid gap-0.5">
                  <AnimatePresence initial={false}>
                    {g.items.map((m) => (
                      <FeedCard
                        key={m.id}
                        item={m}
                        kind="mention"
                        avatarUrl={avatarOf(people, m.sender_id)}
                        conversation={conversationOf(m.channel_id)}
                        me={me}
                        now={now}
                        root={m.parent_message_id ? roots.get(m.parent_message_id) : undefined}
                        href={hrefOf?.(m)}
                        arrival={fresh.get(m.id)}
                        onOpen={(item) => { opened(item); onOpen(item) }}
                      />
                    ))}
                  </AnimatePresence>
                </ul>
              </div>
            ))}
            <FeedFooter
              hasMore={!!feed.hasNextPage}
              loadingMore={feed.isFetchingNextPage}
              moreFailed={feed.isFetchNextPageError}
              onMore={() => void feed.fetchNextPage()}
              doneText="Đã hiện hết tin nhắc tên bạn"
            />
          </>
        )}
      </div>
    </div>
  )
}
