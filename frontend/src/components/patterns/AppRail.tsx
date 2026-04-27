import { useAuthStore } from '../../stores/auth.store'
import { useUiStore } from '../../stores/ui.store'
import { Avatar } from '../primitives'

type RailItem = {
  id: 'messaging' | 'documents' | 'drive' | 'assets' | 'settings'
  icon: string
  label: string
}

const railItems: RailItem[] = [
  { id: 'messaging', icon: '💬', label: 'Messaging' },
  { id: 'documents', icon: '📄', label: 'Documents' },
  { id: 'drive', icon: '💾', label: 'Drive' },
  { id: 'assets', icon: '📦', label: 'Assets' },
  { id: 'settings', icon: '⚙️', label: 'Settings' },
]

/** 48px icon-only navigation rail. Lark-inspired left column. */
export function AppRail() {
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const activeModule = useUiStore((s) => s.activeModule)
  const setActiveModule = useUiStore((s) => s.setActiveModule)

  return (
    <div className="w-12 flex-shrink-0 bg-bg-rail border-r border-border
      flex flex-col items-center py-3 gap-1">
      {/* Logo */}
      <div className="flex items-center justify-center w-8 h-8 rounded-lg
        bg-gradient-to-br from-accent to-[#8b5cf6] mb-3 flex-shrink-0">
        <span className="text-white font-bold text-xs">N</span>
      </div>

      {/* Nav items */}
      <nav className="flex flex-col items-center gap-0.5 flex-1">
        {railItems.map(item => (
          <button
            key={item.id}
            onClick={() => setActiveModule(item.id)}
            title={item.label}
            aria-label={item.label}
            className={`w-9 h-9 flex items-center justify-center rounded-[var(--radius-sm)]
              text-sm transition-all duration-150 cursor-pointer border-none
              ${activeModule === item.id
                ? 'bg-bg-active text-accent-hover'
                : 'bg-transparent text-text-muted hover:text-text-primary hover:bg-bg-hover'
              }`}
          >
            {item.icon}
          </button>
        ))}
      </nav>

      {/* User avatar + logout */}
      <div className="flex flex-col items-center gap-1 mt-auto">
        <button
          onClick={logout}
          title={`Logout (${user?.username})`}
          aria-label="Logout"
          className="w-9 h-9 flex items-center justify-center rounded-[var(--radius-sm)]
            text-sm text-text-muted hover:text-danger hover:bg-danger-bg
            transition-all duration-150 cursor-pointer border-none bg-transparent"
        >
          🚪
        </button>
        <Avatar name={user?.username || 'U'} size="sm" />
      </div>
    </div>
  )
}
