import { ApiError, apiFetch } from './client'

/** Where a text document stands. */
export type TextStatus = 'draft' | 'active' | 'archived'

/** Which documents a listing shows. */
export type TextScope = 'all' | 'mine' | 'drafts' | 'shared'

/**
 * A document written in the app. Ids are for calling the API and picking a
 * colour; the screen shows `title` and `owner_name`. A listing omits `content`.
 */
export interface TextDocument {
  id: string
  workspace_id: string
  folder_id?: string
  title: string
  content?: string
  /** Moves up by one on every save; a save names the version it was based on. */
  version: number
  status: TextStatus
  owner_id: string
  owner_name: string
  can_write?: boolean
  created_at: string
  updated_at: string
}

export interface TextSave {
  base_version: number
  title?: string
  content?: string
  status?: TextStatus
}

/** The 409 body of a save refused because the document moved on. */
export interface VersionConflictBody {
  reason: 'version_conflict'
  current: TextDocument
}

/** The document as it now stands, when `err` is a refused-stale-save; otherwise null. */
export function conflictOf(err: unknown): TextDocument | null {
  if (!(err instanceof ApiError) || err.status !== 409) return null
  const body = err.body as Partial<VersionConflictBody> | undefined
  return body?.reason === 'version_conflict' && body.current ? body.current : null
}

/** One page of a listing; `next` is what to ask for after it, absent at the end. */
export interface TextPage {
  documents: TextDocument[]
  next?: string
}

export const documentApi = {
  list: async (wsId: string, scope: TextScope = 'all', cursor?: string): Promise<TextPage> => {
    const params = new URLSearchParams()
    if (scope !== 'all') params.set('scope', scope)
    if (cursor) params.set('cursor', cursor)
    const qs = params.toString()
    const res = await apiFetch<{ documents?: TextDocument[]; next_cursor?: string }>(
      `/workspaces/${wsId}/documents/texts${qs ? `?${qs}` : ''}`,
    )
    return { documents: res.documents ?? [], next: res.next_cursor || undefined }
  },

  /** How many documents of a listing the caller may read, without loading them. */
  count: async (wsId: string, scope: TextScope = 'all'): Promise<number> => {
    const qs = scope === 'all' ? '' : `?scope=${scope}`
    const res = await apiFetch<{ count?: number }>(`/workspaces/${wsId}/documents/texts/count${qs}`)
    return res.count ?? 0
  },

  get: (id: string) => apiFetch<TextDocument>(`/documents/texts/${id}`),

  create: (wsId: string, body: { title?: string; folder_id?: string }) =>
    apiFetch<TextDocument>(`/workspaces/${wsId}/documents/texts`, { method: 'POST', body: JSON.stringify(body) }),

  save: (id: string, body: TextSave) =>
    apiFetch<TextDocument>(`/documents/texts/${id}`, { method: 'PATCH', body: JSON.stringify(body) }),

  remove: (id: string) => apiFetch<unknown>(`/documents/texts/${id}`, { method: 'DELETE' }),
}
