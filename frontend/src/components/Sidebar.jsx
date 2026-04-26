import { useEffect, useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore, useWorkspaceStore, useMessagingStore } from '../store'

export default function Sidebar() {
  const user = useAuthStore(s => s.user)
  const logout = useAuthStore(s => s.logout)
  const navigate = useNavigate()
  const location = useLocation()
  const { workspaces, current, selectWorkspace, createWorkspace, fetchWorkspaces } = useWorkspaceStore()
  const { channels, dms, fetchChannels, fetchDMs, selectChannel, activeChannel, createChannel, createDM } = useMessagingStore()
  const fetchUsers = useAuthStore(s => s.fetchUsers)

  const [wsDropdownOpen, setWsDropdownOpen] = useState(false)
  const [showNewChannel, setShowNewChannel] = useState(false)
  const [showNewDM, setShowNewDM] = useState(false)
  const [showCreateWs, setShowCreateWs] = useState(false)
  const [newName, setNewName] = useState('')
  const [users, setUsers] = useState([])

  useEffect(() => {
    if (current) {
      fetchChannels(current.id)
      fetchDMs()
    }
  }, [current?.id])

  const handleSelectWorkspace = (ws) => {
    selectWorkspace(ws)
    setWsDropdownOpen(false)
    navigate('/documents')
  }

  const handleCreateWorkspace = async () => {
    if (!newName.trim()) return
    try {
      const ws = await createWorkspace(newName.trim())
      await fetchWorkspaces()
      selectWorkspace(ws)
      setShowCreateWs(false)
      setNewName('')
      navigate('/documents')
    } catch (err) {
      console.error(err)
    }
  }

  const handleCreateChannel = async () => {
    if (!newName.trim() || !current) return
    try {
      const ch = await createChannel(current.id, newName.trim())
      setShowNewChannel(false)
      setNewName('')
      if (ch) {
        selectChannel(ch)
        navigate(`/channels/${ch.id}`)
      }
    } catch (err) {
      console.error(err)
    }
  }

  const handleCreateDM = async (targetUser) => {
    try {
      const ch = await createDM(targetUser.id, targetUser.ngac_node_id || targetUser.ngacNodeId)
      setShowNewDM(false)
      if (ch) {
        selectChannel(ch)
        navigate(`/dms/${ch.id}`)
      }
    } catch (err) {
      console.error(err)
    }
  }

  const openNewDM = async () => {
    try {
      const u = await fetchUsers()
      setUsers(u.filter(x => x.id !== user?.id))
    } catch {}
    setShowNewDM(true)
  }

  const handleChannelClick = (ch) => {
    selectChannel(ch)
    navigate(`/channels/${ch.id}`)
  }

  const handleDMClick = (ch) => {
    selectChannel(ch)
    navigate(`/dms/${ch.id}`)
  }

  const initials = (name) => name ? name.slice(0, 2).toUpperCase() : '?'

  return (
    <aside className="sidebar">
      {/* Workspace Selector */}
      <div className="sidebar-header">
        <div className="workspace-selector">
          <button className="workspace-btn" onClick={() => setWsDropdownOpen(!wsDropdownOpen)}>
            <div className="workspace-icon">{current ? initials(current.name) : '+'}</div>
            <span className="workspace-name">{current?.name || 'Select workspace'}</span>
            <span className="workspace-chevron">▾</span>
          </button>

          {wsDropdownOpen && (
            <div className="workspace-dropdown">
              {workspaces.map(ws => (
                <button key={ws.id} className={`workspace-dropdown-item ${ws.id === current?.id ? 'active' : ''}`} onClick={() => handleSelectWorkspace(ws)}>
                  <div className="workspace-icon" style={{ width: 24, height: 24, fontSize: '0.65rem' }}>{initials(ws.name)}</div>
                  {ws.name}
                </button>
              ))}
              <div className="workspace-dropdown-divider" />
              <button className="workspace-dropdown-item create" onClick={() => { setShowCreateWs(true); setWsDropdownOpen(false) }}>
                + Create Workspace
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Navigation */}
      <nav className="sidebar-nav">
        {/* Documents */}
        <div className="sidebar-section">
          <div onClick={() => navigate('/documents')} style={{ cursor: 'pointer' }}
               className={`sidebar-item ${location.pathname === '/documents' ? 'active' : ''}`}>
            <span className="sidebar-item-icon">📄</span>
            <span className="sidebar-item-text">Documents</span>
          </div>
        </div>

        {/* Channels */}
        <div className="sidebar-section">
          <div className="sidebar-section-header">
            <span className="sidebar-section-title">Channels</span>
            <button className="sidebar-add-btn" onClick={() => { setShowNewChannel(true); setNewName('') }} title="Create channel">+</button>
          </div>
          {(channels || []).map(ch => (
            <div key={ch.id}
                 className={`sidebar-item ${activeChannel?.id === ch.id ? 'active' : ''}`}
                 onClick={() => handleChannelClick(ch)}>
              <span className="sidebar-item-icon">#</span>
              <span className="sidebar-item-text">{ch.name}</span>
            </div>
          ))}
          {(!channels || channels.length === 0) && (
            <div className="sidebar-item" style={{ opacity: 0.5, cursor: 'default' }}>
              <span className="sidebar-item-icon" style={{ fontSize: '0.8rem' }}>💬</span>
              <span className="sidebar-item-text" style={{ fontSize: '0.8rem' }}>No channels yet</span>
            </div>
          )}
        </div>

        {/* DMs */}
        <div className="sidebar-section">
          <div className="sidebar-section-header">
            <span className="sidebar-section-title">Direct Messages</span>
            <button className="sidebar-add-btn" onClick={openNewDM} title="New DM">+</button>
          </div>
          {(dms || []).map(ch => (
            <div key={ch.id}
                 className={`sidebar-item ${activeChannel?.id === ch.id ? 'active' : ''}`}
                 onClick={() => handleDMClick(ch)}>
              <span className="sidebar-item-icon">●</span>
              <span className="sidebar-item-text">{ch.name}</span>
            </div>
          ))}
        </div>

        {/* Settings */}
        <div className="sidebar-section" style={{ marginTop: '0.5rem' }}>
          <div className={`sidebar-item ${location.pathname === '/settings' ? 'active' : ''}`}
               onClick={() => navigate('/settings')}>
            <span className="sidebar-item-icon">⚙️</span>
            <span className="sidebar-item-text">Settings</span>
          </div>
        </div>
      </nav>

      {/* User Footer */}
      <div className="sidebar-footer">
        <div className="sidebar-avatar">{user ? initials(user.username) : '?'}</div>
        <div className="sidebar-user-info">
          <div className="sidebar-user-name">{user?.username}</div>
          <div className="sidebar-user-status">Online</div>
        </div>
        <button className="sidebar-logout" onClick={logout} title="Sign out">⏻</button>
      </div>

      {/* Create Workspace Modal */}
      {showCreateWs && (
        <div className="modal-overlay" onClick={() => setShowCreateWs(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Create Workspace</h2>
            <div className="form-group">
              <label>Name</label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="Workspace name" autoFocus
                     onKeyDown={e => e.key === 'Enter' && handleCreateWorkspace()} />
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowCreateWs(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleCreateWorkspace}>Create</button>
            </div>
          </div>
        </div>
      )}

      {/* Create Channel Modal */}
      {showNewChannel && (
        <div className="modal-overlay" onClick={() => setShowNewChannel(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Create Channel</h2>
            <div className="form-group">
              <label>Channel Name</label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="e.g. general, engineering" autoFocus
                     onKeyDown={e => e.key === 'Enter' && handleCreateChannel()} />
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowNewChannel(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleCreateChannel}>Create</button>
            </div>
          </div>
        </div>
      )}

      {/* New DM Modal */}
      {showNewDM && (
        <div className="modal-overlay" onClick={() => setShowNewDM(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>New Direct Message</h2>
            <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem', fontSize: '0.85rem' }}>Select a user to message</p>
            {users.map(u => (
              <div key={u.id} className="member-row" style={{ cursor: 'pointer' }} onClick={() => handleCreateDM(u)}>
                <div className="member-avatar">{initials(u.username)}</div>
                <div className="member-info">
                  <div className="member-name">{u.username}</div>
                </div>
              </div>
            ))}
            {users.length === 0 && (
              <div style={{ textAlign: 'center', padding: '1rem', color: 'var(--text-muted)' }}>No other users found</div>
            )}
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowNewDM(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
    </aside>
  )
}
