import { useEffect, useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Bell, CheckCheck, CircleAlert, RefreshCw } from 'lucide-react'
import type { AppNotification } from '../../api/notifications'
import { useMarkAllRead, useMarkRead, useNotificationList, useUnreadCount } from '../../hooks/useNotifications'
import { openNotification } from '../../lib/notification-arrivals'
import { groupByDay, summaryText, type OpenTarget } from '../../lib/notification-model'
import { useNotificationUi } from '../../stores/notification.store'
import { Button, Spinner, Text } from '../primitives'
import { NotificationRow } from './NotificationRow'

interface NotificationsViewProps {
  /** Called after a notification has been opened (or the empty state's action taken), to close the surface. */
  onNavigated: () => void
}

/**
 * The list of notifications with its header line, states and paging
 * (DESIGN.md §6, NotificationPanel). The surface around it (the desktop
 * popover, or the Thêm sheet below 1024) supplies the title and the close
 * control. While it is mounted it tells the arrival logic the list is on
 * screen, so a new notification washes in here instead of becoming a toast.
 */
export function NotificationsView({ onNavigated }: NotificationsViewProps) {
  const navigate = useNavigate()
  const list = useNotificationList()
  const unreadQuery = useUnreadCount()
  const markRead = useMarkRead()
  const markAll = useMarkAllRead()
  const fresh = useNotificationUi((s) => s.fresh)
  const burst = useNotificationUi((s) => s.burst)
  const announcement = useNotificationUi((s) => s.announcement)
  const setListVisible = useNotificationUi((s) => s.setListVisible)

  useEffect(() => {
    setListVisible(true)
    return () => setListVisible(false)
  }, [setListVisible])

  const items = useMemo(() => list.data?.pages.flatMap((p) => p.notifications ?? []) ?? [], [list.data])
  const total = list.data?.pages[0]?.total ?? 0
  const unread = unreadQuery.data ?? list.data?.pages[0]?.unread_count ?? 0
  const groups = useMemo(() => groupByDay(items), [items])

  const go = (t: OpenTarget) => {
    void navigate({ to: t.to, search: t.search as never })
    onNavigated()
  }
  const open = (n: AppNotification) => void openNotification(n, go)

  return (
    <div className="flex flex-col min-h-0 flex-1">
      <div className="flex items-center justify-between gap-2 px-4 pb-2 shrink-0">
        <Text variant="small" muted className="tnum">
          {unread > 0 ? `${unread} chưa đọc` : list.isSuccess && items.length > 0 ? 'Đã đọc hết' : ''}
        </Text>
        <Button variant="ghost" size="sm" disabled={unread === 0 || markAll.isPending} onClick={() => markAll.mutate()}>
          <CheckCheck size={16} strokeWidth={1.75} aria-hidden="true" />
          Đánh dấu tất cả đã đọc
        </Button>
      </div>

      {/* Read out by assistive tech: one sentence per arrival, one for a burst. */}
      <div role="status" aria-live="polite" className="sr-only">{announcement}</div>

      <div className="min-h-0 flex-1 overflow-y-auto pb-2">
        {list.isPending ? (
          <ul className="grid m-0 p-0" aria-busy="true" aria-label="Đang tải thông báo">
            {[0, 1, 2, 3].map((i) => (
              <li key={i} data-skeleton className="list-none grid grid-cols-[2rem_minmax(0,1fr)] gap-3 py-3 px-4">
                <div className="skeleton w-8 h-8 rounded-full" />
                <div className="grid gap-2">
                  <div className="skeleton h-3 w-11/12 rounded-md" />
                  <div className="skeleton h-3 w-1/2 rounded-md" />
                </div>
              </li>
            ))}
          </ul>
        ) : list.isError ? (
          <div className="grid justify-items-center gap-3 px-6 py-10 text-center">
            <span aria-hidden="true" className="grid place-items-center w-10 h-10 rounded-surface bg-danger-wash text-danger">
              <CircleAlert size={24} strokeWidth={1.75} />
            </span>
            <Text variant="body">Không tải được thông báo. Kiểm tra kết nối rồi thử lại.</Text>
            <Button variant="soft" size="sm" onClick={() => void list.refetch()}>
              <RefreshCw size={16} strokeWidth={1.75} aria-hidden="true" />
              Thử lại
            </Button>
          </div>
        ) : items.length === 0 ? (
          <div className="grid justify-items-center gap-3 px-6 py-10 text-center">
            <span aria-hidden="true" className="grid place-items-center w-10 h-10 rounded-surface bg-sunk text-ink-muted">
              <Bell size={24} strokeWidth={1.75} />
            </span>
            <Text variant="body">
              Chưa có thông báo nào. Khi ai đó duyệt đề nghị hoặc giao tài sản cho bạn, nó sẽ hiện ở đây.
            </Text>
            <Button variant="soft" size="sm" onClick={() => go({ to: '/approval', search: { tab: 'mine' } })}>
              Xem đề nghị của bạn
            </Button>
          </div>
        ) : (
          <>
            {burst && (
              <p className="m-0 mx-4 mb-1 px-3 py-2 rounded-md bg-accent-wash text-sm text-ink">
                {summaryText(burst.count, burst.actors)}
              </p>
            )}
            {groups.map((g) => (
              <section key={g.label} aria-label={g.label}>
                <div className="flex items-center gap-3 px-4 pt-3 pb-1 text-xs font-semibold text-ink-muted" aria-hidden="true">
                  <span>{g.label}</span>
                  <span className="flex-1 h-px bg-line" />
                </div>
                <ul className="grid m-0 p-0 px-1">
                  {g.items.map((n) => (
                    <NotificationRow
                      key={n.id}
                      notification={n}
                      fresh={fresh[n.id]}
                      showTag={!burst}
                      onOpen={open}
                      onMarkRead={(row) => markRead.mutate(row.id)}
                    />
                  ))}
                </ul>
              </section>
            ))}
            <div className="grid justify-items-center gap-1 px-4 py-3">
              {list.hasNextPage && (
                <Button variant="ghost" size="sm" disabled={list.isFetchingNextPage} onClick={() => void list.fetchNextPage()}>
                  {list.isFetchingNextPage && <Spinner size="sm" />}
                  Tải thêm
                </Button>
              )}
              <Text variant="caption" muted className="tnum">Đã hiện {items.length} trong {Math.max(total, items.length)}</Text>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
