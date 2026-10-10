import { createFileRoute, Outlet, Navigate, retainSearchParams, useMatches } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useUiStore } from '../stores/ui.store'
import { useActiveWorkspace } from '../hooks/useActiveWorkspace'
import { usePreferencesSync } from '../hooks/usePreferences'
import { useWorkspaceRealtime } from '../hooks/useWorkspaceRealtime'
import { useUnreadCounts } from '../hooks/useMessaging'
import { validateWorkspaceSearch, workspaceDisplayName } from '../lib/workspace'
import { useResizable } from '../hooks/useResizable'
import { AppSidebar } from '../components/patterns/AppSidebar'
import { ListPanel } from '../components/patterns/ListPanel'
import { MobileNav } from '../components/patterns/MobileNav'
import { MobileTopBar } from '../components/patterns/MobileTopBar'
import { NotificationsRuntime } from '../components/notifications/NotificationsRuntime'
import { Button, Spinner, Text } from '../components/primitives'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { apiFetch, logoutSession } from '../api/client'

export const Route = createFileRoute('/_workspace')({
  validateSearch: validateWorkspaceSearch,
  // Keep the chosen workspace across in-app navigation; a link that names no
  // `ws` would otherwise drop the user back into their first workspace.
  search: { middlewares: [retainSearchParams(['ws'])] },
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const token = useAuthStore((s) => s.accessToken)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const { workspaceId: wsId, workspaceName, workspaces, isLoading, isError } = useActiveWorkspace()
  usePreferencesSync()
  useWorkspaceRealtime(wsId, !!token)
  const { data: unreadData } = useUnreadCounts()
  const unreadMessages = (unreadData?.channels ?? []).reduce((n, c) => n + c.unread_count, 0)

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
    } else if (currentPath.includes('/drive') || currentPath.includes('/documents')) {
      // Văn bản is a group inside Tài liệu: same module, same list panel (its own).
      if (activeModule !== 'drive') setActiveModule('drive')
    } else if (currentPath.includes('/approval')) {
      if (activeModule !== 'approval') setActiveModule('approval')
    } else if (currentPath.includes('/assets')) {
      if (activeModule !== 'assets') setActiveModule('assets')
    } else if (currentPath.includes('/settings')) {
      if (activeModule !== 'settings') setActiveModule('settings')
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

  /* The screen under the phone top bar; the bar watches it for the screen's own search. */
  const [screenEl, setScreenEl] = useState<HTMLElement | null>(null)

  useEffect(() => {
    if (token) { connect(token); return () => disconnect() }
  }, [token])

  // When workspaces is empty after loading, verify session is still valid.
  const verifiedRef = useRef(false)
  useEffect(() => {
    if (!isLoading && workspaces.length === 0 && token && !verifiedRef.current) {
      verifiedRef.current = true
      apiFetch('/me').catch(() => { void logoutSession() })
    }
  }, [isLoading, workspaces, token])

  if (!token) return <Navigate to="/login" search={Object.fromEntries(new URLSearchParams(window.location.search))} />

  if (isError) {
    return (
      <div className="flex h-dvh bg-background overflow-hidden">
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center px-4">
            <Text variant="body" muted>Unable to load workspaces</Text>
            <Button variant="link" size="link" onClick={() => window.location.reload()} className="mt-4">Retry</Button>
            <Button variant="ghost" size="link" onClick={() => void logoutSession()} className="mt-2 block mx-auto">Logout</Button>
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

  if (workspaces.length === 0) {
    return (
      <div className="flex h-dvh bg-background overflow-hidden">
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center px-4">
            <Spinner size="lg" />
            <Text variant="body" muted className="mt-4">Setting up your workspace...</Text>
            <Button variant="link" size="link" onClick={() => window.location.reload()} className="mt-4">Refresh if this takes too long</Button>
            <Button variant="ghost" size="link" onClick={() => void logoutSession()} className="mt-2 block mx-auto">Logout and re-login</Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-dvh bg-background overflow-hidden">
      {/* Row 2: Sidebar + Content */}
      <div className="flex flex-1 min-h-0">
        {/* Column 1: AppSidebar — hidden on mobile, visible on lg+ */}
        <AppSidebar workspaceName={workspaceDisplayName(workspaceName)} unreadCounts={{ messaging: unreadMessages }} />

        {/* Column 2: ListPanel — only for messaging, documents, and workspace modules */}
        {activeModule !== 'contacts' && activeModule !== 'drive' && activeModule !== 'approval' && activeModule !== 'assets' && activeModule !== 'settings' && activeModule !== 'admin' && (
          <>
            {/* Tin nhắn's navigator column: desktop only. On a phone its job is done by Trang chủ. */}
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
        <main className="flex-1 flex flex-col min-w-0 pb-tabbar lg:pb-0 isolate overflow-hidden">
          <MobileTopBar scope={screenEl} />
          <div ref={setScreenEl} className="flex-1 flex flex-col min-h-0 min-w-0">
            <ErrorBoundary moduleName="workspace-content">
              <Outlet />
            </ErrorBoundary>
          </div>
        </main>
      </div>

      {/* Mobile bottom navigation */}
      <MobileNav />
      <NotificationsRuntime />
    </div>
  )
}
