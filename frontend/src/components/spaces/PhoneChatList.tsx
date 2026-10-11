import { useId, useRef, type KeyboardEvent, type MouseEvent, type ReactNode } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { AtSign, ChevronDown, Ellipsis, Star } from 'lucide-react'
import type { ChatNavigation } from '../../hooks/useChatNavigation'
import type { Arrival } from '../../hooks/useArrivals'
import { useLongPress } from '../../hooks/useLongPress'
import { usePeople } from '../../hooks/usePeople'
import { useMentionsUnreadCount } from '../../hooks/useShortcuts'
import { useWebSocketStore } from '../../stores/websocket.store'
import { collapseKey, useNavCollapseStore } from '../../stores/nav-collapse.store'
import { useMotionPresets } from '../../lib/motion'
import { formatCount } from '../../lib/format'
import { totalUnread, type Conversation } from '../../lib/conversations'
import { Pressable } from '../primitives'
import { ConversationRow } from './ConversationRow'
import { STARRED_SECTION, type SectionOverlays } from './SectionOverlays'
import { UnreadThreadsSection } from './UnreadThreadsSection'

/** Controls that keep their own press; unlike the shared default, a link is not one: the row is a link. */
const OWN_PRESS = 'button, input, textarea, select, [role="button"], [contenteditable="true"]'

/**
 * The Tin nhắn tab on a phone (design/mockups/chat-mobile.html §1): the two
 * shortcut tiles (Được nhắc tới, Đã gắn sao), the followed threads with news,
 * then the same order as the navigator column (Đã gắn sao, the user's
 * sections, Tin nhắn trực tiếp, Nhóm), 64px rows, folding sections that total
 * their unread by notification level while folded. Long-pressing a row opens
 * its sheet ("Chuyển tới mục", Gắn sao, Tắt thông báo); a section's "⋯" opens
 * its own sheet. A muted row is drawn quiet, with an "@" for a mention.
 */
export function PhoneChatList({ nav, workspaceId, fresh, overlays }: {
  nav: ChatNavigation
  workspaceId: string
  fresh: Map<string, Arrival>
  /** Owned by the view, which also opens "Tạo mục" from Trò chuyện mới. */
  overlays: SectionOverlays
}) {
  const { arranged } = nav
  const navigate = useNavigate()
  const people = usePeople(workspaceId)
  const onlineUsers = useWebSocketStore((s) => s.onlineUsers)
  const mentionsUnread = useMentionsUnreadCount(workspaceId).data ?? 0

  const rows = (items: Conversation[]) =>
    items.map((c) => (
      <PressableRow
        key={c.id}
        // A muted conversation does not slide to the top when it speaks: silence is what muting means.
        slide={c.level !== 'off'}
        onMenu={(el) => overlays.openRowMenu({ id: c.id, title: c.title, starred: c.starred, muted: c.level === 'off' }, el)}
      >
        <ConversationRow
          conversation={c}
          arrival={fresh.get(`${c.id}:${c.lastActivity}`)}
          online={!!c.partner && !!onlineUsers[c.partner.userId]}
        />
      </PressableRow>
    ))

  return (
    <div className="grid gap-1">
      <nav aria-label="Lối tắt" className="grid grid-cols-2 gap-2 px-1.5 pt-1 pb-1.5">
        <Tile to="/channels/mentions" label="Được nhắc tới" icon={<AtSign size={18} strokeWidth={1.75} aria-hidden="true" />} count={mentionsUnread} />
        <Tile to="/channels/starred" label="Đã gắn sao" icon={<Star size={18} strokeWidth={1.75} aria-hidden="true" />} />
      </nav>
      <UnreadThreadsSection
        conversations={nav.conversations}
        people={people}
        max={2}
        onOpenThread={(channelId, rootId) =>
          void navigate({ to: '/channels/$channelId', params: { channelId }, search: { thread: rootId } })}
      />
      {arranged.starred.length > 0 && (
        <Group id={STARRED_SECTION} workspaceId={workspaceId} title="Đã gắn sao" unread={totalUnread(arranged.starred)}>
          {rows(arranged.starred)}
        </Group>
      )}
      {arranged.sections.map((s) => (
        <Group
          key={s.id}
          id={s.id}
          workspaceId={workspaceId}
          title={s.name}
          unread={s.unread}
          menuLabel={`Tuỳ chọn cho mục ${s.name}`}
          onMenu={(el) => overlays.openSectionMenu(s.id, el)}
          empty="Chưa có cuộc trò chuyện. Nhấn giữ một cuộc trò chuyện để chuyển vào đây."
        >
          {rows(s.items)}
        </Group>
      ))}
      {arranged.directs.length > 0 && (
        <Group id="dm" workspaceId={workspaceId} title="Tin nhắn trực tiếp" unread={totalUnread(arranged.directs)}>
          {rows(arranged.directs)}
        </Group>
      )}
      {arranged.spaces.length > 0 && (
        <Group id="spaces" workspaceId={workspaceId} title="Nhóm" unread={totalUnread(arranged.spaces)}>
          {rows(arranged.spaces)}
        </Group>
      )}
    </div>
  )
}

/**
 * A row that opens its sheet on a long press, or on the Menu key / Shift+F10.
 * The tap that ends a long press is swallowed so the conversation does not
 * open behind the sheet.
 */
function PressableRow({ onMenu, slide = true, children }: {
  onMenu: (el: HTMLElement) => void
  /** Rows re-order with a slide; a muted one just jumps. */
  slide?: boolean
  children: ReactNode
}) {
  const m = useMotionPresets()
  const ref = useRef<HTMLLIElement>(null)
  const swallowTap = useRef(false)
  const press = useLongPress(() => {
    swallowTap.current = true
    if (ref.current) onMenu(ref.current)
  }, true, OWN_PRESS)

  return (
    <motion.li
      ref={ref}
      layout={slide ? m.layoutProp : false}
      transition={m.layout}
      {...m.row}
      {...press}
      onPointerDown={(e) => {
        swallowTap.current = false
        press.onPointerDown(e)
      }}
      onClickCapture={(e: MouseEvent) => {
        if (!swallowTap.current) return
        swallowTap.current = false
        e.preventDefault()
        e.stopPropagation()
      }}
      onKeyDown={(e: KeyboardEvent) => {
        if (e.key === 'ContextMenu' || (e.key === 'F10' && e.shiftKey)) {
          e.preventDefault()
          if (ref.current) onMenu(ref.current)
        }
      }}
      className="select-none [-webkit-touch-callout:none]"
    >
      {children}
    </motion.li>
  )
}

/** A 48px shortcut tile at the head of the list (design/mockups/shortcuts.html §5). */
function Tile({ to, label, icon, count = 0 }: { to: '/channels/mentions' | '/channels/starred'; label: string; icon: ReactNode; count?: number }) {
  return (
    <Link
      to={to}
      className="flex items-center gap-2.5 min-h-12 px-3.5 rounded-xl bg-raised text-sm font-semibold text-ink
        no-underline focus-ring active:bg-hover"
    >
      <span className="text-ink-muted shrink-0">{icon}</span>
      <span className="truncate">{label}</span>
      {count > 0 && (
        <span
          className="ml-auto inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
            text-on-accent text-2xs font-semibold tnum shrink-0"
          aria-label={`${count} tin nhắc tên bạn chưa đọc`}
        >
          {formatCount(count)}
        </span>
      )}
    </Link>
  )
}

function Group({ id, workspaceId, title, unread, menuLabel, onMenu, empty, children }: {
  id: string
  workspaceId: string
  title: string
  unread: number
  menuLabel?: string
  onMenu?: (trigger: HTMLElement) => void
  empty?: string
  children: ReactNode[]
}) {
  const m = useMotionPresets()
  const listId = useId()
  const key = collapseKey(workspaceId, id)
  const open = !useNavCollapseStore((s) => !!s.collapsed[key])
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const own = !!onMenu

  return (
    <section>
      <div className="flex items-center pl-1.5 pr-1">
        <Pressable
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setCollapsed(key, open)}
          className={`flex-1 min-w-0 flex items-center gap-2 min-h-11 px-2.5 rounded-md text-sm font-semibold
            ${own ? 'text-ink' : 'text-ink-muted'}`}
        >
          <ChevronDown
            size={16}
            strokeWidth={1.75}
            aria-hidden="true"
            className={`shrink-0 transition-transform duration-quick ease-out motion-reduce:transition-none ${open ? '' : '-rotate-90'}`}
          />
          <span className="truncate">{title}</span>
          {!open && unread > 0 && (
            <span
              className="ml-auto inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
                text-on-accent text-2xs font-semibold tnum shrink-0"
              aria-label={`${unread} tin chưa đọc`}
            >
              {formatCount(unread)}
            </span>
          )}
        </Pressable>
        {own && (
          <Pressable
            aria-label={menuLabel}
            aria-haspopup="dialog"
            onClick={(e) => onMenu(e.currentTarget)}
            className="grid place-items-center w-11 h-11 rounded-md text-ink-muted hover:bg-hover active:bg-hover"
          >
            <Ellipsis size={18} strokeWidth={1.75} aria-hidden="true" />
          </Pressable>
        )}
      </div>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div key="list" {...m.row} className="overflow-hidden">
            {children.length === 0 ? (
              <div className="px-4 py-2 text-small text-ink-muted">{empty}</div>
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
