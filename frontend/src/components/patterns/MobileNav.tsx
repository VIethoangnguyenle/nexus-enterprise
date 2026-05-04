import { useLocation, useNavigate } from '@tanstack/react-router'
import { useUiStore } from '../../stores/ui.store'
import { useAuthStore } from '../../stores/auth.store'
import {
  MessageSquare, FolderOpen, Users, Briefcase, ClipboardCheck, LogOut,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

type NavItem = {
  id: 'messaging' | 'drive' | 'contacts' | 'assets' | 'approval'
  icon: LucideIcon
  label: string
  routePath: string
  /** Path segment used to detect active state. */
  activeMatch: string
}

const navItems: NavItem[] = [
  { id: 'messaging', icon: MessageSquare, label: 'Chat', routePath: '/channels', activeMatch: '/channels' },
  { id: 'drive', icon: FolderOpen, label: 'Drive', routePath: '/drive', activeMatch: '/drive' },
  { id: 'approval', icon: ClipboardCheck, label: 'Approvals', routePath: '/approval', activeMatch: '/approval' },
  { id: 'contacts', icon: Users, label: 'Contacts', routePath: '/contacts', activeMatch: '/contacts' },
  { id: 'assets', icon: Briefcase, label: 'Work', routePath: '/dashboard', activeMatch: '/dashboard' },
]

/** Mobile bottom navigation bar — visible on < lg screens. Nexus Hub design tokens.
 *
 * Uses native <a> elements with useNavigate() instead of TanStack <Link> to guarantee
 * all items always render as interactive anchor elements regardless of route context.
 * TanStack Router's <Link> can fail to render as <a> in certain nested route scenarios. */
export function MobileNav() {
  const setActiveModule = useUiStore((s) => s.setActiveModule)
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()
  const { pathname } = useLocation()

  return (
    <nav className="fixed bottom-0 inset-x-0 h-14 bg-surface-container-lowest border-t border-outline-variant/30
      flex items-center justify-around z-[9999] lg:hidden">
      {navItems.map((item) => {
        const isActive = pathname.includes(item.activeMatch)
        const Icon = item.icon
        return (
          <a
            key={item.id}
            href={item.routePath}
            onClick={(e) => {
              e.preventDefault()
              setActiveModule(item.id)
              navigate({ to: item.routePath })
            }}
            className={`flex flex-col items-center justify-center gap-1
              min-h-11 min-w-11 no-underline cursor-pointer
              transition-colors ${isActive ? 'text-primary' : 'text-on-surface-variant'}`}
          >
            <Icon size={20} strokeWidth={isActive ? 2.2 : 1.6} />
            <span className="text-micro font-medium">{item.label}</span>
          </a>
        )
      })}
      <button
        onClick={logout}
        className="flex flex-col items-center justify-center gap-1
          min-h-11 min-w-11 bg-transparent border-none cursor-pointer
          text-on-surface-variant transition-colors"
      >
        <LogOut size={20} strokeWidth={1.6} />
        <span className="text-micro font-medium">Logout</span>
      </button>
    </nav>
  )
}
