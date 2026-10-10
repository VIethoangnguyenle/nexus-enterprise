import { useState, useRef, useEffect } from 'react'
import { Link, useMatches } from '@tanstack/react-router'
import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { useActiveWorkspace } from '../../hooks/useActiveWorkspace'
import { usePeople } from '../../hooks/usePeople'
import { logoutSession } from '../../api/client'
import { workspaceDisplayName } from '../../lib/workspace'
import { formatCount } from '../../lib/format'
import { Avatar, IconButton } from '../primitives'
import {
  MessageSquare, FolderOpen, Users, Package, ClipboardCheck, Settings, LogOut, ChevronDown, Check, ShieldCheck,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type ModuleId = 'messaging' | 'documents' | 'drive' | 'assets' | 'contacts' | 'approval' | 'admin' | 'settings'

type NavItem = {
  id: ModuleId
  icon: LucideIcon
  label: string
  to: string
}

/** DESIGN.md §5: five main destinations, then administration and settings at the foot. */
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

interface AppSidebarProps {
  workspaceName?: string
  unreadCounts?: Partial<Record<string, number>>
}

/**
 * App sidebar (DESIGN.md §5, design/mockups/core-screens.html): sunk tone,
 * 232px, workspace switcher on top, main destinations, then Quản trị, Cài đặt
 * and the signed-in person at the foot.
 */
export function AppSidebar({ workspaceName, unreadCounts = {} }: AppSidebarProps) {
  const user = useAuthStore((s) => s.user)
  const activeModule = useUiStore((s) => s.activeModule)
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const { workspaceId, workspaces } = useActiveWorkspace()
  const people = usePeople(workspaceId)
  const meAsPerson = user?.id ? people.byUserId.get(user.id) : undefined

  const [wsDropdownOpen, setWsDropdownOpen] = useState(false)
  const wsDropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!wsDropdownOpen) return
    const handleClick = (e: MouseEvent) => {
      if (wsDropdownRef.current && !wsDropdownRef.current.contains(e.target as Node)) setWsDropdownOpen(false)
    }
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setWsDropdownOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    document.addEventListener('keydown', handleKey)
    return () => {
      document.removeEventListener('mousedown', handleClick)
      document.removeEventListener('keydown', handleKey)
    }
  }, [wsDropdownOpen])

  const handleSwitchWorkspace = (wsId: string) => {
    setWsDropdownOpen(false)
    const url = new URL(window.location.href)
    url.searchParams.set('ws', wsId)
    // A folder belongs to one workspace; the new one opens at its root.
    url.searchParams.delete('folder')
    url.searchParams.delete('view')
    window.location.href = url.toString()
  }

  const matches = useMatches()
  const currentPath = matches[matches.length - 1]?.pathname || ''
  const effectiveActive: ModuleId = (() => {
    if (currentPath.includes('/admin')) return 'admin'
    if (currentPath.includes('/contacts')) return 'contacts'
    if (currentPath.includes('/drive')) return 'drive'
    if (currentPath.includes('/approval')) return 'approval'
    if (currentPath.includes('/channels')) return 'messaging'
    if (currentPath.includes('/assets')) return 'assets'
    if (currentPath.includes('/settings')) return 'settings'
    if (currentPath.includes('/documents')) return 'documents'
    return activeModule
  })()

  const name = workspaceName || 'Không gian làm việc'
  const navLink = (item: NavItem) => {
    const active = effectiveActive === item.id
    const count = unreadCounts[item.id]
    return (
      <Link
        key={item.id}
        to={item.to}
        onClick={() => setActiveModule(item.id)}
        aria-current={active ? 'page' : undefined}
        className={`flex items-center gap-2.5 h-9 px-2.5 rounded-surface text-sm no-underline focus-ring
          transition-colors duration-quick
          ${active ? 'bg-raised text-ink font-semibold' : 'text-ink-muted font-medium hover:bg-hover hover:text-ink'}`}
      >
        <item.icon size={18} strokeWidth={1.75} aria-hidden="true" />
        <span className="flex-1 truncate">{item.label}</span>
        {count && count > 0 ? (
          <span
            className="inline-flex items-center justify-center h-4.5 min-w-4.5 px-1.5 rounded-full bg-accent
              text-on-accent text-2xs font-semibold tnum"
            aria-label={`${count} chưa đọc`}
          >
            {formatCount(count)}
          </span>
        ) : null}
      </Link>
    )
  }

  return (
    <aside
      aria-label="Điều hướng chính"
      className="hidden lg:grid grid-rows-[auto_1fr_auto] w-58 shrink-0 bg-sunk h-full p-3 gap-3 overflow-hidden"
    >
      <div className="relative" ref={wsDropdownRef}>
        {/* eslint-disable-next-line no-restricted-syntax -- Nút đổi không gian làm việc: logo 32px +
            tên (font display) + chevron xoay khi mở; không phải Button (canh giữa, hình học cố định)
            cũng không phải một hàng điều hướng. */}
        <button
          type="button"
          onClick={() => setWsDropdownOpen(!wsDropdownOpen)}
          aria-haspopup="listbox"
          aria-expanded={wsDropdownOpen}
          className="flex items-center gap-2.5 w-full px-2 py-1.5 rounded-surface border-none bg-transparent
            cursor-pointer text-left focus-ring transition-colors duration-quick hover:bg-hover"
        >
          <span
            aria-hidden="true"
            className="grid place-items-center w-8 h-8 rounded-surface bg-accent text-on-accent font-display
              text-section font-bold shrink-0"
          >
            {Array.from(name.trim())[0]?.toLocaleUpperCase('vi') ?? 'N'}
          </span>
          <span className="min-w-0 flex-1 font-display text-base font-bold text-ink truncate">{name}</span>
          <ChevronDown
            size={16}
            strokeWidth={1.75}
            aria-hidden="true"
            className={`text-ink-muted shrink-0 transition-transform duration-quick motion-reduce:transition-none
              ${wsDropdownOpen ? 'rotate-180' : ''}`}
          />
        </button>

        {wsDropdownOpen && workspaces.length > 0 && (
          <div
            role="listbox"
            aria-label="Không gian làm việc"
            className="absolute left-0 right-0 top-full mt-1.5 z-dropdown p-1.5 rounded-overlay bg-overlay shadow-overlay
              max-h-60 overflow-y-auto animate-fade-in"
          >
            {workspaces.map((ws) => (
              /* eslint-disable-next-line no-restricted-syntax -- Lựa chọn trong listbox (role=option),
                 có dấu tích cho mục đang mở; không có primitive nào cho option của listbox. */
              <button
                key={ws.id}
                type="button"
                role="option"
                aria-selected={ws.id === workspaceId}
                onClick={() => handleSwitchWorkspace(ws.id)}
                className="w-full flex items-center gap-2.5 h-9 px-2.5 rounded-md border-none bg-transparent
                  cursor-pointer text-sm text-ink text-left focus-ring hover:bg-hover"
              >
                <span className="truncate flex-1">{workspaceDisplayName(ws.name)}</span>
                {ws.id === workspaceId && <Check size={16} strokeWidth={1.75} className="text-accent shrink-0" />}
              </button>
            ))}
          </div>
        )}
      </div>

      <nav aria-label="Phân hệ" className="grid content-start gap-0.5">
        {mainNavItems.map(navLink)}
      </nav>

      <div className="grid gap-0.5">
        {footNavItems.map(navLink)}
        <div className="flex items-center gap-2.5 p-2">
          <Avatar
            name={meAsPerson?.name || user?.username || 'Bạn'}
            hueKey={user?.id}
            src={meAsPerson?.avatarUrl}
            online
            size={32}
          />
          <span className="grid min-w-0 flex-1">
            <span className="font-semibold text-ink truncate">{meAsPerson?.name || user?.username}</span>
            {meAsPerson?.role && <span className="text-xs text-ink-muted truncate">{meAsPerson.role}</span>}
          </span>
          <IconButton size="sm" aria-label="Đăng xuất" title="Đăng xuất" onClick={() => void logoutSession()}>
            <LogOut size={16} strokeWidth={1.75} />
          </IconButton>
        </div>
      </div>
    </aside>
  )
}
