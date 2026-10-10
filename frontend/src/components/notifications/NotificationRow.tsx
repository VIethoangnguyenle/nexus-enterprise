import { Fragment } from 'react'
import { motion } from 'motion/react'
import { Link } from '@tanstack/react-router'
import { Bell, CheckCheck, ClipboardCheck, Mail, Package } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { AppNotification } from '../../api/notifications'
import { formatDateTime, formatListTime } from '../../lib/format'
import { useMotionPresets } from '../../lib/motion'
import { domainOf, openTargetOf, sentenceOf, type NotificationDomain } from '../../lib/notification-model'
import { personStyle } from '../../lib/person-hue'
import type { Fresh } from '../../stores/notification.store'
import { Avatar, IconButton } from '../primitives'

const DOMAIN: Record<NotificationDomain, { icon: LucideIcon; label: string }> = {
  approval: { icon: ClipboardCheck, label: 'Phê duyệt' },
  asset: { icon: Package, label: 'Tài sản' },
  workspace: { icon: Mail, label: 'Lời mời' },
  other: { icon: Bell, label: 'Thông báo' },
}

interface NotificationRowProps {
  notification: AppNotification
  /** Set while the row has just arrived over the WebSocket. */
  fresh?: Fresh
  /** A burst replaces the per-row "vừa …" tags with one summary line. */
  showTag: boolean
  onOpen: (n: AppNotification) => void
  onMarkRead: (n: AppNotification) => void
}

/**
 * One notification (DESIGN.md §6, NotificationRow): a real link to what it is
 * about. Unread ones read in ink with 600 names and carry a dot; on a pointer
 * device the dot turns into a "mark read" button on hover or focus. Names come
 * from the notification's own fields and never fall back to an id.
 */
export function NotificationRow({ notification: n, fresh, showTag, onOpen, onMarkRead }: NotificationRowProps) {
  const m = useMotionPresets()
  const { lead, follow, reason } = sentenceOf(n)
  const domain = DOMAIN[domainOf(n.type)]
  const target = openTargetOf(n)
  const nameWeight = n.read ? 'font-medium' : 'font-semibold'
  const hueKey = fresh?.actorKey

  const body = (
    <>
      {n.actor_name ? (
        <Avatar name={n.actor_name} hueKey={n.actor_user_id} size={32} halo={!!fresh} />
      ) : (
        <span aria-hidden="true" className="grid place-items-center w-8 h-8 rounded-surface bg-sunk text-ink-muted">
          <domain.icon size={16} strokeWidth={1.75} />
        </span>
      )}
      <span className="grid min-w-0 gap-1">
        <span className={`text-sm ${n.read ? 'text-ink-muted' : 'text-ink'}`}>
          {lead.map((p, i) => (p.strong ? <strong key={i} className={nameWeight}>{p.text}</strong> : <Fragment key={i}>{p.text}</Fragment>))}
          {follow && <>{' '}{follow}</>}
        </span>
        {reason && (
          <span className="block px-2.5 py-1.5 rounded-md bg-sunk text-xs text-ink-muted line-clamp-2">{reason}</span>
        )}
        <span className="flex items-center gap-1.5 text-xs text-ink-muted">
          <domain.icon size={14} strokeWidth={1.75} aria-hidden="true" />
          <span>{domain.label}</span>
          <span aria-hidden="true">·</span>
          <time dateTime={n.created_at} title={formatDateTime(n.created_at)} className="tnum">{formatListTime(n.created_at)}</time>
          {fresh && showTag && <span className="rt-tag font-semibold">{fresh.tag}</span>}
        </span>
      </span>
    </>
  )

  const rowClass = `grid grid-cols-[2rem_minmax(0,1fr)] items-start gap-3 py-3 pl-4 pr-12 no-underline
    focus-ring rounded-surface hover:bg-hover transition-colors duration-quick`

  return (
    <motion.li
      layout={m.layoutProp}
      transition={m.layout}
      initial={fresh ? m.row.initial : false}
      animate={m.row.animate}
      data-unread={n.read ? undefined : true}
      className={`group relative list-none ${fresh ? 'rt-wash' : ''}`}
      style={fresh && hueKey ? personStyle(hueKey) : undefined}
    >
      {target ? (
        <Link
          to={target.to}
          search={target.search as never}
          onClick={(e) => {
            // A plain click opens it here; a modified click keeps the browser's new-tab behaviour.
            if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return
            e.preventDefault()
            onOpen(n)
          }}
          className={rowClass}
        >
          {body}
        </Link>
      ) : (
        <div className={rowClass}>{body}</div>
      )}
      {!n.read && (
        <>
          <span className="absolute right-4 top-4 grid place-items-center w-3 h-3 pointer-events-none group-hover:opacity-0 group-focus-within:opacity-0">
            <span aria-hidden="true" className="w-2 h-2 rounded-full bg-accent" />
            <span className="sr-only">Chưa đọc</span>
          </span>
          {/* Hover or keyboard focus only; a touch screen marks by opening the row. */}
          <IconButton
            size="md"
            aria-label="Đánh dấu đã đọc"
            title="Đánh dấu đã đọc"
            onClick={() => onMarkRead(n)}
            className="absolute right-2 top-2.5 opacity-0 group-hover:opacity-100 focus:opacity-100 pointer-coarse:hidden"
          >
            <CheckCheck size={16} strokeWidth={1.75} />
          </IconButton>
        </>
      )}
    </motion.li>
  )
}
