import type { BrowseKind } from '../../hooks/useContactsBrowse'
import type { Contact } from '../../hooks/useContacts'
import type { ContactsSearch } from '../../lib/contacts-search'
import { pathLabel, type DeptNode, type DirectoryTree } from './contacts-model'

/**
 * The place a phone's people search is narrowed to (DESIGN.md "Danh bạ" › Điện thoại, "Tìm người"): a department
 * (with everything beneath it), a role, or a group. It is chosen from a picker or comes from the level the search was
 * opened on; the id is for matching and is never shown, the label is.
 */
export interface PlaceFilter {
  kind: 'dept' | 'role' | 'group'
  id: string
  label: string
}

interface PlaceContext {
  tree?: DirectoryTree
  /** The user ids of the group's members, once known. */
  groupMembers?: Set<string>
}

function subtreeIds(node: DeptNode, out = new Set<string>()): Set<string> {
  out.add(node.id)
  node.children.forEach((child) => subtreeIds(child, out))
  return out
}

/** The people of `list` who are in the place. Only narrows: nobody outside `list` (the scoped directory) comes in. */
export function inPlace(list: Contact[], filter: PlaceFilter, ctx: PlaceContext): Contact[] {
  switch (filter.kind) {
    case 'dept': {
      const node = ctx.tree?.byId.get(filter.id)
      const ids = node ? subtreeIds(node) : new Set([filter.id])
      return list.filter((c) => !!c.department_id && ids.has(c.department_id))
    }
    case 'role':
      return list.filter((c) => c.roles.some((r) => r.id === filter.id))
    case 'group': {
      const members = ctx.groupMembers
      return members ? list.filter((c) => members.has(c.user_id)) : []
    }
  }
}

/** Every department the viewer may see, root first, each named with the path above it ("Khối Vận hành › Đối soát"). */
export function deptChoices(tree: DirectoryTree): { id: string; label: string }[] {
  const out: { id: string; label: string }[] = []
  const walk = (node: DeptNode) => {
    out.push({ id: node.id, label: pathLabel(tree, node.id) || node.name })
    node.children.forEach(walk)
  }
  tree.roots.forEach(walk)
  return out
}

/** The place a level of the stack shows, which its search icon narrows the search to; none at a category's top. */
export function placeOfLevel(level: { kind: BrowseKind; title: string }, place: Pick<ContactsSearch, 'dept' | 'role' | 'chat'>): PlaceFilter | undefined {
  if (level.kind === 'dept' && place.dept) return { kind: 'dept', id: place.dept, label: level.title }
  if (level.kind === 'role' && place.role) return { kind: 'role', id: place.role, label: level.title }
  if (level.kind === 'group' && place.chat) return { kind: 'group', id: place.chat, label: level.title }
  return undefined
}
