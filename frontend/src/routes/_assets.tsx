import { createFileRoute, Outlet, Navigate, Link, useMatchRoute } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useWorkspaces } from '../hooks/useWorkspaces'
import NotificationBell from '../components/NotificationBell'
import { Heading, Spinner, Avatar } from '../components/primitives'

export const Route = createFileRoute('/_assets')({
  component: AssetLayout,
})

/** Sidebar navigation items for the asset management module. */
const NAV_ITEMS = [
  { to: '/dashboard', icon: '📊', label: 'Dashboard' },
  { to: '/list', icon: '📦', label: 'All Assets' },
  { to: '/requests', icon: '📋', label: 'Requests' },
  { to: '/types', icon: '🏷️', label: 'Types' },
] as const

function AssetLayout() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const { data, isLoading } = useWorkspaces()
  const matchRoute = useMatchRoute()

  useEffect(() => {
    if (token) { connect(token); return () => disconnect() }
  }, [token])

  if (!token) return <Navigate to="/login" />
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen bg-bg-primary">
        <Spinner size="lg" />
      </div>
    )
  }

  const workspaces = data?.workspaces || []
  const wsName = workspaces[0]?.name || 'Workspace'

  return (
    <div className="flex h-screen bg-bg-primary overflow-hidden">
      {/* Asset sidebar */}
      <div className="w-[220px] flex-shrink-0 bg-bg-secondary border-r border-border flex flex-col">
        {/* Back to workspace */}
        <div className="px-4 py-3 border-b border-border">
          <Link to="/documents" className="flex items-center gap-2 text-sm text-text-secondary
            hover:text-text-primary transition-colors no-underline">
            <span>←</span>
            <span className="truncate">{wsName}</span>
          </Link>
        </div>

        {/* Nav items */}
        <nav className="flex-1 py-2">
          <div className="px-4 py-2">
            <span className="text-[0.65rem] font-semibold text-text-muted uppercase tracking-wider">
              Management
            </span>
          </div>
          {NAV_ITEMS.map((item) => {
            const isActive = !!matchRoute({ to: item.to, fuzzy: true })
            return (
              <Link key={item.to} to={item.to}
                className={`flex items-center gap-2.5 px-4 py-2 text-sm transition-colors no-underline
                  ${isActive
                    ? 'bg-bg-active text-accent-hover font-medium'
                    : 'text-text-secondary hover:bg-bg-hover hover:text-text-primary'
                  }`}
              >
                <span>{item.icon}</span>
                <span>{item.label}</span>
              </Link>
            )
          })}
        </nav>

        {/* Footer */}
        <div className="border-t border-border py-2">
          <Link to="/settings" className="flex items-center gap-2.5 px-4 py-2 text-sm
            text-text-secondary hover:bg-bg-hover hover:text-text-primary transition-colors no-underline">
            <span>⚙️</span><span>Settings</span>
          </Link>
          <button onClick={logout} className="flex items-center gap-2.5 px-4 py-2 text-sm w-full
            text-text-secondary hover:bg-bg-hover hover:text-text-primary transition-colors
            bg-transparent border-none cursor-pointer text-left">
            <span>🚪</span><span>Logout ({user?.username})</span>
          </button>
        </div>
      </div>

      {/* Main content */}
      <main className="flex-1 flex flex-col min-w-0">
        <div className="h-[52px] px-5 border-b border-border flex items-center justify-between
          bg-bg-tertiary/60 backdrop-blur-sm flex-shrink-0">
          <Heading as="h4">Asset Management</Heading>
          <NotificationBell />
        </div>
        <div className="flex-1 overflow-y-auto p-5">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
