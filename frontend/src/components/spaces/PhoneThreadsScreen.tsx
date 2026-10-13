import { useState } from 'react'
import { CircleAlert, MessagesSquare } from 'lucide-react'
import type { Message } from '../../api/messaging'
import { useFollowedThreads } from '../../hooks/useConversationActivity'
import { useUnreadThreads } from '../../hooks/useUnreadThreads'
import { formatFeedTime, toMillis } from '../../lib/format'
import { messageText } from '../../lib/message-model'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { formatLastReply } from '../../lib/stream-model'
import type { Conversation } from '../../lib/conversations'
import { Avatar, Button, Pressable, Spinner } from '../primitives'
import { ConversationIcon } from './ConversationIcon'
import { EmptyState } from './EmptyState'
import { SidePanel } from './SidePanel'

export type ThreadsChip = 'unread' | 'following'

const CHIPS: { id: ThreadsChip; label: string }[] = [
  { id: 'unread', label: 'Có tin mới' },
  { id: 'following', label: 'Đang theo dõi' },
]

const EMPTY_TEXT: Record<ThreadsChip, string> = {
  unread: 'Không có thread nào có tin mới.',
  following: 'Bạn chưa theo dõi thread nào.',
}

/**
 * The threads of every conversation on a phone (DESIGN.md "Chat trên điện
 * thoại", Thread): a full screen over the list, with the chips Có tin mới (the
 * followed threads with unread replies) and Đang theo dõi (all followed threads,
 * a page at a time). Choosing one opens its conversation on that thread.
 */
export function PhoneThreadsScreen({ initial, conversations, people, onOpen, onClose }: {
  initial: ThreadsChip
  /** The conversations of the list; a thread names its conversation from here. */
  conversations: Pick<Conversation, 'id' | 'kind' | 'title' | 'hueKey' | 'partner'>[]
  people: PeopleDirectory
  onOpen: (channelId: string, rootId: string) => void
  onClose: () => void
}) {
  const [chip, setChip] = useState<ThreadsChip>(initial)
  const unread = useUnreadThreads()
  const followed = useFollowedThreads(chip === 'following')
  const byId = new Map(conversations.map((c) => [c.id, c]))

  const source = chip === 'unread' ? unread : followed
  const threads: Message[] = chip === 'unread' ? (unread.data ?? []) : (followed.data?.pages.flatMap((p) => p.messages) ?? [])
  // A thread of a conversation the list does not hold has no name to show.
  const rows = threads.flatMap((m) => {
    const conversation = byId.get(m.channel_id)
    return conversation ? [{ m, conversation }] : []
  })
  const newCount = unread.data?.length ?? 0

  return (
    <SidePanel label="Thread" title="Thread" closeLabel="danh sách thread" sheetBelow="lg" backOnSheet onClose={onClose}>
      <div role="radiogroup" aria-label="Lọc thread" className="flex gap-1.5 px-3 pb-2.5 overflow-x-auto scrollbar-none">
        {CHIPS.map((c) => {
          const on = c.id === chip
          return (
            <Pressable
              key={c.id}
              role="radio"
              aria-checked={on}
              onClick={() => setChip(c.id)}
              className={`inline-flex items-center gap-1.5 shrink-0 h-9 px-3.5 rounded-full text-small-ui transition-colors duration-quick
                motion-reduce:transition-none ${on ? 'bg-accent-wash text-ink font-semibold ring-1 ring-inset ring-accent' : 'bg-sunk text-ink'}`}
            >
              {c.label}
              {c.id === 'unread' && newCount > 0 && ' '}
              {c.id === 'unread' && newCount > 0 && (
                <span
                  aria-label={`${newCount} thread`}
                  className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent
                    text-2xs font-semibold tnum"
                >
                  {newCount}
                </span>
              )}
            </Pressable>
          )
        })}
      </div>

      {source.isPending ? (
        <div className="grid gap-4 p-3" aria-busy="true" aria-label="Đang tải danh sách thread">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="grid gap-2">
              <div className="skeleton h-3.5 w-2/5 rounded-sm" />
              <div className="skeleton h-3.5 w-11/12 rounded-sm" />
            </div>
          ))}
        </div>
      ) : source.isError ? (
        <EmptyState
          compact
          icon={<CircleAlert size={24} strokeWidth={1.75} className="text-danger" />}
          text="Không tải được danh sách thread."
          action={<Button variant="soft" size="sm" onClick={() => void source.refetch()}>Thử lại</Button>}
        />
      ) : rows.length === 0 ? (
        <EmptyState compact icon={<MessagesSquare size={24} strokeWidth={1.75} />} text={EMPTY_TEXT[chip]} />
      ) : (
        <ul aria-label="Thread" className="grid gap-0.5 m-0 p-0 px-1.5 list-none">
          {rows.map(({ m, conversation }) => (
            <li key={m.id}>
              <ThreadRow message={m} conversation={conversation} people={people} onOpen={() => onOpen(m.channel_id, m.id)} />
            </li>
          ))}
        </ul>
      )}

      {chip === 'following' && followed.hasNextPage && (
        <div className="grid justify-items-center py-3">
          <Button variant="soft" size="sm" disabled={followed.isFetchingNextPage} onClick={() => void followed.fetchNextPage()}>
            {followed.isFetchingNextPage && <Spinner size="sm" />}
            Xem thêm thread
          </Button>
        </div>
      )}
    </SidePanel>
  )
}

function ThreadRow({ message: m, conversation, people, onOpen }: {
  message: Message
  conversation: Pick<Conversation, 'kind' | 'title' | 'hueKey' | 'partner'>
  people: PeopleDirectory
  onOpen: () => void
}) {
  const author = displayName(people, m.sender_id, m.sender_name)
  const news = Math.max(0, m.thread_unread_count ?? 0)
  const replies = m.reply_count ?? 0
  const last = formatLastReply(m.last_reply_at)
  const faces = [...new Set(m.reply_participant_ids ?? [])].slice(0, 3)
  const label = `Thread của ${author} trong ${conversation.title}${replies ? `, ${replies} trả lời` : ''}${news ? `, ${news} trả lời mới` : ''}`
  const at = toMillis(m.last_reply_at) || toMillis(m.created_at)

  return (
    <Pressable
      onClick={onOpen}
      aria-label={label}
      className="grid grid-cols-[24px_minmax(0,1fr)] gap-x-3 w-full min-h-16 p-3 rounded-surface text-ink
        transition-colors duration-quick hover:bg-hover active:bg-hover motion-reduce:transition-none"
    >
      <ConversationIcon conversation={conversation} size={24} />
      <span className="grid gap-1 min-w-0">
        <span className="flex items-baseline gap-2 min-w-0 text-xs text-ink-muted">
          <span className="truncate">{conversation.title}</span>
          <span className="ml-auto shrink-0 tnum">{formatFeedTime(at)}</span>
        </span>
        <span className="line-clamp-2 wrap-anywhere"><b className="font-semibold">{author}:</b> {messageText(m.content)}</span>
        <span className="flex items-center gap-2 text-small text-ink-muted">
          {faces.length > 0 && (
            <span className="flex items-center shrink-0" aria-hidden="true">
              {faces.map((id, i) => (
                <Avatar
                  key={id} name={displayName(people, id)} hueKey={id} src={people.byUserId.get(id)?.avatarUrl}
                  size={20} className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`}
                />
              ))}
            </span>
          )}
          {replies > 0 && <span className="font-semibold text-accent whitespace-nowrap">{replies} trả lời</span>}
          {last && <span className="truncate tnum">Lần cuối {last}</span>}
          {news > 0 && (
            <span
              aria-hidden="true"
              className="ml-auto inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
                text-on-accent text-2xs font-semibold tnum shrink-0"
            >
              {news} mới
            </span>
          )}
        </span>
      </span>
    </Pressable>
  )
}
