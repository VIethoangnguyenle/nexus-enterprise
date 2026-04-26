import { useNavigate, useLocation } from '@tanstack/react-router'
import { useChannels } from '../hooks/useMessaging'
import type { Workspace } from '../api/workspaces'
import { useAuthStore } from '../stores/auth.store'
import { useUiStore } from '../stores/ui.store'

interface SidebarProps { workspaces: Workspace[] }

export default function Sidebar({ workspaces }: SidebarProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const logout = useAuthStore((s) => s.logout)
  const user = useAuthStore((s) => s.user)
  const collapsed = useUiStore((s) => s.sidebarCollapsed)
  const toggle = useUiStore((s) => s.toggleSidebar)
  const wsId = workspaces[0]?.id || ''
  const { data: channelsData } = useChannels(wsId)
  const channels = channelsData?.channels || []

  const isActive = (path: string) => location.pathname === path || location.pathname.startsWith(path + '/')

  const navItem = (path: string, icon: string, label: string) => (
    <div className={`sidebar-item ${isActive(path) ? 'active' : ''}`}
         onClick={() => navigate({ to: path })} style={{ cursor: 'pointer' }}>
      <span className="sidebar-item-icon">{icon}</span>
      {!collapsed && <span className="sidebar-item-text">{label}</span>}
    </div>
  )

  return (
    <div className={`sidebar ${collapsed ? 'collapsed' : ''}`}>
      <div className="sidebar-header">
        {!collapsed && <span className="sidebar-logo">NGAC</span>}
        <button className="sidebar-toggle" onClick={toggle}>{collapsed ? '→' : '←'}</button>
      </div>

      <div className="sidebar-nav">
        <div className="sidebar-section">
          <div className="sidebar-section-header"><span className="sidebar-section-title">Documents</span></div>
          {navItem('/documents', '📄', 'All Documents')}
        </div>

        <div className="sidebar-section">
          <div className="sidebar-section-header"><span className="sidebar-section-title">Assets</span></div>
          {navItem('/asset-dashboard', '📊', 'Dashboard')}
          {navItem('/assets', '📦', 'My Assets')}
          {navItem('/asset-requests', '📋', 'Requests')}
          {navItem('/asset-types', '🏷️', 'Type Config')}
        </div>

        <div className="sidebar-section">
          <div className="sidebar-section-header"><span className="sidebar-section-title">Channels</span></div>
          {channels.map((ch) => (
            <div key={ch.id} className={`sidebar-item ${isActive(`/channels/${ch.id}`) ? 'active' : ''}`}
                 onClick={() => navigate({ to: '/channels/$channelId', params: { channelId: ch.id } })} style={{ cursor: 'pointer' }}>
              <span className="sidebar-item-icon">#</span>
              {!collapsed && <span className="sidebar-item-text">{ch.name}</span>}
            </div>
          ))}
        </div>
      </div>

      <div className="sidebar-footer">
        {navItem('/settings', '⚙️', 'Settings')}
        <div className="sidebar-item" onClick={logout} style={{ cursor: 'pointer' }}>
          <span className="sidebar-item-icon">🚪</span>
          {!collapsed && <span className="sidebar-item-text">Logout ({user?.username})</span>}
        </div>
      </div>
    </div>
  )
}
