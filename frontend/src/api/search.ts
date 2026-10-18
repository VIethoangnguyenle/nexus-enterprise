import { apiFetch, ApiError } from './client'
import type { ApprovalRequest } from './approval'
import type { DriveItem } from './drive'

/** Query string from the parameters that are set; none set gives none. */
function query(params: Record<string, string | number | undefined>): string {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === '') continue
    qs.set(k, String(v))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

// --- Tài liệu ---------------------------------------------------------------------------------

export interface DriveSearchResult {
  /** Only items the caller may read, of the named workspace only. */
  items?: DriveItem[]
}

/**
 * Drive items of one workspace whose name holds the words (`GET /api/drive/search`). The drive refuses a workspace
 * the caller is not a member of (403) and checks read on every item it returns.
 */
export function searchDrive(workspaceId: string, q: string, limit: number, signal?: AbortSignal) {
  return apiFetch<DriveSearchResult>(`/drive/search${query({ workspace_id: workspaceId, q, limit })}`, { signal })
}

// --- Đề nghị ----------------------------------------------------------------------------------

export type ApprovalStatus = ApprovalRequest['status']

/** What the approval search can be narrowed by. Ids come from pickers and are never shown. */
export interface ApprovalSearchFilters {
  status?: ApprovalStatus
  /** A template id: the request's Loại. */
  template_id?: string
  /** The requester's user id. */
  created_by?: string
  /** First day, `YYYY-MM-DD` in the reader's zone. */
  from?: string
  /** Last day, inclusive, `YYYY-MM-DD` in the reader's zone. */
  to?: string
}

export interface ApprovalSearchResult {
  items: ApprovalRequest[]
  /** Matches the caller may read, across every page. */
  total: number
}

const NO_APPROVALS: ApprovalSearchResult = { items: [], total: 0 }

/**
 * Requests of the signed-in workspace the caller may read whose title, template or requester holds the words
 * (`GET /api/approval/search`; the workspace is the session's tenant, as on every approval route). Until the
 * approval service serves that route it answers 404 (or 501): that is "nothing found", not an error to show.
 */
export async function searchApprovals(
  q: string, filters: ApprovalSearchFilters, limit: number, signal?: AbortSignal,
): Promise<ApprovalSearchResult> {
  try {
    const res = await apiFetch<Partial<ApprovalSearchResult>>(`/approval/search${query({ q, limit, ...filters })}`, { signal })
    return { items: res.items ?? [], total: res.total ?? res.items?.length ?? 0 }
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 501)) return NO_APPROVALS
    throw err
  }
}
