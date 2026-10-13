import { describe, expect, it } from 'vitest'
import {
  describeNotification, groupByDay, notificationTarget, notificationTime, type NotificationItem,
} from './notification-copy'

const item = (over: Partial<NotificationItem> = {}): NotificationItem => ({
  id: 'n1', type: 'approval_approved', title: 'Your request was approved', body: 'English body', read: false,
  created_at: '2026-10-10T09:41:00', entity_type: 'approval', entity_id: 'r1', ...over,
})
// Saturday 10 Oct 2026, 09:46 local time.
const NOW = new Date(2026, 9, 10, 9, 46)

describe('describeNotification', () => {
  it.each([
    ['approval_approved', 'Một đề nghị của bạn đã được duyệt.', 'approval'],
    ['approval_rejected', 'Một đề nghị của bạn đã bị trả lại.', 'approval'],
    ['asset_request_approved', 'Yêu cầu cấp tài sản của bạn đã được duyệt.', 'asset'],
    ['asset_request_rejected', 'Yêu cầu cấp tài sản của bạn đã bị từ chối.', 'asset'],
    ['asset_assigned', 'Bạn được giao một tài sản.', 'asset'],
    ['asset_returned', 'Một tài sản đã được nhận lại từ bạn.', 'asset'],
    ['asset_request', 'Bạn đã gửi yêu cầu cấp tài sản. Đang chờ duyệt.', 'asset'],
    ['asset_lifecycle', 'Bạn đã đổi trạng thái của một tài sản.', 'asset'],
  ])('%s is written in Vietnamese from its type, never the server text', (type, text, domain) => {
    expect(describeNotification(item({ type }))).toEqual({ text, domain })
  })

  it('writes an invitation from the sentence the server made of names, and falls back without one', () => {
    const inv = { type: 'workspace_invitation', body: 'Lê Thị Hoa mời bạn vào NovaPay Kế toán' }
    expect(describeNotification(item(inv))).toEqual({ text: 'Lê Thị Hoa mời bạn vào NovaPay Kế toán', domain: 'invitation' })
    expect(describeNotification(item({ type: 'workspace_invitation', body: '  ' }))).toEqual({
      text: 'Bạn có một lời mời vào workspace.', domain: 'invitation',
    })
  })

  it('does not print the title or body of the server, or any id', () => {
    const d = describeNotification(item({ type: 'something_new', title: 'Raw English', body: 'r1' }))
    expect(d).toEqual({ text: 'Bạn có một thông báo mới.', domain: null })
  })
})

describe('notificationTarget', () => {
  it('opens the request, the asset or the asset request it is about', () => {
    expect(notificationTarget(item())).toEqual({ to: '/approval', search: { request: 'r1' } })
    expect(notificationTarget(item({ entity_type: 'asset', entity_id: 'a1' }))).toEqual({ to: '/assets', search: { asset: 'a1' } })
    expect(notificationTarget(item({ entity_type: 'asset_request', entity_id: 'q1' }))).toEqual({
      to: '/assets', search: { section: 'requests', request: 'q1' },
    })
  })

  it('sends an invitation to Chọn workspace, whatever id it carries', () => {
    expect(notificationTarget(item({ type: 'workspace_invitation', entity_type: 'invitation', entity_id: 'i1' }))).toEqual({ to: '/workspace-select' })
    expect(notificationTarget(item({ type: 'workspace_invitation', entity_type: undefined, entity_id: undefined }))).toEqual({ to: '/workspace-select' })
  })

  it('has no target when the notification names no entity, or one it does not know', () => {
    expect(notificationTarget(item({ entity_type: undefined, entity_id: undefined }))).toBeNull()
    expect(notificationTarget(item({ entity_type: 'channel', entity_id: 'c1' }))).toBeNull()
  })
})

describe('notificationTime', () => {
  it('reads like the rest of the app', () => {
    expect(notificationTime('2026-10-10T09:41:00', NOW)).toBe('5 phút trước')
    expect(notificationTime('2026-10-10T07:20:00', NOW)).toBe('07:20')
    expect(notificationTime('2026-10-09T16:05:00', NOW)).toBe('Hôm qua 16:05')
    expect(notificationTime('2026-10-07T08:30:00', NOW)).toBe('07/10')
    expect(notificationTime('2025-12-31T08:30:00', NOW)).toBe('31/12/2025')
  })
})

describe('groupByDay', () => {
  it('splits today from earlier, keeps the order, and drops an empty group', () => {
    const a = item({ id: 'a', created_at: '2026-10-10T09:41:00' })
    const b = item({ id: 'b', created_at: '2026-10-09T16:05:00' })
    expect(groupByDay([a, b], NOW)).toEqual([{ label: 'Hôm nay', items: [a] }, { label: 'Trước đó', items: [b] }])
    expect(groupByDay([b], NOW)).toEqual([{ label: 'Trước đó', items: [b] }])
    expect(groupByDay([], NOW)).toEqual([])
  })
})
