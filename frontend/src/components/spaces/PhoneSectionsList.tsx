import { useId, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { ChevronDown, FolderPlus } from 'lucide-react'
import type { ChatNavigation } from '../../hooks/useChatNavigation'
import { useWebSocketStore } from '../../stores/websocket.store'
import { collapseKey, useNavCollapseStore } from '../../stores/nav-collapse.store'
import { useMotionPresets } from '../../lib/motion'
import { formatCount } from '../../lib/format'
import { totalUnread, unreadOf, type Conversation } from '../../lib/conversations'
import { Pressable } from '../primitives'
import { ConversationIcon } from './ConversationIcon'
import { MentionPill, MutedMark } from './ConversationBadges'
import { PhoneListTitle, UnreadToggle } from './PhoneListControls'
import { PressableRow, useMenuPress } from './PressableRow'
import { STARRED_SECTION, rowTargetOf, type SectionOverlays } from './SectionOverlays'

/** The unread-only pill is the one control in a header that keeps its own press. */
const OWN_PRESS = '[data-unread-toggle]'

/**
 * Mục on a phone (DESIGN.md "Chat trên điện thoại", mockup §3): the same order
 * as the navigator column (Đã gắn sao, the user's sections, Tin nhắn trực tiếp,
 * Nhóm), 56px rows with a 36px avatar. A header folds its section (the folded
 * state is the desktop's too) and has a pill that lists only what is unread in
 * that section; while folded it totals the unread by notification level.
 * Long-pressing a row opens its sheet; long-pressing the header of one of the
 * user's own sections opens that section's. "Mục mới" closes the list.
 */
export function PhoneSectionsList({ nav, workspaceId, overlays }: {
  nav: ChatNavigation
  workspaceId: string
  /** Owned by the view, which also opens "Tạo mục" from the + sheet. */
  overlays: SectionOverlays
}) {
  const { arranged } = nav

  return (
    <div className="grid">
      <PhoneListTitle title="Mục" />
      {arranged.starred.length > 0 && (
        <Group id={STARRED_SECTION} workspaceId={workspaceId} title="Đã gắn sao" items={arranged.starred} overlays={overlays} />
      )}
      {arranged.sections.map((s) => (
        <Group
          key={s.id}
          id={s.id}
          workspaceId={workspaceId}
          title={s.name}
          items={s.items}
          overlays={overlays}
          own
          empty="Chưa có cuộc trò chuyện. Nhấn giữ một cuộc trò chuyện để chuyển vào đây."
        />
      ))}
      {arranged.directs.length > 0 && (
        <Group id="dm" workspaceId={workspaceId} title="Tin nhắn trực tiếp" items={arranged.directs} overlays={overlays} />
      )}
      {arranged.spaces.length > 0 && (
        <Group id="spaces" workspaceId={workspaceId} title="Nhóm" items={arranged.spaces} overlays={overlays} />
      )}
      <Pressable
        onClick={(e) => overlays.openCreate(e.currentTarget, undefined, 'bottom-end')}
        aria-haspopup="dialog"
        className="flex items-center gap-3.5 min-h-13 mt-2 px-5 text-md font-semibold text-accent"
      >
        <FolderPlus size={20} strokeWidth={1.75} aria-hidden="true" />
        Mục mới
      </Pressable>
    </div>
  )
}

function Group({ id, workspaceId, title, items, overlays, own = false, empty }: {
  id: string
  workspaceId: string
  title: string
  items: Conversation[]
  overlays: SectionOverlays
  /** One of the user's own sections: its header has a menu on a long press. */
  own?: boolean
  empty?: string
}) {
  const m = useMotionPresets()
  const listId = useId()
  const key = collapseKey(workspaceId, id)
  const open = !useNavCollapseStore((s) => !!s.collapsed[key])
  const setCollapsed = useNavCollapseStore((s) => s.setCollapsed)
  const [unreadOnly, setUnreadOnly] = useState(false)
  const unread = totalUnread(items)
  const shown = unreadOnly ? items.filter((c) => unreadOf(c) > 0) : items
  const { ref, props } = useMenuPress<HTMLDivElement>((el) => own && overlays.openSectionMenu(id, el), OWN_PRESS)

  return (
    <section>
      <div
        ref={ref}
        {...(own ? props : {})}
        className={`flex items-center gap-1 min-h-12 pl-1.5 pr-2.5 pt-1.5 pb-0.5 ${own ? 'select-none [-webkit-touch-callout:none]' : ''}`}
      >
        <Pressable
          aria-expanded={open}
          aria-controls={listId}
          onClick={() => setCollapsed(key, open)}
          className="flex-1 min-w-0 flex items-center gap-2 min-h-11 px-2 rounded-md text-base font-semibold text-ink"
        >
          <ChevronDown
            size={20}
            strokeWidth={1.75}
            aria-hidden="true"
            className={`shrink-0 text-ink-muted transition-transform duration-quick ease-out motion-reduce:transition-none ${open ? '' : '-rotate-90'}`}
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
        <span data-unread-toggle="">
          <UnreadToggle compact pressed={unreadOnly} onChange={setUnreadOnly} label={`Chỉ hiện chưa đọc trong ${title}`} />
        </span>
      </div>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div key="list" {...m.row} className="overflow-hidden">
            {items.length === 0 ? (
              <div className="px-5 py-2 text-small text-ink-muted">{empty}</div>
            ) : shown.length === 0 ? (
              <div role="status" className="px-5 py-2 text-small text-ink-muted">Không có tin chưa đọc trong {title}.</div>
            ) : (
              <ul id={listId} aria-label={title} className="grid list-none m-0 p-0">
                {shown.map((c) => (
                  <PressableRow
                    key={c.id}
                    // A muted conversation does not slide to the top when it speaks: silence is what muting means.
                    slide={c.level !== 'off'}
                    onMenu={(el) => overlays.openRowMenu(rowTargetOf(c), el)}
                  >
                    <SectionRow conversation={c} />
                  </PressableRow>
                ))}
              </ul>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </section>
  )
}

/** A 56px row of Mục: a 36px avatar, the name (bold when unread), and on the right the count, "@" or bell-off. */
function SectionRow({ conversation: c }: { conversation: Conversation }) {
  const online = useWebSocketStore((s) => (c.partner ? !!s.onlineUsers[c.partner.userId] : false))
  const muted = c.level === 'off'
  const unread = !muted && c.unread > 0
  const mentions = muted ? (c.mentions ?? 0) : 0

  return (
    <Link
      to="/channels/$channelId"
      params={{ channelId: c.id }}
      aria-label={muted ? `${c.title}, đã tắt thông báo` : c.title}
      data-unread={unread || undefined}
      data-muted={muted || undefined}
      className={`grid grid-cols-[36px_minmax(0,1fr)_auto] gap-3.5 items-center min-h-14 py-1.5 pl-5 pr-4 no-underline
        focus-ring active:bg-hover ${muted ? 'text-ink-muted' : 'text-ink'}`}
    >
      <span className={muted ? 'opacity-[0.72] inline-flex' : 'inline-flex'}>
        <ConversationIcon conversation={c} size={36} online={online} />
      </span>
      <span className={`truncate text-base ${unread ? 'font-bold' : ''}`}>{c.title}</span>
      <span className="inline-flex items-center gap-1.5 text-ink-subtle">
        {unread && (
          <span
            className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
              text-on-accent text-2xs font-semibold tnum"
            aria-label={`${c.unread} tin chưa đọc`}
          >
            {formatCount(c.unread)}
          </span>
        )}
        {muted && (
          <>
            {mentions > 0 && <MentionPill n={mentions} />}
            <MutedMark />
          </>
        )}
      </span>
    </Link>
  )
}
