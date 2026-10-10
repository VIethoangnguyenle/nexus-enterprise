import { useEffect, useRef, useState } from 'react'
import { Link, useLocation } from '@tanstack/react-router'
import { ClipboardCheck, Ellipsis, FolderOpen, MessageSquare } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { approvalPendingOptions } from '../../hooks/useApproval'
import { useBottomBarLayout } from '../../hooks/usePhone'
import { formatCount } from '../../lib/format'
import { useUiStore } from '../../stores/ui.store'
import { Pressable } from '../primitives'
import { useUnreadCount } from '../../hooks/useNotifications'
import { unreadLabel } from '../../lib/notification-model'
import { useNotificationUi } from '../../stores/notification.store'
import { MORE_ITEMS, MobileMoreSheet, type SheetView } from './MobileMoreSheet'

type Tab = {
  id: 'messaging' | 'drive' | 'approval'
  icon: LucideIcon
  label: string
  to: string
  /** Path prefixes that light this tab up. */
  matches: string[]
}

/** DESIGN.md §5, Điều hướng di động: three labelled tabs; everything else is under Thêm. */
const TABS: Tab[] = [
  { id: 'messaging', icon: MessageSquare, label: 'Tin nhắn', to: '/channels', matches: ['/channels'] },
  // Văn bản is a group inside Tài liệu, so its pages light up Tài liệu.
  { id: 'drive', icon: FolderOpen, label: 'Tài liệu', to: '/drive', matches: ['/drive', '/documents'] },
  { id: 'approval', icon: ClipboardCheck, label: 'Phê duyệt', to: '/approval', matches: ['/approval'] },
]

const under = (pathname: string, prefix: string) => pathname === prefix || pathname.startsWith(`${prefix}/`)

const TAB_CELL = 'relative grid justify-items-center content-center gap-0.5 min-h-12 text-2xs font-medium no-underline focus-ring rounded-surface'
const TAB_ICON = 'relative grid place-items-center w-12 h-6 rounded-surface transition-colors duration-quick'

/**
 * Phone bottom bar (DESIGN.md §5): Tin nhắn, Tài liệu, Phê duyệt (with the
 * number waiting on the person) and Thêm, which opens the sheet holding the
 * rest. Rendered only where the sidebar is not shown (below 1024px).
 */
export function MobileNav() {
  const { pathname } = useLocation()
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  // Above the bar's widths nothing shows the count, so nothing asks for it.
  const shown = useBottomBarLayout()
  const pending = useQuery({ ...approvalPendingOptions(), enabled: shown }).data?.total ?? 0
  const unread = useUnreadCount(shown).data ?? 0
  const [moreOpen, setMoreOpen] = useState(false)
  const [view, setView] = useState<SheetView>('menu')
  const openMore = (v: SheetView = 'menu') => {
    setView(v)
    setMoreOpen(true)
  }
  // "Mở" on a toast brings the list up wherever the person is.
  const openRequest = useNotificationUi((s) => s.openRequest)
  const handledRequest = useRef(openRequest)
  useEffect(() => {
    if (openRequest === handledRequest.current) return
    handledRequest.current = openRequest
    if (shown) openMore('notifications')
  }, [openRequest, shown])
  const moreCurrent = MORE_ITEMS.some((item) => under(pathname, item.to))
  const moreLit = moreCurrent || moreOpen

  if (!shown) return null
  return (
    <>
      <nav
        aria-label="Điều hướng di động"
        className="fixed bottom-0 inset-x-0 z-sticky lg:hidden grid grid-cols-4 gap-1 px-1 pt-1 pb-bar bg-raised"
      >
        {TABS.map((tab) => {
          const active = tab.matches.some((prefix) => under(pathname, prefix))
          const badge = tab.id === 'approval' && pending > 0
          return (
            <Link
              key={tab.id}
              to={tab.to}
              onClick={() => setActiveModule(tab.id)}
              aria-current={active ? 'page' : undefined}
              aria-label={badge ? `${tab.label}, ${pending} chờ bạn` : undefined}
              className={`${TAB_CELL} ${active ? 'text-accent font-semibold' : 'text-ink-muted'}`}
            >
              <span className={`${TAB_ICON} ${active ? 'bg-accent-wash' : ''}`}>
                <tab.icon size={20} strokeWidth={1.75} aria-hidden="true" />
                {badge && (
                  <span
                    aria-hidden="true"
                    className="absolute -top-1.5 right-0.5 inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5
                      rounded-full bg-accent text-on-accent text-2xs font-semibold tnum ring-2 ring-raised"
                  >
                    {formatCount(pending)}
                  </span>
                )}
              </span>
              {tab.label}
            </Link>
          )
        })}
        <Pressable
          onClick={() => openMore()}
          aria-label={unread > 0 ? unreadLabel('Thêm', unread, 'thông báo') : undefined}
          aria-haspopup="dialog"
          aria-expanded={moreOpen}
          data-current={moreCurrent || undefined}
          className={`${TAB_CELL} text-center ${moreLit ? 'text-accent font-semibold' : 'text-ink-muted'}`}
        >
          <span className={`${TAB_ICON} ${moreLit ? 'bg-accent-wash' : ''}`}>
            <Ellipsis size={20} strokeWidth={1.75} aria-hidden="true" />
            {unread > 0 && (
              <span
                aria-hidden="true"
                data-testid="more-dot"
                className="absolute -top-0.5 right-1.5 w-2 h-2 rounded-full bg-accent ring-2 ring-raised"
              />
            )}
          </span>
          Thêm
        </Pressable>
      </nav>
      <MobileMoreSheet
        open={moreOpen} onClose={() => setMoreOpen(false)} pathname={pathname} view={view} onViewChange={setView}
      />
    </>
  )
}
