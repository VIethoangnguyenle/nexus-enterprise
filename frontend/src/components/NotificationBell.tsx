import { useNotifications, useUnreadCount, useMarkRead, useMarkAllRead } from '../hooks/useNotifications'
import { useState, useRef, useEffect } from 'react'
import { Button, Text } from './primitives'
import { Bell, ClipboardList, CheckCircle, Package, Undo2, Trash2, MessageSquare, Info, AtSign } from 'lucide-react'

const NOTIF_ICONS: Record<string, typeof ClipboardList> = {
  asset_requested: ClipboardList, asset_approved: CheckCircle, asset_assigned: Package,
  asset_returned: Undo2, asset_disposed: Trash2, mention: AtSign,
  thread_reply: MessageSquare, system: Info,
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
        className="relative w-8 h-8 flex items-center justify-center rounded
          text-on-surface-variant hover:text-on-surface hover:bg-surface-container-high transition-all
          cursor-pointer border-none bg-transparent"
      >
        <Bell size={18} strokeWidth={1.8} />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 flex items-center
            justify-center bg-error text-on-error text-[10px] font-bold rounded-full">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1 w-[340px] bg-surface-container-lowest border border-outline-variant
          rounded-lg shadow-lg animate-fade-in z-50 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-outline-variant">
            <span className="text-sm font-semibold text-on-surface">Notifications</span>
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
                  hover:bg-surface-container-high border-b border-outline-variant/30
                  ${n.read ? '' : 'bg-primary-container/20'}`}
                onClick={() => { if (!n.read) markRead.mutate(n.id); setOpen(false) }}
              >
                <span className="flex-shrink-0 mt-1">{(() => { const Icon = NOTIF_ICONS[n.type] || NOTIF_ICONS.system; return <Icon size={16} className="text-on-surface-variant" />; })()}</span>
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-on-surface truncate">{n.title || n.type}</div>
                  <div className="text-xs text-on-surface-variant mt-1 truncate">{n.body || n.message || ''}</div>
                  <div className="text-[10px] text-on-surface-variant mt-1">{formatTimeAgo(n.created_at)}</div>
                </div>
                {!n.read && (
                  <div className="w-2 h-2 rounded-full bg-primary flex-shrink-0 mt-2" />
                )}
              </div>
            )) : (
              <div className="px-4 py-8 text-center">
                <span className="text-xs text-on-surface-variant">No notifications yet</span>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
