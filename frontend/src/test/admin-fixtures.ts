/**
 * Fixture backend for the Quản trị screens: a small in-memory workspace that
 * answers what the workspace REST layer answers (internal/rest/admin_*.go) and
 * changes when the screens write to it. Every id is a real-looking UUID so
 * tests can assert that none reaches the screen.
 */
import type {
  Department, Invitation, Member, PermissionArea, RoleDetail, RoleSummary,
} from '../api/admin'
import { CONTACTS, U, WS_ID } from './chat-fixtures'

export { UUID_RE, U, WS_ID } from './chat-fixtures'

export const D = {
  khoi: '44444444-aaaa-4bbb-8ccc-0000000000d1',
  vanhanh: '44444444-aaaa-4bbb-8ccc-0000000000d2',
  doisoat: '44444444-aaaa-4bbb-8ccc-0000000000d3',
  hotro: '44444444-aaaa-4bbb-8ccc-0000000000d4',
  ketoan: '44444444-aaaa-4bbb-8ccc-0000000000d5',
  kiemsoat: '44444444-aaaa-4bbb-8ccc-0000000000d6',
  hanhchinh: '44444444-aaaa-4bbb-8ccc-0000000000d7',
}

export const RID = {
  owners: '55555555-aaaa-4bbb-8ccc-0000000000a1',
  members: '55555555-aaaa-4bbb-8ccc-0000000000a2',
  ketoan: '55555555-aaaa-4bbb-8ccc-0000000000a3',
  kiemsoat: '55555555-aaaa-4bbb-8ccc-0000000000a4',
  taisan: '55555555-aaaa-4bbb-8ccc-0000000000a5',
}

/** What the server answers for the areas of this workspace: the table the editor must render. */
export const AREAS: PermissionArea[] = [
  { area: 'documents', operations: ['read', 'write', 'share'] },
  { area: 'channels', operations: ['read', 'write', 'manage', 'invite', 'create_channel'] },
  { area: 'assets', operations: ['read', 'write', 'approve', 'manage'] },
  { area: 'management', operations: ['manage', 'invite'] },
]

const dept = (id: string, name: string, parent: string, n: number): Department => ({ id, name, parent_id: parent, member_count: n })

const DEPARTMENTS0: Department[] = [
  dept(D.khoi, 'Khối Vận hành', '', 64),
  dept(D.vanhanh, 'Vận hành thanh toán', D.khoi, 31),
  dept(D.doisoat, 'Đối soát', D.vanhanh, 12),
  dept(D.hotro, 'Hỗ trợ đối tác', D.vanhanh, 9),
  dept(D.ketoan, 'Kế toán', D.khoi, 14),
  dept(D.kiemsoat, 'Kiểm soát nội bộ', D.khoi, 6),
  dept(D.hanhchinh, 'Hành chính', '', 13),
]

const role = (id: string, kind: RoleSummary['kind'], name: string | undefined, n: number): RoleSummary => ({
  id, ngac_node_id: id, kind, member_count: n, ...(name ? { name } : {}),
})

const ROLES0: RoleSummary[] = [
  role(RID.owners, 'owners', undefined, 2),
  role(RID.members, 'members', undefined, 64),
  role(RID.ketoan, 'custom', 'Kế toán', 14),
  role(RID.kiemsoat, 'custom', 'Kiểm soát nội bộ', 6),
  role(RID.taisan, 'custom', 'Quản lý tài sản', 3),
]

type Grants = Record<string, string[]>
const GRANTS0: Record<string, Grants> = {
  [RID.owners]: {
    documents: ['read', 'write', 'share'],
    channels: ['read', 'write', 'manage', 'invite', 'create_channel'],
    assets: ['read', 'write', 'approve', 'manage'],
    management: ['manage', 'invite'],
  },
  [RID.members]: { documents: ['read', 'write', 'share'], channels: ['read', 'write', 'create_channel'] },
  [RID.ketoan]: { documents: ['read', 'write', 'share'], assets: ['read'] },
  [RID.kiemsoat]: { documents: ['read'], assets: ['read', 'approve'] },
  [RID.taisan]: { assets: ['read', 'write', 'manage'] },
}

const person = (
  key: keyof typeof U, status: Member['status'], deptId: string | null, roles: string[], owner = false,
): Member => {
  const c = CONTACTS.find((x) => x.ngac_node_id === U[key].node)!
  const d = DEPARTMENTS0.find((x) => x.id === deptId)
  return {
    ngac_node_id: U[key].node, user_id: U[key].id, display_name: c.display_name, email: c.email,
    avatar_url: '', title: c.title, status, is_owner: owner,
    department: d ? { id: d.id, name: d.name } : null,
    roles: roles.map((id) => ({ id, name: ROLES0.find((r) => r.id === id)!.name! })),
  }
}

const MEMBERS0: Member[] = [
  person('hoa', 'active', D.ketoan, [RID.ketoan], true),
  person('duc', 'active', D.vanhanh, [], true),
  person('lan', 'active', D.doisoat, [RID.ketoan]),
  person('vinh', 'active', D.doisoat, []),
  person('yen', 'active', D.vanhanh, [RID.kiemsoat, RID.taisan, RID.ketoan]),
  person('ngoc', 'disabled', D.hanhchinh, []),
]

const DAY = 24 * 3600 * 1000
const invitation = (id: string, email: string, role: string, dept: string, expiresInMs: number): Invitation => ({
  id, email, inviter_name: 'Lê Thị Hoa', role_name: role, department_name: dept,
  created_at: new Date(Date.now() - DAY).toISOString(), expires_at: new Date(Date.now() + expiresInMs).toISOString(),
})

/** Offers still open: one about to lapse, one fresh. */
const INVITATIONS0: Invitation[] = [
  invitation('77777777-aaaa-4bbb-8ccc-000000000001', 'an.pham@novapay.vn', 'Kế toán', 'Đối soát', 1.5 * DAY),
  invitation('77777777-aaaa-4bbb-8ccc-000000000002', 'minh.ho@novapay.vn', '', '', 6 * DAY),
]

export interface Mode {
  /** Every admin endpoint answers 403: the signed-in user may not manage the workspace. */
  forbidden: boolean
  /** The people list answers 500. */
  membersError: boolean
  /** Nothing to list. */
  empty: boolean
  /** The lists (people, departments, roles) never answer (loading state). */
  hang: boolean
  /** The department and role lists answer 500. */
  listError: boolean
  /** Saving permissions answers this status. */
  saveStatus: number
  /** Inviting answers this status (429: over the hourly budget; 403: not allowed). */
  inviteStatus: number
  /** The invitations list answers 403: the person may manage but not invite. */
  invitationsForbidden: boolean
  /** The workspace has no Assets OA yet. */
  noAssets: boolean
  /** What the server says the areas offer, instead of the real table (to prove the screen follows the answer). */
  areas: PermissionArea[] | null
}
const DEFAULT_MODE: Mode = { forbidden: false, membersError: false, empty: false, hang: false, listError: false, inviteStatus: 0, invitationsForbidden: false, saveStatus: 0, noAssets: false, areas: null }
export const mode: Mode = { ...DEFAULT_MODE }

let departments = structuredClone(DEPARTMENTS0)
let roles = structuredClone(ROLES0)
let grants = structuredClone(GRANTS0)
let members = structuredClone(MEMBERS0)
let invitations = structuredClone(INVITATIONS0)
let extra = 0

export const resetFixtures = () => {
  Object.assign(mode, DEFAULT_MODE)
  departments = structuredClone(DEPARTMENTS0)
  roles = structuredClone(ROLES0)
  grants = structuredClone(GRANTS0)
  members = structuredClone(MEMBERS0)
  invitations = structuredClone(INVITATIONS0)
  extra = 0
  calls.length = 0
}

export interface ApiCall { method: string; path: string; body?: Record<string, unknown> }
export const calls: ApiCall[] = []

const fail = (message: string, status: number): Promise<never> =>
  import('../api/client').then(({ ApiError }) => { throw new ApiError(message, status) })

const areas = () => mode.areas ?? (mode.noAssets ? AREAS.filter((a) => a.area !== 'assets') : AREAS)

const count = (id: string) => members.filter((m) => m.roles.some((r) => r.id === id)).length

function summaryOf(r: RoleSummary): RoleSummary {
  if (r.kind === 'owners') return { ...r, member_count: members.filter((m) => m.is_owner).length }
  if (r.kind === 'custom') return { ...r, member_count: count(r.id) + (r.id === RID.ketoan ? 11 : 0) }
  return r
}

/** Stand-in for `apiFetch` on the admin screens. Unknown paths reject so a stray call is loud. */
export function adminFixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const [p] = path.split('?') as [string]
  const body = typeof init?.body === 'string' ? (JSON.parse(init.body) as Record<string, unknown>) : undefined
  calls.push({ method, path: p, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (p === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
  const prefix = `/workspaces/${WS_ID}`
  if (!p.startsWith(prefix)) return Promise.reject(new Error(`adminFixtureApi: unexpected ${method} ${path}`))
  const rest = p.slice(prefix.length)

  // Roles and departments are what members can read; the rest is for managers.
  const memberReadable = rest === '/roles' || rest === '/departments' || rest === '/permission-areas'
  if (mode.forbidden && !(method === 'GET' && memberReadable)) return fail('forbidden', 403)

  const isList = method === 'GET' && (rest === '/departments' || rest === '/roles')
  if (isList && mode.hang) return new Promise(() => {})
  if (isList && mode.listError) return fail('boom', 500)

  if (rest === '/departments' && method === 'GET') {
    if (mode.empty) return ok({ departments: [] })
    return ok({ departments })
  }
  if (rest === '/departments' && method === 'POST') {
    const d = dept(`44444444-aaaa-4bbb-8ccc-0000000001${String(++extra).padStart(2, '0')}`, String(body?.name), String(body?.parent_id ?? ''), 0)
    departments.push(d)
    return ok(d)
  }
  let m = /^\/departments\/([^/]+)$/.exec(rest)
  if (m && method === 'PUT') {
    const d = departments.find((x) => x.id === m![1])
    if (!d) return fail('not found', 404)
    d.name = String(body?.name)
    members.forEach((x) => { if (x.department?.id === d.id) x.department.name = d.name })
    return ok(d)
  }
  if (m && method === 'DELETE') {
    departments = departments.filter((x) => x.id !== m![1])
    members.forEach((x) => { if (x.department?.id === m![1]) x.department = null })
    return ok({})
  }
  m = /^\/departments\/([^/]+)\/move$/.exec(rest)
  if (m && method === 'PUT') {
    const d = departments.find((x) => x.id === m![1])
    if (!d) return fail('not found', 404)
    d.parent_id = String(body?.new_parent_id ?? '')
    return ok(d)
  }

  if (rest === '/admin/members' && method === 'GET') {
    if (mode.hang) return new Promise(() => {})
    if (mode.membersError) return fail('boom', 500)
    // The server lists people by name.
    return ok({ members: mode.empty ? [] : [...members].sort((a, b) => a.display_name.toLowerCase().localeCompare(b.display_name.toLowerCase())) })
  }
  if (rest === '/members' && method === 'POST') {
    // The same answer for any well-formed address: nothing about accounts or members.
    if (mode.inviteStatus) return fail('refused', mode.inviteStatus)
    const email = String(body?.email ?? '').trim().toLowerCase()
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return fail('not an address', 400)
    const open = invitations.find((i) => i.email === email)
    const role = roles.find((r) => r.id === body?.role_id)?.name ?? ''
    const dept = departments.find((d) => d.id === body?.department_id)?.name ?? ''
    if (open) Object.assign(open, { role_name: role, department_name: dept })
    else invitations.push(invitation(`77777777-aaaa-4bbb-8ccc-0000000003${String(++extra).padStart(2, '0')}`, email, role, dept, 7 * DAY))
    return ok({ status: 'invited' })
  }
  if (rest === '/invitations' && method === 'GET') {
    if (mode.invitationsForbidden) return fail('forbidden', 403)
    return ok({ invitations })
  }
  m = /^\/invitations\/([^/]+)$/.exec(rest)
  if (m && method === 'DELETE') {
    if (!invitations.some((i) => i.id === m![1])) return fail('not found', 404)
    invitations = invitations.filter((i) => i.id !== m![1])
    return ok({})
  }
  m = /^\/members\/([^/]+)$/.exec(rest)
  if (m && method === 'DELETE') {
    members = members.filter((x) => x.ngac_node_id !== m![1])
    return ok({ status: 'ok' })
  }
  m = /^\/members\/([^/]+)\/department$/.exec(rest)
  if (m && method === 'PUT') {
    const who = members.find((x) => x.ngac_node_id === m![1])
    if (!who) return fail('not found', 404)
    const d = departments.find((x) => x.id === body?.department_id)
    who.department = d ? { id: d.id, name: d.name } : null
    return ok({ status: 'ok' })
  }
  m = /^\/members\/([^/]+)\/roles\/([^/]+)$/.exec(rest)
  if (m) {
    const who = members.find((x) => x.ngac_node_id === m![1])
    const r = roles.find((x) => x.id === m![2] && x.kind === 'custom')
    if (!who || !r) return fail('not found', 404)
    if (method === 'PUT') {
      if (!who.roles.some((x) => x.id === r.id)) who.roles.push({ id: r.id, name: r.name! })
    } else {
      who.roles = who.roles.filter((x) => x.id !== r.id)
    }
    return ok({ status: 'ok' })
  }

  if (rest === '/permission-areas' && method === 'GET') return ok({ areas: areas() })

  if (rest === '/roles' && method === 'GET') {
    if (mode.empty) return ok({ roles: [], system_roles: roles.filter((r) => r.kind !== 'custom').map(summaryOf) })
    return ok({
      roles: roles.filter((r) => r.kind === 'custom').map(summaryOf),
      system_roles: roles.filter((r) => r.kind !== 'custom').map(summaryOf),
    })
  }
  if (rest === '/roles' && method === 'POST') {
    const name = String(body?.name ?? '').trim()
    if (!name || /^(Dept_|Role_|PC_)/i.test(name)) return fail('invalid role name', 400)
    const created = role(`55555555-aaaa-4bbb-8ccc-0000000002${String(++extra).padStart(2, '0')}`, 'custom', name, 0)
    roles.push(created)
    grants[created.id] = {}
    return ok(created)
  }
  m = /^\/roles\/([^/]+)$/.exec(rest)
  if (m && method === 'GET') {
    const r = roles.find((x) => x.id === m![1])
    if (!r) return fail('not found', 404)
    const holders = members.filter((x) => (r.kind === 'owners' ? x.is_owner : r.kind === 'members' ? true : x.roles.some((y) => y.id === r.id)))
    const detail: RoleDetail = {
      role: summaryOf(r),
      members: holders.slice(0, 3).map((x) => ({ ngac_node_id: x.ngac_node_id, user_id: x.user_id, display_name: x.display_name, avatar_url: '' })),
      permissions: areas()
        .filter((a) => (grants[r.id]?.[a.area] ?? []).length > 0)
        .map((a) => ({ area: a.area, operations: grants[r.id]![a.area]! })),
    }
    return ok(detail)
  }
  if (m && method === 'DELETE') {
    const r = roles.find((x) => x.id === m![1])
    if (!r || r.kind !== 'custom') return fail('not found', 404)
    roles = roles.filter((x) => x.id !== r.id)
    members.forEach((x) => { x.roles = x.roles.filter((y) => y.id !== r.id) })
    return ok({})
  }
  m = /^\/roles\/([^/]+)\/permissions\/([^/]+)$/.exec(rest)
  if (m && method === 'PUT') {
    if (mode.saveStatus) return fail('save failed', mode.saveStatus)
    const r = roles.find((x) => x.id === m![1] && x.kind === 'custom')
    const area = areas().find((a) => a.area === m![2])
    if (!r || !area) return fail('not found', 404)
    const ops = (body?.operations as string[]) ?? []
    if (ops.some((op) => !area.operations.includes(op))) return fail('invalid operation', 400)
    grants[r.id] = { ...grants[r.id], [area.area]: area.operations.filter((o) => ops.includes(o)) }
    return ok({ area: area.area, operations: grants[r.id]![area.area] })
  }

  return Promise.reject(new Error(`adminFixtureApi: unexpected ${method} ${path}`))
}
