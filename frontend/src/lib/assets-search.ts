import { ASSET_STATES, REQUEST_FILTERS, type RequestFilter } from './asset-model'
import { validateWorkspaceSearch } from './workspace'

export type AssetsTab = 'overview' | 'list' | 'requests' | 'types'
const TABS: AssetsTab[] = ['overview', 'list', 'requests', 'types']

/**
 * What the Tài sản URL carries: the workspace (`ws`), the tab, the open asset,
 * request or type, the list's filters and page, the request filter, and the
 * new-request form. Keeping them here, not in component state, is what makes
 * reload, Back/Forward and a pasted link land in the same place. The first tab
 * and the first page are the absence of a parameter.
 */
export interface AssetsSearch {
  ws?: string
  /**
   * The tab. Named `section`, not `tab`: Phê duyệt's `?tab=` has other values, and the router
   * types `prev` in a search reducer as the union of every route's search.
   */
  section?: Exclude<AssetsTab, 'overview'>
  asset?: string
  request?: string
  type?: string
  state?: string
  /** Asset type filter on the list: a type id. Never shown. */
  kind?: string
  q?: string
  page?: number
  show?: RequestFilter
  compose?: 'request'
}

const str = (v: unknown) => (typeof v === 'string' && v ? v : undefined)

/** `validateSearch` for the route: unknown or malformed values are dropped, not rejected. */
export function validateAssetsSearch(search: Record<string, unknown>): AssetsSearch {
  const out: AssetsSearch = { ...validateWorkspaceSearch(search) }
  if (TABS.includes(search.section as AssetsTab) && search.section !== 'overview') out.section = search.section as AssetsSearch['section']
  for (const key of ['asset', 'request', 'type', 'kind'] as const) {
    const v = str(search[key])
    if (v) out[key] = v
  }
  if (ASSET_STATES.includes(search.state as (typeof ASSET_STATES)[number])) out.state = search.state as string
  if (typeof search.q === 'string' && search.q.trim()) out.q = search.q
  const page = Number(search.page)
  if (Number.isInteger(page) && page > 1) out.page = page
  if (REQUEST_FILTERS.some((f) => f.id === search.show) && search.show !== 'pending') out.show = search.show as RequestFilter
  if (search.compose === 'request') out.compose = 'request'
  return out
}

export const tabOf = (s: AssetsSearch): AssetsTab => s.section ?? 'overview'

/** Search for moving to a tab: nothing open, no filters, workspace kept. */
export function tabSearch(prev: AssetsSearch, tab: AssetsTab): AssetsSearch {
  const { ws } = prev
  const base: AssetsSearch = ws ? { ws } : {}
  return tab === 'overview' ? base : { ...base, section: tab }
}

/** Opens (or, with no id, closes) an asset on the list, keeping the list's filters. */
export function assetSearch(prev: AssetsSearch, id?: string): AssetsSearch {
  const { asset: _a, request: _r, type: _t, compose: _c, show: _s, ...rest } = prev
  return id ? { ...rest, section: 'list', asset: id } : rest
}

/** Opens (or closes) a request on the requests tab, keeping its filter. */
export function requestSearch(prev: AssetsSearch, id?: string): AssetsSearch {
  const { request: _r, compose: _c, ...rest } = prev
  return id ? { ...rest, request: id } : rest
}

/** Opens (or closes) an asset type on the types tab. */
export function typeSearch(prev: AssetsSearch, id?: string): AssetsSearch {
  const { type: _t, ...rest } = prev
  return id ? { ...rest, type: id } : rest
}

/** Opens (or, with `open === false`, closes) the new-request form on the requests tab. */
export function composeSearch(prev: AssetsSearch, open = true): AssetsSearch {
  const { request: _r, compose: _c, ...rest } = prev
  return open ? { ...rest, section: 'requests', compose: 'request' } : rest
}

/** Changes the list's filters. A new filter starts again from the first page. */
export function filterSearch(
  prev: AssetsSearch,
  next: { state?: string | undefined; kind?: string | undefined; q?: string | undefined },
): AssetsSearch {
  const { page: _p, state: _s, kind: _k, q: _q, ...rest } = prev
  const out: AssetsSearch = { ...rest }
  const state = 'state' in next ? next.state : prev.state
  const kind = 'kind' in next ? next.kind : prev.kind
  const q = 'q' in next ? next.q : prev.q
  if (state) out.state = state
  if (kind) out.kind = kind
  if (q?.trim()) out.q = q
  return out
}

export function pageSearch(prev: AssetsSearch, page: number): AssetsSearch {
  const { page: _p, ...rest } = prev
  return page > 1 ? { ...rest, page } : rest
}

/** Changes the request filter. "Đang chờ" is the default and is not written. */
export function showSearch(prev: AssetsSearch, show: RequestFilter): AssetsSearch {
  const { show: _s, request: _r, compose: _c, ...rest } = prev
  return show === 'pending' ? rest : { ...rest, show }
}

/**
 * The search an old /assets URL maps to. The section pages became tabs of one
 * screen; `/assets/<id>` opens that asset on the list; the old "new request"
 * page became the form beside the request table.
 */
export function legacyAssetsRedirect(
  from: 'dashboard' | 'list' | 'requests' | 'types' | 'new-request' | { asset: string },
  prev: { ws?: string },
): AssetsSearch {
  const base: AssetsSearch = prev.ws ? { ws: prev.ws } : {}
  if (typeof from === 'object') return { ...base, section: 'list', asset: from.asset }
  switch (from) {
    case 'dashboard': return base
    case 'new-request': return { ...base, section: 'requests', compose: 'request' }
    default: return { ...base, section: from }
  }
}
