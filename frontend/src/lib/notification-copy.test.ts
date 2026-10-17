import { describe, expect, it } from 'vitest'
import {
  DOMAIN_LABEL, describeNotification, groupByDay, notificationTarget, notificationTime, type NotificationItem,
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

const calBody = (over: Record<string, unknown> = {}) =>
  JSON.stringify({ ws: 'w1', actor: 'Lê Thị Hoa', start: new Date(2026, 9, 17, 9, 0).toISOString(), ...over })

describe('calendar notifications', () => {
  const cal = (type: string, over: Record<string, unknown> = {}, title = 'Họp giao ban') =>
    item({ type, title, body: calBody(over), entity_type: 'calendar_event', entity_id: 'e1' })

  it.each([
    ['calendar_invitation', {}, 'Lê Thị Hoa mời bạn tới Họp giao ban'],
    ['calendar_rsvp', { response: 'accepted' }, 'Lê Thị Hoa đã nhận lời Họp giao ban'],
    ['calendar_rsvp', { response: 'declined' }, 'Lê Thị Hoa đã từ chối Họp giao ban'],
    ['calendar_rsvp', { response: 'tentative' }, 'Lê Thị Hoa có thể tham gia Họp giao ban'],
    ['calendar_reminder', {}, 'Họp giao ban bắt đầu lúc 09:00'],
    ['calendar_updated', {}, 'Lê Thị Hoa đã dời Họp giao ban'],
    ['calendar_cancelled', {}, 'Lê Thị Hoa đã huỷ Họp giao ban'],
    ['calendar_shared', {}, 'Lê Thị Hoa đã chia sẻ lịch với bạn'],
  ])('%s %j reads as a sentence of names and times', (type, over, text) => {
    expect(describeNotification(cal(type, over))).toEqual({ text, domain: 'calendar' })
  })

  it('labels the domain Lịch', () => {
    expect(DOMAIN_LABEL.calendar).toBe('Lịch')
  })

  it('gives the time of a reminder on the viewer clock and says so for an all-day event', () => {
    expect(describeNotification(cal('calendar_reminder', { start: new Date(2026, 9, 17, 14, 30).toISOString() })).text)
      .toBe('Họp giao ban bắt đầu lúc 14:30')
    expect(describeNotification(cal('calendar_reminder', { allDay: true })).text).toBe('Họp giao ban diễn ra cả ngày hôm nay')
    expect(describeNotification(cal('calendar_reminder', { start: undefined })).text).toBe('Họp giao ban sắp bắt đầu')
  })

  it('stays readable without a name, a title or a body, and never prints the raw body', () => {
    expect(describeNotification(cal('calendar_invitation', { actor: undefined })).text).toBe('Bạn được mời tới Họp giao ban')
    expect(describeNotification(cal('calendar_updated', { actor: undefined })).text).toBe('Họp giao ban đã được dời')
    expect(describeNotification(cal('calendar_cancelled', { actor: undefined })).text).toBe('Họp giao ban đã bị huỷ')
    expect(describeNotification(cal('calendar_rsvp', { actor: undefined, response: 'accepted' })).text).toBe('Một khách đã nhận lời Họp giao ban')
    expect(describeNotification(cal('calendar_rsvp', { response: 'who-knows' })).text).toBe('Lê Thị Hoa đã trả lời Họp giao ban')
    expect(describeNotification(cal('calendar_shared', { actor: undefined })).text).toBe('Một lịch đã được chia sẻ với bạn')
    expect(describeNotification(cal('calendar_invitation', {}, '  ')).text).toBe('Lê Thị Hoa mời bạn tới một sự kiện')
    const broken = describeNotification(item({ type: 'calendar_invitation', title: 'Họp', body: '{"ws":"w1","actor":' }))
    expect(broken).toEqual({ text: 'Bạn được mời tới Họp', domain: 'calendar' })
    expect(describeNotification(item({ type: 'calendar_invitation', title: 'Họp', body: '[1,2]' })).text).toBe('Bạn được mời tới Họp')
    expect(describeNotification(item({ type: 'calendar_invitation', title: 'Họp', body: 'null' })).text).toBe('Bạn được mời tới Họp')
  })

  it('opens the week of the event, in its workspace', () => {
    expect(notificationTarget(cal('calendar_invitation'))).toEqual({
      to: '/calendar', search: { mode: 'week', date: '2026-10-17', ws: 'w1' },
    })
    expect(notificationTarget(cal('calendar_reminder'))).toEqual({
      to: '/calendar', search: { mode: 'week', date: '2026-10-17', ws: 'w1' },
    })
  })

  it('opens the current week when no day or workspace is named, and for a share', () => {
    expect(notificationTarget(item({ type: 'calendar_shared', entity_type: 'calendar', entity_id: 'c1', body: '{"actor":"A"}' }))).toEqual({
      to: '/calendar', search: { mode: 'week' },
    })
    expect(notificationTarget(item({ type: 'calendar_updated', body: 'not json' }))).toEqual({ to: '/calendar', search: { mode: 'week' } })
    expect(notificationTarget(item({ type: 'calendar_updated', body: '{"start":"garbage"}' }))).toEqual({ to: '/calendar', search: { mode: 'week' } })
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
