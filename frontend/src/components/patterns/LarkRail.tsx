/**
 * @deprecated Use AppSidebar instead. This component is kept only for backward compatibility.
 * It re-exports AppSidebar as LarkRail so existing imports don't break.
 */
import { AppSidebar } from './AppSidebar'
export { AppSidebar as LarkRail }

/* Original LarkRail implementation below — preserved for reference during migration. */

import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { Avatar } from '../primitives'
import {
  MessageSquare, FileText, Package, Settings, LogOut, Search, PlusCircle,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type NavItem = {
  id: 'messaging' | 'documents' | 'drive' | 'assets' | 'settings'
  icon: LucideIcon
  label: string
  /** If set, clicking opens a new browser tab instead of SPA nav. */
  externalPath?: string
}

const navItems: NavItem[] = [
  { id: 'messaging', icon: MessageSquare, label: 'Messenger' },
  { id: 'documents', icon: FileText, label: 'Drive', externalPath: '/drive' },
  { id: 'assets', icon: Package, label: 'Assets' },
  { id: 'settings', icon: Settings, label: 'Settings' },
]

interface LarkRailProps {
  /** Per-module unread counts, e.g. { messaging: 5 }. */
  unreadCounts?: Partial<Record<string, number>>
  /** Workspace name shown at the top of the rail. */
  workspaceName?: string
}

/** Lark-style sidebar (~160px) — avatar, search, full-width nav items with badges. */
export function LarkRail({ unreadCounts = {}, workspaceName: _wsName }: LarkRailProps) {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const activeModule = useUiStore((s) => s.activeModule)
  const setActiveModule = useUiStore((s) => s.setActiveModule)

  const handleClick = (item: NavItem) => {
    if (item.externalPath) {
      window.open(item.externalPath, '_blank')
      return
    }
    setActiveModule(item.id)
  }

  return (
    <div className="lark-sidebar hidden lg:flex">
      {/* Top: Avatar + action */}
      <div className="lark-sidebar__header">
        <div className="flex items-center gap-2 min-w-0">
          <Avatar name={user?.username || 'U'} size="sm" />
          <span className="text-small-ui text-on-surface truncate">{user?.username || 'User'}</span>
        </div>
        <button
          className="lark-sidebar__action"
          title="New"
          aria-label="New"
        >
          <PlusCircle size={18} />
        </button>
      </div>

      {/* Search bar */}
      <div className="lark-sidebar__search">
        <Search size={14} className="text-on-surface-variant flex-shrink-0" />
        <span className="text-on-surface-variant">Search</span>
        <kbd className="lark-sidebar__kbd">⌘K</kbd>
      </div>

      {/* Navigation items */}
      <nav className="lark-sidebar__nav">
        {navItems.map((item) => {
          const isActive = activeModule === item.id && !item.externalPath
          const Icon = item.icon
          const count = unreadCounts[item.id]
          return (
            <button
              key={item.id}
              onClick={() => handleClick(item)}
              title={item.label}
              aria-label={item.label}
              className={`lark-sidebar__item ${isActive ? 'lark-sidebar__item--active' : ''}`}
            >
              <Icon size={18} strokeWidth={isActive ? 2.2 : 1.6} />
              <span className="lark-sidebar__item-label">{item.label}</span>
              {count && count > 0 ? (
                <span className="lark-sidebar__badge">
                  {count > 99 ? '99+' : count}
                </span>
              ) : null}
            </button>
          )
        })}
      </nav>

      {/* Bottom: Logout */}
      <div className="lark-sidebar__footer">
        <button
          onClick={logout}
          title={`Logout (${user?.username})`}
          aria-label="Logout"
          className="lark-sidebar__item"
        >
          <LogOut size={16} strokeWidth={1.6} />
          <span className="lark-sidebar__item-label">Logout</span>
        </button>
      </div>
    </div>
  )
}
