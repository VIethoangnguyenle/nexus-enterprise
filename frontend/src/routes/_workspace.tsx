import { createFileRoute, Outlet, Navigate, useMatches } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useUiStore } from '../stores/ui.store'
import { useWorkspaces } from '../hooks/useWorkspaces'
import { useResizable } from '../hooks/useResizable'
import { AppSidebar } from '../components/patterns/AppSidebar'
import { TopBar } from '../components/patterns/TopBar'
import { ListPanel } from '../components/patterns/ListPanel'
import { MobileNav } from '../components/patterns/MobileNav'
import { Spinner, Text } from '../components/primitives'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { apiFetch } from '../api/client'
import { PanelLeft, X } from 'lucide-react'

export const Route = createFileRoute('/_workspace')({
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const token = useAuthStore((s) => s.token)
  const logout = useAuthStore((s) => s.logout)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const { data, isLoading, isError } = useWorkspaces()

  const listPanelWidth = useUiStore((s) => s.listPanelWidth)
  const setListPanelWidth = useUiStore((s) => s.setListPanelWidth)
  const activeModule = useUiStore((s) => s.activeModule)
  const setActiveModule = useUiStore((s) => s.setActiveModule)

  /* Sync activeModule store with current route path so ListPanel shows correct context.
   * Without this, navigating via URL or sidebar routePath doesn't update the store. */
  const matches = useMatches()
  const currentPath = matches[matches.length - 1]?.pathname || ''
  useEffect(() => {
    if (currentPath.includes('/channels')) {
      if (activeModule !== 'messaging') setActiveModule('messaging')
    } else if (currentPath.includes('/admin')) {
      if (activeModule !== 'admin') setActiveModule('admin')
    } else if (currentPath.includes('/contacts')) {
      if (activeModule !== 'contacts') setActiveModule('contacts')
    } else if (currentPath.includes('/drive')) {
      if (activeModule !== 'drive') setActiveModule('drive')
    } else if (currentPath.includes('/approval')) {
      if (activeModule !== 'approval') setActiveModule('approval')
    } else if (currentPath.includes('/assets')) {
      if (activeModule !== 'assets') setActiveModule('assets')
    } else if (currentPath.includes('/settings')) {
      if (activeModule !== 'settings') setActiveModule('settings')
    } else if (currentPath.includes('/documents')) {
      if (activeModule !== 'documents') setActiveModule('documents')
    }
  }, [currentPath])

  const { size, isDragging, handleProps } = useResizable({
    direction: 'horizontal',
    defaultSize: 240,
    minSize: 180,
    maxSize: 420,
    initialSize: listPanelWidth,
    onResize: setListPanelWidth,
  })

  /* Mobile: toggle list panel overlay */
  const [showMobileList, setShowMobileList] = useState(false)

  /* Listen for child routes requesting mobile list panel (e.g. channel back button) */
  useEffect(() => {
    const openList = () => setShowMobileList(true)
    window.addEventListener('open-mobile-list', openList)
    return () => window.removeEventListener('open-mobile-list', openList)
  }, [])

  useEffect(() => {
    if (token) { connect(token); return () => disconnect() }
  }, [token])

  // When workspaces is empty after loading, verify session is still valid.
  const verifiedRef = useRef(false)
  useEffect(() => {
    const workspaces = data?.workspaces || []
    if (!isLoading && workspaces.length === 0 && token && !verifiedRef.current) {
      verifiedRef.current = true
      apiFetch('/me').catch(() => { logout() })
    }
  }, [isLoading, data, token, logout])

  if (!token) return <Navigate to="/login" search={Object.fromEntries(new URLSearchParams(window.location.search))} />

  if (isError) {
    return (
      <div className="flex h-dvh bg-background overflow-hidden">
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center px-4">
            <Text variant="body" muted>Unable to load workspaces</Text>
            <button
              onClick={() => window.location.reload()}
              className="mt-4 text-small text-primary hover:text-primary-hover bg-transparent border-none cursor-pointer focus-ring"
            >Retry</button>
            <button
              onClick={() => logout()}
              className="mt-2 block mx-auto text-small text-on-surface-variant hover:text-on-surface bg-transparent border-none cursor-pointer focus-ring"
            >Logout</button>
          </div>
        </div>
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-dvh bg-background">
        <Spinner size="lg" />
      </div>
    )
  }

  const workspaces = data?.workspaces || []

  if (workspaces.length === 0) {
    return (
      <div className="flex h-dvh bg-background overflow-hidden">
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center px-4">
            <Spinner size="lg" />
            <Text variant="body" muted className="mt-4">Setting up your workspace...</Text>
            <button
              onClick={() => window.location.reload()}
              className="mt-4 text-small text-primary hover:text-primary-hover bg-transparent border-none cursor-pointer focus-ring"
            >Refresh if this takes too long</button>
            <button
              onClick={() => logout()}
              className="mt-2 block mx-auto text-small text-on-surface-variant hover:text-on-surface bg-transparent border-none cursor-pointer focus-ring"
            >Logout and re-login</button>
          </div>
        </div>
      </div>
    )
  }

  const wsParam = new URLSearchParams(window.location.search).get('ws')
  const selectedWs = wsParam ? workspaces.find(w => w.id === wsParam) : undefined
  const wsId = selectedWs?.id || workspaces[0].id

  return (
    <div className="flex flex-col h-dvh bg-background overflow-hidden">
      {/* Row 1: TopBar — full width */}
      <TopBar />

      {/* Row 2: Sidebar + Content */}
      <div className="flex flex-1 min-h-0">
        {/* Column 1: AppSidebar — hidden on mobile, visible on lg+ */}
        <AppSidebar workspaceName={formatWorkspaceName(workspaces.find(w => w.id === wsId)?.name)} />

        {/* Column 2: ListPanel — only for messaging, documents, and workspace modules */}
        {activeModule !== 'contacts' && activeModule !== 'drive' && activeModule !== 'approval' && activeModule !== 'assets' && activeModule !== 'settings' && activeModule !== 'admin' && (
          <>
            {showMobileList && (
              <div className="fixed inset-0 bottom-14 z-40 bg-surface-bright lg:hidden animate-slide-left">
                <div className="flex items-center justify-end px-3 py-2 border-b border-outline-variant/30">
                  <button
                    onClick={() => setShowMobileList(false)}
                    className="min-h-11 min-w-11 flex items-center justify-center
                      bg-transparent border-none cursor-pointer text-on-surface-variant hover:text-on-surface"
                  >
                    <X size={20} />
                  </button>
                </div>
                <div className="flex-1 overflow-hidden h-[calc(100%-44px)]">
                  <ListPanel workspaceId={wsId} />
                </div>
              </div>
            )}
            {/* Desktop inline */}
            <div style={{ width: size, flexShrink: 0 }} className="h-full hidden lg:block">
              <ListPanel workspaceId={wsId} />
            </div>

            {/* Resize handle — desktop only */}
            <div
              {...handleProps}
              className={`resize-handle hidden lg:block ${isDragging ? 'is-dragging' : ''}`}
            />
          </>
        )}

        {/* Column 3: Content */}
        <main className="flex-1 flex flex-col min-w-0 pb-14 lg:pb-0 isolate overflow-hidden">
          {/* Mobile: toggle list panel button */}
          <div className="flex items-center h-11 px-3 border-b border-outline-variant/20 bg-surface-container-lowest lg:hidden">
            <button
              onClick={() => setShowMobileList(true)}
              className="min-h-9 min-w-9 flex items-center justify-center gap-2
                bg-transparent border-none cursor-pointer text-on-surface-variant hover:text-on-surface
                text-small"
            >
              <PanelLeft size={18} />
              <span>Menu</span>
            </button>
          </div>
          <ErrorBoundary moduleName="workspace-content">
            <Outlet />
          </ErrorBoundary>
        </main>
      </div>

      {/* Mobile bottom navigation */}
      <MobileNav />
    </div>
  )
}

/** Format raw workspace name for display.
 *  Converts auto-generated names like "user_123's workspace" to "My Workspace". */
function formatWorkspaceName(name: string | undefined): string {
  if (!name) return 'Nexus Workspace'
  // Personal workspace pattern: "{username}'s workspace"
  if (name.match(/^user_\d+'s\s+workspace$/i)) {
    return 'My Workspace'
  }
  return name
}
