import { useEffect } from 'react'
import { Link, useLocation, useSearch } from '@tanstack/react-router'
import { Ellipsis, House, MessageCircle, Rows3 } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { useConversations } from '../../hooks/useConversations'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { useMentionsUnreadCount } from '../../hooks/useShortcuts'
import { totalUnread, unreadOf } from '../../lib/conversations'
import { formatCount } from '../../lib/format'
import { chatTabOf, isChatListPath, type ChatTab } from '../../lib/nav-destinations'
import { useUiStore } from '../../stores/ui.store'
import { NewChatButton } from '../spaces/NewChatButton'

const TABS: { id: ChatTab; icon: LucideIcon; label: string }[] = [
  { id: 'home', icon: House, label: 'Trang chủ' },
  { id: 'direct', icon: MessageCircle, label: 'Trực tiếp' },
  { id: 'sections', icon: Rows3, label: 'Mục' },
  { id: 'more', icon: Ellipsis, label: 'Thêm' },
]

/**
 * The phone's Chat bar (DESIGN.md §5, Điều hướng di động): on the list of
 * conversations only, never inside one and never in another module (those are
 * reached from ☰). Where the desktop's sidebar is shown it is not drawn at all,
 * and nothing below it asks for any data.
 */
export function MobileNav() {
  const { pathname } = useLocation()
  return useBottomBarLayout() && isChatListPath(pathname) ? <ChatTabBar /> : null
}

/** A floating pill of four icons, and the + beside it. The name of each is its `aria-label`, with what it reports. */
function ChatTabBar() {
  const { workspaceId } = useActiveWorkspace()
  const { spaces, directs, unreadTotal } = useConversations(workspaceId)
  const mentions = useMentionsUnreadCount(workspaceId).data ?? 0
  const current = chatTabOf(useSearch({ strict: false }) as { tab?: unknown })
  const setChatBarShown = useUiStore((s) => s.setChatBarShown)

  // What floats at the bottom edge (toasts, the update notice) lifts itself above the bar while it is here.
  useEffect(() => {
    setChatBarShown(true)
    return () => setChatBarShown(false)
  }, [setChatBarShown])

  // Counted the way the sidebar counts: a muted conversation adds only the times someone named you.
  const directUnread = totalUnread(directs)
  const groupsWithNews = spaces.filter((c) => unreadOf(c) > 0).length

  const report = (id: ChatTab): { count: number; dot: boolean; label: string } => {
    const label = TABS.find((t) => t.id === id)!.label
    switch (id) {
      case 'home':
        return { count: 0, dot: unreadTotal > 0, label: unreadTotal > 0 ? `${label}, có tin chưa đọc` : label }
      case 'direct':
        return { count: directUnread, dot: false, label: directUnread > 0 ? `${label}, ${directUnread} chưa đọc` : label }
      case 'sections':
        return { count: groupsWithNews, dot: false, label: groupsWithNews > 0 ? `${label}, ${groupsWithNews} chưa đọc` : label }
      case 'more':
        return { count: 0, dot: mentions > 0, label: mentions > 0 ? `${label}, có lượt nhắc tên mới` : label }
    }
  }

  return (
    <>
      <nav
        aria-label="Điều hướng Chat"
        className="fixed left-3.5 right-22.5 bottom-[calc(1.375rem+env(safe-area-inset-bottom,0px))] z-sticky lg:hidden
          grid grid-cols-4 items-center h-16 px-1.5 rounded-full bg-overlay/90 backdrop-blur-lg shadow-overlay
          ring-1 ring-inset ring-line"
      >
        {TABS.map((tab) => {
          const active = current === tab.id
          const { count, dot, label } = report(tab.id)
          return (
            <Link
              key={tab.id}
              to="/channels"
              search={tab.id === 'home' ? {} : { tab: tab.id }}
              // Trang chủ is the bare address, which every other tab's address also starts with.
              activeOptions={{ exact: true }}
              aria-current={active ? 'page' : undefined}
              aria-label={label}
              className={`relative grid place-items-center h-13 rounded-full no-underline focus-ring ${active ? 'text-accent' : 'text-ink-muted'}`}
            >
              <span
                className={`relative grid place-items-center w-15 h-11 rounded-full transition-colors duration-quick
                  motion-reduce:transition-none ${active ? 'bg-accent-wash' : ''}`}
              >
                <tab.icon size={24} strokeWidth={active ? 2.1 : 1.75} aria-hidden="true" />
                {count > 0 && (
                  <span
                    aria-hidden="true"
                    className="absolute top-0.5 right-1.5 inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full
                      bg-accent text-on-accent text-2xs font-semibold tnum ring-2 ring-overlay"
                  >
                    {formatCount(count)}
                  </span>
                )}
                {dot && (
                  <span aria-hidden="true" className="absolute top-2 right-3.5 w-2.5 h-2.5 rounded-full bg-accent ring-2 ring-overlay" />
                )}
              </span>
            </Link>
          )
        })}
      </nav>
      <NewChatButton />
    </>
  )
}
