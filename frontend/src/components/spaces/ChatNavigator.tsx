import { useId, useState, type ReactNode } from 'react'
import { Link, useParams, useRouterState } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { ChevronDown, House, Plus } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useConversations } from '../../hooks/useConversations'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useMotionPresets } from '../../lib/motion'
import { formatCount } from '../../lib/format'
import type { Conversation } from '../../lib/conversations'
import { Button, IconButton, Pressable } from '../primitives'
import { NewChatMenu } from './NewChatMenu'
import { ConversationIcon } from './ConversationIcon'
import { CreateSpaceDialog } from './CreateSpaceDialog'
import { StartChatDialog } from './StartChatDialog'

/**
 * Left column of Tin nhắn, in the Google Chat model (design/mockups/spaces.html §1):
 * "Trò chuyện mới", Trang chủ with the unread total, then collapsible
 * "Tin nhắn trực tiếp" and "Nhóm". Mentions, Starred and Browse spaces are
 * not shown: the backend has no API for them yet.
 */
export function ChatNavigator() {
  const { workspaceId } = useActiveWorkspace()
  const { spaces, directs, unreadTotal, isLoading, isError, refetch } = useConversations(workspaceId)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const params = useParams({ strict: false }) as { channelId?: string }
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const onHome = /^\/channels\/?$/.test(pathname)

  const [dialog, setDialog] = useState<'space' | 'dm' | null>(null)

  const row = (c: Conversation) => (
    <ConversationLink
      key={c.id}
      conversation={c}
      current={params.channelId === c.id}
      online={!!c.partner && !!onlineUsers[c.partner.userId]}
    />
  )

  return (
    <nav aria-label="Trò chuyện" className="flex flex-col h-full min-h-0 bg-base">
      <div className="px-3 pt-4 pb-2">
        <NewChatMenu
          className="press inline-flex items-center gap-2.5 h-11 pl-3.5 pr-4.5 rounded-xl bg-accent-wash
            text-section text-ink hover:bg-hover"
        />
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto px-2 pb-4">
        <Link
          to="/channels"
          aria-current={onHome ? 'page' : undefined}
          className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-sm no-underline focus-ring
            transition-colors duration-quick ${onHome ? 'bg-raised text-ink font-semibold' : 'text-ink hover:bg-hover'}`}
        >
          <House size={16} strokeWidth={1.75} className="text-ink-muted shrink-0" aria-hidden="true" />
          <span className="truncate">Trang chủ</span>
          {unreadTotal > 0 && <CountPill n={unreadTotal} label={`${unreadTotal} tin chưa đọc`} />}
        </Link>

        {isError ? (
          <div className="grid gap-2 justify-items-start px-2.5 pt-4 text-sm text-ink-muted">
            <span>Chưa tải được danh sách trò chuyện.</span>
            <Button variant="soft" size="sm" onClick={refetch}>Thử lại</Button>
          </div>
        ) : (
          <>
            <Section
              title="Tin nhắn trực tiếp"
              addLabel="Nhắn tin trực tiếp"
              onAdd={() => setDialog('dm')}
              loading={isLoading}
              empty="Chưa có tin nhắn trực tiếp."
            >
              {directs.map(row)}
            </Section>
            <Section
              title="Nhóm"
              addLabel="Tạo nhóm"
              onAdd={() => setDialog('space')}
              loading={isLoading}
              empty="Bạn chưa ở nhóm nào."
            >
              {spaces.map(row)}
            </Section>
          </>
        )}
      </div>

      <CreateSpaceDialog open={dialog === 'space'} onClose={() => setDialog(null)} />
      <StartChatDialog open={dialog === 'dm'} onClose={() => setDialog(null)} />
    </nav>
  )
}

function CountPill({ n, label }: { n: number; label?: string }) {
  return (
    <span
      className="ml-auto inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full
        bg-accent text-on-accent text-2xs font-semibold tnum shrink-0"
      aria-label={label}
    >
      {formatCount(n)}
    </span>
  )
}

function ConversationLink({ conversation: c, current, online }: {
  conversation: Conversation
  current: boolean
  online: boolean
}) {
  const unread = c.unread > 0
  return (
    <li>
      <Link
        to="/channels/$channelId"
        params={{ channelId: c.id }}
        aria-current={current ? 'page' : undefined}
        data-unread={unread || undefined}
        className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-sm text-ink no-underline min-w-0
          focus-ring transition-colors duration-quick
          ${current ? 'bg-raised' : 'hover:bg-hover'} ${unread || current ? 'font-semibold' : ''}`}
      >
        <ConversationIcon conversation={c} size={24} online={online} />
        <span className="truncate">{c.title}</span>
        {unread && <CountPill n={c.unread} label={`${c.unread} tin chưa đọc`} />}
      </Link>
    </li>
  )
}

function Section({ title, addLabel, onAdd, loading, empty, children }: {
  title: string
  addLabel: string
  onAdd: () => void
  loading: boolean
  empty: string
  children: ReactNode[]
}) {
  const m = useMotionPresets()
  const listId = useId()
  const [open, setOpen] = useState(true)
  return (
    <section className="mt-1">
      <div className="flex items-center gap-1 pl-1 pr-0.5 pt-3 pb-1">
        <Pressable
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setOpen((o) => !o)}
          className="flex-1 flex items-center gap-1.5 h-7 px-1.5 rounded-md text-small font-semibold text-ink-muted
            hover:text-ink"
        >
          <ChevronDown
            size={16}
            strokeWidth={1.75}
            aria-hidden="true"
            className={`transition-transform duration-quick ease-out motion-reduce:transition-none ${open ? '' : '-rotate-90'}`}
          />
          {title}
        </Pressable>
        <IconButton size="sm" aria-label={addLabel} title={addLabel} onClick={onAdd}>
          <Plus size={16} strokeWidth={1.75} />
        </IconButton>
      </div>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div key="list" {...m.row} className="overflow-hidden">
            {loading ? (
              <div className="grid gap-1.5 px-2.5 py-1" aria-busy="true">
                {[0, 1, 2].map((i) => <div key={i} className="skeleton h-6 rounded-md" />)}
              </div>
            ) : children.length === 0 ? (
              <div className="px-2.5 py-1.5 text-small text-ink-muted">{empty}</div>
            ) : (
              <ul id={listId} aria-label={title} className="grid gap-0.5 list-none m-0 p-0">
                {children}
              </ul>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </section>
  )
}
