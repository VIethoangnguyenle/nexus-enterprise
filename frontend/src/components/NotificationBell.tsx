import { useNotifications, useUnreadCount, useMarkRead, useMarkAllRead } from '../hooks/useNotifications'
import { useState, useRef, useEffect } from 'react'
import { Button, Text } from './primitives'

const NOTIF_ICONS: Record<string, string> = {
  asset_requested: '📋', asset_approved: '✅', asset_assigned: '📦',
  asset_returned: '↩️', asset_disposed: '🗑️', mention: '@',
  thread_reply: '💬', system: 'ℹ️',
}

export default function NotificationBell() {
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
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen(!open)}
        className="relative w-8 h-8 flex items-center justify-center rounded-[var(--radius-sm)]
          text-text-muted hover:text-text-primary hover:bg-bg-hover transition-all
          cursor-pointer border-none bg-transparent"
      >
        🔔
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 flex items-center
            justify-center bg-danger text-white text-[0.6rem] font-bold rounded-full">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1 w-[340px] bg-bg-tertiary border border-border
          rounded-[var(--radius-md)] shadow-lg animate-fade-in z-50 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-border">
            <Text variant="body" className="font-semibold">Notifications</Text>
            {unreadCount > 0 && (
              <Button variant="ghost" size="sm" onClick={() => markAll.mutate()}>Mark all read</Button>
            )}
          </div>

          {/* List */}
          <div className="max-h-[360px] overflow-y-auto">
            {notifications.length > 0 ? notifications.map(n => (
              <div
                key={n.id}
                className={`flex items-start gap-3 px-4 py-3 cursor-pointer transition-colors
                  hover:bg-bg-hover border-b border-border/30
                  ${n.read ? '' : 'bg-bg-active/30'}`}
                onClick={() => { if (!n.read) markRead.mutate(n.id); setOpen(false) }}
              >
                <span className="text-sm flex-shrink-0 mt-0.5">{NOTIF_ICONS[n.type] || NOTIF_ICONS.system}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-text-primary truncate">{n.title || n.type}</div>
                  <div className="text-xs text-text-muted mt-0.5 truncate">{n.body || n.message || ''}</div>
                  <div className="text-[0.65rem] text-text-muted mt-1">{formatTimeAgo(n.created_at)}</div>
                </div>
                {!n.read && (
                  <div className="w-2 h-2 rounded-full bg-accent flex-shrink-0 mt-1.5" />
                )}
              </div>
            )) : (
              <div className="px-4 py-8 text-center">
                <Text variant="caption" muted>No notifications yet</Text>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
