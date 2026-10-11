import { useMemo } from 'react'
import { Star } from 'lucide-react'
import { useAuthStore } from '../../stores/auth.store'
import type { FeedMessage } from '../../api/shortcuts'
import type { Conversation } from '../../lib/conversations'
import { formatCount } from '../../lib/format'
import { useStarConversation, useStarMessage, useStarredFeed, useThreadRoots } from '../../hooks/useShortcuts'
import { Heading, Pressable } from '../primitives'
import { BackBar, FeedCard, FeedEmpty, FeedError, FeedFooter, FeedSkeleton } from './FeedParts'
import { ConversationIcon } from './ConversationIcon'

const MINI_EMPTY = 'mx-1 rounded-xl bg-sunk px-3.5 py-3 text-small text-ink-muted'

function Count({ n, more }: { n: number; more?: boolean }) {
  return <span className="inline-flex items-center h-5 px-2 rounded-full bg-hover text-xs text-ink-muted tnum">{formatCount(n)}{more ? '+' : ''}</span>
}

/**
 * Đã gắn sao (design/mockups/shortcuts.html §2): the conversations I starred
 * (rows like the navigator's, from the list the caller already holds) and the
 * messages I starred (cards, newest star first, paged 25 at a time).
 */
export function StarredView({ workspaceId, starredConversations, conversationOf, onOpen, onOpenConversation, onHome, onBack, hrefOf, phone = false }: {
  workspaceId: string
  /** The starred conversations, in the order of the navigator's "Đã gắn sao" section. */
  starredConversations: Conversation[]
  /** The conversation a channel id stands for; its name and icon head each message card. */
  conversationOf: (channelId: string) => Conversation | undefined
  /** Open a starred message (and, for a reply, its thread). The caller routes. */
  onOpen: (item: FeedMessage) => void
  onOpenConversation: (c: Conversation) => void
  onHome: () => void
  onBack?: () => void
  hrefOf?: (item: FeedMessage) => string
  phone?: boolean
}) {
  const me = useAuthStore((s) => s.user?.id)
  const feed = useStarredFeed(workspaceId)
  const starMessage = useStarMessage()
  const starConversation = useStarConversation()
  const items = useMemo(() => feed.data?.pages.flatMap((p) => p.messages) ?? [], [feed.data])
  const roots = useThreadRoots(items)
  // Day labels are relative to now; recompute whenever the feed does.
  const now = useMemo(() => new Date(), [feed.dataUpdatedAt])
  const nothing = starredConversations.length === 0 && items.length === 0 && !feed.hasNextPage

  return (
    <div className="flex-1 flex flex-col min-h-0 min-w-0 bg-base">
      {onBack && <BackBar title="Đã gắn sao" onBack={onBack} />}
      <header className="grid gap-px px-5 pt-3.5 pb-2">
        <Heading as="h1" look="panel">Đã gắn sao</Heading>
        <span className="text-small text-ink-muted">Cuộc trò chuyện và tin nhắn bạn đã gắn sao</span>
      </header>
      <div className="flex-1 min-h-0 overflow-y-auto px-2 lg:px-4 pb-6">
        {feed.isPending ? (
          <div aria-busy="true"><span className="sr-only">Đang tải</span><FeedSkeleton rows={3} /></div>
        ) : feed.isError && !feed.data ? (
          <FeedError text="Không tải được mục đã gắn sao." onRetry={() => void feed.refetch()} />
        ) : nothing ? (
          <FeedEmpty
            kind="star"
            text="Chưa gắn sao gì. Gắn sao cuộc trò chuyện bằng menu ⋯ của hàng, gắn sao tin bằng Thêm → Gắn sao tin."
            homeLabel={phone ? 'Về Tin nhắn' : 'Về Trang chủ'}
            onHome={onHome}
          />
        ) : (
          <>
            <div className="flex items-center gap-2 px-3 pt-3 pb-1.5">
              <h4 id="starred-conversations" className="m-0 text-small font-semibold text-ink-muted">Cuộc trò chuyện</h4>
              <Count n={starredConversations.length} />
            </div>
            {starredConversations.length === 0 ? (
              <p className={MINI_EMPTY}>
                Chưa gắn sao cuộc trò chuyện nào. {phone ? 'Nhấn giữ một cuộc trò chuyện' : 'Mở menu ⋯ của một cuộc trò chuyện'} rồi chọn Gắn sao.
              </p>
            ) : (
              <ul aria-labelledby="starred-conversations" className="m-0 p-0 grid gap-0.5">
                {starredConversations.map((c) => (
                  <li key={c.id} className="relative list-none">
                    <a
                      href="#"
                      onClick={(e) => { e.preventDefault(); onOpenConversation(c) }}
                      data-unread={c.unread > 0 || undefined}
                      className="flex items-center gap-3 min-h-11 pl-3 pr-14 lg:pr-12 py-2 rounded-surface no-underline text-ink
                        focus-ring hover:bg-raised"
                    >
                      <ConversationIcon conversation={c} size={24} />
                      <span className={`truncate text-sm ${c.unread > 0 ? 'font-semibold' : 'font-medium'}`}>{c.title}</span>
                      {c.unread > 0 && (
                        <span
                          className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
                            text-on-accent text-2xs font-semibold"
                          aria-label={`${c.unread} tin chưa đọc`}
                        >
                          {formatCount(c.unread)}
                        </span>
                      )}
                    </a>
                    <Pressable
                      aria-pressed="true"
                      aria-label={`Gắn sao ${c.title}`}
                      title="Bỏ gắn sao"
                      onClick={() => void starConversation({ channelId: c.id, name: c.title, starred: false })}
                      className="absolute top-0 right-1 inline-flex items-center justify-center w-11 h-11 rounded-md
                        text-accent hover:bg-hover"
                    >
                      <Star size={16} strokeWidth={1.75} fill="currentColor" aria-hidden="true" />
                    </Pressable>
                  </li>
                ))}
              </ul>
            )}

            <div className="flex items-center gap-2 px-3 pt-5 pb-1.5">
              <h4 id="starred-messages" className="m-0 text-small font-semibold text-ink-muted">Tin nhắn</h4>
              <Count n={items.length} more={!!feed.hasNextPage} />
            </div>
            {items.length === 0 ? (
              <p className={MINI_EMPTY}>
                Chưa gắn sao tin nào. {phone ? 'Nhấn giữ một tin' : 'Trỏ vào một tin, chọn Thêm'} rồi Gắn sao tin.
              </p>
            ) : (
              <>
                <ul aria-labelledby="starred-messages" className="m-0 p-0 grid gap-0.5">
                  {items.map((m) => (
                    <FeedCard
                      key={m.id}
                      item={m}
                      kind="star"
                      conversation={conversationOf(m.channel_id)}
                      me={me}
                      now={now}
                      root={m.parent_message_id ? roots.get(m.parent_message_id) : undefined}
                      href={hrefOf?.(m)}
                      onOpen={onOpen}
                      onUnstar={(item) => void starMessage({ message: item, starred: false })}
                    />
                  ))}
                </ul>
                <FeedFooter
                  hasMore={!!feed.hasNextPage}
                  loadingMore={feed.isFetchingNextPage}
                  moreFailed={feed.isFetchNextPageError}
                  onMore={() => void feed.fetchNextPage()}
                  doneText="Đã hiện hết tin đã gắn sao"
                />
              </>
            )}
          </>
        )}
      </div>
    </div>
  )
}
