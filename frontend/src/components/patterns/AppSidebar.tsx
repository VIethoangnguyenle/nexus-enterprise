import { useState, useRef, useEffect } from 'react'
import { useNavigate, useMatches } from '@tanstack/react-router'
import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { useWorkspaces } from '../../hooks/useWorkspaces'
import {
  MessageSquare, FolderOpen, Users, Briefcase, ClipboardCheck, Settings, HelpCircle, LogOut, Plus, ChevronDown, Check, ShieldCheck,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type ModuleId = 'messaging' | 'documents' | 'drive' | 'assets' | 'contacts' | 'approval' | 'admin' | 'settings'

type NavItem = {
  id: ModuleId
  icon: LucideIcon
  label: string
  /** Route path to navigate to (within workspace layout). */
  routePath?: string
}

const mainNavItems: NavItem[] = [
  { id: 'messaging', icon: MessageSquare, label: 'Chat', routePath: '/channels' },
  { id: 'drive', icon: FolderOpen, label: 'Drive', routePath: '/drive' },
  { id: 'approval', icon: ClipboardCheck, label: 'Approvals', routePath: '/approval' },
  { id: 'contacts', icon: Users, label: 'Contacts', routePath: '/contacts' },
  { id: 'assets', icon: Briefcase, label: 'Workplace', routePath: '/assets/dashboard' },
  { id: 'admin', icon: ShieldCheck, label: 'Admin', routePath: '/admin' },
]

interface AppSidebarProps {
  workspaceName?: string
  unreadCounts?: Partial<Record<string, number>>
}

/** Nexus Enterprise sidebar matching Stitch nexus-chat.html:
 *  w-[280px], bg-surface-bright, p-6, gap-y-4.
 *  Active nav: bg-surface-container-highest text-primary shadow-sm rounded-lg.
 *  Inactive: text-on-surface-variant hover:bg-surface-container rounded-lg.
 *  "New Project" CTA: bg-primary rounded-lg (not rounded-full).
 *  Footer: border-t border-outline-variant/30 with Settings + Support. */
export function AppSidebar({ workspaceName, unreadCounts = {} }: AppSidebarProps) {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const activeModule = useUiStore((s) => s.activeModule)
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const navigate = useNavigate()
  const { data: wsData } = useWorkspaces()
  const workspaces = wsData?.workspaces || []

  // Workspace switcher dropdown state
  const [wsDropdownOpen, setWsDropdownOpen] = useState(false)
  const wsDropdownRef = useRef<HTMLDivElement>(null)
  const currentWsParam = new URLSearchParams(window.location.search).get('ws')
  const activeWsId = currentWsParam || workspaces[0]?.id || ''

  // Close dropdown on outside click
  useEffect(() => {
    if (!wsDropdownOpen) return
    const handleClick = (e: MouseEvent) => {
      if (wsDropdownRef.current && !wsDropdownRef.current.contains(e.target as Node)) {
        setWsDropdownOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [wsDropdownOpen])

  const handleSwitchWorkspace = (wsId: string) => {
    setWsDropdownOpen(false)
    const url = new URL(window.location.href)
    url.searchParams.set('ws', wsId)
    window.location.href = url.toString()
  }

  /** Detect active module from current route path for proper highlighting. */
  const matches = useMatches()
  const currentPath = matches[matches.length - 1]?.pathname || ''

  const getEffectiveActiveModule = (): ModuleId => {
    if (currentPath.includes('/admin')) return 'admin'
    if (currentPath.includes('/contacts')) return 'contacts'
    if (currentPath.includes('/drive')) return 'drive'
    if (currentPath.includes('/approval')) return 'approval'
    if (currentPath.includes('/channels')) return 'messaging'
    if (currentPath.includes('/assets')) return 'assets'
    if (currentPath.includes('/settings')) return 'settings'
    if (currentPath.includes('/documents')) return 'documents'
    return activeModule
  }

  const effectiveActive = getEffectiveActiveModule()

  const handleClick = (item: NavItem) => {
    setActiveModule(item.id)
    if (item.routePath) {
      navigate({ to: item.routePath })
    }
  }

  const initial = workspaceName?.charAt(0).toUpperCase() || 'N'

  return (
    <aside
      className="hidden lg:flex w-[280px] shrink-0 bg-surface-bright border-r border-outline-variant/30
        flex-col overflow-hidden h-full p-6 gap-y-4"
    >
      {/* Workspace identity — Stitch: hover:bg-surface-container-high p-2 rounded-lg */}
      <div className="relative" ref={wsDropdownRef}>
        <button
          onClick={() => setWsDropdownOpen(!wsDropdownOpen)}
          className="flex items-center gap-2 p-2 rounded-lg cursor-pointer border-none
            bg-transparent w-full text-left transition-colors
            hover:bg-surface-container-high"
          title="Switch workspace"
        >
          {/* Avatar — Stitch: w-10 h-10 rounded-lg bg-primary-container */}
          <div
            className="w-10 h-10 rounded-lg bg-primary-container text-on-primary-container
              flex items-center justify-center font-bold text-lg shrink-0"
          >
            {initial}
          </div>
          <div className="min-w-0 flex-1 text-left">
            <div className="text-small font-semibold text-on-surface truncate">
              {workspaceName || 'Nexus Workspace'}
            </div>
            <div className="text-small text-on-surface-variant">Enterprise Tier</div>
          </div>
          <ChevronDown
            size={16}
            className={`text-on-surface-variant shrink-0 transition-transform
              ${wsDropdownOpen ? 'rotate-180' : ''}`}
          />
        </button>

        {/* Dropdown menu */}
        {wsDropdownOpen && workspaces.length > 0 && (
          <div className="absolute left-2 right-2 top-full mt-1 z-50
            bg-surface-container-lowest border border-outline-variant rounded-lg shadow-lg
            py-1 max-h-60 overflow-y-auto animate-fade-in">
            <div className="px-3 py-1.5 text-label-caps text-on-surface-variant uppercase tracking-wider">
              Workspaces
            </div>
            {workspaces.map((ws: { id: string; name: string }) => (
              <button
                key={ws.id}
                onClick={() => handleSwitchWorkspace(ws.id)}
                className={`w-full flex items-center gap-2 px-3 py-2 text-small text-left
                  border-none cursor-pointer transition-colors
                  ${ws.id === activeWsId
                    ? 'bg-primary-fixed text-primary font-medium'
                    : 'bg-transparent text-on-surface hover:bg-surface-container-high'
                  }`}
              >
                <div className="w-6 h-6 rounded-md bg-surface-container-high text-primary flex items-center justify-center
                  text-label-caps font-bold shrink-0">
                  {ws.name.charAt(0).toUpperCase()}
                </div>
                <span className="truncate flex-1">{ws.name}</span>
                {ws.id === activeWsId && <Check size={14} className="text-primary shrink-0" />}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* New Project CTA — Stitch: rounded-lg (not rounded-full), py-2.5 */}
      <button
        className="w-full py-2.5 px-4 bg-primary text-on-primary rounded-lg text-small font-semibold
          border-none cursor-pointer flex items-center justify-center gap-2
          transition-colors shadow-sm hover:bg-primary/90"
      >
        <Plus size={18} />
        New Project
      </button>

      {/* Navigation — Stitch: flex flex-col gap-1 mt-space-md */}
      <nav className="flex flex-col gap-1 flex-1 mt-4">
        {mainNavItems.map((item) => {
          const isActive = effectiveActive === item.id
          const Icon = item.icon
          const count = unreadCounts[item.id]
          return (
            <button
              key={item.id}
              onClick={() => handleClick(item)}
              title={item.label}
              aria-label={item.label}
              className={`flex items-center gap-3 py-2.5 px-3 rounded-lg cursor-pointer
                border-none w-full text-left text-sm font-semibold transition-all duration-200
                ${isActive
                  ? 'bg-surface-container-highest text-primary shadow-sm'
                  : 'bg-transparent text-on-surface-variant hover:bg-surface-container hover:text-on-surface'
                }`}
            >
              <Icon size={20} strokeWidth={isActive ? 2.2 : 1.6} />
              <span className="flex-1 whitespace-nowrap overflow-hidden text-ellipsis">{item.label}</span>
              {count && count > 0 ? (
                <span
                  className="text-label-caps font-semibold text-on-primary bg-primary rounded-full
                    px-1.5 min-w-[18px] h-[18px] flex items-center justify-center leading-none shrink-0"
                >
                  {count > 99 ? '99+' : count}
                </span>
              ) : null}
            </button>
          )
        })}
      </nav>

      {/* Footer — Stitch: border-t border-outline-variant/30, gap-1 */}
      <div className="pt-4 border-t border-outline-variant/30 flex flex-col gap-1">
        <button
          onClick={() => setActiveModule('settings')}
          className="flex items-center gap-3 py-2.5 px-3 rounded-lg cursor-pointer
            border-none bg-transparent text-on-surface-variant w-full text-left
            text-sm font-semibold transition-all duration-200
            hover:bg-surface-container hover:text-on-surface"
          title="Settings"
        >
          <Settings size={18} strokeWidth={1.6} />
          <span className="flex-1 whitespace-nowrap overflow-hidden text-ellipsis">Settings</span>
        </button>
        <button
          className="flex items-center gap-3 py-2.5 px-3 rounded-lg cursor-pointer
            border-none bg-transparent text-on-surface-variant w-full text-left
            text-sm font-semibold transition-all duration-200
            hover:bg-surface-container hover:text-on-surface"
          title="Support"
        >
          <HelpCircle size={18} strokeWidth={1.6} />
          <span className="flex-1 whitespace-nowrap overflow-hidden text-ellipsis">Support</span>
        </button>
        <button
          onClick={logout}
          title={`Logout (${user?.username})`}
          aria-label="Logout"
          className="flex items-center gap-3 py-2.5 px-3 rounded-lg cursor-pointer
            border-none bg-transparent text-on-surface-variant w-full text-left
            text-sm font-semibold transition-all duration-200
            hover:bg-surface-container hover:text-on-surface"
        >
          <LogOut size={16} strokeWidth={1.6} />
          <span className="flex-1 whitespace-nowrap overflow-hidden text-ellipsis">Logout</span>
        </button>
      </div>
    </aside>
  )
}
