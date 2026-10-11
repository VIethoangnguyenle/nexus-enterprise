import { useId, useState, type DragEvent, type ReactNode } from 'react'
import { Link, useParams, useRouterState } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { ChevronDown, Ellipsis, House, Plus } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useWebSocketStore } from '../../stores/websocket.store'
import { collapseKey, useNavCollapseStore } from '../../stores/nav-collapse.store'
import { useMotionPresets } from '../../lib/motion'
import { formatCount } from '../../lib/format'
import type { Conversation } from '../../lib/conversations'
import { Button, IconButton, Pressable } from '../primitives'
import { NewChatMenu } from './NewChatMenu'
import { ConversationIcon } from './ConversationIcon'
import { CreateSpaceDialog } from './CreateSpaceDialog'
import { StartChatDialog } from './StartChatDialog'
import { useSectionOverlays } from './SectionOverlays'

/**
 * Left column of Tin nhắn, in the Google Chat model (design/mockups/spaces.html §1):
 * "Trò chuyện mới", Trang chủ with the unread total, the user's own sections,
 * then collapsible "Tin nhắn trực tiếp" and "Nhóm" ("Tạo mục" lives in "Trò chuyện mới"). Mentions,
 * Starred and Browse spaces are not shown: the backend has no API for them yet.
 */
export function ChatNavigator() {
  const { workspaceId } = useActiveWorkspace()
  const nav = useChatNavigation(workspaceId)
  const { arranged, unreadTotal, isLoading, isError, refetch, sectionsApi } = nav
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const params = useParams({ strict: false }) as { channelId?: string }
  const pathname = useRouterState({ select: (s) => s.location.pathname })
  const onHome = /^\/channels\/?$/.test(pathname)

  const [dialog, setDialog] = useState<'space' | 'dm' | null>(null)
  const [dragging, setDragging] = useState<Conversation | null>(null)

  const overlays = useSectionOverlays(sectionsApi, false, (id) => setCollapsed(collapseKey(workspaceId, id), false))

  const row = (c: Conversation) => (
    <ConversationLink
      key={c.id}
      conversation={c}
      current={params.channelId === c.id}
      online={!!c.partner && !!onlineUsers[c.partner.userId]}
      onMenu={(el) => overlays.openRowMenu({ id: c.id, title: c.title }, el)}
      onDragStart={() => setDragging(c)}
      onDragEnd={() => setDragging(null)}
    />
  )

  const dropOn = (sectionId: string) => {
    const c = dragging
    setDragging(null)
    if (!c) return
    setCollapsed(collapseKey(workspaceId, sectionId), false)
    void sectionsApi.placeConversation({ id: c.id, title: c.title }, sectionId)
  }

  return (
    <nav aria-label="Trò chuyện" className="flex flex-col h-full min-h-0 bg-base">
      <div className="px-3 pt-4 pb-2">
        <NewChatMenu
          onNewSection={(el) => overlays.openCreate(el)}
          className="press inline-flex items-center gap-2.5 h-11 pl-3.5 pr-4.5 rounded-xl bg-accent-wash
            text-section text-ink hover:bg-hover"
        />
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto px-2 pb-4">
        <Link
          to="/channels"
          aria-current={onHome ? 'page' : undefined}
          className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-md no-underline focus-ring
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
            {!isLoading && arranged.sections.map((s) => (
              <Section
                key={s.id}
                id={s.id}
                title={s.name}
                unread={s.unread}
                loading={false}
                empty="Chưa có cuộc trò chuyện. Mở menu ⋯ của một cuộc trò chuyện để chuyển vào đây."
                menuLabel={`Tuỳ chọn cho mục ${s.name}`}
                onMenu={(el) => overlays.openSectionMenu(s.id, el)}
                dragging={dragging !== null}
                onDropRow={() => dropOn(s.id)}
              >
                {s.items.map(row)}
              </Section>
            ))}
            <Section
              id="dm"
              title="Tin nhắn trực tiếp"
              addLabel="Nhắn tin trực tiếp"
              onAdd={() => setDialog('dm')}
              loading={isLoading}
              empty="Chưa có tin nhắn trực tiếp."
              plain
            >
              {arranged.directs.map(row)}
            </Section>
            <Section
              id="spaces"
              title="Nhóm"
              addLabel="Tạo nhóm"
              onAdd={() => setDialog('space')}
              loading={isLoading}
              empty="Bạn chưa ở nhóm nào."
              plain
            >
              {arranged.spaces.map(row)}
            </Section>
          </>
        )}
      </div>

      {overlays.node}
      <CreateSpaceDialog open={dialog === 'space'} onClose={() => setDialog(null)} />
      <StartChatDialog open={dialog === 'dm'} onClose={() => setDialog(null)} />
    </nav>
  )
}

function CountPill({ n, label, className = '' }: { n: number; label?: string; className?: string }) {
  return (
    <span
      className={`ml-auto inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full
        bg-accent text-on-accent text-2xs font-semibold tnum shrink-0 ${className}`}
      aria-label={label}
    >
      {formatCount(n)}
    </span>
  )
}

function ConversationLink({ conversation: c, current, online, onMenu, onDragStart, onDragEnd }: {
  conversation: Conversation
  current: boolean
  online: boolean
  onMenu: (trigger: HTMLElement) => void
  onDragStart: () => void
  onDragEnd: () => void
}) {
  const unread = c.unread > 0
  return (
    <li
      className="group relative"
      draggable
      onDragStart={(e: DragEvent) => {
        // The row's name, so a drop outside the app pastes something readable; never its id.
        e.dataTransfer.setData('text/plain', c.title)
        e.dataTransfer.effectAllowed = 'move'
        onDragStart()
      }}
      onDragEnd={onDragEnd}
    >
      <Link
        to="/channels/$channelId"
        params={{ channelId: c.id }}
        aria-current={current ? 'page' : undefined}
        data-unread={unread || undefined}
        className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-md text-ink no-underline min-w-0
          focus-ring transition-colors duration-quick
          ${current ? 'bg-raised' : 'hover:bg-hover'} ${unread || current ? 'font-semibold' : ''}`}
      >
        <ConversationIcon conversation={c} size={24} online={online} />
        <span className="truncate">{c.title}</span>
        {unread && (
          <CountPill
            n={c.unread}
            label={`${c.unread} tin chưa đọc`}
            className="group-hover:invisible group-focus-within:invisible pointer-coarse:visible pointer-coarse:mr-7"
          />
        )}
      </Link>
      <IconButton
        size="sm"
        aria-label={`Tuỳ chọn cho ${c.title}`}
        aria-haspopup="menu"
        title="Tuỳ chọn"
        onClick={(e) => onMenu(e.currentTarget)}
        // No hover on a touch screen: there the ⋯ stays visible (the count moves aside).
        className="absolute right-1.5 top-1/2 -translate-y-1/2 opacity-0 group-hover:opacity-100
          group-focus-within:opacity-100 focus-visible:opacity-100 pointer-coarse:opacity-100"
      >
        <Ellipsis size={16} strokeWidth={1.75} />
      </IconButton>
    </li>
  )
}

/**
 * A collapsible group of the column. `plain` is a default one (Tin nhắn trực
 * tiếp, Nhóm): a "+" and no menu. Otherwise it is one of the user's sections: a
 * "⋯" menu, a drop target for rows, the unread total while folded, and a hint
 * when empty. Folded state is remembered per device.
 */
function Section({
  id, title, addLabel, onAdd, menuLabel, onMenu, unread = 0, loading, empty, plain, dragging, onDropRow, children,
}: {
  id: string
  title: string
  addLabel?: string
  onAdd?: () => void
  menuLabel?: string
  onMenu?: (trigger: HTMLElement) => void
  unread?: number
  loading: boolean
  empty: string
  plain?: boolean
  dragging?: boolean
  onDropRow?: () => void
  children: ReactNode[]
}) {
  const m = useMotionPresets()
  const listId = useId()
  const { workspaceId } = useActiveWorkspace()
  const key = collapseKey(workspaceId, id)
  const folded = useNavCollapseStore((s) => !!s.collapsed[key])
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const [over, setOver] = useState(false)
  const open = !folded
  const target = !!onDropRow

  return (
    <section className="mt-1">
      <div
        onDragOver={target && dragging ? (e) => { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; setOver(true) } : undefined}
        onDragLeave={target ? () => setOver(false) : undefined}
        onDrop={target ? (e) => { e.preventDefault(); setOver(false); onDropRow!() } : undefined}
        className={`group/head flex items-center gap-1 pl-1 pr-0.5 pt-3 pb-1 rounded-surface transition-colors duration-quick
          ${over ? 'bg-accent-wash' : ''}`}
      >
        <Pressable
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setCollapsed(key, open)}
          className={`flex-1 min-w-0 flex items-center gap-1.5 h-7 px-1.5 rounded-md text-sm font-semibold
            ${plain ? 'text-ink-muted hover:text-ink' : 'text-ink'}`}
        >
          <ChevronDown
            size={16}
            strokeWidth={1.75}
            aria-hidden="true"
            className={`shrink-0 transition-transform duration-quick ease-out motion-reduce:transition-none ${open ? '' : '-rotate-90'}`}
          />
          <span className="truncate">{title}</span>
          {!open && unread > 0 && <CountPill n={unread} label={`${unread} tin chưa đọc`} />}
        </Pressable>
        {plain ? (
          <IconButton size="sm" aria-label={addLabel} title={addLabel} onClick={onAdd}>
            <Plus size={16} strokeWidth={1.75} />
          </IconButton>
        ) : (
          <IconButton
            size="sm"
            aria-label={menuLabel}
            aria-haspopup="menu"
            title="Tuỳ chọn"
            onClick={(e) => onMenu?.(e.currentTarget)}
            className="opacity-0 group-hover/head:opacity-100 focus-visible:opacity-100"
          >
            <Ellipsis size={16} strokeWidth={1.75} />
          </IconButton>
        )}
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
