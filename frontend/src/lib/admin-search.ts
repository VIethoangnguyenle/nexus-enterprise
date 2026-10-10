import { validateWorkspaceSearch } from './workspace'

/**
 * What the Quản trị URL carries: the workspace (`ws`) and what is open: a
 * department (`dept`), a person (`member`), a role (`role`) and, on a role, the
 * permission editor (`edit`). The three screens are routes of their own, so
 * each reads only its own keys; the rest are dropped on navigation. Reload,
 * Back/Forward and a pasted link land in the same place.
 */
export interface AdminSearch {
  ws?: string
  dept?: string
  /** The person open in Người dùng. Not `user`: the auth store reads `?user=` as which account's session to use. */
  member?: string
  role?: string
  /** `'1'` while the permission editor of `role` is open. */
  edit?: string
}

/** `validateSearch` for the admin routes: unknown or malformed values are dropped, not rejected. */
export function validateAdminSearch(search: Record<string, unknown>): AdminSearch {
  const out: AdminSearch = { ...validateWorkspaceSearch(search) }
  for (const key of ['dept', 'member', 'role'] as const) {
    const v = search[key]
    if (typeof v === 'string' && v) out[key] = v
  }
  if (out.role && (search.edit === true || search.edit === 'true' || search.edit === 1 || search.edit === '1')) out.edit = '1'
  return out
}

/** Search for opening one thing (or, with no id, closing it): everything else open is closed, the workspace is kept. */
export function openSearch(prev: AdminSearch, key: 'dept' | 'member' | 'role', id?: string): AdminSearch {
  const { dept: _d, member: _m, role: _r, edit: _e, ...rest } = prev
  return id ? { ...rest, [key]: id } : rest
}

/** Search for opening (or leaving) the permission editor of the open role. */
export function editSearch(prev: AdminSearch, on: boolean): AdminSearch {
  const { edit: _e, ...rest } = prev
  return on && prev.role ? { ...rest, edit: '1' } : rest
}
