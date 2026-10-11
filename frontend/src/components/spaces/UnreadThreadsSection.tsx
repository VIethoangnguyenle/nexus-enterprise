import { useId, useState } from 'react'
import type { Message } from '../../api/messaging'
import { useUnreadThreads } from '../../hooks/useUnreadThreads'
import type { Conversation } from '../../lib/conversations'
import { displayName, type PeopleDirectory } from '../../lib/people'
import { messageText } from '../../lib/message-model'
import { formatLastReply } from '../../lib/stream-model'
import { Avatar, Button, Pressable } from '../primitives'
import { ConversationIcon } from './ConversationIcon'

const COLLAPSED_KEY = 'ngac-home-threads-collapsed'

/** Remembered per device. Storage can be blocked or full; the section then just forgets. */
function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}
function writeCollapsed(collapsed: boolean) {
  try {
    localStorage.setItem(COLLAPSED_KEY, collapsed ? '1' : '0')
  } catch {
    // Not remembering is the only consequence.
  }
}

interface Props {
  /** The conversations Trang chủ lists; a thread's group is named from here. */
  conversations: Pick<Conversation, 'id' | 'kind' | 'title' | 'hueKey' | 'partner'>[]
  people: PeopleDirectory
  /** Open the group and its thread panel on this root message. */
  onOpenThread: (channelId: string, rootId: string) => void
  /** Rows shown before "Xem thêm": 3 on desktop, 2 on a phone. */
  max?: number
}

/**
 * "Thread có tin mới" (design/mockups/notification-levels.html §1, §4): the
 * threads the person follows that have unread replies, newest reply first, above
 * the conversation list. Hidden while there are none; collapsible, remembered
 * per device. Muted conversations never arrive from the server.
 */
export function UnreadThreadsSection({ conversations, people, onOpenThread, max = 3 }: Props) {
  const { data } = useUnreadThreads()
  const [collapsed, setCollapsed] = useState(readCollapsed)
  const [all, setAll] = useState(false)
  const headingId = useId()
  const listId = useId()

  const byId = new Map(conversations.map((c) => [c.id, c]))
  const rows = (data ?? []).flatMap((m) => {
    const conversation = byId.get(m.channel_id)
    return conversation ? [{ m, conversation }] : []
  })
  if (rows.length === 0) return null

  const shown = all ? rows : rows.slice(0, max)
  const hidden = rows.length - max
  const toggle = () => {
    writeCollapsed(!collapsed)
    setCollapsed(!collapsed)
  }

  return (
    <section aria-labelledby={headingId} className="px-2 pb-2">
      <div className="flex items-center gap-2 min-h-9 pt-2 pl-1">
        <h2 id={headingId} className="text-small font-semibold text-ink-muted m-0">Thread có tin mới</h2>
        <span
          aria-label={`${rows.length} thread`}
          className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
            text-on-accent text-2xs font-semibold tnum"
        >
          {rows.length}
        </span>
        <Button
          variant="ghost" size="sm" className="ml-auto max-lg:min-h-11"
          aria-expanded={!collapsed} aria-controls={listId} onClick={toggle}
        >
          {collapsed ? 'Mở rộng' : 'Thu gọn'}
        </Button>
      </div>
      <div id={listId} hidden={collapsed}>
        {!collapsed && (
          <div role="list" aria-labelledby={headingId} className="grid gap-0.5 p-1 rounded-surface bg-raised">
            {shown.map(({ m, conversation }) => (
              <div role="listitem" key={m.id}>
                <ThreadRow message={m} conversation={conversation} people={people} onOpen={() => onOpenThread(m.channel_id, m.id)} />
              </div>
            ))}
            {rows.length > max && (
              <Button
                variant="ghost" size="sm" className="justify-self-start max-lg:min-h-11" aria-expanded={all}
                onClick={() => setAll(!all)}
              >
                {all ? 'Ẩn bớt' : `Xem thêm ${hidden} thread`}
              </Button>
            )}
          </div>
        )}
      </div>
    </section>
  )
}

function ThreadRow({ message: m, conversation, people, onOpen }: {
  message: Message
  conversation: Props['conversations'][number]
  people: PeopleDirectory
  onOpen: () => void
}) {
  const author = m.sender_name || displayName(people, m.sender_id)
  const unread = Math.max(0, m.thread_unread_count ?? 0)
  const last = formatLastReply(m.last_reply_at)
  const faces = [...new Set(m.reply_participant_ids ?? [])].slice(0, 3)
  const label = `Thread của ${author} trong ${conversation.title}, ${unread} trả lời mới${last ? `, lần cuối ${last}` : ''}`
  return (
    <Pressable
      onClick={onOpen}
      aria-label={label}
      className="grid grid-cols-[24px_minmax(0,1fr)_auto] gap-x-3 items-start w-full min-h-16 p-2.5 rounded-md
        text-ink transition-colors duration-quick hover:bg-hover"
    >
      <ConversationIcon conversation={conversation} size={24} />
      <span className="grid min-w-0">
        <span className="truncate text-xs text-ink-muted">{conversation.title}</span>
        <span className="truncate"><b className="font-semibold">{author}:</b> {messageText(m.content)}</span>
        <span className="flex items-center gap-2 mt-1.5 text-small text-ink-muted">
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
          <span className="font-semibold text-accent whitespace-nowrap">{unread} trả lời mới</span>
          {last && <span className="truncate tnum">Lần cuối {last}</span>}
        </span>
      </span>
      <span className="grid justify-items-end gap-1.5 text-xs text-ink-muted tnum pt-0.5">
        <span
          aria-hidden="true"
          className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
            text-on-accent text-2xs font-semibold"
        >
          {unread}
        </span>
      </span>
    </Pressable>
  )
}
