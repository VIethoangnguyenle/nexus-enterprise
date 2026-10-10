import { describe, expect, it } from 'vitest'
import { buildDirectory } from './people'
import {
  actionLabel, assetPerson, categoryLabel, lifecycleNotes, requestPill, stateLabel, statePill, stepSegments,
  urgencyLabel, urgencyPill, ASSET_STATES, REQUEST_FILTERS, requestFilterStatuses, formatStamp, actionDone, isFinalAction, explainAsset,
} from './asset-model'
import type { ActivityEntry, HistoryRecord } from '../api/assets'
import { ApiError } from '../api/client'
import { CONTACTS, U } from '../test/chat-fixtures'

const people = buildDirectory(CONTACTS)
const rec = (over: Partial<HistoryRecord>): HistoryRecord => ({
  id: 'h1', action: 'assign', from_state: 'available', to_state: 'assigned', actor_name: 'Lê Thị Hoa', ...over,
})
const text = (segs: ReturnType<typeof stepSegments>) =>
  segs.map((s) => (typeof s === 'string' ? s : 'b' in s ? `[${s.b}]` : `{${s.pill.label}}`)).join('')

describe('states follow the backend lifecycle', () => {
  it('names the six states in words and gives each a status tone', () => {
    expect(ASSET_STATES).toEqual(['requested', 'available', 'assigned', 'maintenance', 'retired', 'disposed'])
    expect(ASSET_STATES.map(stateLabel)).toEqual(['Chờ duyệt', 'Sẵn sàng', 'Đang giao', 'Bảo trì', 'Ngừng dùng', 'Đã thanh lý'])
    expect(statePill('available')).toEqual({ tone: 'ok', label: 'Sẵn sàng' })
    expect(statePill('assigned')).toEqual({ tone: 'info', label: 'Đang giao' })
    expect(statePill('maintenance')).toEqual({ tone: 'wait', label: 'Bảo trì' })
    expect(statePill('requested')).toEqual({ tone: 'acc', label: 'Chờ duyệt' })
  })

  it('the old frontend states are not states any more', () => {
    for (const old of ['in_use', 'pending', 'returned', 'approved']) {
      expect(ASSET_STATES).not.toContain(old)
    }
  })

  it('a state the screen was not taught reads as a neutral word, never its code', () => {
    expect(stateLabel('in_use')).toBe('Khác')
    expect(statePill('in_use')).toEqual({ tone: 'idle', label: 'Khác' })
  })
})

describe('actions are verbs, not transition names', () => {
  it('labels the default lifecycle steps', () => {
    expect(actionLabel('approve', 'available')).toBe('Duyệt nhập kho')
    expect(actionLabel('return', 'available')).toBe('Thu hồi')
    expect(actionLabel('flag_maintenance', 'maintenance')).toBe('Đưa đi bảo trì')
    expect(actionLabel('complete_maintenance', 'available')).toBe('Hoàn tất bảo trì')
    expect(actionLabel('retire', 'retired')).toBe('Ngừng dùng')
    expect(actionLabel('dispose', 'disposed')).toBe('Thanh lý')
  })

  it('a step it was not taught is named by where it leads, never printed as written', () => {
    expect(actionLabel('quarantine', 'maintenance')).toBe('Chuyển sang Bảo trì')
    expect(actionLabel('quarantine', 'somewhere_else')).toBe('Cập nhật')
    expect(actionLabel('quarantine', 'somewhere_else')).not.toMatch(/quarantine|_/)
  })
})

describe('history reads as a sentence with the people named', () => {
  it('a hand-over names who it went to', () => {
    expect(text(stepSegments(rec({ subject_name: 'Nguyễn Thu Lan' })))).toBe('đã giao cho [Nguyễn Thu Lan]')
  })

  it('a hand-over to someone the server could not name says so without an id', () => {
    expect(text(stepSegments(rec({ subject_user_id: U.lan.id })))).toBe('đã giao cho [Thành viên]')
    expect(text(stepSegments(rec({})))).toBe('đã giao tài sản')
  })

  it('a return names who it was taken from', () => {
    expect(text(stepSegments(rec({ action: 'return', from_state: 'assigned', to_state: 'available', subject_name: 'Vũ Anh Thư' })))).toBe('đã thu hồi từ [Vũ Anh Thư]')
    expect(text(stepSegments(rec({ action: 'return', from_state: 'assigned', to_state: 'available' })))).toBe('đã thu hồi')
  })

  it('maintenance and the rest keep their meaning', () => {
    expect(text(stepSegments(rec({ action: 'flag_maintenance', to_state: 'maintenance' })))).toBe('đã đưa đi bảo trì')
    expect(text(stepSegments(rec({ action: 'complete_maintenance', from_state: 'maintenance', to_state: 'available' })))).toBe('đã hoàn tất bảo trì, chuyển sang {Sẵn sàng}')
    expect(text(stepSegments(rec({ action: 'approve', from_state: 'requested', to_state: 'available' })))).toBe('đã duyệt nhập kho')
    expect(text(stepSegments(rec({ action: 'retire', to_state: 'retired' })))).toBe('đã cho ngừng dùng')
    expect(text(stepSegments(rec({ action: 'dispose', to_state: 'disposed' })))).toBe('đã thanh lý')
  })

  it('an unknown step says where the asset went, in words', () => {
    expect(text(stepSegments(rec({ action: 'quarantine', to_state: 'maintenance' })))).toBe('đã chuyển sang {Bảo trì}')
    expect(text(stepSegments(rec({ action: 'quarantine', to_state: 'nowhere' })))).toBe('đã cập nhật')
  })

  it('in the activity feed the asset is named after the verb', () => {
    const e: ActivityEntry = { ...rec({ action: 'return', from_state: 'assigned', to_state: 'available', subject_name: 'Vũ Anh Thư' }), asset_id: 'a', asset_name: 'MacBook Pro 14 inch, máy số 3' }
    expect(text(stepSegments(e, { asset: e.asset_name }))).toBe('đã thu hồi [MacBook Pro 14 inch, máy số 3] từ [Vũ Anh Thư]')
    const give: ActivityEntry = { ...rec({ subject_name: 'Nguyễn Thu Lan' }), asset_id: 'a', asset_name: 'Máy in' }
    expect(text(stepSegments(give, { asset: give.asset_name }))).toBe('đã giao [Máy in] cho [Nguyễn Thu Lan]')
  })
})

describe('requests', () => {
  it('say where a request stands in words', () => {
    expect(requestPill('pending')).toEqual({ tone: 'wait', label: 'Đang chờ' })
    expect(requestPill('approved')).toEqual({ tone: 'ok', label: 'Đã duyệt, chờ giao' })
    expect(requestPill('fulfilled')).toEqual({ tone: 'ok', label: 'Đã giao' })
    expect(requestPill('rejected')).toEqual({ tone: 'bad', label: 'Từ chối' })
    expect(requestPill('weird')).toEqual({ tone: 'idle', label: 'Khác' })
  })

  it('urgency has four words and only the top two are loud', () => {
    expect(['low', 'normal', 'high', 'urgent'].map(urgencyLabel)).toEqual(['Thấp', 'Bình thường', 'Cao', 'Khẩn'])
    expect(urgencyPill('urgent').tone).toBe('bad')
    expect(urgencyPill('high').tone).toBe('wait')
    expect(urgencyPill('normal').tone).toBe('idle')
    expect(urgencyPill(undefined)).toEqual({ tone: 'idle', label: 'Bình thường' })
  })

  it('the filters ask the server for the statuses they mean', () => {
    expect(REQUEST_FILTERS.map((f) => f.id)).toEqual(['pending', 'approved', 'rejected', 'all', 'mine'])
    expect(requestFilterStatuses('pending')).toEqual({ status: 'pending' })
    expect(requestFilterStatuses('approved')).toEqual({ status: 'approved,fulfilled' })
    expect(requestFilterStatuses('rejected')).toEqual({ status: 'rejected' })
    expect(requestFilterStatuses('all')).toEqual({})
    expect(requestFilterStatuses('mine')).toEqual({ mine: true })
  })
})

describe('categories and people', () => {
  it('shows a category in Vietnamese, and a free-typed one as typed', () => {
    expect(categoryLabel('hardware')).toBe('Phần cứng')
    expect(categoryLabel('software')).toBe('Phần mềm')
    expect(categoryLabel('license')).toBe('Giấy phép')
    expect(categoryLabel('furniture')).toBe('Nội thất')
    expect(categoryLabel('other')).toBe('Khác')
    expect(categoryLabel('Thiết bị mạng')).toBe('Thiết bị mạng')
  })

  it('names a person by the server, else the directory, else a neutral word', () => {
    expect(assetPerson(people, U.lan.id, 'Tên do máy chủ').name).toBe('Tên do máy chủ')
    expect(assetPerson(people, U.lan.id).name).toBe('Nguyễn Thu Lan')
    expect(assetPerson(people, U.lan.id).hueKey).toBe(U.lan.id)
    expect(assetPerson(people, 'a-stranger-id').name).toBe('Thành viên')
    expect(assetPerson(people, undefined).name).toBe('Thành viên')
  })
})

describe('what each lifecycle step needs, in words', () => {
  it('groups the steps by the right they need and never prints an operation name', () => {
    const notes = lifecycleNotes([
      { from_state: 'requested', to_state: 'available', operation: 'approve', ngac_permission: 'approve' },
      { from_state: 'available', to_state: 'assigned', operation: 'assign', ngac_permission: 'manage' },
      { from_state: 'assigned', to_state: 'available', operation: 'return', ngac_permission: 'manage' },
    ])
    expect(notes).toEqual([
      { right: 'Duyệt', steps: ['Duyệt nhập kho'] },
      { right: 'Quản lý', steps: ['Giao tài sản', 'Thu hồi'] },
    ])
  })
})

describe('time stamps', () => {
  const now = new Date(2026, 9, 10, 12, 0)
  it('reads as a clock today, "Hôm qua" with the clock, then the day and the clock', () => {
    expect(formatStamp(new Date(2026, 9, 10, 9, 20), now)).toBe('09:20')
    expect(formatStamp(new Date(2026, 9, 9, 16, 10), now)).toBe('Hôm qua 16:10')
    expect(formatStamp(new Date(2026, 9, 8, 10, 2), now)).toBe('08/10 10:02')
    expect(formatStamp(new Date(2025, 11, 31, 8, 5), now)).toBe('31/12/2025 08:05')
    expect(formatStamp(undefined, now)).toBe('')
  })
})

describe('doing a step', () => {
  it('says what was done in words, and asks first only for the steps that cannot be undone', () => {
    expect(actionDone('flag_maintenance')).toBe('Đã đưa đi bảo trì')
    expect(actionDone('whatever')).toBe('Đã cập nhật tài sản')
    expect(['retire', 'dispose'].every(isFinalAction)).toBe(true)
    expect(['return', 'flag_maintenance', 'approve'].some(isFinalAction)).toBe(false)
  })
})

describe('request decisions in the feed', () => {
  const e = (over: Partial<ActivityEntry>): ActivityEntry => ({
    id: 'a', action: 'request_approved', from_state: '', to_state: '', actor_name: 'Lê Thị Hoa', subject_name: 'Nguyễn Thu Lan',
    asset_id: '', asset_name: 'Màn hình', request_status: 'approved', ...over,
  })
  it('reads as who decided which request of whom', () => {
    expect(text(stepSegments(e({}), { asset: 'Màn hình' }))).toBe('đã duyệt yêu cầu [Màn hình] của [Nguyễn Thu Lan]')
    expect(text(stepSegments(e({ action: 'request_rejected', request_status: 'rejected' }), { asset: 'Màn hình' }))).toBe('đã từ chối yêu cầu [Màn hình] của [Nguyễn Thu Lan]')
    expect(text(stepSegments(e({ subject_name: undefined, subject_user_id: U.lan.id }), { asset: 'Màn hình' }))).toBe('đã duyệt yêu cầu [Màn hình] của [Thành viên]')
  })
})

describe('why an asset action failed', () => {
  const fail = (status: number, reason?: string) => new ApiError('x', status, reason ? { reason } : {})
  it('tells the refusals that share a status apart', () => {
    expect(explainAsset(fail(409, 'request_not_open'), 'duyệt')).toMatch(/vừa được xử lý/)
    expect(explainAsset(fail(409, 'asset_unavailable'), 'duyệt')).toMatch(/vừa được giao hoặc không còn Sẵn sàng/)
    expect(explainAsset(fail(409, 'state_changed'), 'cập nhật tài sản')).toMatch(/vừa thay đổi/)
    expect(explainAsset(fail(400, 'not_a_member'), 'giao tài sản')).toMatch(/không còn là thành viên/)
    expect(explainAsset(fail(400, 'wrong_type'), 'duyệt')).toMatch(/không thuộc loại/)
    expect(explainAsset(fail(400, 'same_holder'), 'giao tài sản')).toMatch(/đang giữ tài sản này/)
  })
  it('falls back to the general sentence for the status', () => {
    expect(explainAsset(fail(403), 'duyệt')).toMatch(/Bạn chưa có quyền duyệt/)
    expect(explainAsset(fail(409), 'duyệt')).toMatch(/vừa được người khác thay đổi/)
    expect(explainAsset(new Error('offline'), 'duyệt')).toMatch(/Chưa duyệt được/)
  })
})
