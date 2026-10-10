import { createElement } from 'react'
import type { AppNotification } from '../api/notifications'
import { ApiError } from '../api/client'
import { approvalRequestOptions } from '../hooks/useApproval'
import { assetQueryOptions, assetRequestQueryOptions } from '../hooks/useAssets'
import { markReadNow } from '../hooks/useNotifications'
import { keys } from '../hooks/keys'
import { Avatar } from '../components/primitives/Avatar'
import { toast, useToastStore } from '../components/primitives/Toast'
import { useAuthStore } from '../stores/auth.store'
import { useNotificationUi } from '../stores/notification.store'
import { BURST_MIN, BURST_WINDOW_MS, REALTIME_MS } from '../hooks/useArrivals'
import { queryClient } from './query-client'
import {
  arrivalTag, openTargetOf, plainText, sentenceOf, summaryText, toastText, type OpenTarget,
} from './notification-model'

/** Said when a notification's subject cannot be opened (DESIGN.md mockup §3). */
export const UNAVAILABLE = 'Không mở được. Mục này đã bị xoá hoặc bạn không còn quyền xem.'

/** What the wire frame carries (the generated `NotificationEvent`, as far as this file reads it). */
export interface NotificationFrame {
  id: string
  type: string
  targetType: string
  targetId: string
  actorUserId: string
  actorName: string
  targetName: string
  workspaceId: string
  params: Record<string, string>
  createdAt?: { seconds: string | number | bigint; nanos: number }
}

export function fromFrame(f: NotificationFrame): AppNotification {
  const seconds = f.createdAt ? Number(f.createdAt.seconds) : 0
  const created = seconds > 0 ? new Date(seconds * 1000 + Math.floor((f.createdAt?.nanos ?? 0) / 1e6)) : new Date()
  return {
    id: f.id, type: f.type, read: false, created_at: created.toISOString(), workspace_id: f.workspaceId,
    actor_user_id: f.actorUserId || undefined, actor_name: f.actorName || undefined,
    target_type: f.targetType || undefined, target_id: f.targetId || undefined,
    target_name: f.targetName || undefined, params: { ...f.params },
  }
}

// --- Navigation from outside a component -------------------------------------------------

type Navigator = (target: OpenTarget) => void
let navigator: Navigator | null = null

/** The shell registers how to move around, so a toast's "Xem" can open a screen. */
export function registerNavigator(fn: Navigator | null) {
  navigator = fn
}

/**
 * Whether the person can open what a notification is about. Only an answer of
 * "not yours" or "gone" says no; a failed connection does not stop them trying.
 */
async function canOpen(n: AppNotification): Promise<boolean> {
  const id = n.target_id ?? ''
  try {
    switch (n.target_type) {
      case 'approval': await queryClient.fetchQuery(approvalRequestOptions(id)); break
      case 'asset_request': await queryClient.fetchQuery(assetRequestQueryOptions(id)); break
      case 'asset': await queryClient.fetchQuery(assetQueryOptions(id)); break
      default: break
    }
    return true
  } catch (err) {
    return !(err instanceof ApiError && (err.status === 403 || err.status === 404))
  }
}

/**
 * Opens a notification: it is read first (so it stays read even when its subject
 * is gone), then the subject opens where it lives. A subject that cannot be
 * opened leaves the person where they are, with a toast that says why.
 * Resolves true when it navigated.
 */
export async function openNotification(n: AppNotification, go: Navigator | null = navigator): Promise<boolean> {
  if (!n.read) void markReadNow(n.id)
  const target = openTargetOf(n)
  if (!target || !(await canOpen(n))) {
    toast.error(UNAVAILABLE)
    return false
  }
  go?.(target)
  return true
}

// --- Arrivals -----------------------------------------------------------------------------

interface Window {
  start: number
  items: AppNotification[]
  toasts: number[]
}
let win: Window = { start: 0, items: [], toasts: [] }
const timers = new Set<ReturnType<typeof setTimeout>>()

/** Forgets the coalescing window and any pending timers (tests, sign-out). */
export function resetArrivals() {
  win = { start: 0, items: [], toasts: [] }
  timers.forEach(clearTimeout)
  timers.clear()
  useNotificationUi.setState({ fresh: {}, burst: null, announcement: '' })
}

/** True when the person is looking at the very thing the notification is about. */
function isOnScreen(n: AppNotification): boolean {
  const q = new URLSearchParams(window.location.search)
  const path = window.location.pathname
  switch (n.target_type) {
    case 'approval': return path.startsWith('/approval') && q.get('request') === n.target_id
    case 'asset_request': return path.startsWith('/assets') && q.get('request') === n.target_id
    case 'asset': return path.startsWith('/assets') && q.get('asset') === n.target_id
    default: return false
  }
}

function later(ms: number, fn: () => void) {
  const t = setTimeout(() => {
    timers.delete(t)
    fn()
  }, ms)
  timers.add(t)
}

/**
 * A notification arrived over the WebSocket. The list and count refetch through
 * the store's invalidation; this decides what the person sees (DESIGN.md §6, §7):
 *
 * - the subject is already open: read silently, nothing shown;
 * - the list is open: the row gets the author's wash and a "vừa …" tag
 *   (three within 2 s become one summary line instead of tags);
 * - the list is closed: a toast with "Xem" (three within 2 s become one toast
 *   with "Mở").
 *
 * A frame for another workspace is ignored (the server does not send one), except
 * an invitation, which belongs to the person rather than to a workspace.
 */
export function receiveNotification(frame: NotificationFrame) {
  const n = fromFrame(frame)
  const ws = useAuthStore.getState().tenantId
  if (n.type !== 'workspace_invitation' && n.workspace_id && ws && n.workspace_id !== ws) return
  if (n.type === 'workspace_invitation') {
    void queryClient.invalidateQueries({ queryKey: keys.auth.invitations() })
  }

  if (isOnScreen(n)) {
    void markReadNow(n.id)
    return
  }

  const now = Date.now()
  if (now - win.start > BURST_WINDOW_MS) win = { start: now, items: [], toasts: [] }
  win.items.push(n)
  const bursting = win.items.length >= BURST_MIN
  const actors = win.items.map((i) => i.actor_name ?? '').filter(Boolean)
  const ui = useNotificationUi.getState()

  if (ui.listVisible) {
    const next = { ...ui.fresh, [n.id]: { actorKey: n.actor_user_id ?? n.actor_name ?? n.id, tag: arrivalTag(n.type) } }
    ui.patch({
      fresh: next,
      burst: bursting ? { count: win.items.length, actors } : ui.burst,
      announcement: bursting ? `${win.items.length} thông báo mới` : `Thông báo mới: ${toastText(n)}`,
    })
    later(REALTIME_MS, () => {
      const cur = useNotificationUi.getState()
      const { [n.id]: _gone, ...rest } = cur.fresh
      cur.patch({ fresh: rest, ...(Object.keys(rest).length === 0 ? { burst: null } : {}) })
    })
    return
  }

  if (bursting) {
    win.toasts.forEach((id) => useToastStore.getState().dismiss(id))
    const id = toast.info(summaryText(win.items.length, actors), {
      action: { label: 'Mở', onClick: () => useNotificationUi.getState().requestOpen() },
    })
    win.toasts = [id]
    return
  }
  const id = toast.info(plainText(sentenceOf(n).lead), {
    icon: n.actor_name ? createElement(Avatar, { name: n.actor_name, hueKey: n.actor_user_id, size: 24 }) : undefined,
    action: { label: 'Xem', onClick: () => void openNotification(n) },
  })
  win.toasts.push(id)
}
