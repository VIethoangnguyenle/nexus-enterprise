/**
 * Fixture data for the approval screens. Every id is a real-looking UUID so
 * tests can assert that none reaches the screen. Shapes follow what the REST
 * layer returns: names beside ids, empty fields omitted, snapshots as strings.
 */
import type {
  ApprovalAssignment, ApprovalRequest, ApprovalRequestDetail, ApprovalStep, ApprovalTemplate,
  AuditEntry, FormFieldDefinition, RequestWithAssignment,
} from '../api/approval'
import { CONTACTS, U, WS_ID } from './chat-fixtures'

export { UUID_RE, U, WS_ID } from './chat-fixtures'

export const R = {
  tamung: '55555555-aaaa-4bbb-8ccc-000000000001',
  manhinh: '55555555-aaaa-4bbb-8ccc-000000000002',
  sms: '55555555-aaaa-4bbb-8ccc-000000000003',
  kho: '55555555-aaaa-4bbb-8ccc-000000000004',
  vpp: '55555555-aaaa-4bbb-8ccc-000000000005',
  nhom: '55555555-aaaa-4bbb-8ccc-000000000006',
}
export const T = {
  tamung: '99999999-aaaa-4bbb-8ccc-000000000001',
  muasam: '99999999-aaaa-4bbb-8ccc-000000000002',
  cu: '99999999-aaaa-4bbb-8ccc-000000000003',
}
/** A role the signed-in user belongs to. */
export const ROLE_UA = '33333333-aaaa-4bbb-8ccc-0000000000a1'
const KHAI = { id: '11111111-aaaa-4bbb-8ccc-0000000000f1', node: '22222222-aaaa-4bbb-8ccc-0000000000f1' }
const ROLE_NODE = ROLE_UA
const DEPT_ID = '44444444-aaaa-4bbb-8ccc-0000000000d1'

export const ROLES = [
  { id: ROLE_NODE, name: 'Kế toán trưởng', ngac_node_id: ROLE_NODE },
  { id: '33333333-aaaa-4bbb-8ccc-0000000000a2', name: 'Members', ngac_node_id: '33333333-aaaa-4bbb-8ccc-0000000000a2' },
]
export const DEPARTMENTS = [
  { id: DEPT_ID, name: 'Vận hành thanh toán', parent_id: '', member_count: 6 },
  { id: '44444444-aaaa-4bbb-8ccc-0000000000d2', name: 'Đối soát', parent_id: '', member_count: 4 },
]

const iso = (day: number, h: number, m: number) => new Date(2026, 9, day, h, m).toISOString()

const FIELDS: FormFieldDefinition[] = [
  { label: 'Lý do', field_type: 'textarea', required: true, field_order: 1, placeholder: 'Mục đích của khoản tạm ứng' },
  { label: 'Số tiền', field_type: 'currency', required: true, field_order: 2 },
  { label: 'Ngày cần', field_type: 'date', required: false, field_order: 3 },
]

const step = (order: number, name: string, who: { node: string }, label: string, type = 'specific_user'): ApprovalStep => ({
  id: `66666666-aaaa-4bbb-8ccc-00000000000${order}`, step_order: order, name, approver_type: type,
  approver_value: who.node, approver_name: label, required_count: 1, timeout_hours: 0,
})

const STEPS_TAMUNG = [
  step(1, 'Trưởng phòng', U.duc, 'Trần Minh Đức'),
  step(2, 'Kế toán trưởng', U.hoa, 'Lê Thị Hoa'),
  step(3, 'Giám đốc khối', KHAI, 'Đỗ Văn Khải'),
]
const STEPS_ONE = [step(1, 'Kế toán trưởng', U.hoa, 'Lê Thị Hoa')]

const snapshot = (steps: ApprovalStep[]) => JSON.stringify({ steps, form_fields: FIELDS })

const request = (id: string, over: Partial<ApprovalRequest>): ApprovalRequest => ({
  id, entity_type: 'expense', entity_id: '77777777-aaaa-4bbb-8ccc-000000000001', template_id: T.tamung,
  template_name: 'Tạm ứng', form_data_json: null, current_step: 1, status: 'pending', scope_oa_id: 'default',
  department_id: DEPT_ID, department_name: 'Vận hành thanh toán', created_by: U.yen.node,
  created_by_name: 'Phạm Hải Yến', created_at: iso(9, 8, 30), completed_at: null,
  template_snapshot: snapshot(STEPS_ONE), ...over,
})

const form = (reason: string, amount: string) => JSON.stringify({ 'Lý do': reason, 'Số tiền': amount })

export const REQUESTS: Record<string, ApprovalRequest> = {
  [R.tamung]: request(R.tamung, {
    template_name: 'Tạm ứng công tác phí tháng 10', current_step: 2, template_snapshot: snapshot(STEPS_TAMUNG),
    form_data_json: form('Công tác Đà Nẵng 14 đến 17/10 để làm việc với ngân hàng đối tác về quy trình đối soát.', '12450000'),
  }),
  [R.manhinh]: request(R.manhinh, {
    template_name: 'Mua 4 màn hình cho tổ đối soát', created_by: U.vinh.node, created_by_name: 'Lê Quang Vinh',
    created_at: iso(8, 15, 5), form_data_json: form('Thay màn hình cũ của tổ đối soát.', '18960000'),
  }),
  [R.sms]: request(R.sms, {
    template_name: 'Thanh toán phí SMS OTP quý III', created_by: U.lan.node, created_by_name: 'Nguyễn Thu Lan',
    created_at: iso(7, 9, 0), current_step: 3, template_snapshot: snapshot(STEPS_TAMUNG),
    form_data_json: form('Phí SMS OTP quý III.', '46218300'),
  }),
  [R.kho]: request(R.kho, {
    template_name: 'Gia hạn hợp đồng thuê kho Long Biên', created_by: KHAI.node, created_by_name: 'Đỗ Văn Khải',
    created_at: iso(5, 10, 0), status: 'approved', completed_at: iso(6, 9, 0),
    form_data_json: form('Gia hạn thêm 12 tháng.', '132000000'),
  }),
  // Given to a role, not a person: the user acts through it.
  [R.nhom]: request(R.nhom, {
    template_name: 'Mua vật tư cho tổ kế toán', created_by: U.yen.node, created_by_name: 'Phạm Hải Yến',
    created_at: iso(9, 14, 0), form_data_json: form('Vật tư quý IV.', '2300000'),
    template_snapshot: snapshot([{ ...STEPS_ONE[0]!, approver_type: 'role_in_dept', approver_value: ROLE_UA, approver_name: 'Kế toán trưởng' }]),
  }),
  [R.vpp]: request(R.vpp, {
    template_name: 'Mua văn phòng phẩm', created_by: U.hoa.node, created_by_name: 'Lê Thị Hoa',
    created_at: iso(10, 8, 0), template_snapshot: snapshot(STEPS_ONE), form_data_json: form('Giấy in và bút.', '1200000'),
  }),
}

const assignment = (id: string, over: Partial<ApprovalAssignment>): ApprovalAssignment => ({
  id, step_order: 1, step_name: '', user_node_id: U.hoa.node, user_name: 'Lê Thị Hoa', grant_source: 'direct',
  status: 'pending', acted_at: null, comment: '', ...over,
})

export const ASSIGNMENTS: Record<string, ApprovalAssignment[]> = {
  [R.tamung]: [
    assignment('88888888-aaaa-4bbb-8ccc-000000000001', { step_order: 1, user_node_id: U.duc.node, user_name: 'Trần Minh Đức', status: 'approved', acted_at: iso(9, 8, 41) }),
    assignment('88888888-aaaa-4bbb-8ccc-000000000002', { step_order: 2 }),
  ],
  [R.manhinh]: [assignment('88888888-aaaa-4bbb-8ccc-000000000003', { step_order: 1 })],
  [R.sms]: [
    assignment('88888888-aaaa-4bbb-8ccc-000000000004', { step_order: 1, user_node_id: U.duc.node, user_name: 'Trần Minh Đức', status: 'approved', acted_at: iso(7, 9, 30) }),
    assignment('88888888-aaaa-4bbb-8ccc-000000000005', { step_order: 2, status: 'approved', acted_at: iso(7, 11, 0) }),
    assignment('88888888-aaaa-4bbb-8ccc-000000000006', { step_order: 3, user_node_id: KHAI.node, user_name: 'Đỗ Văn Khải' }),
  ],
  [R.kho]: [assignment('88888888-aaaa-4bbb-8ccc-000000000007', { step_order: 1, status: 'approved', acted_at: iso(6, 9, 0), comment: 'Đồng ý gia hạn.' })],
  [R.vpp]: [assignment('88888888-aaaa-4bbb-8ccc-000000000008', { step_order: 1 })],
  [R.nhom]: [assignment('88888888-aaaa-4bbb-8ccc-000000000009', {
    step_order: 1, user_node_id: ROLE_UA, user_name: 'Kế toán trưởng', grant_source: `role:${ROLE_UA}`,
  })],
}

const entry = (id: string, requestId: string, action: string, actor: { node: string } | null, name: string | undefined, at: string, detail = '{}', step = 0): AuditEntry => ({
  id, request_id: requestId, action, actor_node_id: actor?.node ?? '', ...(name ? { actor_name: name } : {}),
  step_order: step, detail_json: detail, ip_address: '', created_at: at,
})

export const AUDIT: Record<string, AuditEntry[]> = {
  [R.tamung]: [
    entry('aaaaaaaa-0000-4000-8000-000000000001', R.tamung, 'created', U.yen, 'Phạm Hải Yến', iso(9, 8, 30)),
    entry('aaaaaaaa-0000-4000-8000-000000000002', R.tamung, 'approved', U.duc, 'Trần Minh Đức', iso(9, 8, 41), '{"comment":"Đồng ý."}', 1),
    entry('aaaaaaaa-0000-4000-8000-000000000003', R.tamung, 'step_advanced', null, undefined, iso(9, 8, 41), '{"from_step":"1"}', 1),
  ],
}

const toRwa = (id: string): RequestWithAssignment => ({
  request: REQUESTS[id]!, assignment: ASSIGNMENTS[id]!.find((a) => (a.user_node_id === U.hoa.node || a.user_node_id === ROLE_UA) && a.status === 'pending')
    ?? ASSIGNMENTS[id]![0]!,
})

export const TEMPLATES: Record<string, ApprovalTemplate> = {
  [T.tamung]: {
    id: T.tamung, name: 'Tạm ứng', entity_type: 'expense', is_active: true, priority: 5, form_fields: FIELDS, conditions: [],
    steps: STEPS_TAMUNG, step_count: 3, condition_count: 0, created_by: U.hoa.node, created_by_name: 'Lê Thị Hoa',
    created_at: iso(1, 9, 0), updated_at: iso(1, 9, 0),
  },
  [T.muasam]: {
    id: T.muasam, name: 'Mua sắm thiết bị', entity_type: 'purchase', is_active: true, priority: 3, form_fields: FIELDS.slice(0, 2),
    conditions: [], steps: STEPS_ONE.map((s) => ({ ...s, approver_type: 'role_in_dept', approver_value: ROLE_NODE, approver_name: 'Kế toán trưởng' })),
    step_count: 1, condition_count: 0, created_by: U.hoa.node, created_by_name: 'Lê Thị Hoa', created_at: iso(2, 9, 0), updated_at: iso(2, 9, 0),
  },
  [T.cu]: {
    id: T.cu, name: 'Mẫu cũ', entity_type: 'custom', is_active: false, priority: 0, form_fields: null, conditions: [],
    steps: STEPS_ONE, step_count: 1, condition_count: 0, created_by: U.hoa.node, created_by_name: 'Lê Thị Hoa',
    created_at: iso(3, 9, 0), updated_at: iso(3, 9, 0),
  },
}

export interface Mode {
  /** The audit endpoint answers 403. */
  audit403: boolean
  /** Every list answers 500. */
  listError: boolean
  /** Lists are empty. */
  empty: boolean
  /** Lists never answer (loading state). */
  hang: boolean
  /** Approve/reject/batch answer 500. */
  mutationError: boolean
  /** The signed-in user may not manage templates. */
  member: boolean
  /** Saving a template answers 409. */
  stale: boolean
  /** One more request is waiting on a role the user belongs to. */
  group: boolean
}
const DEFAULT_MODE: Mode = { audit403: false, listError: false, empty: false, hang: false, mutationError: false, member: false, stale: false, group: false }
export const mode: Mode = { ...DEFAULT_MODE }

const gone = new Set<string>()
export const resetFixtures = () => {
  Object.assign(mode, DEFAULT_MODE)
  gone.clear()
  calls.length = 0
}

export interface ApiCall { method: string; path: string; body?: Record<string, unknown> }
export const calls: ApiCall[] = []

const fail = (message: string, status: number): Promise<never> =>
  import('../api/client').then(({ ApiError }) => { throw new ApiError(message, status) })

const PENDING_IDS = () => [R.tamung, R.manhinh, ...(mode.group ? [R.nhom] : [])].filter((id) => !gone.has(id))

/** Stand-in for `apiFetch` on the approval screens. Unknown paths reject so a stray call is loud. */
export function approvalFixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const [p, query = ''] = path.split('?') as [string, string?]
  const body = typeof init?.body === 'string' ? (JSON.parse(init.body) as Record<string, unknown>) : undefined
  calls.push({ method, path: p, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (p === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
  if (p === `/workspaces/${WS_ID}/contacts`) return ok({ contacts: CONTACTS, total: CONTACTS.length })
  if (p === `/workspaces/${WS_ID}/roles`) return ok({ roles: ROLES })
  if (p === `/workspaces/${WS_ID}/departments`) return ok({ departments: DEPARTMENTS })

  if (p === '/approval/permissions') return ok({ can_manage_templates: !mode.member })

  const isList = ['/approval/pending', '/approval/history', '/approval/my-requests', '/approval/department-requests'].includes(p)
  if (isList && method === 'GET') {
    if (mode.hang) return new Promise(() => {})
    if (mode.listError) return fail('boom', 500)
    if (mode.empty) return ok(p === '/approval/pending' ? { items: [], total: 0 } : { items: [], next_cursor: '' })
  }
  if (p === '/approval/pending') {
    const ids = PENDING_IDS()
    return ok({ items: ids.map(toRwa), total: ids.length })
  }
  if (p === '/approval/history') return ok({ items: [{ request: REQUESTS[R.kho]!, assignment: ASSIGNMENTS[R.kho]![0]! }], next_cursor: '' })
  if (p === '/approval/my-requests') return ok({ items: [REQUESTS[R.vpp]], next_cursor: '' })
  if (p === '/approval/department-requests') {
    const params = new URLSearchParams(query)
    const all = [R.tamung, R.manhinh, R.sms, R.kho].map((id) => REQUESTS[id]!)
    return params.get('cursor')
      ? ok({ items: [REQUESTS[R.vpp]], next_cursor: '' })
      : ok({ items: all.filter((r) => !gone.has(r.id)), next_cursor: '2026-10-05T10:00:00Z' })
  }

  let m = /^\/approval\/requests\/([^/]+)\/audit$/.exec(p)
  if (m) {
    if (mode.audit403) return fail('forbidden', 403)
    return ok({ entries: AUDIT[m[1]!] ?? [] })
  }
  m = /^\/approval\/requests\/([^/]+)$/.exec(p)
  if (m && method === 'GET') {
    const id = m[1]!
    const req = REQUESTS[id]
    if (!req) return fail('forbidden', 403)
    const snap = JSON.parse(req.template_snapshot ?? '{}') as { steps?: ApprovalStep[] }
    const detail: ApprovalRequestDetail = {
      request: { ...req, template_snapshot: undefined },
      steps: snap.steps ?? [], form_fields: FIELDS, assignments: ASSIGNMENTS[id] ?? [],
      can_act: (ASSIGNMENTS[id] ?? []).some((a) => (a.user_node_id === U.hoa.node || a.user_node_id === ROLE_UA) && a.status === 'pending' && a.step_order === req.current_step && req.status === 'pending'),
    }
    return ok(detail)
  }

  if (p === '/approval/templates' && method === 'GET') {
    const all = Object.values(TEMPLATES).map((t) => ({ ...t, steps: null, form_fields: t.form_fields }))
    const params = new URLSearchParams(query)
    return ok({ templates: params.get('active_only') === 'false' ? all : all.filter((t) => t.is_active) })
  }
  m = /^\/approval\/templates\/([^/]+)$/.exec(p)
  if (m && method === 'GET') return TEMPLATES[m[1]!] ? ok(TEMPLATES[m[1]!]) : fail('not found', 404)
  if (p === '/approval/templates' && method === 'POST') return ok({ ...TEMPLATES[T.tamung], id: 'new', name: body?.name })
  if (m && method === 'PUT') return mode.stale ? fail('conflict', 409) : ok(TEMPLATES[m[1]!])

  if (['/approval/approve', '/approval/reject', '/approval/batch-approve'].includes(p)) {
    if (mode.mutationError) return fail('boom', 500)
    if (p === '/approval/batch-approve') {
      const ids = body?.request_ids as string[]
      ids.forEach((id) => gone.add(id))
      return ok({ approved_count: ids.length, approved_ids: ids })
    }
    gone.add(body?.request_id as string)
    return ok({ status: p.endsWith('reject') ? 'rejected' : 'approved' })
  }
  if (p === '/approval/requests' && method === 'POST') {
    return ok({ ...REQUESTS[R.vpp], id: '55555555-aaaa-4bbb-8ccc-0000000000ff' })
  }
  return Promise.reject(new Error(`approvalFixtureApi: unexpected ${method} ${path}`))
}
