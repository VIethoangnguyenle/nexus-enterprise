import { useEffect, useState } from 'react'
import { useWorkspaceStore, useAuthStore } from '../store'

export default function SettingsView() {
  const current = useWorkspaceStore(s => s.current)
  const { members, roles, folders, fetchMembers, fetchRoles, fetchFolders, createRole, createFolder, inviteMember, removeMember, createPermission } = useWorkspaceStore()
  const fetchUsers = useAuthStore(s => s.fetchUsers)
  const [tab, setTab] = useState('members')
  const [showInvite, setShowInvite] = useState(false)
  const [showCreateRole, setShowCreateRole] = useState(false)
  const [showCreateFolder, setShowCreateFolder] = useState(false)
  const [showCreatePerm, setShowCreatePerm] = useState(false)
  const [newName, setNewName] = useState('')
  const [users, setUsers] = useState([])
  const [permForm, setPermForm] = useState({ uaId: '', oaId: '', ops: [] })

  useEffect(() => {
    if (current) {
      fetchMembers(current.id)
      fetchRoles(current.id)
      fetchFolders(current.id)
    }
  }, [current?.id])

  const openInvite = async () => {
    try {
      const u = await fetchUsers()
      setUsers(u)
    } catch {}
    setShowInvite(true)
  }

  const handleInvite = async (user) => {
    if (!current) return
    try {
      await inviteMember(current.id, user.ngac_node_id || user.ngacNodeId)
      setShowInvite(false)
    } catch (err) {
      console.error(err)
    }
  }

  const handleRemove = async (nodeId) => {
    if (!current) return
    try {
      await removeMember(current.id, nodeId)
    } catch (err) {
      console.error(err)
    }
  }

  const handleCreateRole = async () => {
    if (!newName.trim() || !current) return
    await createRole(current.id, newName.trim())
    setShowCreateRole(false)
    setNewName('')
  }

  const handleCreateFolder = async () => {
    if (!newName.trim() || !current) return
    await createFolder(current.id, newName.trim())
    setShowCreateFolder(false)
    setNewName('')
  }

  const handleCreatePermission = async () => {
    if (!permForm.uaId || !permForm.oaId || permForm.ops.length === 0 || !current) return
    await createPermission(current.id, permForm.uaId, permForm.oaId, permForm.ops)
    setShowCreatePerm(false)
    setPermForm({ uaId: '', oaId: '', ops: [] })
  }

  const toggleOp = (op) => {
    setPermForm(f => ({
      ...f,
      ops: f.ops.includes(op) ? f.ops.filter(o => o !== op) : [...f.ops, op],
    }))
  }

  const initials = (name) => name ? name.slice(0, 2).toUpperCase() : '?'
  const allOps = ['read', 'write', 'approve', 'upload', 'share', 'manage', 'invite', 'create_channel']

  if (!current) {
    return (
      <>
        <div className="content-topbar"><div className="topbar-title">⚙️ Settings</div></div>
        <div className="content-body"><div className="empty-state"><h3>Select a workspace</h3></div></div>
      </>
    )
  }

  return (
    <>
      <div className="content-topbar">
        <div className="topbar-title"><span>⚙️</span> {current.name} — Settings</div>
      </div>

      <div className="content-body">
        <div className="settings-page">
          <div className="tabs">
            {['members', 'roles', 'folders', 'permissions'].map(t => (
              <button key={t} className={`tab ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}>
                {t.charAt(0).toUpperCase() + t.slice(1)}
              </button>
            ))}
          </div>

          {/* ===== MEMBERS TAB ===== */}
          {tab === 'members' && (
            <div className="settings-card">
              <div className="settings-card-header">
                <h3>Members ({(members || []).length})</h3>
                <button className="btn btn-primary btn-sm" onClick={openInvite}>+ Invite</button>
              </div>
              {(members || []).map(m => (
                <div key={m.ngac_node_id || m.ngacNodeId} className="member-row">
                  <div className="member-avatar">{initials(m.username)}</div>
                  <div className="member-info">
                    <div className="member-name">{m.username}</div>
                    <div className="member-role">
                      {(m.roles || []).map((r, i) => (
                        <span key={i} className={`role-tag ${r.name?.includes('Owner') ? 'owner' : ''}`}>
                          {r.name || r}
                        </span>
                      ))}
                    </div>
                  </div>
                  <button className="btn btn-danger btn-sm" onClick={() => handleRemove(m.ngac_node_id || m.ngacNodeId)}>Remove</button>
                </div>
              ))}
              {(!members || members.length === 0) && (
                <div style={{ padding: '1rem', textAlign: 'center', color: 'var(--text-muted)' }}>No members found</div>
              )}
            </div>
          )}

          {/* ===== ROLES TAB ===== */}
          {tab === 'roles' && (
            <div className="settings-card">
              <div className="settings-card-header">
                <h3>Roles ({(roles || []).length})</h3>
                <button className="btn btn-primary btn-sm" onClick={() => { setShowCreateRole(true); setNewName('') }}>+ Create Role</button>
              </div>
              {(roles || []).map(r => (
                <div key={r.id} className="member-row">
                  <div className="member-avatar" style={{ background: 'var(--accent-glow)' }}>UA</div>
                  <div className="member-info">
                    <div className="member-name">{r.name}</div>
                    <div className="member-role" style={{ fontFamily: 'monospace', fontSize: '0.7rem' }}>{r.ngac_node_id || r.ngacNodeId || r.id}</div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* ===== FOLDERS TAB ===== */}
          {tab === 'folders' && (
            <div className="settings-card">
              <div className="settings-card-header">
                <h3>Folders / Object Attributes ({(folders || []).length})</h3>
                <button className="btn btn-primary btn-sm" onClick={() => { setShowCreateFolder(true); setNewName('') }}>+ Create Folder</button>
              </div>
              {(folders || []).map(f => (
                <div key={f.id} className="member-row">
                  <div className="member-avatar" style={{ background: 'var(--info-bg)', color: 'var(--info)' }}>OA</div>
                  <div className="member-info">
                    <div className="member-name">{f.name}</div>
                    <div className="member-role" style={{ fontFamily: 'monospace', fontSize: '0.7rem' }}>{f.ngac_node_id || f.ngacNodeId || f.id}</div>
                  </div>
                </div>
              ))}
            </div>
          )}

          {/* ===== PERMISSIONS TAB ===== */}
          {tab === 'permissions' && (
            <div className="settings-card">
              <div className="settings-card-header">
                <h3>Permissions (Associations)</h3>
                <button className="btn btn-primary btn-sm" onClick={() => setShowCreatePerm(true)}>+ Create Permission</button>
              </div>
              <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', padding: '0.5rem 0' }}>
                Permissions link a User Attribute (role) to an Object Attribute (folder) with specific operations.
              </p>
              <div style={{ padding: '0.5rem', background: 'var(--bg-glass)', borderRadius: 'var(--radius-sm)', fontFamily: 'monospace', fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                UA (role) --[operations]--→ OA (folder)
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Invite Modal */}
      {showInvite && (
        <div className="modal-overlay" onClick={() => setShowInvite(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Invite Member</h2>
            {users.map(u => (
              <div key={u.id} className="member-row" style={{ cursor: 'pointer' }} onClick={() => handleInvite(u)}>
                <div className="member-avatar">{initials(u.username)}</div>
                <div className="member-info"><div className="member-name">{u.username}</div></div>
              </div>
            ))}
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowInvite(false)}>Cancel</button>
            </div>
          </div>
        </div>
      )}

      {/* Create Role Modal */}
      {showCreateRole && (
        <div className="modal-overlay" onClick={() => setShowCreateRole(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Create Role</h2>
            <div className="form-group">
              <label>Role Name</label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="e.g. Editor, Reviewer" autoFocus
                     onKeyDown={e => e.key === 'Enter' && handleCreateRole()} />
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowCreateRole(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleCreateRole}>Create</button>
            </div>
          </div>
        </div>
      )}

      {/* Create Folder Modal */}
      {showCreateFolder && (
        <div className="modal-overlay" onClick={() => setShowCreateFolder(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Create Folder</h2>
            <div className="form-group">
              <label>Folder Name</label>
              <input value={newName} onChange={e => setNewName(e.target.value)} placeholder="e.g. Contracts, Reports" autoFocus
                     onKeyDown={e => e.key === 'Enter' && handleCreateFolder()} />
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowCreateFolder(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleCreateFolder}>Create</button>
            </div>
          </div>
        </div>
      )}

      {/* Create Permission Modal */}
      {showCreatePerm && (
        <div className="modal-overlay" onClick={() => setShowCreatePerm(false)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2>Create Permission</h2>
            <div className="form-group">
              <label>User Attribute (Role)</label>
              <select value={permForm.uaId} onChange={e => setPermForm(f => ({ ...f, uaId: e.target.value }))}>
                <option value="">Select role...</option>
                {(roles || []).map(r => (
                  <option key={r.id} value={r.ngac_node_id || r.ngacNodeId || r.id}>{r.name}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>Object Attribute (Folder)</label>
              <select value={permForm.oaId} onChange={e => setPermForm(f => ({ ...f, oaId: e.target.value }))}>
                <option value="">Select folder...</option>
                {(folders || []).map(f => (
                  <option key={f.id} value={f.ngac_node_id || f.ngacNodeId || f.id}>{f.name}</option>
                ))}
              </select>
            </div>
            <div className="form-group">
              <label>Operations</label>
              <div className="checkbox-group">
                {allOps.map(op => (
                  <label key={op} className="checkbox-label">
                    <input type="checkbox" checked={permForm.ops.includes(op)} onChange={() => toggleOp(op)} />
                    {op}
                  </label>
                ))}
              </div>
            </div>
            <div className="modal-actions">
              <button className="btn btn-secondary" onClick={() => setShowCreatePerm(false)}>Cancel</button>
              <button className="btn btn-primary" onClick={handleCreatePermission}>Create</button>
            </div>
          </div>
        </div>
      )}
    </>
  )
}
