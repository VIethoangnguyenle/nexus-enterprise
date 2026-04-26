import { useNavigate } from '@tanstack/react-router'
import { useNotifications, useUnreadCount, useMarkRead, useMarkAllRead } from '../hooks/useNotifications'
import { useState, useRef, useEffect } from 'react'

const NOTIF_ICONS: Record<string, string> = {
  asset_requested: '📋', asset_approved: '✅', asset_assigned: '📦',
  asset_returned: '↩️', asset_disposed: '🗑️', mention: '@',
  thread_reply: '💬', system: 'ℹ️',
}

export default function NotificationBell() {
  const navigate = useNavigate()
  const { data: notifsData } = useNotifications(10)
  const { data: countData } = useUnreadCount()
  const markRead = useMarkRead()
  const markAll = useMarkAllRead()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  const notifications = notifsData?.notifications || []
  const unreadCount = countData?.count || 0

  useEffect(() => {
    const handler = (e: MouseEvent) => { if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false) }
    if (open) document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open])

  const formatTimeAgo = (ts?: string) => {
    if (!ts) return ''
    const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
    if (diff < 60) return 'just now'
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
    return new Date(ts).toLocaleDateString()
  }

  return (
    <div className="notification-bell-container" ref={ref}>
      <button className="notification-bell-btn" onClick={() => setOpen(!open)}>
        <span className="bell-icon">🔔</span>
        {unreadCount > 0 && <span className="notification-badge">{unreadCount > 99 ? '99+' : unreadCount}</span>}
      </button>
      {open && (
        <div className="notification-dropdown">
          <div className="notification-dropdown-header"><h4>Notifications</h4>
            {unreadCount > 0 && <button className="btn btn-ghost btn-sm" onClick={() => markAll.mutate()}>Mark all read</button>}
          </div>
          <div className="notification-list">
            {notifications.length > 0 ? notifications.map(n => (
              <div key={n.id} className={`notification-item ${n.read ? '' : 'unread'}`}
                   onClick={() => { if (!n.read) markRead.mutate(n.id); setOpen(false) }}>
                <div className="notification-icon">{NOTIF_ICONS[n.type] || NOTIF_ICONS.system}</div>
                <div className="notification-content">
                  <div className="notification-title">{n.title || n.type}</div>
                  <div className="notification-body">{n.body || n.message || ''}</div>
                  <div className="notification-time">{formatTimeAgo(n.created_at)}</div>
                </div>
                {!n.read && <div className="notification-unread-dot" />}
              </div>
            )) : <div className="notification-empty"><p>No notifications yet</p></div>}
          </div>
        </div>
      )}
    </div>
  )
}
