import { describe, expect, it } from 'vitest'
import type { ApprovalAssignment, ApprovalRequest } from '../api/approval'
import {
  APPROVER_KINDS, amountOfRequest, assignmentPill, auditLine, chainOf, entityLabel, parseFormData, parseSnapshot,
  requestPill, stepOf,
} from './approval-model'

const SNAPSHOT = JSON.stringify({
  steps: [
    { step_order: 1, name: 'Trưởng phòng', approver_type: 'specific_user', approver_value: 'n-duc' },
    { step_order: 2, name: 'Kế toán trưởng', approver_type: 'specific_user', approver_value: 'n-hoa' },
    { step_order: 3, name: 'Giám đốc', approver_type: 'specific_user', approver_value: 'n-khai' },
  ],
  form_fields: [
    { label: 'Lý do', field_type: 'textarea', required: true, field_order: 1 },
    { label: 'Số tiền', field_type: 'currency', required: true, field_order: 2 },
  ],
})

const req = (over: Partial<ApprovalRequest> = {}): ApprovalRequest => ({
  id: 'r1', entity_type: 'expense', entity_id: 'e1', template_id: 't1', template_name: 'Tạm ứng công tác phí',
  form_data_json: JSON.stringify({ 'Lý do': 'Công tác', 'Số tiền': '12450000' }), current_step: 2, status: 'pending',
  scope_oa_id: 's', department_id: 'd', created_by: 'n-yen', created_at: '2026-10-09T01:30:00Z', completed_at: null,
  template_snapshot: SNAPSHOT, ...over,
})

const asg = (over: Partial<ApprovalAssignment>): ApprovalAssignment => ({
  id: 'a', step_order: 1, step_name: '', user_node_id: 'n-duc', grant_source: 'direct', status: 'pending',
  acted_at: null, comment: '', ...over,
})

describe('snapshot and form', () => {
  it('reads the frozen steps and fields, and survives a damaged snapshot', () => {
    expect(parseSnapshot(SNAPSHOT).steps).toHaveLength(3)
    expect(parseSnapshot('{nope')).toEqual({ steps: [], formFields: [] })
    expect(parseSnapshot(undefined)).toEqual({ steps: [], formFields: [] })
    expect(parseFormData('{"a":"b"}')).toEqual({ a: 'b' })
    expect(parseFormData('oops')).toEqual({})
    expect(parseFormData(null)).toEqual({})
  })

  it('takes the amount from the first currency field, and only then', () => {
    expect(amountOfRequest(req())).toBe(12450000)
    expect(amountOfRequest(req({ form_data_json: '{"Số tiền":""}' }))).toBeNull()
    expect(amountOfRequest(req({ form_data_json: null }))).toBeNull()
    expect(amountOfRequest(req({ template_snapshot: undefined }))).toBeNull()
  })

  it('finds the step a request is on by order', () => {
    expect(stepOf(req())?.name).toBe('Kế toán trưởng')
    expect(stepOf(req({ current_step: 9 }))).toBeUndefined()
  })
})

describe('status pills', () => {
  it('says "Chờ bạn" only when the viewer holds the pending assignment of the current step', () => {
    const mine = asg({ step_order: 2, status: 'pending' })
    expect(requestPill({ request: req(), assignment: mine })).toEqual({ tone: 'wait', label: 'Chờ bạn' })
    // An assignment on a step that is not the current one is not waiting on me.
    expect(requestPill({ request: req(), assignment: asg({ step_order: 3 }) }).label).toBe('Chờ Kế toán trưởng')
  })

  it('names the step a pending request waits on, and always has a text label', () => {
    expect(requestPill({ request: req() })).toEqual({ tone: 'idle', label: 'Chờ Kế toán trưởng' })
    expect(requestPill({ request: req({ template_snapshot: undefined }) }).label).toBe('Đang chờ duyệt')
    expect(requestPill({ request: req({ status: 'approved' }) })).toEqual({ tone: 'ok', label: 'Đã duyệt' })
    expect(requestPill({ request: req({ status: 'rejected' }) })).toEqual({ tone: 'bad', label: 'Trả lại' })
    expect(requestPill({ request: req({ status: 'cancelled' }) })).toEqual({ tone: 'idle', label: 'Đã huỷ' })
  })

  it('labels an assignment by what it means for the chain', () => {
    expect(assignmentPill(asg({ status: 'approved' }), 2, 'pending')).toEqual({ tone: 'ok', label: 'Đã duyệt' })
    expect(assignmentPill(asg({ status: 'rejected' }), 2, 'rejected')).toEqual({ tone: 'bad', label: 'Trả lại' })
    expect(assignmentPill(asg({ status: 'pending', step_order: 2 }), 2, 'pending')).toEqual({ tone: 'wait', label: 'Đang chờ' })
    expect(assignmentPill(asg({ status: 'pending', step_order: 3 }), 2, 'pending')).toEqual({ tone: 'idle', label: 'Chưa tới' })
    expect(assignmentPill(asg({ status: 'skipped' }), 2, 'rejected')).toEqual({ tone: 'idle', label: 'Bỏ qua' })
    expect(assignmentPill(asg({ status: 'revoked' }), 2, 'pending')).toEqual({ tone: 'idle', label: 'Hết quyền duyệt' })
  })
})

describe('chain', () => {
  const names = (c: ReturnType<typeof chainOf>) => c.map((i) => i.name)

  it('lists every step, with the approver where one is assigned and a placeholder where not yet', () => {
    const chain = chainOf({
      request: req(),
      steps: [
        { id: '', step_order: 1, name: 'Trưởng phòng', approver_type: 'specific_user', approver_value: 'n-duc', approver_name: 'Trần Minh Đức', required_count: 1, timeout_hours: 0 },
        { id: '', step_order: 2, name: 'Kế toán trưởng', approver_type: 'specific_user', approver_value: 'n-hoa', approver_name: 'Lê Thị Hoa', required_count: 1, timeout_hours: 0 },
        { id: '', step_order: 3, name: 'Giám đốc', approver_type: 'specific_user', approver_value: 'n-khai', approver_name: 'Đỗ Văn Khải', required_count: 1, timeout_hours: 0 },
      ],
      assignments: [
        asg({ id: 'a1', step_order: 1, user_node_id: 'n-duc', user_name: 'Trần Minh Đức', status: 'approved', acted_at: '2026-10-09T01:41:00Z' }),
        asg({ id: 'a2', step_order: 2, user_node_id: 'n-hoa', user_name: 'Lê Thị Hoa', status: 'pending' }),
      ],
      meNodeId: 'n-hoa',
    })
    expect(names(chain)).toEqual(['Trần Minh Đức', 'Lê Thị Hoa', 'Đỗ Văn Khải'])
    expect(chain.map((i) => i.pill.label)).toEqual(['Đã duyệt', 'Đang chờ', 'Chưa tới'])
    expect(chain.map((i) => i.isMe)).toEqual([false, true, false])
    expect(chain.map((i) => i.stepName)).toEqual(['Trưởng phòng', 'Kế toán trưởng', 'Giám đốc'])
    // The step still to come is a placeholder, not an assignment.
    expect(chain[2]!.assignmentId).toBeUndefined()
  })

  it('falls back to the assignments alone when the steps are unknown', () => {
    const chain = chainOf({
      request: req({ template_snapshot: undefined }), steps: [], meNodeId: 'x',
      assignments: [asg({ user_name: 'Trần Minh Đức', status: 'approved' })],
    })
    expect(names(chain)).toEqual(['Trần Minh Đức'])
  })

  it('never uses an id as a name', () => {
    const chain = chainOf({
      request: req(), meNodeId: 'x', steps: [],
      assignments: [asg({ user_node_id: '9f0c1d2e-0000-4000-8000-00000000abcd', user_name: undefined })],
    })
    expect(chain[0]!.name).toBe('Người duyệt')
  })

  it('shows a role assignment by the role name the server gave, as is', () => {
    const chain = chainOf({
      request: req(), meNodeId: 'x', steps: [],
      assignments: [asg({ user_node_id: 'ua-1', user_name: 'Kế toán trưởng', grant_source: 'role:ua-1' })],
    })
    expect(chain[0]!.name).toBe('Kế toán trưởng')
  })

  describe('through a role', () => {
    const group = asg({ id: 'g', step_order: 2, user_node_id: 'ua-1', user_name: 'Kế toán trưởng', grant_source: 'role:ua-1', status: 'pending' })
    const person = asg({ id: 'p', step_order: 2, user_node_id: 'n-hoa', user_name: 'Lê Thị Hoa', grant_source: 'role:ua-1', status: 'approved', acted_at: '2026-10-09T02:00:00Z' })

    it('lists the group while it is awaited and the people who acted for it, marking my turn from the server', () => {
      const chain = chainOf({ request: req({ current_step: 2 }), steps: [], assignments: [group, person], meNodeId: 'n-bob', canAct: true })
      expect(names(chain)).toEqual(['Kế toán trưởng', 'Lê Thị Hoa'])
      expect(chain.map((i) => i.pill.label)).toEqual(['Đang chờ', 'Đã duyệt'])
      expect(chain.map((i) => i.isMe)).toEqual([true, false])
    })

    it('is not mine when the server says it is not my turn', () => {
      const chain = chainOf({ request: req({ current_step: 2 }), steps: [], assignments: [group], meNodeId: 'n-dave', canAct: false })
      expect(chain[0]!.isMe).toBe(false)
    })

    it('drops the group row once the step is done, leaving the people who acted', () => {
      const chain = chainOf({
        request: req({ current_step: 3 }), steps: [], meNodeId: 'x',
        assignments: [{ ...group, status: 'skipped' }, person],
      })
      expect(names(chain)).toEqual(['Lê Thị Hoa'])
    })

    it('keeps a skipped group row when nobody acted for it', () => {
      const chain = chainOf({ request: req({ status: 'rejected' }), steps: [], meNodeId: 'x', assignments: [{ ...group, status: 'skipped' }] })
      expect(names(chain)).toEqual(['Kế toán trưởng'])
    })
  })
})

describe('labels', () => {
  it('turns audit codes into sentences and gives system entries no actor', () => {
    expect(auditLine({ action: 'created', detail_json: '{}' })).toEqual({ text: 'đã tạo đề nghị', system: false })
    expect(auditLine({ action: 'approved', detail_json: '{"comment":"OK"}' })).toEqual({ text: 'đã duyệt', system: false, comment: 'OK' })
    expect(auditLine({ action: 'rejected', detail_json: '{"comment":"Thiếu hoá đơn"}' }).comment).toBe('Thiếu hoá đơn')
    expect(auditLine({ action: 'step_advanced', detail_json: '{"from_step":"1"}' })).toEqual({ text: 'Chuyển sang bước tiếp theo', system: true })
    expect(auditLine({ action: 'completed', detail_json: '{"final_status":"approved"}' })).toEqual({ text: 'Đề nghị đã được duyệt', system: true })
    expect(auditLine({ action: 'completed', detail_json: '{"final_status":"rejected"}' }).text).toBe('Đề nghị đã bị trả lại')
    expect(auditLine({ action: 'assigned', detail_json: '{"grant_source":"direct"}' }).text).toBe('được giao duyệt')
    // An unknown code reads as a sentence, never as the code.
    expect(auditLine({ action: 'weird_internal_code', detail_json: '' }).text).toBe('đã cập nhật đề nghị')
  })

  it('names entity types, and has a word for the unknown', () => {
    expect(entityLabel('expense')).toBe('Chi phí')
    expect(entityLabel('asset_request_x')).toBe('Khác')
  })

  it('offers only approver kinds the server can resolve', () => {
    expect(APPROVER_KINDS.map((k) => k.type)).toEqual(['specific_user', 'role_in_dept', 'department'])
  })
})
