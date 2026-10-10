import { validateWorkspaceSearch } from './workspace'

export type ApprovalTab = 'pending' | 'mine' | 'history' | 'department' | 'templates'
export const APPROVAL_TABS: ApprovalTab[] = ['pending', 'mine', 'history', 'department', 'templates']

/**
 * What the Phê duyệt URL carries: the workspace (`ws`), the tab, the open
 * request or template (`request`, `template`) and the template builder
 * (`edit`: a template id, or `new`). Keeping them here, not in component
 * state, is what makes reload, Back/Forward and a pasted link land in place.
 */
export interface ApprovalSearch {
  ws?: string
  tab?: ApprovalTab
  request?: string
  template?: string
  edit?: string
}

/** `validateSearch` for the route: unknown or malformed values are dropped, not rejected. */
export function validateApprovalSearch(search: Record<string, unknown>): ApprovalSearch {
  const out: ApprovalSearch = { ...validateWorkspaceSearch(search) }
  if (APPROVAL_TABS.includes(search.tab as ApprovalTab) && search.tab !== 'pending') out.tab = search.tab as ApprovalTab
  for (const key of ['request', 'template', 'edit'] as const) {
    const v = search[key]
    if (typeof v === 'string' && v) out[key] = v
  }
  return out
}

/** The tab shown for this search (the first one when the URL names none). */
export const tabOf = (s: ApprovalSearch): ApprovalTab => s.tab ?? 'pending'

/** Search for moving to a tab: nothing open, builder closed, workspace kept. */
export function tabSearch(prev: ApprovalSearch, tab: ApprovalTab): ApprovalSearch {
  const { tab: _t, request: _r, template: _m, edit: _e, ...rest } = prev
  return tab === 'pending' ? rest : { ...rest, tab }
}

/** Search for opening (or, with no id, closing) a request on the current tab. */
export function requestSearch(prev: ApprovalSearch, id?: string): ApprovalSearch {
  const { request: _r, ...rest } = prev
  return id ? { ...rest, request: id } : rest
}

export function templateSearch(prev: ApprovalSearch, id?: string): ApprovalSearch {
  const { template: _m, ...rest } = prev
  return id ? { ...rest, template: id } : rest
}

/** Search for opening (or, with no id, leaving) the template builder. */
export function editSearch(prev: ApprovalSearch, id?: string): ApprovalSearch {
  const { edit: _e, template: _m, ...rest } = prev
  return id ? { ...rest, tab: 'templates', edit: id } : rest
}
