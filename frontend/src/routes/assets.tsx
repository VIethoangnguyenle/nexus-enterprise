import { createFileRoute, Outlet, Navigate, Link, useMatchRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useWorkspaces } from '../hooks/useWorkspaces'
import NotificationBell from '../components/NotificationBell'
import { MobileNav } from '../components/patterns/MobileNav'
import { Heading, Spinner, Avatar } from '../components/primitives'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { LayoutDashboard, Package, ClipboardList, Tag, Settings, LogOut, ArrowLeft, Menu, X } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

export const Route = createFileRoute('/assets')({
  component: AssetLayout,
})

/** Sidebar navigation items for the asset management module. */
const NAV_ITEMS: { to: string; icon: LucideIcon; label: string }[] = [
  { to: '/assets/dashboard', icon: LayoutDashboard, label: 'Dashboard' },
  { to: '/assets/list', icon: Package, label: 'All Assets' },
  { to: '/assets/requests', icon: ClipboardList, label: 'Requests' },
  { to: '/assets/types', icon: Tag, label: 'Types' },
]

function AssetLayout() {
  const token = useAuthStore((s) => s.token)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const { data, isLoading } = useWorkspaces()
  const matchRoute = useMatchRoute()
  const [showMobileSidebar, setShowMobileSidebar] = useState(false)

  useEffect(() => {
    if (token) { connect(token); return () => disconnect() }
  }, [token])

  if (!token) return <Navigate to="/login" />
  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-dvh bg-surface">
        <Spinner size="lg" />
      </div>
    )
  }

  const workspaces = data?.workspaces || []
  const wsName = workspaces[0]?.name || 'Workspace'

  const sidebarContent = (
    <>
      {/* Back to workspace */}
      <div className="px-4 py-3 border-b border-outline-variant">
        <Link to="/channels" className="flex items-center gap-2 text-sm text-on-surface-variant
          hover:text-on-surface transition-colors no-underline">
          <ArrowLeft size={16} />
          <span className="truncate">{wsName}</span>
        </Link>
      </div>

      {/* Nav items */}
      <nav className="flex-1 py-2">
        <div className="px-4 py-2">
          <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">
            Management
          </span>
        </div>
        {NAV_ITEMS.map((item) => {
          const isActive = !!matchRoute({ to: item.to, fuzzy: true })
          return (
            <Link key={item.to} to={item.to}
              onClick={() => setShowMobileSidebar(false)}
              className={`flex items-center gap-2 px-4 py-2 text-sm rounded transition-colors no-underline
                ${isActive
                  ? 'bg-surface-container-high text-primary font-bold'
                  : 'text-on-surface-variant hover:bg-surface-container-high hover:text-on-surface'
                }`}
            >
              <item.icon size={16} strokeWidth={isActive ? 2.2 : 1.8} />
              <span>{item.label}</span>
            </Link>
          )
        })}
      </nav>

      {/* Footer */}
      <div className="border-t border-outline-variant py-2">
        <Link to="/settings" className="flex items-center gap-2 px-4 py-2 text-sm
          text-on-surface-variant hover:bg-surface-container-high hover:text-on-surface transition-colors no-underline">
          <Settings size={16} /><span>Settings</span>
        </Link>
        <button onClick={logout} className="flex items-center gap-2 px-4 py-2 text-sm w-full
          text-on-surface-variant hover:bg-surface-container-high hover:text-on-surface transition-colors
          bg-transparent border-none cursor-pointer text-left">
          <LogOut size={16} /><span>Logout ({user?.username})</span>
        </button>
      </div>
    </>
  )

  return (
    <div className="flex flex-col h-dvh bg-surface overflow-hidden">
      <div className="flex flex-1 min-h-0">
        {/* Sidebar — hidden on mobile, visible on lg+ */}
        <div className="hidden lg:flex w-[260px] flex-shrink-0 bg-surface-container-low border-r border-outline-variant flex-col">
          {sidebarContent}
        </div>

        {/* Mobile sidebar overlay */}
        {showMobileSidebar && (
          <div className="fixed inset-0 z-50 lg:hidden">
            {/* Backdrop */}
            <div className="absolute inset-0 bg-black/40" onClick={() => setShowMobileSidebar(false)} />
            {/* Panel */}
            <div className="absolute left-0 top-0 bottom-0 w-[280px] bg-surface-container-low flex flex-col animate-slide-left">
              <div className="flex items-center justify-end px-3 py-2 border-b border-outline-variant">
                <button
                  onClick={() => setShowMobileSidebar(false)}
                  className="min-h-11 min-w-11 flex items-center justify-center
                    bg-transparent border-none cursor-pointer text-on-surface-variant"
                >
                  <X size={20} />
                </button>
              </div>
              {sidebarContent}
            </div>
          </div>
        )}

        {/* Main content */}
        <main className="flex-1 flex flex-col min-w-0 pb-14 lg:pb-0">
          <div className="h-11 px-3 lg:px-4 border-b border-outline-variant flex items-center justify-between
            bg-surface-container-lowest flex-shrink-0">
            <div className="flex items-center gap-2">
              {/* Mobile hamburger */}
              <button
                onClick={() => setShowMobileSidebar(true)}
                className="min-h-9 min-w-9 flex items-center justify-center
                  bg-transparent border-none cursor-pointer text-on-surface-variant lg:hidden"
              >
                <Menu size={18} />
              </button>
              <h2 className="text-sm font-semibold text-on-surface">Asset Management</h2>
            </div>
            <NotificationBell />
          </div>
          <div className="flex-1 overflow-y-auto p-4 md:p-6">
            <ErrorBoundary moduleName="assets-content">
              <Outlet />
            </ErrorBoundary>
          </div>
        </main>
      </div>

      {/* Mobile bottom navigation — allows escaping back to other modules */}
      <MobileNav />
    </div>
  )
}
