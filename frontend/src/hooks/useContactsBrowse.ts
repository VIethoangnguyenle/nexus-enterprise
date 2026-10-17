import { useMemo } from 'react'
import type { Channel } from '../api/messaging'
import {
  filterContacts, pathOf, sortContacts, NO_FILTER,
  type DeptNode, type DirectoryTree, type PathSegment, type RoleGroup,
} from '../components/contacts/contacts-model'
import type { ContactsCat, ContactsSearch } from '../lib/contacts-search'
import { useDepartmentTree, useGroupMembers, useMyGroups, useRoleGroups, type GroupMember } from './useContactCategories'
import { useContacts, type Contact } from './useContacts'

/** What the middle column is showing. */
export type BrowseKind = 'search' | 'dept' | 'roles' | 'role' | 'groups' | 'group' | 'members'

/**
 * `tree-error`: the people are there but the departments are not; Phòng ban says so and everything else, search
 * included, keeps working.
 */
export type BrowseStatus = 'loading' | 'error' | 'tree-error' | 'ready'

export interface BrowseInput {
  wsId: string
  workspaceName: string
  isPersonal: boolean
  /** The category in use; `search` is the phone's search screen. */
  cat: ContactsCat
  /** The place inside it, from the address. */
  place: Pick<ContactsSearch, 'dept' | 'role' | 'chat' | 'person'>
  /** What the search box holds. */
  query: string
}

export interface BrowseView {
  kind: BrowseKind
  /** What the address names, with a search typed over it left out: the screen's bar keeps this name while typing. */
  level: { kind: BrowseKind; title: string; sub: string }
  status: BrowseStatus
  /** Asks again for whatever failed. */
  retry: () => void
  /** The whole directory the caller may browse, by given name. */
  all: Contact[]
  /** The caller sees only part of the workspace. */
  limited: boolean
  tree: DirectoryTree | undefined
  title: string
  sub: string
  /** Phòng ban: the departments from the top to the open one; empty at the top. */
  trail: PathSegment[]
  atRoot: boolean
  depts: DeptNode[]
  /** The people of the open level (a department, a role, a search, the members), before the online filter. */
  people: Contact[]
  roles: RoleGroup[]
  role?: RoleGroup
  groups: Channel[]
  group?: Channel
  members: GroupMember[]
}

/** What the nav and the chips need to know without a category being open. */
export interface BrowseFacts {
  roleCount: number | undefined
  /** Undefined until the chat list was asked for: it is not asked for here. */
  groupCount: number | undefined
  /** Undefined until Phòng ban was opened. */
  rootCount: number | undefined
  tree: DirectoryTree | undefined
  /** The departments failed to load. */
  treeFailed: boolean
  retryTree: () => void
  limited: boolean
  directoryLoaded: boolean
}

/**
 * Everything the middle column of Danh bạ shows, for the category and place in the address. The directory is
 * the app's, loaded already; each category adds only its own request, and only while it is open: the
 * departments for Phòng ban (and wherever a department path is shown), the chat list for Nhóm của tôi, the
 * members for one opened group. Roles are read from the people.
 */
export function useContactsBrowse(i: BrowseInput): { view: BrowseView; facts: BrowseFacts } {
  const contacts = useContacts(i.wsId)
  const roleQ = useRoleGroups(i.wsId)
  const searching = i.query.trim() !== ''
  const inGroups = i.cat === 'groups'
  // A department path is shown by search, by a role's holders, by a group's members and by a profile.
  const wantsTree = !i.isPersonal && (i.cat === 'org' || searching || !!i.place.person
    || (i.cat === 'roles' && !!i.place.role) || (inGroups && !!i.place.chat))
  const treeQ = useDepartmentTree(i.wsId, wantsTree)
  const groupsQ = useMyGroups(i.wsId, inGroups)
  const membersQ = useGroupMembers(i.wsId, inGroups ? i.place.chat ?? null : null)

  const list = contacts.data?.contacts
  const limited = contacts.data?.visibility === 'limited'
  // Departments are a company workspace's data: a personal workspace lists people without them.
  const all = useMemo(
    () => sortContacts(i.isPersonal ? (list ?? []).map((c) => ({ ...c, department: '', department_id: '' })) : list ?? []),
    [list, i.isPersonal],
  )

  const view = useMemo<BrowseView>(() => {
    const tree = treeQ.tree
    const role = i.cat === 'roles' ? roleQ.groups.find((r) => r.id === i.place.role) : undefined
    const group = inGroups ? groupsQ.groups.find((g) => g.id === i.place.chat) : undefined
    // What the place in the address is, whatever is typed over it; a search typed over it is the content, not the level.
    const levelKind: BrowseKind = i.cat === 'search' ? 'search'
      : i.cat === 'org' ? 'dept'
        : i.cat === 'roles' ? (role ? 'role' : 'roles')
          : inGroups ? (group ? 'group' : 'groups')
            : 'members'
    const kind: BrowseKind = searching ? 'search' : levelKind

    const base: BrowseView = {
      kind, level: { kind: levelKind, title: '', sub: '' }, status: 'ready', retry: () => {}, all, limited, tree, title: '', sub: '', trail: [], atRoot: false,
      depts: [], people: [], roles: roleQ.groups, role, groups: groupsQ.groups, group, members: membersQ.members,
    }

    // --- status: each category depends on its own sources. "Not loaded" is not "empty": a source that has not
    // started (the workspace is not known yet) or has not answered is loading, never an empty list. ---
    const refetchContacts = () => { void contacts.refetch() }
    const peopleLoaded = !!contacts.data
    if (kind === 'groups' || kind === 'group') {
      base.retry = () => {
        if (groupsQ.isError) groupsQ.refetch()
        if (membersQ.isError) membersQ.refetch()
        if (contacts.isError) refetchContacts()
      }
      const failed = groupsQ.isError || (kind === 'group' && (membersQ.isError || contacts.isError))
      const waiting = !groupsQ.known || (kind === 'group' && (membersQ.isLoading || !peopleLoaded))
      base.status = failed ? 'error' : waiting ? 'loading' : 'ready'
    } else if (kind === 'dept') {
      base.retry = treeQ.refetch
      base.status = contacts.isError ? 'error' : treeQ.isError ? 'tree-error' : !tree ? 'loading' : 'ready'
    } else {
      base.retry = refetchContacts
      base.status = contacts.isError ? 'error' : !peopleLoaded ? 'loading' : 'ready'
    }

    // --- content: the level the address names ---
    switch (levelKind) {
      case 'search':
        break
      case 'dept': {
        const node = i.place.dept ? tree?.byId.get(i.place.dept) : undefined
        base.atRoot = !node
        base.depts = node ? node.children : tree?.roots ?? []
        base.people = node ? node.people : tree?.unassigned ?? []
        base.trail = tree && node ? pathOf(tree, node.id).map((n) => ({ id: n.id, name: n.name })) : []
        if (node) {
          base.title = node.name
          base.sub = `${node.peopleCount} người${node.children.length ? ` · ${node.children.length} phòng con` : ''}`
        } else {
          const total = (tree?.roots ?? []).reduce((n, r) => n + r.peopleCount, 0) + (tree?.unassigned.length ?? 0)
          base.title = i.workspaceName
          base.sub = `${total} người${tree?.roots.length ? ` · ${tree.roots.length} khối` : ''}`
        }
        break
      }
      case 'roles':
        base.title = 'Vai trò'
        base.sub = `${roleQ.groups.length} vai trò`
        break
      case 'role':
        base.people = role!.people
        base.title = role!.name
        base.sub = `${role!.people.length} người${role!.kind === 'owners' ? ' · quản lý workspace' : ''}`
        break
      case 'groups':
        base.title = 'Nhóm của tôi'
        base.sub = `${groupsQ.groups.length} nhóm`
        break
      case 'group':
        base.title = group!.name
        base.sub = `${membersQ.members.length} thành viên`
        break
      case 'members':
        base.people = all
        base.title = 'Thành viên'
        base.sub = `${all.length} người trong ${i.workspaceName}`
        break
    }
    base.level = { kind: levelKind, title: base.title, sub: base.sub }

    // --- a search typed over it: nothing typed lists nobody (there is no list of everyone), something typed is the whole visible scope ---
    if (kind === 'search') {
      const found = searching ? filterContacts(all, { ...NO_FILTER, query: i.query }, () => false) : []
      base.people = found
      base.title = searching ? `“${i.query.trim()}”` : ''
      base.sub = searching ? `${found.length} người khớp` : ''
    }
    return base
  }, [i.cat, i.place.dept, i.place.role, i.place.chat, i.query, i.workspaceName, searching, inGroups, all, limited,
    treeQ.tree, treeQ.isError, treeQ.refetch, roleQ.groups, groupsQ.groups, groupsQ.isLoading, groupsQ.isError,
    groupsQ.known, groupsQ.refetch, membersQ.members, membersQ.isLoading, membersQ.isError, membersQ.refetch,
    contacts.data, contacts.isError, contacts.refetch])

  const facts = useMemo<BrowseFacts>(() => ({
    roleCount: contacts.data ? roleQ.groups.length : undefined,
    groupCount: groupsQ.known ? groupsQ.groups.length : undefined,
    rootCount: treeQ.tree ? treeQ.tree.roots.length : undefined,
    tree: treeQ.tree,
    // The departments failed on their own; when the people failed too, that is the one problem and it is said once.
    treeFailed: treeQ.isError && !contacts.isError,
    retryTree: treeQ.refetch,
    limited,
    directoryLoaded: !!contacts.data,
  }), [contacts.data, roleQ.groups, groupsQ.known, groupsQ.groups, treeQ.tree, treeQ.isError, treeQ.refetch, contacts.isError, limited])

  return { view, facts }
}

/** The people the open level lists, as directory entries: what "Đang trực tuyến" counts and filters. */
export function levelContacts(view: BrowseView): Contact[] {
  if (view.kind !== 'group') return view.people
  const byId = new Map(view.all.map((c) => [c.user_id, c]))
  return view.members.map((m) => byId.get(m.userId)).filter((c): c is Contact => !!c)
}
