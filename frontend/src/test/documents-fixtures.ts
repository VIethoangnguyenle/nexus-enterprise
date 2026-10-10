/**
 * Fixture data for Văn bản. Ids are UUIDs so tests can assert none reaches the
 * screen. The fake server keeps real state (versions, content), so a save that
 * ignores the version precondition behaves as it would against the service.
 */
import type { TextDocument, TextScope, TextStatus } from '../api/documents'
import { driveFixtureApi } from './drive-fixtures'
import { CONTACTS, U, WS_ID } from './chat-fixtures'

export { UUID_RE, U, WS_ID } from './chat-fixtures'

export const T = {
  quytrinh: '99999999-aaaa-4bbb-8ccc-000000000001',
  sla: '99999999-aaaa-4bbb-8ccc-000000000002',
  nhap: '99999999-aaaa-4bbb-8ccc-000000000003',
  luutru: '99999999-aaaa-4bbb-8ccc-000000000004',
  readonly: '99999999-aaaa-4bbb-8ccc-000000000005',
  fresh: '99999999-aaaa-4bbb-8ccc-0000000000ff',
}

const at = (day: number, h: number, m: number) => new Date(2026, 9, day, h, m).toISOString()

function doc(
  id: string, title: string, owner: keyof typeof U, status: TextStatus, extra: Partial<TextDocument> = {},
): TextDocument {
  return {
    id, workspace_id: WS_ID, title, content: `<p>Nội dung ${title}</p>`, version: 1, status,
    owner_id: U[owner].id, owner_name: CONTACTS.find((c) => c.user_id === U[owner].id)!.display_name,
    can_write: owner === 'hoa', created_at: at(8, 9, 0), updated_at: at(10, 9, 41), ...extra,
  }
}

const initial = (): Record<string, TextDocument> => ({
  [T.quytrinh]: doc(T.quytrinh, 'Quy trình đối soát cuối ngày', 'hoa', 'draft', {
    content: '<h2>1. Chuẩn bị số liệu</h2><p>Trước 16:30, tải sao kê từ cổng ngân hàng.</p><p>Đặt vào thư mục Đối soát.</p>',
  }),
  [T.sla]: doc(T.sla, 'Cam kết SLA với ngân hàng đối tác', 'duc', 'active', { updated_at: at(9, 15, 0), can_write: false }),
  [T.nhap]: doc(T.nhap, 'Mẫu biên bản chênh lệch', 'hoa', 'draft', { updated_at: at(8, 8, 0) }),
  [T.luutru]: doc(T.luutru, 'Quy chế tạm ứng 2025', 'lan', 'archived', { updated_at: at(1, 8, 0), can_write: true }),
})

export let DOCS = initial()
export const calls: { method: string; path: string; body?: Record<string, unknown> }[] = []

/** Hooks a test into the next save, to simulate someone else saving first. */
export const hooks: {
  beforeSave?: (id: string) => void
  /** Fails the next listing of this scope, or the next read of a document (scope 'document'). */
  failNext?: { scope: TextScope | 'document'; make: () => Promise<never> }
} = {}

export function resetDocs() {
  DOCS = initial()
  calls.length = 0
  hooks.beforeSave = undefined
  hooks.failNext = undefined
}

const fail = (message: string, status: number, body?: unknown): Promise<never> =>
  import('../api/client').then(({ ApiError }) => { throw new ApiError(message, status, body) })

/** What the service does to a save: the version precondition, then the change. */
function applySave(id: string, body: Record<string, unknown>): Promise<unknown> {
  const d = DOCS[id]
  if (!d || !d.can_write) return fail('access denied', 403)
  hooks.beforeSave?.(id)
  if (body.base_version !== d.version) {
    return fail('conflict', 409, { error: 'Văn bản đã được sửa ở nơi khác', reason: 'version_conflict', current: structuredClone(d) })
  }
  if (typeof body.title === 'string') d.title = body.title.trim()
  if (typeof body.content === 'string') d.content = body.content
  if (typeof body.status === 'string') d.status = body.status as TextStatus
  d.version += 1
  d.updated_at = new Date().toISOString()
  return Promise.resolve(structuredClone(d))
}

function listing(scope: TextScope | null): TextDocument[] {
  const all = Object.values(DOCS).map((d) => ({ ...structuredClone(d), content: undefined }))
  const me = U.hoa.id
  const pick = scope === 'mine' ? all.filter((d) => d.owner_id === me)
    : scope === 'drafts' ? all.filter((d) => d.owner_id === me && d.status === 'draft')
    : scope === 'shared' ? all.filter((d) => d.owner_id !== me)
    : all
  return pick.sort((a, b) => b.updated_at.localeCompare(a.updated_at)).map(({ content: _c, ...rest }) => rest as TextDocument)
}

export function docsFixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const [p, qs] = path.split('?') as [string, string | undefined]
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (p === `/workspaces/${WS_ID}/documents/texts`) {
    calls.push({ method, path: path, ...(body ? { body } : {}) })
    if (method === 'GET') {
      const scope = new URLSearchParams(qs).get('scope') as TextScope | null
      if (hooks.failNext?.scope === (scope ?? 'all')) { const f = hooks.failNext.make; hooks.failNext = undefined; return f() }
      return ok({ documents: listing(scope) })
    }
    if (method === 'POST') {
      const d = doc(T.fresh, body.title || 'Văn bản chưa đặt tên', 'hoa', 'draft', { content: '', updated_at: new Date().toISOString() })
      DOCS[T.fresh] = d
      return ok(d)
    }
  }
  if (p === `/workspaces/${WS_ID}/documents/texts/count`) {
    calls.push({ method, path })
    return ok({ count: listing(new URLSearchParams(qs).get('scope') as TextScope | null).length })
  }
  const m = /^\/documents\/texts\/([^/]+)$/.exec(p)
  if (m) {
    const id = m[1]!
    calls.push({ method, path: p, ...(body ? { body } : {}) })
    if (method === 'GET') {
      if (hooks.failNext?.scope === 'document') { const f = hooks.failNext.make; hooks.failNext = undefined; return f() }
      return DOCS[id] ? ok(DOCS[id]) : fail('access denied', 403)
    }
    if (method === 'PATCH') return applySave(id, body)
    if (method === 'DELETE') {
      if (!DOCS[id]?.can_write) return fail('access denied', 403)
      delete DOCS[id]
      return ok({})
    }
  }
  return driveFixtureApi(path, init)
}
