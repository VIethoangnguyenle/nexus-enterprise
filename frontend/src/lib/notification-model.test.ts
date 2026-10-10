import { describe, expect, it } from 'vitest'
import type { AppNotification } from '../api/notifications'
import {
  arrivalTag, domainOf, groupByDay, openTargetOf, plainText, sentenceOf, summaryText, toastText, unreadLabel,
} from './notification-model'

const UUID = '9c1d7e52-4a3b-4c8d-9e0f-1a2b3c4d5e6f'

const n = (over: Partial<AppNotification>): AppNotification => ({
  id: UUID, type: 'approval_approved', read: false, created_at: '2026-10-10T08:00:00Z', workspace_id: UUID,
  actor_user_id: UUID, actor_name: 'Trần Minh Đức', target_type: 'approval', target_id: UUID,
  target_name: 'Tạm ứng công tác phí tháng 10', params: {}, ...over,
})

describe('sentenceOf: the screen words every type from names', () => {
  it('says who did what to which thing, with the names set apart', () => {
    const s = sentenceOf(n({}))
    expect(plainText(s.lead)).toBe('Trần Minh Đức đã duyệt Tạm ứng công tác phí tháng 10.')
    expect(s.lead.filter((p) => p.strong).map((p) => p.text)).toEqual(['Trần Minh Đức', 'Tạm ứng công tác phí tháng 10'])
    expect(s.follow).toBe('Đề nghị đã hoàn tất.')
  })

  it('tells a passed step from a finished request', () => {
    const step = sentenceOf(n({ type: 'approval_step_approved' }))
    expect(plainText(step.lead)).toBe('Trần Minh Đức đã duyệt bước của mình cho Tạm ứng công tác phí tháng 10.')
    expect(step.follow).toBe('Đề nghị vẫn đang chờ các bước sau.')
    expect(sentenceOf(n({ type: 'approval_approved' })).follow).not.toBe(step.follow)
  })

  it('words each remaining type', () => {
    const said = (type: string, over: Partial<AppNotification> = {}) => plainText(sentenceOf(n({ type, ...over })).lead)
    expect(said('approval_rejected')).toBe('Trần Minh Đức đã trả lại Tạm ứng công tác phí tháng 10.')
    expect(said('asset_request_approved', { target_name: 'Điện thoại công vụ' })).toBe('Trần Minh Đức đã duyệt yêu cầu cấp Điện thoại công vụ của bạn.')
    expect(said('asset_request_rejected', { target_name: 'Laptop' })).toBe('Trần Minh Đức đã từ chối yêu cầu cấp Laptop.')
    expect(said('asset_assigned', { target_name: 'MacBook Pro 14 inch' })).toBe('Trần Minh Đức đã giao MacBook Pro 14 inch cho bạn.')
    expect(said('asset_returned', { target_name: 'Màn hình Dell 27 inch' })).toBe('Trần Minh Đức đã nhận lại Màn hình Dell 27 inch từ bạn.')
    expect(said('workspace_invitation', { target_name: 'NovaPay' })).toBe('Trần Minh Đức đã mời bạn vào NovaPay.')
  })

  it('shows the reason of a rejection, and only then', () => {
    expect(sentenceOf(n({ type: 'approval_rejected', params: { reason: 'Đã có màn hình dự phòng ở kho.' } })).reason).toBe('Đã có màn hình dự phòng ở kho.')
    expect(sentenceOf(n({ type: 'approval_rejected' })).reason).toBeUndefined()
    expect(sentenceOf(n({ type: 'approval_approved', params: { reason: 'x' } })).reason).toBeUndefined()
  })

  it('never falls back to an id: a missing name becomes a plain word', () => {
    const noTarget = sentenceOf(n({ target_name: '' }))
    expect(plainText(noTarget.lead)).toBe('Trần Minh Đức đã duyệt một đề nghị.')
    expect(plainText(sentenceOf(n({ type: 'asset_assigned', target_name: undefined })).lead)).toBe('Trần Minh Đức đã giao một tài sản cho bạn.')

    const noActor = sentenceOf(n({ actor_name: '', actor_user_id: '' }))
    expect(plainText(noActor.lead)).toBe('Tạm ứng công tác phí tháng 10 đã được duyệt.')
    expect(plainText(noActor.lead)).not.toMatch(/[0-9a-f]{8}-[0-9a-f]{4}/)
  })

  it('does not print server text or ids for an unknown type', () => {
    const s = sentenceOf(n({ type: 'something_new', target_name: 'x' }))
    expect(plainText(s.lead)).toBe('Có một thông báo mới.')
  })
})

describe('metadata', () => {
  it('names the domain', () => {
    expect(domainOf('approval_rejected')).toBe('approval')
    expect(domainOf('asset_assigned')).toBe('asset')
    expect(domainOf('asset_request_approved')).toBe('asset')
    expect(domainOf('workspace_invitation')).toBe('workspace')
    expect(domainOf('unknown')).toBe('other')
  })

  it('tags an arrival with what just happened', () => {
    expect(arrivalTag('approval_approved')).toBe('vừa duyệt')
    expect(arrivalTag('approval_rejected')).toBe('vừa trả lại')
    expect(arrivalTag('asset_request_rejected')).toBe('vừa từ chối')
    expect(arrivalTag('asset_assigned')).toBe('vừa giao')
    expect(arrivalTag('asset_returned')).toBe('vừa nhận lại')
    expect(arrivalTag('unknown')).toBe('vừa có')
  })

  it('labels the unread count for assistive tech', () => {
    expect(unreadLabel('Thông báo', 3)).toBe('Thông báo, 3 chưa đọc')
    expect(unreadLabel('Thông báo', 0)).toBe('Thông báo')
    expect(unreadLabel('Thêm', 3, 'thông báo')).toBe('Thêm, 3 thông báo chưa đọc')
  })
})

describe('openTargetOf: each target opens in its own place', () => {
  it('opens a request under Của bạn, and an asset or its request in Tài sản', () => {
    expect(openTargetOf(n({}))).toEqual({ to: '/approval', search: { tab: 'mine', request: UUID } })
    expect(openTargetOf(n({ type: 'asset_request_approved', target_type: 'asset_request' }))).toEqual({
      to: '/assets', search: { section: 'requests', request: UUID },
    })
    expect(openTargetOf(n({ type: 'asset_assigned', target_type: 'asset' }))).toEqual({
      to: '/assets', search: { section: 'list', asset: UUID },
    })
  })

  it('sends an invitation to the workspace picker, which shows the offer', () => {
    expect(openTargetOf(n({ type: 'workspace_invitation', target_type: 'workspace_invitation' }))).toEqual({ to: '/workspace-select', search: {} })
  })

  it('has nowhere to go without a target', () => {
    expect(openTargetOf(n({ target_type: '', target_id: '' }))).toBeNull()
    expect(openTargetOf(n({ target_type: 'mystery' }))).toBeNull()
  })
})

describe('groupByDay', () => {
  const now = new Date(2026, 9, 10, 15, 0)
  const at = (d: Date) => d.toISOString()
  it('splits today from before, newest order kept, and hides an empty group', () => {
    const today = n({ id: 'a', created_at: at(new Date(2026, 9, 10, 9, 14)) })
    const earlier = n({ id: 'b', created_at: at(new Date(2026, 9, 9, 23, 59)) })
    const old = n({ id: 'c', created_at: at(new Date(2026, 8, 1)) })
    expect(groupByDay([today, earlier, old], now).map((g) => [g.label, g.items.map((i) => i.id)])).toEqual([
      ['Hôm nay', ['a']], ['Trước đó', ['b', 'c']],
    ])
    expect(groupByDay([earlier], now).map((g) => g.label)).toEqual(['Trước đó'])
    expect(groupByDay([], now)).toEqual([])
  })
})

describe('toasts', () => {
  it('takes the lead of the sentence, without the follow-up or the reason', () => {
    expect(toastText(n({ type: 'approval_rejected', params: { reason: 'Không đủ ngân sách' } }))).toBe('Trần Minh Đức đã trả lại Tạm ứng công tác phí tháng 10.')
    expect(toastText(n({}))).toBe('Trần Minh Đức đã duyệt Tạm ứng công tác phí tháng 10.')
  })

  it('summarises a burst by count and who', () => {
    expect(summaryText(3, ['Lê Quang Vinh', 'Phạm Hải Yến', 'Đỗ Văn Khải'])).toBe('3 thông báo mới · Lê Quang Vinh và 2 người khác')
    expect(summaryText(3, ['Lê Quang Vinh'])).toBe('3 thông báo mới · Lê Quang Vinh')
    expect(summaryText(4, [])).toBe('4 thông báo mới')
  })
})
