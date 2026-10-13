import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { Link, useNavigate } from '@tanstack/react-router'
import { AnimatePresence, motion } from 'motion/react'
import { AlertCircle, ArrowLeft, Bell, Box, Check, CheckCheck, ClipboardCheck, Mail, X } from 'lucide-react'
import { DURATION, EASE, useMotionPresets } from '../../lib/motion'
import {
  DOMAIN_LABEL, describeNotification, groupByDay, notificationTarget, notificationTime,
  type NotificationItem,
} from '../../lib/notification-copy'
import { formatDateTime } from '../../lib/format'
import {
  useMarkAllNotificationsRead, useMarkNotificationRead, useNotificationList, useUnreadNotificationCount,
} from '../../hooks/useNotifications'
import { Button, Heading, IconButton, Pressable } from '../primitives'

interface NotificationPanelProps {
  open: boolean
  onClose: () => void
  /** The Thông báo item: a press on it is its own toggle, and focus returns to it on Esc. */
  anchorRef: RefObject<HTMLElement | null>
  /** The menu column the panel sits beside (12px off its right edge). */
  sideRef: RefObject<HTMLElement | null>
}

const GAP = 12
const DOMAIN_ICON = { approval: ClipboardCheck, asset: Box, invitation: Mail } as const

/**
 * Notification panel (DESIGN.md §6, design/mockups/notifications.html): a large
 * popover beside the main menu. No scrim and no focus trap; Esc, a press
 * outside or Đóng closes it and focus goes back to the menu item.
 */
export function NotificationPanel({ open, onClose, anchorRef, sideRef }: NotificationPanelProps) {
  const m = useMotionPresets()
  const titleId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const [left, setLeft] = useState(0)

  useLayoutEffect(() => {
    if (!open) return
    const place = () => setLeft((sideRef.current?.getBoundingClientRect().right ?? 0) + GAP)
    place()
    window.addEventListener('resize', place)
    return () => window.removeEventListener('resize', place)
  }, [open, sideRef])

  useEffect(() => {
    if (!open) return
    panelRef.current?.focus()
    const onPointer = (e: PointerEvent) => {
      const t = e.target as Node
      if (panelRef.current?.contains(t) || anchorRef.current?.contains(t)) return
      onClose()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      onClose()
      anchorRef.current?.focus()
    }
    document.addEventListener('pointerdown', onPointer)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('pointerdown', onPointer)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, onClose, anchorRef])

  // DESIGN.md §6: translateX(-16px) scale(.985) → 0 in 280ms expo, out 210ms; reduced motion fades only.
  const motionProps = m.reduced
    ? { initial: { opacity: 0 }, animate: { opacity: 1, transition: { duration: DURATION.reducedFade } }, exit: { opacity: 0, transition: { duration: DURATION.reducedFade } } }
    : {
        initial: { opacity: 0, x: -16, scale: 0.985 },
        animate: { opacity: 1, x: 0, scale: 1, transition: { duration: DURATION.layout, ease: EASE.outExpo } },
        exit: { opacity: 0, x: -16, scale: 0.985, transition: { duration: DURATION.exit, ease: EASE.in } },
      }

  return createPortal(
    <AnimatePresence>
      {open && (
        <motion.div
          ref={panelRef}
          role="dialog"
          aria-modal="false"
          aria-labelledby={titleId}
          tabIndex={-1}
          {...motionProps}
          style={{ left }}
          className="fixed top-3 bottom-3 z-dropdown w-95 max-w-[calc(100vw-1.5rem)] grid grid-rows-[auto_1fr] min-h-0
            overflow-hidden rounded-overlay bg-overlay shadow-overlay text-ink outline-none"
        >
          <PanelContent titleId={titleId} onClose={onClose} />
        </motion.div>
      )}
    </AnimatePresence>,
    document.body,
  )
}

export function PanelContent({ titleId, onClose, back = false }: { titleId: string; onClose: () => void; back?: boolean }) {
  const list = useNotificationList(true)
  const badge = useUnreadNotificationCount()
  const unread = list.unread ?? badge
  const markAll = useMarkAllNotificationsRead()
  const navigate = useNavigate()

  return (
    <>
      <div className="grid gap-1 pt-4 pr-3 pb-2 pl-4">
        <div className="flex items-center gap-2">
          {back && (
            <IconButton size="md" aria-label="Quay lại" onClick={onClose} className="-ml-2">
              <ArrowLeft size={18} strokeWidth={1.75} aria-hidden="true" />
            </IconButton>
          )}
          <Heading as="h2" look="panel" id={titleId}>Thông báo</Heading>
          {!back && (
            <IconButton size="md" aria-label="Đóng thông báo" onClick={onClose} className="ml-auto">
              <X size={18} strokeWidth={1.75} aria-hidden="true" />
            </IconButton>
          )}
        </div>
        <div className="flex items-center gap-2 min-h-8">
          <span className="text-sm text-ink-muted tnum">{list.isError || list.isLoading ? '' : `${unread} chưa đọc`}</span>
          <Button
            variant="ghost"
            size="sm"
            className="ml-auto -mr-1"
            disabled={unread === 0 || list.isError || list.isLoading}
            onClick={() => markAll.mutate()}
          >
            <CheckCheck size={16} strokeWidth={1.75} aria-hidden="true" />
            Đánh dấu tất cả đã đọc
          </Button>
        </div>
      </div>

      <div className="overflow-y-auto min-h-0 px-2 pb-3">
        {list.isLoading ? <Skeleton />
          : list.isError ? (
            <State icon={<AlertCircle size={24} strokeWidth={1.75} aria-hidden="true" />} bad title="Không tải được thông báo" hint="Kiểm tra kết nối rồi thử lại.">
              <Button variant="soft" size="sm" onClick={() => void list.refetch()}>Thử lại</Button>
            </State>
          ) : list.items.length === 0 ? (
            <State icon={<Bell size={24} strokeWidth={1.75} aria-hidden="true" />} title="Chưa có thông báo nào" hint="Khi ai đó duyệt đề nghị hoặc giao tài sản cho bạn, nó sẽ hiện ở đây.">
              <Button
                variant="soft"
                size="sm"
                onClick={() => { void navigate({ to: '/approval', search: { tab: 'mine' } }); onClose() }}
              >
                Xem đề nghị của bạn
              </Button>
            </State>
          ) : (
            <>
              {groupByDay(list.items).map((g) => (
                <section key={g.label}>
                  <div className="flex items-center gap-3 px-2 pt-3 pb-1.5 text-xs font-semibold text-ink-muted after:content-[''] after:h-px after:flex-1 after:bg-line">
                    {g.label}
                  </div>
                  <ul className="grid gap-0.5 m-0 p-0 list-none">
                    {g.items.map((n) => <Row key={n.id} n={n} onClose={onClose} />)}
                  </ul>
                </section>
              ))}
              <div className="grid justify-items-center gap-1.5 pt-3 pb-1">
                {list.hasMore && (
                  <Button variant="ghost" size="sm" loading={list.loadingMore} onClick={() => void list.loadMore()}>Tải thêm</Button>
                )}
                <span className="text-xs text-ink-muted tnum">Đã hiện {list.items.length} trong {list.total}</span>
              </div>
            </>
          )}
      </div>
    </>
  )
}

function Row({ n, onClose }: { n: NotificationItem; onClose: () => void }) {
  const markRead = useMarkNotificationRead()
  const { text, domain } = describeNotification(n)
  const target = notificationTarget(n)
  const Domain = domain ? DOMAIN_ICON[domain] : Bell
  const open = () => {
    if (!n.read) markRead.mutate(n.id)
    onClose()
  }
  const face = (
    <span aria-hidden="true" className="grid place-items-center w-8 h-8 rounded-surface bg-sunk text-ink-muted shrink-0">
      <Domain size={18} strokeWidth={1.75} />
    </span>
  )
  const body = (
    <span className="grid min-w-0">
      <span className={`leading-normal ${n.read ? 'text-ink-muted' : 'text-ink font-semibold'}`}>
        {!n.read && <span className="sr-only">Chưa đọc</span>}
        {!n.read && ' '}
        {text}
      </span>
      <span className="flex items-center gap-1.5 mt-1 text-xs leading-4 text-ink-muted tnum">
        {domain && (
          <>
            <Domain size={14} strokeWidth={1.75} aria-hidden="true" />
            <span>{DOMAIN_LABEL[domain]}</span>
            <span aria-hidden="true">·</span>
          </>
        )}
        <time dateTime={n.created_at} title={formatDateTime(n.created_at)}>{notificationTime(n.created_at)}</time>
      </span>
    </span>
  )
  const linkClass = 'grid grid-cols-[2rem_minmax(0,1fr)] gap-3 py-2.5 pl-2.5 text-left text-inherit no-underline rounded-surface min-w-0 focus-ring'

  let main: ReactNode
  if (target?.to === '/approval') {
    main = <Link to="/approval" search={target.search} onClick={open} className={linkClass}>{face}{body}</Link>
  } else if (target?.to === '/workspace-select') {
    main = <Link to="/workspace-select" onClick={open} className={linkClass}>{face}{body}</Link>
  } else if (target?.to === '/assets') {
    main = <Link to="/assets" search={target.search} onClick={open} className={linkClass}>{face}{body}</Link>
  } else {
    main = <Pressable onClick={open} className={`${linkClass} w-full`}>{face}{body}</Pressable>
  }

  return (
    <li className="group grid grid-cols-[minmax(0,1fr)_2rem] items-start gap-1 rounded-surface transition-colors duration-quick hover:bg-hover">
      {main}
      <span className="relative grid place-items-center w-8 h-8 mt-2.5">
        {!n.read && (
          <>
            <span aria-hidden="true" className="w-2 h-2 rounded-full bg-accent transition-opacity duration-quick group-hover:opacity-0 group-focus-within:opacity-0" />
            <IconButton
              size="md"
              aria-label={`Đánh dấu đã đọc: ${text}`}
              onClick={() => markRead.mutate(n.id)}
              className="absolute inset-0 opacity-0 group-hover:opacity-100 focus-visible:opacity-100 [@media(hover:none)]:hidden"
            >
              <Check size={16} strokeWidth={1.75} aria-hidden="true" />
            </IconButton>
          </>
        )}
      </span>
    </li>
  )
}

function Skeleton() {
  return (
    <div aria-busy="true" aria-label="Đang tải thông báo">
      {[92, 78, 86, 70].map((w) => (
        <div key={w} className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3 p-2.5">
          <span className="skeleton w-8 h-8 rounded-full" />
          <span className="grid gap-2 pt-0.5">
            <span className="skeleton h-3 rounded-md" style={{ width: `${w}%` }} />
            <span className="skeleton h-3 rounded-md w-2/5" />
          </span>
        </div>
      ))}
    </div>
  )
}

function State({ icon, title, hint, bad = false, children }: { icon: ReactNode; title: string; hint: string; bad?: boolean; children: ReactNode }) {
  return (
    <div className="grid justify-items-center gap-2 px-6 py-12 text-center">
      <span className={`grid place-items-center w-10 h-10 rounded-lg ${bad ? 'bg-danger-wash text-danger' : 'bg-sunk text-ink-muted'}`}>{icon}</span>
      <b className="font-semibold">{title}</b>
      <p className="m-0 max-w-[30ch] text-ink-muted">{hint}</p>
      {children}
    </div>
  )
}
