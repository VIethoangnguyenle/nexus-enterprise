import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, ApiError } from '../api/client'
import { keys } from '../hooks/keys'
import { useToastStore } from '../components/primitives/Toast'
import { useAuthStore } from '../stores/auth.store'
import { useNotificationUi } from '../stores/notification.store'
import { queryClient } from './query-client'
import {
  UNAVAILABLE, openNotification, receiveNotification, registerNavigator, resetArrivals, type NotificationFrame,
} from './notification-arrivals'
import type { AppNotification } from '../api/notifications'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const WS = '0a8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b'
const OTHER_WS = '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c'
const REQ = '77777777-aaaa-4bbb-8ccc-000000000001'
const ACTOR = '11111111-aaaa-4bbb-8ccc-000000000004'

const frame = (over: Partial<NotificationFrame> = {}): NotificationFrame => ({
  id: crypto.randomUUID(), type: 'approval_approved', targetType: 'approval', targetId: REQ, actorUserId: ACTOR,
  actorName: 'Lê Quang Vinh', targetName: 'Tạm ứng công tác phí tháng 10', workspaceId: WS, params: {},
  createdAt: { seconds: String(Math.floor(Date.now() / 1000)), nanos: 0 }, ...over,
})

const toasts = () => useToastStore.getState().toasts

beforeEach(() => {
  vi.useFakeTimers()
  api.mockReset()
  api.mockResolvedValue({ status: 'ok' })
  useAuthStore.setState({ tenantId: WS })
  useNotificationUi.setState({ listVisible: false, openRequest: 0 })
  window.history.replaceState({}, '', '/channels')
})
afterEach(() => {
  vi.useRealTimers()
  resetArrivals()
  registerNavigator(null)
  useToastStore.getState().clear()
  queryClient.clear()
})

describe('an arrival while the list is closed', () => {
  it('shows one toast naming who did what, with a way to open it', () => {
    receiveNotification(frame())
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]?.message).toBe('Lê Quang Vinh đã duyệt Tạm ứng công tác phí tháng 10.')
    expect(toasts()[0]?.action?.label).toBe('Xem')
  })

  it('"Xem" reads it and opens its subject', async () => {
    const go = vi.fn()
    registerNavigator(go)
    const f = frame()
    // The subject can be opened.
    api.mockImplementation((path: string) => (path.startsWith('/approval/') ? Promise.resolve({ request: {} }) : Promise.resolve({ status: 'ok' })))
    receiveNotification(f)
    toasts()[0]!.action!.onClick()
    await vi.runAllTimersAsync()
    expect(api).toHaveBeenCalledWith(`/notifications/${f.id}/read`, { method: 'POST' })
    expect(go).toHaveBeenCalledWith({ to: '/approval', search: { tab: 'mine', request: REQ } })
  })

  it('folds three within two seconds into one toast, and keeps folding', () => {
    receiveNotification(frame({ actorName: 'Lê Quang Vinh' }))
    vi.advanceTimersByTime(600)
    receiveNotification(frame({ actorName: 'Phạm Hải Yến' }))
    expect(toasts()).toHaveLength(2)
    vi.advanceTimersByTime(600)
    receiveNotification(frame({ actorName: 'Đỗ Văn Khải' }))

    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]?.message).toBe('3 thông báo mới · Lê Quang Vinh và 2 người khác')
    expect(toasts()[0]?.action?.label).toBe('Mở')

    vi.advanceTimersByTime(300)
    receiveNotification(frame({ actorName: 'Lê Quang Vinh' }))
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]?.message).toMatch(/^4 thông báo mới/)
  })

  it('"Mở" asks the screen to open the list', () => {
    for (let i = 0; i < 3; i++) receiveNotification(frame())
    const before = useNotificationUi.getState().openRequest
    toasts()[0]!.action!.onClick()
    expect(useNotificationUi.getState().openRequest).toBe(before + 1)
  })

  it('starts a new count once the window has passed', () => {
    receiveNotification(frame())
    receiveNotification(frame())
    vi.advanceTimersByTime(2500)
    receiveNotification(frame())
    expect(toasts().map((t) => t.message).every((m) => !m.includes('thông báo mới'))).toBe(true)
  })
})

describe('an arrival while the list is open', () => {
  beforeEach(() => useNotificationUi.setState({ listVisible: true }))

  it('washes the new row in its author\'s hue with a "vừa …" tag, and shows no toast', () => {
    const f = frame()
    receiveNotification(f)
    expect(toasts()).toHaveLength(0)
    expect(useNotificationUi.getState().fresh[f.id]).toEqual({ actorKey: ACTOR, tag: 'vừa duyệt' })
    expect(useNotificationUi.getState().announcement).toBe('Thông báo mới: Lê Quang Vinh đã duyệt Tạm ứng công tác phí tháng 10.')
    vi.advanceTimersByTime(2400)
    expect(useNotificationUi.getState().fresh[f.id]).toBeUndefined()
  })

  it('replaces the tags with one summary line from the third within two seconds', () => {
    for (const name of ['Lê Quang Vinh', 'Phạm Hải Yến', 'Đỗ Văn Khải']) receiveNotification(frame({ actorName: name }))
    expect(useNotificationUi.getState().burst).toEqual({ count: 3, actors: ['Lê Quang Vinh', 'Phạm Hải Yến', 'Đỗ Văn Khải'] })
    expect(useNotificationUi.getState().announcement).toBe('3 thông báo mới')
    expect(toasts()).toHaveLength(0)
    vi.advanceTimersByTime(2400)
    expect(useNotificationUi.getState().burst).toBeNull()
  })
})

describe('an arrival that needs nothing shown', () => {
  it('reads silently when the person already has that very request open', async () => {
    window.history.replaceState({}, '', `/approval?tab=mine&request=${REQ}`)
    const f = frame()
    receiveNotification(f)
    await vi.runAllTimersAsync()
    expect(toasts()).toHaveLength(0)
    expect(api).toHaveBeenCalledWith(`/notifications/${f.id}/read`, { method: 'POST' })
  })

  it('still shows a different request\'s notification', () => {
    window.history.replaceState({}, '', '/approval?request=other')
    receiveNotification(frame())
    expect(toasts()).toHaveLength(1)
  })

  it('ignores another workspace\'s notification, but not an invitation, which is personal', () => {
    receiveNotification(frame({ workspaceId: OTHER_WS }))
    expect(toasts()).toHaveLength(0)

    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    receiveNotification(frame({
      type: 'workspace_invitation', targetType: 'workspace_invitation', workspaceId: OTHER_WS, targetName: 'NovaPay',
    }))
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]?.message).toBe('Lê Quang Vinh đã mời bạn vào NovaPay.')
    // The switcher's "Lời mời đang chờ" row learns of it.
    expect(spy).toHaveBeenCalledWith({ queryKey: keys.auth.invitations() })
  })
})

describe('openNotification', () => {
  const n: AppNotification = {
    id: 'n1', type: 'asset_assigned', read: false, created_at: new Date().toISOString(), workspace_id: WS,
    target_type: 'asset', target_id: REQ, target_name: 'MacBook Pro 14', actor_name: 'Nguyễn Thu Lan', params: {},
  }

  it('marks it read, then opens the asset', async () => {
    const go = vi.fn()
    api.mockImplementation((path: string) => Promise.resolve(path.startsWith('/assets/') ? { id: REQ } : { status: 'ok' }))
    await expect(openNotification(n, go)).resolves.toBe(true)
    expect(api).toHaveBeenCalledWith('/notifications/n1/read', { method: 'POST' })
    expect(go).toHaveBeenCalledWith({ to: '/assets', search: { section: 'list', asset: REQ } })
  })

  it('stays put, still reads it, and says why when the subject is gone or off limits', async () => {
    const go = vi.fn()
    for (const status of [404, 403]) {
      useToastStore.getState().clear()
      queryClient.clear()
      api.mockImplementation((path: string) =>
        path.startsWith('/assets/') ? Promise.reject(new ApiError('x', status)) : Promise.resolve({ status: 'ok' }))
      await expect(openNotification(n, go)).resolves.toBe(false)
      expect(toasts().map((t) => t.message)).toEqual([UNAVAILABLE])
      expect(api).toHaveBeenCalledWith('/notifications/n1/read', { method: 'POST' })
    }
    expect(go).not.toHaveBeenCalled()
  })

  it('does not stop the person trying when the connection is what failed', async () => {
    const go = vi.fn()
    api.mockImplementation((path: string) =>
      path.startsWith('/assets/') ? Promise.reject(new ApiError('boom', 500)) : Promise.resolve({ status: 'ok' }))
    const opened = openNotification(n, go)
    await vi.runAllTimersAsync() // the one retry the query is allowed
    await expect(opened).resolves.toBe(true)
    expect(go).toHaveBeenCalled()
  })

  it('has nowhere to go for a notification with no subject', async () => {
    const go = vi.fn()
    await expect(openNotification({ ...n, target_type: undefined, target_id: undefined }, go)).resolves.toBe(false)
    expect(toasts().map((t) => t.message)).toEqual([UNAVAILABLE])
  })
})
