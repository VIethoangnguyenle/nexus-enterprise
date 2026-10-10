/**
 * Fixture backend for the notification screens: a small in-memory list that
 * answers what the messaging service answers and changes when the screens
 * write to it. Every id is a real-looking UUID so tests can assert that none
 * of them reaches the screen.
 */
import { ApiError } from '../api/client'
import type { AppNotification, NotificationPage } from '../api/notifications'
import { U, WS_ID } from './chat-fixtures'

export const N_ID = {
  approved: '88888888-aaaa-4bbb-8ccc-000000000001',
  rejected: '88888888-aaaa-4bbb-8ccc-000000000002',
  assigned: '88888888-aaaa-4bbb-8ccc-000000000003',
  older: '88888888-aaaa-4bbb-8ccc-000000000004',
}
export const T_ID = {
  request: '99999999-aaaa-4bbb-8ccc-000000000001',
  asset: '99999999-aaaa-4bbb-8ccc-000000000002',
  assetRequest: '99999999-aaaa-4bbb-8ccc-000000000003',
}

const minutesAgo = (m: number) => new Date(Date.now() - m * 60_000).toISOString()

export function notification(over: Partial<AppNotification> & { id: string }): AppNotification {
  return {
    type: 'approval_approved', read: false, created_at: minutesAgo(5), workspace_id: WS_ID,
    actor_user_id: U.vinh.id, actor_name: 'Lê Quang Vinh', target_type: 'approval', target_id: T_ID.request,
    target_name: 'Tạm ứng công tác phí tháng 10', params: {}, ...over,
  }
}

/** Three unread today and one read from an earlier day. */
export const seed = (): AppNotification[] => [
  notification({ id: N_ID.approved }),
  notification({
    id: N_ID.rejected, type: 'approval_rejected', created_at: minutesAgo(30), actor_user_id: U.duc.id, actor_name: 'Trần Minh Đức',
    target_name: 'Mua 2 màn hình cho tổ kế toán', params: { reason: 'Đã có màn hình dự phòng ở kho.' },
  }),
  notification({
    id: N_ID.assigned, type: 'asset_assigned', created_at: minutesAgo(60), actor_user_id: U.lan.id, actor_name: 'Nguyễn Thu Lan',
    target_type: 'asset', target_id: T_ID.asset, target_name: 'MacBook Pro 14 inch',
  }),
  notification({
    id: N_ID.older, type: 'asset_request_approved', read: true, created_at: new Date(2026, 0, 3, 9, 0).toISOString(),
    actor_user_id: U.lan.id, actor_name: 'Nguyễn Thu Lan', target_type: 'asset_request', target_id: T_ID.assetRequest,
    target_name: 'Điện thoại công vụ',
  }),
]

export interface NotificationServer {
  rows: AppNotification[]
  /** Makes list requests fail. */
  listFails: boolean
  /** Makes mark requests fail. */
  markFails: boolean
  /** Subjects that answer 404. */
  gone: Set<string>
  /** The total the list reports (for paging tests); defaults to the rows held. */
  total?: number
  calls: string[]
  handler: (path: string, init?: RequestInit) => Promise<unknown>
}

export function notificationServer(rows: AppNotification[] = seed()): NotificationServer {
  const s: NotificationServer = {
    rows, listFails: false, markFails: false, gone: new Set(), calls: [],
    handler: async (path, init) => {
      const method = init?.method ?? 'GET'
      s.calls.push(`${method} ${path}`)
      if (path.startsWith('/notifications?')) {
        if (s.listFails) throw new ApiError('boom', 500)
        const q = new URLSearchParams(path.split('?')[1])
        const offset = Number(q.get('offset') ?? 0)
        const limit = Number(q.get('limit') ?? 25)
        const page: NotificationPage = {
          notifications: s.rows.slice(offset, offset + limit),
          total: s.total ?? s.rows.length,
          unread_count: s.rows.filter((n) => !n.read).length,
        }
        return page
      }
      if (path === '/notifications/unread-count') return { count: s.rows.filter((n) => !n.read).length }
      if (path === '/notifications/read-all' || /^\/notifications\/[^/]+\/read$/.test(path) || path === '/notifications/read-about') {
        if (s.markFails) throw new ApiError('boom', 500)
        if (path === '/notifications/read-all') s.rows = s.rows.map((n) => ({ ...n, read: true }))
        else if (path === '/notifications/read-about') {
          const b = JSON.parse(String(init?.body)) as { id: string }
          s.rows = s.rows.map((n) => (n.target_id === b.id ? { ...n, read: true } : n))
        } else {
          const id = path.split('/')[2]
          s.rows = s.rows.map((n) => (n.id === id ? { ...n, read: true } : n))
        }
        return { status: 'ok' }
      }
      // The subjects a notification opens: asked only to know whether they can be opened.
      const subject = /^\/(approval\/requests|assets|asset-requests)\/([^/]+)$/.exec(path)
      if (subject) {
        if (s.gone.has(subject[2]!)) throw new ApiError('gone', 404)
        return { id: subject[2] }
      }
      throw new Error(`unexpected ${method} ${path}`)
    },
  }
  return s
}
