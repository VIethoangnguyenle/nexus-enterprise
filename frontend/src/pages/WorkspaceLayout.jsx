import { useEffect, useState } from 'react'
import { Routes, Route, Navigate, useNavigate } from 'react-router-dom'
import { useAuthStore, useWorkspaceStore, useMessagingStore } from '../store'
import Sidebar from '../components/Sidebar'
import DocumentsView from './DocumentsView'
import ChatView from './ChatView'
import SettingsView from './SettingsView'

export default function WorkspaceLayout() {
  const user = useAuthStore(s => s.user)
  const token = useAuthStore(s => s.token)
  const { workspaces, current, fetchWorkspaces, selectWorkspace } = useWorkspaceStore()
  const connectWebSocket = useMessagingStore(s => s.connectWebSocket)
  const disconnectWebSocket = useMessagingStore(s => s.disconnectWebSocket)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    fetchWorkspaces().then(() => setReady(true))
  }, [])

  // Connect WebSocket on mount
  useEffect(() => {
    if (token) {
      connectWebSocket(token)
      return () => disconnectWebSocket()
    }
  }, [token])

  // Auto-select first workspace if none selected
  useEffect(() => {
    if (ready && workspaces.length > 0 && !current) {
      selectWorkspace(workspaces[0])
    }
  }, [ready, workspaces, current])

  if (!ready) {
    return (
      <div className="auth-page">
        <div className="spinner" style={{ width: 40, height: 40 }} />
      </div>
    )
  }

  if (!current && workspaces.length === 0) {
    return <NoWorkspace />
  }

  return (
    <div className="app-layout">
      <Sidebar />
      <div className="main-content">
        <Routes>
          <Route path="/documents" element={<DocumentsView />} />
          <Route path="/channels/:channelId" element={<ChatView />} />
          <Route path="/dms/:channelId" element={<ChatView />} />
          <Route path="/settings" element={<SettingsView />} />
          <Route path="*" element={<Navigate to="/documents" replace />} />
        </Routes>
      </div>
    </div>
  )
}

function NoWorkspace() {
  const [name, setName] = useState('')
  const [loading, setLoading] = useState(false)
  const createWorkspace = useWorkspaceStore(s => s.createWorkspace)
  const selectWorkspace = useWorkspaceStore(s => s.selectWorkspace)
  const fetchWorkspaces = useWorkspaceStore(s => s.fetchWorkspaces)
  const logout = useAuthStore(s => s.logout)

  const handleCreate = async (e) => {
    e.preventDefault()
    if (!name.trim()) return
    setLoading(true)
    try {
      const ws = await createWorkspace(name.trim())
      await fetchWorkspaces()
      selectWorkspace(ws)
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  return (
    <div className="auth-page">
      <div className="auth-card fade-in">
        <h1>Create a workspace</h1>
        <p>Workspaces let you organize your team's documents and channels with fine-grained access control.</p>
        <form onSubmit={handleCreate}>
          <div className="form-group">
            <label>Workspace Name</label>
            <input value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Engineering, Marketing" autoFocus />
          </div>
          <button className="btn btn-primary" type="submit" disabled={loading}>
            {loading ? <span className="spinner" /> : 'Create Workspace'}
          </button>
        </form>
        <div className="auth-link">
          <a href="#" onClick={(e) => { e.preventDefault(); logout() }}>Sign out</a>
        </div>
      </div>
    </div>
  )
}
