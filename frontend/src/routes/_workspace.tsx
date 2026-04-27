import { createFileRoute, Outlet, Navigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useUiStore } from '../stores/ui.store'
import { useWorkspaces, useCreateWorkspace } from '../hooks/useWorkspaces'
import { AppRail } from '../components/patterns/AppRail'
import { ListPanel } from '../components/patterns/ListPanel'
import NotificationBell from '../components/NotificationBell'
import { Spinner, Heading, Button, Input, Text } from '../components/primitives'

export const Route = createFileRoute('/_workspace')({
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const token = useAuthStore((s) => s.token)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const listPanelOpen = useUiStore((s) => s.listPanelOpen)
  const { data, isLoading } = useWorkspaces()

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

  if (workspaces.length === 0) {
    return (
      <div className="flex h-screen bg-bg-primary overflow-hidden">
        <AppRail />
        <div className="flex-1 flex items-center justify-center">
          <CreateWorkspaceCard />
        </div>
      </div>
    )
  }

  const wsId = workspaces[0].id

  return (
    <div className="flex h-screen bg-bg-primary overflow-hidden">
      {/* Column 1: Rail (48px) */}
      <AppRail />

      {/* Column 2: List Panel (~280px, collapsible) */}
      {listPanelOpen && <ListPanel workspaceId={wsId} />}

      {/* Column 3: Content (flex) */}
      <main className="flex-1 flex flex-col min-w-0">
        {/* Topbar */}
        <div className="h-[52px] px-5 border-b border-border flex items-center justify-between
          bg-bg-tertiary/60 backdrop-blur-sm flex-shrink-0">
          <div className="flex items-center gap-2">
            <Heading as="h4">{workspaces[0].name}</Heading>
          </div>
          <div className="flex items-center gap-2">
            <NotificationBell />
          </div>
        </div>

        {/* Page content */}
        <div className="flex-1 overflow-y-auto p-5">
          <Outlet />
        </div>
      </main>
    </div>
  )
}

/** Onboarding card shown when user has no workspaces. */
function CreateWorkspaceCard() {
  const [name, setName] = useState('')
  const createWs = useCreateWorkspace()

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return
    createWs.mutate(trimmed)
  }

  return (
    <div className="max-w-md w-full p-8 bg-bg-secondary rounded-[var(--radius-lg)] border border-border shadow-xl">
      <div className="text-center mb-6">
        <div className="text-4xl mb-3">🏢</div>
        <Heading as="h3">Welcome to NGAC Platform</Heading>
        <Text variant="body" muted className="mt-2">
          Create your first workspace to get started.
        </Text>
      </div>
      <form onSubmit={handleCreate} className="flex flex-col gap-4">
        {createWs.error && (
          <div className="bg-danger-bg text-danger px-3 py-2 rounded-[var(--radius-sm)] text-sm">
            {createWs.error.message}
          </div>
        )}
        <Input
          label="Workspace Name"
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. My Team, Engineering"
        />
        <Button type="submit" disabled={!name.trim() || createWs.isPending}>
          {createWs.isPending ? <Spinner size="sm" /> : 'Create Workspace'}
        </Button>
      </form>
    </div>
  )
}
