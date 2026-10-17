import type { Department } from '../../api/admin'
import type { Channel } from '../../api/messaging'
import type { ContactsCat } from '../../lib/contacts-search'
import type { Contact } from '../../hooks/useContacts'
import { normalize, UNKNOWN_PERSON } from '../../lib/people'

const VI = 'vi'
const byText = (a: string, b: string) => a.localeCompare(b, VI, { sensitivity: 'base' })

/** The name a person is shown by. An id is never a fallback. */
export function nameOf(c: Pick<Contact, 'display_name'>): string {
  // A username is a login handle, not a name: never shown.
  return c.display_name?.trim() || UNKNOWN_PERSON
}

/** "Trưởng phòng · Vận hành thanh toán": title and department, either may be missing. */
export function roleLine(c: Pick<Contact, 'title' | 'department'>): string {
  return [c.title, c.department].filter(Boolean).join(' · ')
}

/** The name people call someone by: the last word ("Nguyễn Văn An" is "An"). */
export function givenNameOf(fullName: string): string {
  const words = fullName.trim().split(/\s+/)
  return words[words.length - 1] ?? ''
}

/** People sorted the way a Vietnamese list is read: by given name, then by full name, accents collated. */
export function sortContacts(list: Contact[]): Contact[] {
  return [...list].sort((a, b) => {
    const x = nameOf(a)
    const y = nameOf(b)
    return byText(givenNameOf(x), givenNameOf(y)) || byText(x, y)
  })
}

export interface ContactFilter {
  query: string
  onlineOnly: boolean
}

export const NO_FILTER: ContactFilter = { query: '', onlineOnly: false }

export const isFiltering = (f: ContactFilter) => !!(f.query.trim() || f.onlineOnly)

/** Matches name, title and email, ignoring case and accents ("duc" finds "Đức"). */
export function filterContacts(list: Contact[], f: ContactFilter, isOnline: (c: Contact) => boolean): Contact[] {
  const q = normalize(f.query)
  return list.filter((c) => {
    if (f.onlineOnly && !isOnline(c)) return false
    if (!q) return true
    return [nameOf(c), c.title, c.email].some((part) => normalize(part ?? '').includes(q))
  })
}

/** Others in the same department as `of` (by id, never by name), never `of` themselves. */
export function colleaguesOf(list: Contact[], of: Contact): Contact[] {
  if (!of.department_id) return []
  return sortContacts(list.filter((c) => c.department_id === of.department_id && c.user_id !== of.user_id))
}

// --- The department tree ---------------------------------------------------------------------

export interface DeptNode {
  id: string
  name: string
  /** The parent in the list; empty for a root. */
  parentId: string
  /** An administrator's mark: the department is not shown to members. */
  hidden: boolean
  children: DeptNode[]
  /** The people directly in this department. */
  people: Contact[]
  /** The people in this department and everything beneath it (only those the caller sees). */
  peopleCount: number
  /** The departments directly beneath this one. */
  subDepartmentCount: number
}

export interface DirectoryTree {
  /** Departments whose parent is not in the list: for a limited caller, their own department is one. */
  roots: DeptNode[]
  byId: Map<string, DeptNode>
  /** People in no department the caller can see: they sit at the organisation's root. */
  unassigned: Contact[]
}

/**
 * The departments as a tree, nodes keyed by id so that two departments with one name keep their
 * own people. A person is placed by `department_id`; one whose department is not in the list
 * (none, or one the caller may not see) is at the organisation's root.
 */
export function buildTree(departments: Department[], contacts: Contact[]): DirectoryTree {
  const byId = new Map<string, DeptNode>()
  for (const d of departments) {
    byId.set(d.id, {
      id: d.id, name: d.name, parentId: d.parent_id ?? '', hidden: !!d.hidden,
      children: [], people: [], peopleCount: 0, subDepartmentCount: 0,
    })
  }
  const roots: DeptNode[] = []
  for (const node of byId.values()) {
    const parent = node.parentId ? byId.get(node.parentId) : undefined
    if (parent && parent !== node) parent.children.push(node)
    else roots.push(node)
  }
  const byName = (a: DeptNode, b: DeptNode) => byText(a.name, b.name) || a.id.localeCompare(b.id)
  // Every node reachable from a root has exactly one path to it, so this cannot loop.
  const settle = (node: DeptNode): number => {
    node.people = sortContacts(node.people)
    node.children.sort(byName)
    node.subDepartmentCount = node.children.length
    node.peopleCount = node.people.length + node.children.reduce((n, child) => n + settle(child), 0)
    return node.peopleCount
  }
  roots.sort(byName)
  // A cycle of parents has no root: its departments are never reached, so they are dropped and
  // their people are filed at the organisation's root rather than lost.
  const reached = new Set<string>()
  const walk = (n: DeptNode) => { reached.add(n.id); n.children.forEach(walk) }
  roots.forEach(walk)
  for (const id of [...byId.keys()]) if (!reached.has(id)) byId.delete(id)
  const unassigned: Contact[] = []
  for (const c of contacts) {
    const node = c.department_id ? byId.get(c.department_id) : undefined
    if (node) node.people.push(c)
    else unassigned.push(c)
  }
  roots.forEach(settle)
  return { roots, byId, unassigned: sortContacts(unassigned) }
}

/** The department and every ancestor above it, root first; empty for an unknown id. */
export function pathOf(tree: DirectoryTree, deptId: string): DeptNode[] {
  const path: DeptNode[] = []
  const seen = new Set<string>()
  let node = deptId ? tree.byId.get(deptId) : undefined
  while (node && !seen.has(node.id)) {
    seen.add(node.id)
    path.unshift(node)
    node = node.parentId ? tree.byId.get(node.parentId) : undefined
  }
  return path
}

export const PATH_SEPARATOR = ' › '

/** "Khối Tài chính › Đối soát". */
export function pathLabel(tree: DirectoryTree, deptId: string): string {
  return pathOf(tree, deptId).map((n) => n.name).join(PATH_SEPARATOR)
}

/** The path above a department ("Khối Tài chính" for Kế toán); empty at the top, where there is nothing above it. */
export function parentPathLabel(tree: DirectoryTree, deptId: string): string {
  return pathOf(tree, deptId).slice(0, -1).map((n) => n.name).join(PATH_SEPARATOR)
}

export interface PathSegment { id: string; name: string }

/**
 * The department path of a person, root first. From the tree when it is there; without it (not
 * loaded, failed, or a department the caller cannot see in it) the department they sit in is still
 * known by id and name, so a row never goes without a path it could have.
 */
export function pathSegments(tree: DirectoryTree | undefined, c: Pick<Contact, 'department_id' | 'department'>): PathSegment[] {
  const nodes = tree ? pathOf(tree, c.department_id) : []
  if (nodes.length > 0) return nodes.map((n) => ({ id: n.id, name: n.name }))
  return c.department ? [{ id: c.department_id, name: c.department }] : []
}

/** A search result's second line: "Chuyên viên · Khối Tài chính › Đối soát". */
export function searchSubline(c: Pick<Contact, 'title' | 'department_id' | 'department'>, tree: DirectoryTree | undefined): string {
  return [c.title, pathSegments(tree, c).map((s) => s.name).join(PATH_SEPARATOR)].filter(Boolean).join(' · ')
}

/** The name of the top of Phòng ban: the organisation, or just the part of it a limited caller sees. */
export function rootLabel(workspaceName: string, limited: boolean): string {
  return limited ? 'Phạm vi của bạn' : `Toàn ${workspaceName}`
}

// --- Roles ---------------------------------------------------------------------------------

/** What Danh bạ calls the owners; Quản trị shows "Chủ sở hữu" for the same role. */
export const OWNERS_LABEL = 'Quản trị viên'

export interface RoleGroup {
  id: string
  name: string
  kind: 'owners' | 'custom'
  /** The holders the caller can see, by given name. */
  people: Contact[]
}

/**
 * The roles of the loaded people: Quản trị viên first, then the administrator's own roles by name,
 * each with its visible holders. "Members" is left out: everyone holds it. A role nobody visible
 * holds is not listed.
 */
export function rolesOf(contacts: Contact[]): RoleGroup[] {
  const groups = new Map<string, RoleGroup>()
  for (const c of contacts) {
    for (const r of c.roles ?? []) {
      if (r.kind === 'members') continue
      let g = groups.get(r.id)
      if (!g) {
        g = { id: r.id, name: r.kind === 'owners' ? OWNERS_LABEL : r.name, kind: r.kind, people: [] }
        groups.set(r.id, g)
      }
      g.people.push(c)
    }
  }
  const out = [...groups.values()]
  for (const g of out) g.people = sortContacts(g.people)
  return out.sort((a, b) => (a.kind === b.kind ? byText(a.name, b.name) : a.kind === 'owners' ? -1 : 1))
}

// --- Groups --------------------------------------------------------------------------------

/**
 * The groups of "Nhóm của tôi": private groups only. #general is the workspace-wide channel
 * (`workspace` type, everyone is in it) and a direct message is not a group.
 */
export function privateGroups(channels: Channel[] | undefined): Channel[] {
  return (channels ?? []).filter((c) => c.channel_type === 'private')
}

// --- Categories ----------------------------------------------------------------------------

export interface CategoryDef {
  id: Exclude<ContactsCat, 'search'>
  title: string
  /** One muted line of what is inside. */
  sub: string
  /** Which of the eight person hues the icon square wears. */
  hue: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8
}

interface CategoryInputs {
  isPersonal: boolean
  workspaceName: string
  /** The caller sees only part of the workspace. */
  limited: boolean
  memberCount: number
  /** Roles with someone in them; undefined while the people are not loaded. */
  roleCount: number | undefined
  /** The caller's groups; undefined until the chat list is known (it is not asked for here). */
  groupCount: number | undefined
  /** Top-level departments; undefined until Phòng ban has been opened. */
  rootCount: number | undefined
}

/**
 * The categories of Danh bạ, the same on every width (DESIGN.md "Danh bạ"). An organisation: Phòng ban,
 * Vai trò, Nhóm của tôi. A personal workspace: Thành viên, Nhóm của tôi. A category known to be empty
 * is left out; one not yet known stays, and does not claim a count it has not loaded.
 */
export function categoriesOf(i: CategoryInputs): CategoryDef[] {
  const groups: CategoryDef | undefined = i.groupCount === 0 ? undefined : {
    id: 'groups', title: 'Nhóm của tôi', hue: 5,
    sub: i.groupCount === undefined ? 'Nhóm chat bạn đang ở' : `${i.groupCount} nhóm`,
  }
  if (i.isPersonal) {
    return [
      { id: 'members', title: 'Thành viên', hue: 1, sub: `${i.memberCount} người trong không gian này` },
      ...(groups ? [groups] : []),
    ]
  }
  const org: CategoryDef = {
    id: 'org', title: 'Phòng ban', hue: 1,
    sub: i.limited ? 'Chỉ phòng của bạn' : i.rootCount === undefined ? i.workspaceName : `${i.workspaceName} · ${i.rootCount} khối`,
  }
  const roles: CategoryDef | undefined = i.roleCount === 0 ? undefined : {
    id: 'roles', title: 'Vai trò', hue: 3,
    sub: i.roleCount === undefined ? 'Ai giữ vai trò nào' : `${i.roleCount} vai trò`,
  }
  return [org, ...(roles ? [roles] : []), ...(groups ? [groups] : [])]
}
