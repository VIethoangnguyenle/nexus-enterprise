import { useEffect, useRef } from 'react'
import { Check, MessagesSquare } from 'lucide-react'
import type { Arrival } from '../../hooks/useArrivals'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { useWebSocketStore } from '../../stores/websocket.store'
import { filterConversations, orderConversations, type Conversation, type ConversationFilter } from '../../lib/conversations'
import { SwipeRow, SwipeRowGroup } from '../patterns/SwipeRow'
import { useChatRowSwipe } from './chat-row-swipe'
import { EmptyState } from './EmptyState'
import { FeedRow } from './FeedRow'
import { PressableRow } from './PressableRow'
import { rowTargetOf, type SectionOverlays } from './SectionOverlays'

export type FeedScope = 'all' | 'unread' | 'direct' | 'spaces'

const COPY: Record<FeedScope, { empty: string; filter: ConversationFilter }> = {
  all: { empty: 'Chưa có cuộc trò chuyện nào. Chạm + để bắt đầu.', filter: 'all' },
  unread: { empty: 'Chưa có cuộc trò chuyện nào. Chạm + để bắt đầu.', filter: 'unread' },
  direct: { empty: 'Chưa có tin nhắn trực tiếp. Chạm + để nhắn cho một người.', filter: 'direct' },
  spaces: { empty: 'Bạn chưa ở nhóm nào. Chạm + để tạo nhóm.', filter: 'spaces' },
}

/**
 * The list under the chips of Chat on a phone (DESIGN.md "Chat trên điện thoại"): Tất cả, Chưa đọc, Trực
 * tiếp or Nhóm, the conversations of that chip as `FeedRow`s, newest activity first or by name (`sort`, Cài
 * đặt Chat → Sắp xếp). Long-pressing a row
 * opens its sheet. Chưa đọc with nothing left says "Bạn đã đọc hết".
 *
 * A swipe on a row reveals its buttons (`useChatRowSwipe`); one row is open at a time.
 */
export function PhoneHomeFeed({ scope, conversations, fresh, overlays, sort = 'activity' }: {
  scope: FeedScope
  sort?: 'activity' | 'name'
  /** Every conversation, newest activity first; the chip's share of them is picked here. */
  conversations: Conversation[]
  fresh: Map<string, Arrival>
  overlays: SectionOverlays
}) {
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const copy = COPY[scope]
  const phone = useBottomBarLayout()
  const swipeOf = useChatRowSwipe(overlays)

  // The rows in the list as it mounts are drawn at full size: the device back remounts this list, and rows
  // growing in from nothing are a blank list for its first frames. A row that arrives later still enters.
  const listed = useRef(false)
  useEffect(() => { listed.current = true }, [])

  const shown = orderConversations(filterConversations(conversations, copy.filter), sort)

  return (
    <div className="grid">
      {shown.length === 0 ? (
        scope === 'unread' && conversations.length > 0 ? (
          <div role="status" className="grid justify-items-center gap-2 px-8 py-14 text-center text-ink-muted">
            <Check size={32} strokeWidth={1.75} aria-hidden="true" className="text-accent" />
            <b className="text-ink">Bạn đã đọc hết</b>
            <span>Chọn Tất cả để xem mọi cuộc trò chuyện.</span>
          </div>
        ) : (
          <EmptyState icon={<MessagesSquare size={24} strokeWidth={1.75} />} text={copy.empty} />
        )
      ) : (
        <SwipeRowGroup>
          <ul aria-label="Cuộc trò chuyện" className="grid list-none m-0 p-0">
            {shown.map((c) => (
              <PressableRow
                key={c.id}
                // A muted conversation does not slide to the top when it speaks: silence is what muting means.
                slide={c.level !== 'off'}
                enter={listed.current}
                onMenu={(el) => overlays.openRowMenu(rowTargetOf(c), el)}
              >
                <SwipeRow id={c.id} disabled={!phone} fullSwipe {...swipeOf(c)}>
                  <FeedRow
                    conversation={c}
                    arrival={fresh.get(`${c.id}:${c.lastActivity}`)}
                    online={!!c.partner && !!onlineUsers[c.partner.userId]}
                  />
                </SwipeRow>
              </PressableRow>
            ))}
          </ul>
        </SwipeRowGroup>
      )}
    </div>
  )
}
