import { createFileRoute, Outlet, Navigate, retainSearchParams, useMatches } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useUiStore } from '../stores/ui.store'
import { useActiveWorkspace } from '../hooks/useActiveWorkspace'
import { useTenantSync } from '../hooks/useTenantSync'
import { usePreferencesSync } from '../hooks/usePreferences'
import { useWorkspaceRealtime } from '../hooks/useWorkspaceRealtime'
import { useUnreadCounts } from '../hooks/useMessaging'
import { useChatState } from '../hooks/useChatState'
import { validateWorkspaceSearch, workspaceDisplayName } from '../lib/workspace'
import { isChatListPath } from '../lib/nav-destinations'
import { unreadByLevel } from '../lib/conversations'
import { useResizable } from '../hooks/useResizable'
import { useBottomBarLayout } from '../hooks/usePhone'
import { AppSidebar } from '../components/patterns/AppSidebar'
import { ListPanel } from '../components/patterns/ListPanel'
import { MobileNav } from '../components/patterns/MobileNav'
import { PhoneSearchBar } from '../components/patterns/PhoneSearchBar'
import { OfflineBar } from '../components/app/OfflineBar'
import { UpdateBar } from '../components/app/UpdateBar'
import { UpdatingBar } from '../components/app/UpdatingBar'
import { Button, Spinner, Text } from '../components/primitives'
import { ErrorBoundary } from '../components/ErrorBoundary'
import { CallProvider } from '../components/calls/CallProvider'
import { IncomingCallLayer } from '../components/calls/IncomingCallLayer'
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
  // While the boot refresh runs the shell may already render from the device copy;
  // only once it has failed is there no session.
  const restoring = useAuthStore((s) => s.bootstrapping)
  const { workspaces, isLoading, isError } = useActiveWorkspace()
  const { syncing } = useTenantSync()
  usePreferencesSync()

  // When workspaces is empty after loading, verify session is still valid.
  const verifiedRef = useRef(false)
  useEffect(() => {
    if (!isLoading && workspaces.length === 0 && token && !verifiedRef.current) {
      verifiedRef.current = true
      apiFetch('/me').catch(() => { void logoutSession() })
    }
  }, [isLoading, workspaces, token])

  if (!token && !restoring) return <Navigate to="/login" search={Object.fromEntries(new URLSearchParams(window.location.search))} />

  if (isError) {
    return (
      <div className="flex h-app bg-background overflow-hidden">
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
      <div className="flex items-center justify-center h-app bg-background">
        <Spinner size="lg" />
      </div>
    )
  }

  if (workspaces.length === 0) {
    return (
      <div className="flex h-app bg-background overflow-hidden">
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

  // The token still speaks for another workspace: nothing tenant-scoped may
  // mount or fetch until the session has been re-scoped to the open one.
  if (syncing) {
    return (
      <div className="flex items-center justify-center h-app bg-background">
        <Spinner size="lg" />
      </div>
    )
  }

  // The call room lives above every screen, so a call survives moving around the app.
  return <CallProvider><WorkspaceShell /></CallProvider>
}

function WorkspaceShell() {
  const token = useAuthStore((s) => s.accessToken)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const { workspaceId: wsId, workspaceName } = useActiveWorkspace()
  useWorkspaceRealtime(wsId, !!token)
  // One request for the navigator's state when the app opens; the 304 on the
  // next visit keeps what the cache holds.
  useChatState(wsId)
  const { data: unreadData } = useUnreadCounts()
  // By notification level: a muted conversation adds only the times someone named the reader.
  const unreadMessages = unreadByLevel(unreadData?.channels)

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

  // The floating Chat bar sits over the bottom of the list; only then does the screen keep room under its last row.
  const chatBarShown = useBottomBarLayout() && isChatListPath(currentPath)

  const { size, isDragging, handleProps } = useResizable({
    direction: 'horizontal',
    defaultSize: 240,
    minSize: 180,
    maxSize: 420,
    initialSize: listPanelWidth,
    onResize: setListPanelWidth,
  })

  /* The screen under the phone search bar; the bar watches it for the screen's own search. */
  const [screenEl, setScreenEl] = useState<HTMLElement | null>(null)

  useEffect(() => {
    if (token) { connect(token); return () => disconnect() }
  }, [token])

  return (
    <div className="flex flex-col h-app bg-background overflow-hidden">
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
        {/* 108px: the bar's 64, its 22 off the edge and 22 of air; the safe-area inset comes on top. */}
        <main className={`flex-1 flex flex-col min-w-0 ${chatBarShown ? 'pb-[calc(6.75rem+env(safe-area-inset-bottom,0px))]' : ''} lg:pb-0 isolate overflow-hidden`}>
          <PhoneSearchBar scope={screenEl} />
          <OfflineBar />
          <UpdatingBar />
          <div ref={setScreenEl} className="flex-1 flex flex-col min-h-0 min-w-0">
            <ErrorBoundary moduleName="workspace-content">
              <Outlet />
            </ErrorBoundary>
          </div>
        </main>
      </div>

      {/* Phone: Chat's bottom bar and the + beside it, on the conversation list only */}
      <MobileNav />
      <UpdateBar />
      {/* A call ringing this person, on every screen of the workspace */}
      <IncomingCallLayer />
    </div>
  )
}
