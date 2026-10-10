/**
 * Fixture data and an in-memory stand-in for the asset service. Every id is a
 * real-looking UUID so tests can assert that none reaches the screen. Shapes
 * follow what the REST layer returns (proto JSON: empty fields omitted, names
 * beside ids). The stand-in keeps state, so a test can approve a request and
 * see the asset it handed over move.
 */
import type {
  ActivityEntry, Asset, AssetRequest, AssetType, HistoryRecord, Transition,
} from '../api/assets'
import { CONTACTS, U, WS_ID } from './chat-fixtures'

export { UUID_RE, U, WS_ID } from './chat-fixtures'

export const T = {
  laptop: '99999999-aaaa-4bbb-8ccc-000000000001',
  monitor: '99999999-aaaa-4bbb-8ccc-000000000002',
  license: '99999999-aaaa-4bbb-8ccc-000000000003',
  printer: '99999999-aaaa-4bbb-8ccc-000000000004',
}
export const A = {
  mac7: '88888888-aaaa-4bbb-8ccc-000000000007',
  mac4: '88888888-aaaa-4bbb-8ccc-000000000004',
  mac5: '88888888-aaaa-4bbb-8ccc-000000000005',
  dell12: '88888888-aaaa-4bbb-8ccc-000000000012',
  printer: '88888888-aaaa-4bbb-8ccc-000000000030',
  iphone: '88888888-aaaa-4bbb-8ccc-000000000040',
  chair: '88888888-aaaa-4bbb-8ccc-000000000050',
  m365: '88888888-aaaa-4bbb-8ccc-000000000060',
  dell13: '88888888-aaaa-4bbb-8ccc-000000000013',
}
export const R = {
  laptop: '55555555-aaaa-4bbb-8ccc-000000000001',
  monitor: '55555555-aaaa-4bbb-8ccc-000000000002',
  approved: '55555555-aaaa-4bbb-8ccc-000000000003',
  fulfilled: '55555555-aaaa-4bbb-8ccc-000000000004',
  rejected: '55555555-aaaa-4bbb-8ccc-000000000005',
  mine: '55555555-aaaa-4bbb-8ccc-000000000006',
}

const ts = (day: number, h: number, m: number) => ({ seconds: Math.floor(new Date(2026, 9, day, h, m).getTime() / 1000), nanos: 0 })
const name = (key: keyof typeof U) => CONTACTS.find((c) => c.user_id === U[key].id)!.display_name

const LAPTOP_SCHEMA = JSON.stringify({
  type: 'object',
  properties: {
    cfg: { title: 'Cấu hình', type: 'string', 'x-kind': 'text' },
    exp: { title: 'Hết bảo hành', type: 'string', 'x-kind': 'date' },
    owner: { title: 'Người phụ trách', type: 'string', 'x-kind': 'person' },
  },
  required: ['cfg'],
})

const LIFECYCLE = {
  states: ['requested', 'available', 'assigned', 'maintenance', 'retired', 'disposed'],
  initial_state: 'requested',
  transitions: [
    { from_state: 'requested', to_state: 'available', operation: 'approve', ngac_permission: 'approve' },
    { from_state: 'available', to_state: 'assigned', operation: 'assign', ngac_permission: 'manage' },
    { from_state: 'assigned', to_state: 'available', operation: 'return', ngac_permission: 'manage' },
    { from_state: 'assigned', to_state: 'maintenance', operation: 'flag_maintenance', ngac_permission: 'manage' },
    { from_state: 'available', to_state: 'maintenance', operation: 'flag_maintenance', ngac_permission: 'manage' },
    { from_state: 'maintenance', to_state: 'available', operation: 'complete_maintenance', ngac_permission: 'manage' },
    { from_state: 'available', to_state: 'retired', operation: 'retire', ngac_permission: 'manage' },
    { from_state: 'maintenance', to_state: 'retired', operation: 'retire', ngac_permission: 'manage' },
    { from_state: 'retired', to_state: 'disposed', operation: 'dispose', ngac_permission: 'manage' },
  ],
}

type TypeRow = AssetType & { lifecycle: typeof LIFECYCLE }
const typeRow = (id: string, nm: string, category: string, permissions: string[], schema = '{}'): TypeRow => ({
  id, name: nm, category, permissions, fields_schema: schema, lifecycle: LIFECYCLE,
})
const ALL = ['read', 'write', 'approve', 'manage']

export const TYPES: Record<string, TypeRow> = {}
export const ASSETS: Record<string, Asset> = {}
export const REQUESTS: Record<string, AssetRequest> = {}
export const HISTORY: Record<string, HistoryRecord[]> = {}
export const ACTIVITY: ActivityEntry[] = []

const asset = (id: string, nm: string, typeId: string, state: string, holder?: keyof typeof U, fields?: Record<string, unknown>): Asset => ({
  id, name: nm, type_id: typeId, type_name: TYPES[typeId]!.name, state,
  ...(holder ? { assigned_to_user_id: U[holder].id, assigned_to_name: name(holder) } : {}),
  ...(fields ? { custom_fields: fields } : {}),
  created_at: ts(1, 9, 0), updated_at: ts(8, 10, 2),
})

const request = (id: string, typeId: string, by: keyof typeof U, status: string, over: Partial<AssetRequest> = {}): AssetRequest => ({
  id, type_id: typeId, type_name: TYPES[typeId]!.name, requester_id: U[by].id, requester_name: name(by), status,
  justification: 'Cần máy cài sẵn phần mềm đối chiếu và VPN trước ngày đầu.', quantity: 1, urgency: 'normal',
  created_at: ts(10, 8, 52), updated_at: ts(10, 8, 52), ...over,
})

export interface Mode {
  /** The caller holds nothing: no manage on types, no approve, no manage on assets. */
  member: boolean
  empty: boolean
  listError: boolean
  hang: boolean
  /** The next mutation fails with this status. */
  mutationStatus: number | null
  /** ...and this machine-readable reason. */
  mutationReason: string | null
  /** Opening one asset or request answers 403. */
  detail403: boolean
  historyError: boolean
}
const DEFAULT_MODE: Mode = { member: false, empty: false, listError: false, hang: false, mutationStatus: null, mutationReason: null, detail403: false, historyError: false }
export const mode: Mode = { ...DEFAULT_MODE }

export interface ApiCall { method: string; path: string; query: URLSearchParams; body?: Record<string, unknown> }
export const calls: ApiCall[] = []

/** Rebuilds every table to its starting state; call from `beforeEach`. */
export function resetFixtures() {
  Object.assign(mode, DEFAULT_MODE)
  calls.length = 0
  for (const t of [TYPES, ASSETS, REQUESTS, HISTORY]) for (const k of Object.keys(t)) delete (t as Record<string, unknown>)[k]
  ACTIVITY.length = 0

  Object.assign(TYPES, {
    [T.laptop]: typeRow(T.laptop, 'Laptop', 'hardware', ALL, LAPTOP_SCHEMA),
    [T.monitor]: typeRow(T.monitor, 'Màn hình', 'hardware', ['read', 'write', 'approve']),
    [T.license]: typeRow(T.license, 'Giấy phép phần mềm', 'license', ['read', 'write']),
    [T.printer]: typeRow(T.printer, 'Máy in', 'hardware', ['read']),
  })
  const put = (a: Asset) => { ASSETS[a.id] = a }
  put(asset(A.mac7, 'MacBook Pro 14 inch, máy số 7', T.laptop, 'assigned', 'lan', { cfg: 'M3 Pro, 18 GB, 512 GB', exp: '2027-03-14', owner: U.lan.id }))
  put(asset(A.mac4, 'MacBook Pro 14 inch, máy số 4', T.laptop, 'available', undefined, { cfg: 'M3 Pro, 18 GB' }))
  put(asset(A.mac5, 'MacBook Air 13 inch, máy số 5', T.laptop, 'available', undefined, { cfg: 'M2, 8 GB' }))
  put(asset(A.dell12, 'Màn hình Dell 27 inch, bàn 12', T.monitor, 'assigned', 'vinh'))
  put(asset(A.dell13, 'Màn hình Dell 24 inch, kho', T.monitor, 'available'))
  put(asset(A.printer, 'Máy in HP LaserJet tầng 3', T.printer, 'available'))
  put(asset(A.iphone, 'iPhone 15 kiểm thử OTP', T.laptop, 'maintenance', 'yen', { cfg: 'iPhone' }))
  put(asset(A.chair, 'Ghế công thái học, phòng họp nhỏ', T.printer, 'retired'))
  put(asset(A.m365, 'Microsoft 365 E3, gói 25 người', T.license, 'available'))

  Object.assign(REQUESTS, {
    [R.laptop]: request(R.laptop, T.laptop, 'yen', 'pending', { urgency: 'urgent', can_decide: true, can_assign: true }),
    [R.monitor]: request(R.monitor, T.monitor, 'vinh', 'pending', {
      justification: 'Màn hình thứ hai để đối chiếu sao kê.', created_at: ts(9, 15, 0), can_decide: true,
    }),
    [R.approved]: request(R.approved, T.license, 'ngoc', 'approved', {
      justification: 'Thêm 2 suất Microsoft 365 cho cộng tác viên.', urgency: 'high', approver_id: U.hoa.id, approver_name: name('hoa'),
      created_at: ts(7, 9, 0), can_assign: true,
    }),
    [R.fulfilled]: request(R.fulfilled, T.laptop, 'lan', 'fulfilled', {
      approver_id: U.hoa.id, approver_name: name('hoa'), assigned_asset_id: A.mac7, assigned_asset_name: ASSETS[A.mac7]!.name, created_at: ts(2, 9, 0),
    }),
    [R.rejected]: request(R.rejected, T.monitor, 'duc', 'rejected', {
      approver_id: U.hoa.id, approver_name: name('hoa'), approver_comment: 'Kho còn máy đời cũ đủ dùng.', created_at: ts(3, 9, 0),
    }),
    [R.mine]: request(R.mine, T.printer, 'hoa', 'pending', { justification: 'Máy in tầng 3 hết mực.', created_at: ts(6, 9, 0) }),
  })

  const rec = (id: string, action: string, from: string, to: string, actor: keyof typeof U, over: Partial<HistoryRecord> = {}): HistoryRecord => ({
    id, action, from_state: from, to_state: to, actor_id: U[actor].id, actor_name: name(actor), created_at: ts(8, 10, 2), ...over,
  })
  HISTORY[A.mac7] = [
    rec('h-1', 'approve', 'requested', 'available', 'duc', { created_at: ts(1, 9, 5) }),
    rec('h-2', 'assign', 'available', 'assigned', 'duc', { subject_user_id: U.vinh.id, subject_name: name('vinh'), created_at: ts(2, 9, 0) }),
    rec('h-3', 'return', 'assigned', 'available', 'duc', { subject_user_id: U.vinh.id, subject_name: name('vinh'), created_at: ts(6, 17, 30) }),
    rec('h-4', 'assign', 'available', 'assigned', 'hoa', { subject_user_id: U.lan.id, subject_name: name('lan'), comment: 'Thay máy cũ hỏng bản lề', created_at: ts(8, 10, 2) }),
  ]
  ACTIVITY.push(
    { ...rec('act-0', 'request_rejected', '', '', 'hoa', { subject_user_id: U.duc.id, subject_name: name('duc'), created_at: ts(10, 9, 50) }), asset_id: '', asset_name: 'Màn hình', type_name: 'Màn hình', request_status: 'rejected' },
    { ...rec('act-1', 'return', 'assigned', 'available', 'duc', { subject_user_id: U.vinh.id, subject_name: name('vinh'), created_at: ts(10, 9, 20) }), asset_id: A.mac4, asset_name: ASSETS[A.mac4]!.name, type_name: 'Laptop' },
    { ...rec('act-2', 'assign', 'available', 'assigned', 'hoa', { subject_user_id: U.lan.id, subject_name: name('lan'), created_at: ts(10, 8, 47) }), asset_id: A.mac7, asset_name: ASSETS[A.mac7]!.name, type_name: 'Laptop' },
    { ...rec('act-3', 'flag_maintenance', 'available', 'maintenance', 'ngoc', { created_at: ts(9, 16, 10) }), asset_id: A.printer, asset_name: ASSETS[A.printer]!.name, type_name: 'Máy in' },
  )
}
resetFixtures()

const fail = (message: string, status: number, reason?: string): Promise<never> =>
  import('../api/client').then(({ ApiError }) => { throw new ApiError(message, status, reason ? { message, reason } : { message }) })

/** What the caller holds on a type: nothing, when `mode.member`. */
const held = (t: TypeRow): string[] => (mode.member ? [] : t.permissions ?? [])
const mayDo = (typeId: string, op: string) => held(TYPES[typeId]!).includes(op)

function transitionsFor(a: Asset): Transition[] {
  return LIFECYCLE.transitions
    .filter((t) => t.from_state === a.state && mayDo(a.type_id, t.ngac_permission))
    .map((t) => ({ action: t.operation, to_state: t.to_state, ngac_permission: t.ngac_permission }))
}

const visibleAssets = () => Object.values(ASSETS).filter((a) => mayDo(a.type_id, 'read'))

function stamp(a: Asset) { a.updated_at = ts(10, 11, 30) }

/** Stand-in for `apiFetch` on the asset screens. Unknown paths reject so a stray call is loud. */
export function assetFixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const [p, qs = ''] = path.split('?') as [string, string?]
  const query = new URLSearchParams(qs)
  const body = typeof init?.body === 'string' ? (JSON.parse(init.body) as Record<string, unknown>) : undefined
  calls.push({ method, path: p, query, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (p === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
  if (p === `/workspaces/${WS_ID}/contacts`) return ok({ contacts: CONTACTS, total: CONTACTS.length })

  if (method !== 'GET' && mode.mutationStatus) {
    const status = mode.mutationStatus
    const reason = mode.mutationReason ?? undefined
    mode.mutationStatus = null
    mode.mutationReason = null
    return fail('boom', status, reason)
  }
  const isList = method === 'GET' && /^\/workspaces\/[^/]+\/(assets|asset-requests|asset-types)(\/summary|\/activity)?$/.test(p)
  if (isList) {
    if (mode.hang) return new Promise(() => {})
    if (mode.listError) return fail('boom', 500)
  }

  // --- types ---
  if (p === `/workspaces/${WS_ID}/asset-types` && method === 'GET') {
    if (mode.empty) return ok({ can_manage: !mode.member })
    const types = Object.values(TYPES)
      .filter((t) => held(t).length > 0)
      .map((t) => ({
        ...t,
        permissions: held(t),
        asset_count: Object.values(ASSETS).filter((a) => a.type_id === t.id).length,
        available_count: Object.values(ASSETS).filter((a) => a.type_id === t.id && a.state === 'available').length,
      }))
    return ok({ types, can_manage: !mode.member })
  }
  if (p === `/workspaces/${WS_ID}/asset-types` && method === 'POST') {
    const id = '99999999-aaaa-4bbb-8ccc-0000000000ff'
    TYPES[id] = typeRow(id, String(body?.name), String(body?.category), ALL)
    return ok(TYPES[id])
  }
  let m = /^\/asset-types\/([^/]+)\/schema$/.exec(p)
  if (m && method === 'PUT') {
    TYPES[m[1]!]!.fields_schema = String(body?.fields_schema)
    return ok(TYPES[m[1]!])
  }

  // --- assets ---
  if (p === `/workspaces/${WS_ID}/assets/summary`) {
    if (mode.empty) return ok({})
    const list = visibleAssets()
    const by_state: Record<string, number> = {}
    const byType = new Map<string, number>()
    for (const a of list) {
      by_state[a.state] = (by_state[a.state] ?? 0) + 1
      byType.set(a.type_id, (byType.get(a.type_id) ?? 0) + 1)
    }
    return ok({
      total: list.length, by_state,
      by_type: [...byType].map(([id, count]) => ({ type_id: id, type_name: TYPES[id]!.name, count })).sort((a, b) => b.count - a.count),
      holders: new Set(list.filter((a) => a.state === 'assigned').map((a) => a.assigned_to_user_id).filter(Boolean)).size,
      maintenance_overdue: list.filter((a) => a.state === 'maintenance').length,
    })
  }
  if (p === `/workspaces/${WS_ID}/assets/activity`) {
    if (mode.empty) return ok({})
    return ok({ entries: ACTIVITY.slice(0, Number(query.get('limit') || 10)) })
  }
  if (p === `/workspaces/${WS_ID}/assets` && method === 'GET') {
    if (mode.empty) return ok({})
    let list = visibleAssets()
    if (query.get('type_id')) list = list.filter((a) => a.type_id === query.get('type_id'))
    if (query.get('state')) list = list.filter((a) => a.state === query.get('state'))
    const q = (query.get('search') ?? '').toLowerCase()
    if (q) list = list.filter((a) => a.name.toLowerCase().includes(q) || (a.assigned_to_name ?? '').toLowerCase().includes(q))
    const limit = Number(query.get('limit') || 25)
    const offset = Number(query.get('offset') || 0)
    return ok({ assets: list.slice(offset, offset + limit), total: list.length })
  }
  if (p === `/workspaces/${WS_ID}/assets` && method === 'POST') {
    const id = '88888888-aaaa-4bbb-8ccc-0000000000ff'
    ASSETS[id] = { id, name: String(body?.name), type_id: String(body?.type_id), type_name: TYPES[String(body?.type_id)]?.name, state: 'requested', custom_fields: body?.custom_fields as Record<string, unknown> }
    return ok(ASSETS[id])
  }

  m = /^\/assets\/([^/]+)$/.exec(p)
  if (m && method === 'GET') {
    if (mode.detail403) return fail('forbidden', 403)
    return ASSETS[m[1]!] ? ok(ASSETS[m[1]!]) : fail('not found', 404)
  }
  m = /^\/assets\/([^/]+)\/transitions$/.exec(p)
  if (m) {
    const a = ASSETS[m[1]!]!
    return ok({ transitions: transitionsFor(a), current_state: a.state, can_assign: mayDo(a.type_id, 'manage') && ['available', 'assigned'].includes(a.state) })
  }
  m = /^\/assets\/([^/]+)\/history$/.exec(p)
  if (m) {
    if (mode.historyError) return fail('forbidden', 403)
    return ok({ records: HISTORY[m[1]!] ?? [] })
  }
  m = /^\/assets\/([^/]+)\/transition$/.exec(p)
  if (m && method === 'POST') {
    const a = ASSETS[m[1]!]!
    const t = LIFECYCLE.transitions.find((x) => x.from_state === a.state && x.operation === body?.action)
    if (!t) return fail('no such step', 409, 'state_changed')
    a.state = t.to_state
    if (['available', 'retired', 'disposed'].includes(t.to_state)) { delete a.assigned_to_user_id; delete a.assigned_to_name }
    stamp(a)
    return ok(a)
  }
  m = /^\/assets\/([^/]+)\/assign$/.exec(p)
  if (m && method === 'POST') {
    const a = ASSETS[m[1]!]!
    const who = CONTACTS.find((c) => c.user_id === body?.assignee_id)
    if (!who) return fail('not a member', 400)
    a.state = 'assigned'
    a.assigned_to_user_id = who.user_id
    a.assigned_to_name = who.display_name
    stamp(a)
    return ok(a)
  }

  // --- requests ---
  if (p === `/workspaces/${WS_ID}/asset-requests` && method === 'GET') {
    if (mode.empty) return ok({})
    let list = Object.values(REQUESTS)
    if (query.get('status')) list = list.filter((r) => query.get('status')!.split(',').includes(r.status))
    if (query.get('mine') === 'true') list = list.filter((r) => r.requester_id === U.hoa.id)
    if (mode.member) list = list.filter((r) => r.requester_id === U.hoa.id)
    list = list.map((r) => ({ ...r, can_decide: !mode.member && r.can_decide, can_assign: !mode.member && r.can_assign }))
    return ok({ requests: list, total: list.length })
  }
  if (p === `/workspaces/${WS_ID}/asset-requests` && method === 'POST') {
    const id = '55555555-aaaa-4bbb-8ccc-0000000000ff'
    REQUESTS[id] = request(id, String(body?.type_id), 'hoa', 'pending', { justification: String(body?.reason), urgency: String(body?.urgency), created_at: ts(10, 11, 0) })
    return ok(REQUESTS[id])
  }
  m = /^\/asset-requests\/([^/]+)$/.exec(p)
  if (m && method === 'GET') {
    if (mode.detail403) return fail('forbidden', 403)
    return REQUESTS[m[1]!] ? ok(REQUESTS[m[1]!]) : fail('not found', 404)
  }
  m = /^\/asset-requests\/([^/]+)\/(approve|reject|assign)$/.exec(p)
  if (m && method === 'POST') {
    const r = REQUESTS[m[1]!]!
    const verb = m[2]
    if (verb !== 'assign' && r.status !== 'pending') return fail('decided', 409, 'request_not_open')
    if (verb === 'reject') {
      if (!String(body?.reason ?? '').trim()) return fail('reason', 400)
      r.status = 'rejected'
      r.approver_comment = String(body?.reason)
      return ok(r)
    }
    const assetId = verb === 'assign' ? String(body?.asset_id) : body?.asset_id ? String(body.asset_id) : ''
    if (assetId) {
      const a = ASSETS[assetId]
      if (!a) return fail('not found', 404)
      if (a.type_id !== r.type_id) return fail('wrong type', 400, 'wrong_type')
      if (a.state !== 'available') return fail('taken', 409, 'asset_unavailable')
      a.state = 'assigned'
      a.assigned_to_user_id = r.requester_id
      a.assigned_to_name = r.requester_name
      stamp(a)
      r.status = 'fulfilled'
      r.assigned_asset_id = a.id
      r.assigned_asset_name = a.name
    } else {
      r.status = 'approved'
    }
    r.approver_id = U.hoa.id
    r.approver_name = name('hoa')
    r.can_decide = false
    return ok(r)
  }

  return Promise.reject(new Error(`assetFixtureApi: unexpected ${method} ${path}`))
}
