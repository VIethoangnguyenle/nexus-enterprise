import { createFileRoute, Outlet, Navigate, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useWorkspaces } from '../hooks/useWorkspaces'
import Sidebar from '../components/Sidebar'
import NotificationBell from '../components/NotificationBell'

export const Route = createFileRoute('/_workspace')({
  component: WorkspaceLayout,
})

function WorkspaceLayout() {
  const token = useAuthStore((s) => s.token)
  const connect = useWebSocketStore((s) => s.connect)
  const disconnect = useWebSocketStore((s) => s.disconnect)
  const { data, isLoading } = useWorkspaces()

  useEffect(() => {
    if (token) { connect(token); return () => disconnect() }
  }, [token])

  if (!token) return <Navigate to="/login" />
  if (isLoading) return <div className="auth-page"><div className="spinner" style={{ width: 40, height: 40 }} /></div>

  const workspaces = data?.workspaces || []

  return (
    <div className="app-layout">
      <Sidebar workspaces={workspaces} />
      <div className="main-content">
        <div className="workspace-topbar">
          <div className="topbar-breadcrumb">
            <span className="topbar-ws-name">{workspaces[0]?.name || 'Workspace'}</span>
          </div>
          <div className="topbar-actions"><NotificationBell /></div>
        </div>
        <div className="content-body"><Outlet /></div>
      </div>
    </div>
  )
}
