import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { Link, useLocation, useMatches, useNavigate } from '@tanstack/react-router'
import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { useWorkspaceSwitcher } from '../../hooks/useSwitchWorkspace'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { sidebarShortcutLabel, useSidebarCollapsed } from '../../hooks/useSidebarCollapsed'
import { useUnreadNotificationCount } from '../../hooks/useNotifications'
import { useOpenInvitationCount } from '../../hooks/useInvitations'
import { logoutSession } from '../../api/client'
import { workspaceRowDetail } from '../../lib/auth-flow'
import { workspaceDisplayName } from '../../lib/workspace'
import { formatCount } from '../../lib/format'
import { DURATION, useMotionPresets } from '../../lib/motion'
import { UNKNOWN_PERSON } from '../../lib/people'
import { Avatar, IconButton, MenuItem, MenuSeparator, Popover, Pressable, Tooltip } from '../primitives'
import { NotificationPanel } from './NotificationPanel'
import {
  MessageSquare, FolderOpen, Users, Package, ClipboardCheck, Settings, LogOut, ChevronDown, ShieldCheck, Plus,
  Bell, Mail, PanelLeft,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type ModuleId = 'messaging' | 'documents' | 'drive' | 'assets' | 'contacts' | 'approval' | 'admin' | 'settings'

type NavItem = {
  id: ModuleId
  icon: LucideIcon
  label: string
  to: string
}

/** DESIGN.md §5: five main destinations, then Thông báo, Quản trị and Cài đặt at the foot. */
const mainNavItems: NavItem[] = [
  { id: 'messaging', icon: MessageSquare, label: 'Tin nhắn', to: '/channels' },
  { id: 'drive', icon: FolderOpen, label: 'Tài liệu', to: '/drive' },
  { id: 'approval', icon: ClipboardCheck, label: 'Phê duyệt', to: '/approval' },
  { id: 'assets', icon: Package, label: 'Tài sản', to: '/assets' },
  { id: 'contacts', icon: Users, label: 'Danh bạ', to: '/contacts' },
]
const footNavItems: NavItem[] = [
  { id: 'admin', icon: ShieldCheck, label: 'Quản trị', to: '/admin' },
  { id: 'settings', icon: Settings, label: 'Cài đặt', to: '/settings' },
]

/** What the number on a destination counts, for the hidden text read after its name. */
const countUnit = (id: string) => (id === 'approval' ? 'chờ bạn' : 'chưa đọc')

const initialOf = (name: string) => Array.from(name.trim())[0]?.toLocaleUpperCase('vi') ?? 'N'

interface AppSidebarProps {
  workspaceName?: string
  unreadCounts?: Partial<Record<string, number>>
}

/**
 * Main menu (DESIGN.md §5 "Menu chính", design/mockups/app-rail.html). One
 * `<nav>` in two states the person chooses between: the 72px rail (icon and a
 * small label per destination, the default) and the 232px sidebar. The toggle
 * sits alone on the top row in both, Ctrl+\ (⌘\) does the same, and the choice
 * is remembered per person on this device.
 */
export function AppSidebar({ workspaceName, unreadCounts = {} }: AppSidebarProps) {
  const user = useAuthStore((s) => s.user)
  const activeModule = useUiStore((s) => s.activeModule)
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const { workspaceId } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  // Only workspaces the person can enter, and choosing one re-scopes the session.
  const switcher = useWorkspaceSwitcher()
  const people = usePeople(workspaceId)
  const meAsPerson = user?.id ? people.byUserId.get(user.id) : undefined
  const notificationCount = useUnreadNotificationCount()
  const offerCount = useOpenInvitationCount()
  const navigate = useNavigate()
  const motion = useMotionPresets()

  const [wsOpen, setWsOpen] = useState(false)
  const [acctOpen, setAcctOpen] = useState(false)
  const [notifOpen, setNotifOpen] = useState(false)
  const navRef = useRef<HTMLElement>(null)
  const wsRef = useRef<HTMLButtonElement | null>(null)
  const acctRef = useRef<HTMLButtonElement | null>(null)
  const bellRef = useRef<HTMLButtonElement>(null)

  // Toggle with the transition of DESIGN.md §5: content fades out (100ms), the
  // column changes width (280ms), content fades in. Reduced motion swaps at once.
  const [swapping, setSwapping] = useState(false)
  const timers = useRef<ReturnType<typeof setTimeout>[]>([])
  const swapRef = useRef<() => void>(() => {})
  const onShortcut = useCallback(() => swapRef.current(), [])
  const { collapsed: rail, toggle } = useSidebarCollapsed({ shortcut: onShortcut })
  useEffect(() => {
    swapRef.current = () => {
      setWsOpen(false)
      setAcctOpen(false)
      setNotifOpen(false)
      if (motion.reduced) return toggle()
      if (timers.current.length > 0) return
      setSwapping(true)
      timers.current = [setTimeout(() => {
        toggle()
        timers.current = [setTimeout(() => {
          setSwapping(false)
          timers.current = []
        }, DURATION.layout * 1000)]
      }, 100)]
    }
  })
  useEffect(() => () => timers.current.forEach(clearTimeout), [])

  const matches = useMatches()
  // Where "Tạo workspace" sends Back to: the screen the person is on now.
  const here = useLocation({ select: (l) => l.href })
  const currentPath = matches[matches.length - 1]?.pathname || ''
  const effectiveActive: ModuleId = (() => {
    if (currentPath.includes('/admin')) return 'admin'
    if (currentPath.includes('/contacts')) return 'contacts'
    if (currentPath.includes('/drive')) return 'drive'
    if (currentPath.includes('/approval')) return 'approval'
    if (currentPath.includes('/channels')) return 'messaging'
    if (currentPath.includes('/assets')) return 'assets'
    if (currentPath.includes('/settings')) return 'settings'
    // Văn bản is a group inside Tài liệu, so its pages light up Tài liệu.
    if (currentPath.includes('/documents')) return 'drive'
    return activeModule
  })()

  const name = workspaceName || 'Không gian làm việc'
  const current = switcher.workspaces.find((w) => w.id === workspaceId)
  const others = switcher.workspaces.filter((w) => w.id !== workspaceId)
  const typeLine = isPersonal ? 'Workspace cá nhân' : workspaceRowDetail(current, { withRole: false })
  const myName = meAsPerson?.name || UNKNOWN_PERSON
  const myRole = meAsPerson?.role

  const itemClass = (active: boolean) => rail
    ? `group relative flex flex-col items-center justify-center gap-1 w-16 h-14 rounded-surface text-2xs leading-3.5 no-underline
        focus-ring ${active ? 'text-ink font-semibold' : 'text-ink-muted font-medium hover:text-ink'}`
    : `relative flex items-center gap-2.5 h-9 px-2.5 w-full rounded-surface text-sm no-underline focus-ring
        transition-colors duration-quick ${active ? 'bg-raised text-ink font-semibold' : 'text-ink-muted font-medium hover:bg-hover hover:text-ink'}`

  /** Icon, visible label and count of one destination; the label is its accessible name. */
  const itemInner = (Icon: LucideIcon, label: string, active: boolean, count: number | undefined, unit: string): ReactNode => (
    <>
      {rail ? (
        <span
          className={`grid place-items-center w-12 h-7 rounded-surface transition-colors duration-quick
            ${active ? 'bg-accent-wash text-accent' : 'group-hover:bg-hover'}`}
        >
          <Icon size={20} strokeWidth={1.75} aria-hidden="true" />
        </span>
      ) : (
        <Icon size={18} strokeWidth={1.75} aria-hidden="true" />
      )}
      <span className={rail ? 'max-w-16 truncate' : 'flex-1 truncate text-left'}>{label}</span>
      {count && count > 0 ? (
        <>
          <span
            aria-hidden="true"
            className={rail
              ? `absolute top-0.5 left-1/2 ml-1 inline-flex items-center justify-center h-4 min-w-4 px-1 rounded-full bg-accent
                text-on-accent text-[0.625rem] font-semibold tnum ring-2 ring-sunk`
              : `inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
                text-on-accent text-2xs font-semibold tnum`}
          >
            {formatCount(count)}
          </span>
          <span className="sr-only">, {count} {unit}</span>
        </>
      ) : null}
    </>
  )

  const navLink = (item: NavItem) => {
    const active = effectiveActive === item.id
    return (
      <Link
        key={item.id}
        to={item.to}
        onClick={() => { setActiveModule(item.id); setNotifOpen(false) }}
        aria-current={active ? 'page' : undefined}
        className={itemClass(active)}
      >
        {itemInner(item.icon, item.label, active, unreadCounts[item.id], countUnit(item.id))}
      </Link>
    )
  }

  const toggleLabel = rail ? 'Mở rộng menu chính' : 'Thu gọn menu chính'
  const itemsGap = rail ? 'justify-items-center gap-1' : 'gap-0.5'

  return (
    <nav
      ref={navRef}
      aria-label="Điều hướng chính"
      data-rail={rail || undefined}
      data-swapping={swapping || undefined}
      style={{ transitionTimingFunction: 'var(--ease-out-expo)' }}
      className={`hidden lg:grid grid-rows-[auto_1fr_auto] shrink-0 bg-sunk h-full py-3 gap-3 overflow-x-hidden overflow-y-auto
        [scrollbar-width:none] transition-[width] duration-layout motion-reduce:transition-none
        *:transition-opacity ${swapping ? '*:opacity-0 *:duration-100 *:ease-in' : '*:duration-120 *:ease-out'}
        ${rail ? 'w-18 px-1' : 'w-58 px-3'}`}
    >
      <div className={`grid gap-2 ${rail ? 'justify-items-center' : ''}`}>
        <div className={rail ? '' : 'ml-2'}>
          <Tooltip
            content={<>{toggleLabel}<kbd className="ml-1.5 px-1.5 py-0.5 rounded-sm bg-sunk font-sans text-2xs font-semibold">{sidebarShortcutLabel()}</kbd></>}
          >
            {(t) => (
              <IconButton
                {...t}
                size="md"
                aria-label={toggleLabel}
                aria-expanded={!rail}
                aria-keyshortcuts="Control+Backslash"
                onClick={() => swapRef.current()}
              >
                <PanelLeft size={18} strokeWidth={1.75} aria-hidden="true" />
              </IconButton>
            )}
          </Tooltip>
        </div>

        <Tooltip
          disabled={!rail}
          content={<><b className="block text-sm font-semibold">{name}</b><span className="text-ink-muted">{typeLine ? `${typeLine} · ` : ''}bấm để đổi workspace{offerCount > 0 ? ' · có lời mời' : ''}</span></>}
        >
          {(t) => (
            <Pressable
              {...t}
              ref={(el) => { wsRef.current = el; t.ref(el) }}
              onClick={() => { setNotifOpen(false); setWsOpen(!wsOpen) }}
              aria-haspopup="dialog"
              aria-expanded={wsOpen}
              aria-label={rail ? `Đổi workspace, đang ở ${name}${typeLine ? ` (${typeLine})` : ''}${offerCount > 0 ? ', có lời mời' : ''}` : undefined}
              className={rail
                ? 'grid place-items-center w-12 h-12 rounded-surface bg-transparent transition-colors duration-quick hover:bg-hover'
                : `flex items-center gap-2.5 w-full px-2 py-1.5 rounded-surface bg-transparent
                  transition-colors duration-quick hover:bg-hover`}
            >
              <span
                aria-hidden="true"
                className={`relative grid place-items-center rounded-surface bg-accent text-on-accent font-display font-bold shrink-0
                  ${rail ? 'w-10 h-10 text-lg' : 'w-8 h-8 text-section'}`}
              >
                {initialOf(name)}
                {offerCount > 0 && (
                  <span
                    data-testid="invitation-dot"
                    className="absolute -top-0.5 -right-0.5 w-2.5 h-2.5 rounded-full bg-accent ring-2 ring-sunk"
                  />
                )}
              </span>
              {!rail && (
                <>
                  <span className="grid min-w-0 flex-1 text-left">
                    <span className="font-display text-base font-bold text-ink truncate">{name}</span>
                    {isPersonal && <span className="text-xs text-ink-muted truncate">Workspace cá nhân</span>}
                  </span>
                  <ChevronDown
                    size={16}
                    strokeWidth={1.75}
                    aria-hidden="true"
                    className={`text-ink-muted shrink-0 transition-transform duration-quick motion-reduce:transition-none
                      ${wsOpen ? 'rotate-180' : ''}`}
                  />
                </>
              )}
              {!rail && offerCount > 0 && <span className="sr-only">, có lời mời</span>}
            </Pressable>
          )}
        </Tooltip>

        <Popover
          open={wsOpen}
          onClose={() => setWsOpen(false)}
          anchorRef={wsRef}
          placement={rail ? 'right-start' : 'bottom-start'}
          role="dialog"
          label="Đổi workspace"
          className="w-75 max-w-[calc(100vw-1.5rem)] max-h-[calc(100vh-1.5rem)] overflow-y-auto"
        >
          <div className="grid grid-cols-[2.5rem_minmax(0,1fr)] items-center gap-3 px-2.5 pt-2.5 pb-2">
            <span aria-hidden="true" className="grid place-items-center w-10 h-10 rounded-surface bg-accent text-on-accent font-display text-lg font-bold">
              {initialOf(name)}
            </span>
            <span className="grid min-w-0">
              <h2 className="m-0 font-display text-base font-bold leading-tight text-ink break-words">{name}</h2>
              {current && <span className="text-xs text-ink-muted break-words">{workspaceRowDetail(current)}</span>}
            </span>
          </div>
          {others.length > 0 && (
            <>
              <MenuSeparator />
              <div className="text-label text-ink-muted px-2.5 pt-2 pb-1">Chuyển sang</div>
              <div role="listbox" aria-label="Chuyển sang">
                <ul className="grid gap-0.5 m-0 p-0 list-none">
                  {others.map((ws) => {
                    const wsName = workspaceDisplayName(ws.name)
                    return (
                      <li key={ws.id} role="presentation" className="list-none">
                        <Pressable
                          role="option"
                          aria-selected={false}
                          disabled={switcher.switchingId !== undefined}
                          onClick={() => switcher.choose(ws.id, () => setWsOpen(false))}
                          className="w-full grid grid-cols-[1.75rem_minmax(0,1fr)] items-center gap-2.5 min-h-11 px-2.5 py-1.5 rounded-md
                            bg-transparent text-sm text-ink hover:bg-hover"
                        >
                          <span aria-hidden="true" className="grid place-items-center w-7 h-7 rounded-md bg-accent text-on-accent font-display text-xs font-bold shrink-0">
                            {initialOf(wsName)}
                          </span>
                          <span className="grid min-w-0">
                            <span className="font-semibold text-ink break-words">{wsName}</span>
                            <span className="text-xs text-ink-muted break-words">{workspaceRowDetail(ws, { withRole: false })}</span>
                          </span>
                        </Pressable>
                      </li>
                    )
                  })}
                </ul>
              </div>
            </>
          )}
          {offerCount > 0 && (
            <>
              <MenuSeparator />
              <Link
                to="/workspace-select"
                onClick={() => setWsOpen(false)}
                className="grid grid-cols-[1.75rem_minmax(0,1fr)_auto] items-center gap-2.5 min-h-11 px-2.5 py-1.5 rounded-md text-sm text-ink no-underline
                  hover:bg-hover transition-colors duration-quick focus-ring"
              >
                <span className="grid place-items-center w-7 h-7 rounded-md bg-sunk text-ink-muted shrink-0">
                  <Mail size={16} strokeWidth={1.75} aria-hidden="true" />
                </span>
                <span className="font-semibold text-ink truncate">Lời mời</span>
                <span
                  aria-hidden="true"
                  className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent text-on-accent text-2xs font-semibold tnum"
                >
                  {formatCount(offerCount)}
                </span>
                <span className="sr-only">, {offerCount} lời mời đang chờ</span>
              </Link>
            </>
          )}
          <MenuSeparator />
          <Link
            to="/onboarding"
            search={{ from: here }}
            onClick={() => setWsOpen(false)}
            className="grid grid-cols-[1.75rem_minmax(0,1fr)] items-center gap-2.5 min-h-11 px-2.5 py-1.5 rounded-md text-sm text-ink no-underline
              hover:bg-hover transition-colors duration-quick focus-ring"
          >
            <span className="grid place-items-center w-7 h-7 rounded-md bg-sunk text-ink-muted shrink-0">
              <Plus size={16} strokeWidth={1.75} aria-hidden="true" />
            </span>
            <span className="font-semibold text-ink truncate">Tạo workspace</span>
          </Link>
        </Popover>
      </div>

      <div className={`grid content-start ${itemsGap}`}>
        {(isPersonal ? mainNavItems.filter((i) => i.id !== 'approval' && i.id !== 'assets') : mainNavItems).map(navLink)}
      </div>

      <div className={`grid ${itemsGap}`}>
        <Pressable
          ref={bellRef}
          onClick={() => setNotifOpen(!notifOpen)}
          aria-haspopup="dialog"
          aria-expanded={notifOpen}
          className={`${itemClass(notifOpen)} bg-transparent ${rail ? '' : 'text-left'}`}
        >
          {itemInner(Bell, 'Thông báo', notifOpen, notificationCount, 'chưa đọc')}
        </Pressable>
        {footNavItems.map(navLink)}

        {rail ? (
          <>
            <Tooltip
              content={<><b className="block text-sm font-semibold">{myName}</b><span className="text-ink-muted">{myRole ? `${myRole} · ` : ''}Đang hoạt động</span></>}
            >
              {(t) => (
                <Pressable
                  {...t}
                  ref={(el) => { acctRef.current = el; t.ref(el) }}
                  onClick={() => { setNotifOpen(false); setAcctOpen(!acctOpen) }}
                  aria-haspopup="menu"
                  aria-expanded={acctOpen}
                  aria-label={`Tài khoản: ${myName}${myRole ? `, ${myRole}` : ''}, đang hoạt động`}
                  className="grid place-items-center w-12 h-12 rounded-surface bg-transparent transition-colors duration-quick hover:bg-hover"
                >
                  <Avatar name={myName} hueKey={user?.id} src={meAsPerson?.avatarUrl} online size={32} />
                </Pressable>
              )}
            </Tooltip>
            <Popover
              open={acctOpen}
              onClose={() => setAcctOpen(false)}
              anchorRef={acctRef}
              placement="right-end"
              role="menu"
              label="Tài khoản"
              className="w-65"
            >
              <div className="grid grid-cols-[2.5rem_minmax(0,1fr)] items-center gap-3 p-2.5">
                <Avatar name={myName} hueKey={user?.id} src={meAsPerson?.avatarUrl} online size={40} />
                <span className="grid min-w-0">
                  <b className="font-semibold text-ink break-words">{myName}</b>
                  <span className="text-xs text-ink-muted">{myRole ? `${myRole} · ` : ''}Đang hoạt động</span>
                </span>
              </div>
              <MenuSeparator />
              <MenuItem
                icon={<Settings size={16} strokeWidth={1.75} aria-hidden="true" />}
                onClick={() => { setActiveModule('settings'); void navigate({ to: '/settings' }) }}
              >
                Cài đặt tài khoản
              </MenuItem>
              <MenuItem icon={<LogOut size={16} strokeWidth={1.75} aria-hidden="true" />} onClick={() => void logoutSession()}>
                Đăng xuất
              </MenuItem>
            </Popover>
          </>
        ) : (
          <div className="flex items-center gap-2.5 p-2">
            <Avatar name={myName} hueKey={user?.id} src={meAsPerson?.avatarUrl} online size={32} />
            <span className="grid min-w-0 flex-1">
              <span className="font-semibold text-ink truncate">{myName}</span>
              {myRole && <span className="text-xs text-ink-muted truncate">{myRole}</span>}
            </span>
            <IconButton size="sm" aria-label="Đăng xuất" title="Đăng xuất" onClick={() => void logoutSession()}>
              <LogOut size={16} strokeWidth={1.75} />
            </IconButton>
          </div>
        )}
      </div>

      <NotificationPanel open={notifOpen} onClose={() => setNotifOpen(false)} anchorRef={bellRef} sideRef={navRef} />
    </nav>
  )
}
