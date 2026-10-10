import type { AreaPermission, Department, Member, MemberStatus, PermissionArea, RoleKind, RoleSummary } from '../api/admin'
import { NGAC_OPS } from '../api/access'
import { normalize, type Person } from './people'

// --- Words ---------------------------------------------------------------
//
// Which operations exist on which area is the server's answer
// (GET /permission-areas). What follows only says how to *word* them: a label
// per NGAC operation and per area. An operation or area the screen has no word
// for gets a neutral one, never its code.

const OP_WORDS: Record<string, { label: string; hint: string }> = {
  read: { label: 'Xem', hint: 'Mở và đọc nội dung' },
  write: { label: 'Sửa', hint: 'Đổi nội dung đã có' },
  upload: { label: 'Tải lên', hint: 'Đưa tệp mới lên' },
  approve: { label: 'Duyệt', hint: 'Duyệt yêu cầu' },
  share: { label: 'Chia sẻ', hint: 'Chia sẻ với người khác' },
  manage: { label: 'Quản lý', hint: 'Quản lý và đổi thiết lập' },
  invite: { label: 'Mời', hint: 'Mời người vào và đưa người ra' },
  create_channel: { label: 'Tạo nhóm chat', hint: 'Tạo nhóm hoặc kênh mới' },
}

const OTHER_OP = 'Quyền khác'

export const opLabel = (op: string): string => OP_WORDS[op]?.label ?? OTHER_OP
export const opHint = (op: string): string => OP_WORDS[op]?.hint ?? ''

const AREA_WORDS: Record<string, { label: string; hint: string }> = {
  documents: { label: 'Tài liệu', hint: 'Thư mục và tệp của workspace' },
  channels: { label: 'Tin nhắn', hint: 'Kênh và nhóm chat' },
  assets: { label: 'Tài sản', hint: 'Kho tài sản và yêu cầu cấp tài sản' },
  management: { label: 'Quản trị', hint: 'Thành viên, vai trò và phòng ban' },
}

const OTHER_AREA = 'Vùng khác'

export const areaLabel = (area: string): string => AREA_WORDS[area]?.label ?? OTHER_AREA
export const areaHint = (area: string): string => AREA_WORDS[area]?.hint ?? ''

const SYSTEM_ROLE: Record<Exclude<RoleKind, 'custom'>, { name: string; line: string }> = {
  owners: { name: 'Chủ sở hữu', line: 'Toàn quyền, kể cả quản trị' },
  members: { name: 'Thành viên', line: 'Mặc định cho mọi người trong workspace' },
}

export const OWNER_LABEL = SYSTEM_ROLE.owners.name
export const MEMBER_LABEL = SYSTEM_ROLE.members.name

/** The name a role goes by on screen. The built-in ones are worded here; a custom one is what its maker typed. */
export function roleName(r: Pick<RoleSummary, 'kind' | 'name'>): string {
  if (r.kind === 'custom') return r.name?.trim() || 'Vai trò'
  return SYSTEM_ROLE[r.kind].name
}

/** The line under a role's name in the list. */
export function roleLine(r: Pick<RoleSummary, 'kind'>): string {
  return r.kind === 'custom' ? 'Vai trò tuỳ chỉnh' : SYSTEM_ROLE[r.kind].line
}

const STATUS_WORDS: Record<MemberStatus, string> = {
  active: 'Đang hoạt động',
  invited: 'Đã mời',
  disabled: 'Đã khoá',
}

export const statusLabel = (s: string): string => STATUS_WORDS[s as MemberStatus] ?? 'Đang hoạt động'

// --- People ---------------------------------------------------------------

/** Name, email or title contains the query, ignoring case and accents. */
export function matchesMember(m: Member, query: string): boolean {
  const q = normalize(query)
  if (!q) return true
  return [m.display_name, m.email, m.title].some((s) => normalize(s ?? '').includes(q))
}

/** A member as the people pickers know a person. The ids ride along for API calls and colour; they are never shown. */
export const personFromMember = (m: Member): Person => ({
  userId: m.user_id,
  nodeId: m.ngac_node_id,
  username: '',
  name: m.display_name,
  role: m.title,
  avatarUrl: m.avatar_url,
})

/** What the role cell shows: the pills in order, and how many more there are. */
export interface RolePills {
  shown: string[]
  more: number
}

const PILLS_SHOWN = 2

/**
 * A person's roles as pills: Owner first, then the administrator's roles by
 * name, at most two and then "+N". The default "Thành viên" appears only for a
 * person who holds nothing else.
 */
export function rolePills(m: Pick<Member, 'is_owner' | 'roles'>): RolePills {
  const all = [...(m.is_owner ? [OWNER_LABEL] : []), ...m.roles.map((r) => r.name)]
  if (all.length === 0) return { shown: [MEMBER_LABEL], more: 0 }
  return { shown: all.slice(0, PILLS_SHOWN), more: Math.max(0, all.length - PILLS_SHOWN) }
}

// --- Departments ------------------------------------------------------------

/** Departments by parent. Roots are under the empty key; a department whose parent is unknown is a root too. */
export function childrenIndex(depts: Department[]): Map<string, Department[]> {
  const known = new Set(depts.map((d) => d.id))
  const out = new Map<string, Department[]>()
  for (const d of depts) {
    const key = d.parent_id && known.has(d.parent_id) ? d.parent_id : ''
    out.set(key, [...(out.get(key) ?? []), d])
  }
  return out
}

/** Names from the root to this department: ["Khối Vận hành", "Đối soát"]. */
export function deptPath(depts: Department[], id: string): string[] {
  const byId = new Map(depts.map((d) => [d.id, d]))
  const path: string[] = []
  const seen = new Set<string>()
  for (let cur = byId.get(id); cur && !seen.has(cur.id); cur = byId.get(cur.parent_id)) {
    seen.add(cur.id)
    path.unshift(cur.name)
  }
  return path
}

/** The departments above this one, nearest first. */
export function deptAncestors(depts: Department[], id: string): string[] {
  const byId = new Map(depts.map((d) => [d.id, d]))
  const out: string[] = []
  const seen = new Set<string>([id])
  for (let cur = byId.get(byId.get(id)?.parent_id ?? ''); cur && !seen.has(cur.id); cur = byId.get(cur.parent_id)) {
    seen.add(cur.id)
    out.push(cur.id)
  }
  return out
}

export const DEPT_SEPARATOR = ' › '

export const deptPathLabel = (depts: Department[], id: string): string => deptPath(depts, id).join(DEPT_SEPARATOR)

/** How many levels the organisation has (1 for a flat list, 0 for none). */
export function deptDepth(depts: Department[]): number {
  const index = childrenIndex(depts)
  const depth = (id: string, seen: Set<string>): number => {
    if (seen.has(id)) return 0
    const next = new Set(seen).add(id)
    return 1 + Math.max(0, ...(index.get(id) ?? []).map((c) => depth(c.id, next)))
  }
  return Math.max(0, ...(index.get('') ?? []).map((r) => depth(r.id, new Set())))
}

/** The department and everything beneath it; a department cannot be moved under itself or its own children. */
export function deptWithDescendants(depts: Department[], id: string): Set<string> {
  const index = childrenIndex(depts)
  const out = new Set<string>()
  const walk = (cur: string) => {
    if (out.has(cur)) return
    out.add(cur)
    for (const c of index.get(cur) ?? []) walk(c.id)
  }
  walk(id)
  return out
}

// --- The permission matrix --------------------------------------------------

/** area → the operations held there, in the order the server lists them. */
export type Grants = Record<string, string[]>

export const grantsOf = (permissions: AreaPermission[]): Grants =>
  Object.fromEntries(permissions.map((p) => [p.area, [...p.operations]]))

/** The columns of the matrix: every operation any area offers, in NGAC order; unfamiliar ones last. */
export function matrixColumns(areas: PermissionArea[]): string[] {
  const offered = new Set(areas.flatMap((a) => a.operations))
  const known: string[] = NGAC_OPS.filter((op) => offered.has(op))
  const rest = [...offered].filter((op) => !known.includes(op))
  return [...known, ...rest]
}

/** Sets or clears one operation on one area, keeping the server's order for the area. */
export function setOp(draft: Grants, area: PermissionArea, op: string, on: boolean): Grants {
  if (!area.operations.includes(op)) return draft
  const held = new Set(draft[area.area] ?? [])
  if (on) held.add(op)
  else held.delete(op)
  return { ...draft, [area.area]: area.operations.filter((o) => held.has(o)) }
}

export interface AreaChange {
  area: string
  added: string[]
  removed: string[]
}

/** Which areas differ between what is saved and what is drafted, and how. */
export function changesBetween(saved: Grants, draft: Grants, areas: PermissionArea[]): AreaChange[] {
  const out: AreaChange[] = []
  for (const a of areas) {
    const before = new Set(saved[a.area] ?? [])
    const after = new Set(draft[a.area] ?? [])
    const added = a.operations.filter((o) => after.has(o) && !before.has(o))
    const removed = a.operations.filter((o) => before.has(o) && !after.has(o))
    if (added.length || removed.length) out.push({ area: a.area, added, removed })
  }
  return out
}

const words = (ops: string[]) => ops.map(opLabel).join(', ')

/** The sentence under the save bar: who is affected and what changes for them. */
export function consequenceLine(role: string, people: number, changes: AreaChange[]): string {
  if (changes.length === 0) return ''
  if (people === 0) return `Chưa ai giữ vai trò ${role}, nên thay đổi này chưa ảnh hưởng ai.`
  const parts = changes.flatMap((c) => [
    ...(c.added.length ? [`được ${words(c.added)} trên ${areaLabel(c.area)}`] : []),
    ...(c.removed.length ? [`không còn ${words(c.removed)} trên ${areaLabel(c.area)}`] : []),
  ])
  return `${people} người có vai trò ${role} sẽ ${parts.join(', ')}.`
}

/** "1 thay đổi chưa lưu" / "3 thay đổi chưa lưu": one per operation that differs. */
export function changeCount(changes: AreaChange[]): number {
  return changes.reduce((n, c) => n + c.added.length + c.removed.length, 0)
}
