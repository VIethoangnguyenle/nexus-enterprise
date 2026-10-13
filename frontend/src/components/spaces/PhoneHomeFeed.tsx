import { Check, MessagesSquare } from 'lucide-react'
import type { Arrival } from '../../hooks/useArrivals'
import { usePersistedFlag } from '../../hooks/usePersistedFlag'
import { useAuthStore } from '../../stores/auth.store'
import { useWebSocketStore } from '../../stores/websocket.store'
import { filterConversations, type Conversation } from '../../lib/conversations'
import { EmptyState } from './EmptyState'
import { FeedRow } from './FeedRow'
import { PressableRow } from './PressableRow'
import { PhoneListTitle, ThreadButton, UnreadToggle } from './PhoneListControls'
import { rowTargetOf, type SectionOverlays } from './SectionOverlays'

export type FeedScope = 'home' | 'direct'

const COPY: Record<FeedScope, { title: string; empty: string }> = {
  home: { title: 'Trang chủ', empty: 'Chưa có cuộc trò chuyện nào. Chạm + để bắt đầu.' },
  direct: { title: 'Trực tiếp', empty: 'Chưa có tin nhắn trực tiếp. Chạm + để nhắn cho một người.' },
}

/**
 * Trang chủ and Trực tiếp on a phone (DESIGN.md "Chat trên điện thoại"): the
 * title, the Thread button (Trang chủ only), the unread-only pill (remembered on
 * this device for this person), and the conversations newest activity first as
 * `FeedRow`s. Long-pressing a row opens its sheet.
 */
export function PhoneHomeFeed({ scope, conversations, fresh, overlays, threadsWithNews, onOpenThreads }: {
  scope: FeedScope
  /** The conversations of this tab, newest activity first. */
  conversations: Conversation[]
  fresh: Map<string, Arrival>
  overlays: SectionOverlays
  /** Followed threads with unread replies: the number on the Thread button. */
  threadsWithNews: number
  onOpenThreads: () => void
}) {
  const me = useAuthStore((s) => s.user?.id ?? '')
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const [unreadOnly, setUnreadOnly] = usePersistedFlag(`ngac-feed-unread-only:${me}:${scope}`)
  const copy = COPY[scope]

  const shown = unreadOnly ? filterConversations(conversations, 'unread') : conversations

  return (
    <div className="grid">
      <PhoneListTitle title={copy.title}>
        {scope === 'home' && <ThreadButton news={threadsWithNews} onClick={onOpenThreads} />}
        <UnreadToggle pressed={unreadOnly} onChange={setUnreadOnly} label="Chỉ hiện chưa đọc" />
      </PhoneListTitle>

      {conversations.length === 0 ? (
        <EmptyState icon={<MessagesSquare size={24} strokeWidth={1.75} />} text={copy.empty} />
      ) : shown.length === 0 ? (
        <div role="status" className="grid justify-items-center gap-2 px-8 py-14 text-center text-ink-muted">
          <Check size={32} strokeWidth={1.75} aria-hidden="true" className="text-accent" />
          <b className="text-ink">Bạn đã đọc hết</b>
          <span>Tắt bộ lọc để xem mọi cuộc trò chuyện.</span>
        </div>
      ) : (
        <ul aria-label="Cuộc trò chuyện" className="grid list-none m-0 p-0">
          {shown.map((c) => (
            <PressableRow
              key={c.id}
              // A muted conversation does not slide to the top when it speaks: silence is what muting means.
              slide={c.level !== 'off'}
              onMenu={(el) => overlays.openRowMenu(rowTargetOf(c), el)}
            >
              <FeedRow
                conversation={c}
                arrival={fresh.get(`${c.id}:${c.lastActivity}`)}
                online={!!c.partner && !!onlineUsers[c.partner.userId]}
              />
            </PressableRow>
          ))}
        </ul>
      )}
    </div>
  )
}
