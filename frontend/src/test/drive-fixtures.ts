/**
 * Fixture data for the drive. Every id is a real-looking UUID so tests can
 * assert that none of them ever reaches the screen. Shapes follow what the
 * REST layer returns: empty fields are omitted, an empty folder is `{}`, and
 * a folder's owner is an NGAC node id while a file's is a user id.
 */
import type { DriveItem, DriveShare } from '../api/drive'
import { CONTACTS, U, WS_ID } from './chat-fixtures'

export { UUID_RE, U, WS_ID } from './chat-fixtures'

export const D = {
  doisoat: '66666666-aaaa-4bbb-8ccc-0000000000a1',
  y2026: '66666666-aaaa-4bbb-8ccc-0000000000a2',
  thang10: '66666666-aaaa-4bbb-8ccc-0000000000a3',
  hopdong: '66666666-aaaa-4bbb-8ccc-0000000000a4',
  trong: '66666666-aaaa-4bbb-8ccc-0000000000a5',
  xlsx: '66666666-aaaa-4bbb-8ccc-0000000000b1',
  sakePdf: '66666666-aaaa-4bbb-8ccc-0000000000b2',
  bienban: '66666666-aaaa-4bbb-8ccc-0000000000b3',
  huongdan: '66666666-aaaa-4bbb-8ccc-0000000000b4',
  hopdongPdf: '66666666-aaaa-4bbb-8ccc-0000000000b5',
  chiaSe: '66666666-aaaa-4bbb-8ccc-0000000000b6',
}

const ts = (day: number, h: number, m: number) => ({
  seconds: Math.floor(new Date(2026, 9, day, h, m).getTime() / 1000),
  nanos: 0,
})

const XLSX_MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'

function item(
  id: string, name: string, type: 'file' | 'folder', owner: string, parent: string | undefined,
  extra: Partial<DriveItem> = {},
): DriveItem {
  return {
    id, workspace_id: WS_ID, drive_context: 'workspace', item_type: type, name,
    ngac_node_id: `77777777-aaaa-4bbb-8ccc-${id.slice(-12)}`, owner_id: owner, status: 'active',
    created_at: ts(8, 9, 12), updated_at: ts(8, 9, 12), ...(parent ? { parent_id: parent } : {}), ...extra,
  }
}

export const ITEMS: Record<string, DriveItem> = {
  [D.doisoat]: item(D.doisoat, 'Đối soát', 'folder', U.lan.node, undefined),
  [D.y2026]: item(D.y2026, '2026', 'folder', U.lan.node, D.doisoat),
  [D.thang10]: item(D.thang10, 'Tháng 10', 'folder', U.vinh.node, D.y2026),
  [D.hopdong]: item(D.hopdong, 'Hợp đồng', 'folder', U.hoa.node, undefined),
  [D.trong]: item(D.trong, 'Thư mục trống', 'folder', U.vinh.node, undefined),
  [D.xlsx]: item(D.xlsx, 'doi-soat-08-10.xlsx', 'file', U.duc.id, D.doisoat, {
    mime_type: XLSX_MIME, size_bytes: 412 * 1024, updated_at: ts(9, 9, 12),
  }),
  [D.sakePdf]: item(D.sakePdf, 'sao-ke-ngan-hang-10-2026.pdf', 'file', U.yen.id, D.doisoat, {
    mime_type: 'application/pdf', size_bytes: 1.2 * 1024 * 1024, updated_at: ts(8, 16, 5),
  }),
  [D.bienban]: item(D.bienban, 'bien-ban-chenh-lech.docx', 'file', U.vinh.id, D.doisoat, {
    mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', size_bytes: 86 * 1024,
  }),
  [D.huongdan]: item(D.huongdan, 'huong-dan.pdf', 'file', U.yen.id, undefined, {
    mime_type: 'application/pdf', size_bytes: 300 * 1024,
  }),
  [D.hopdongPdf]: item(D.hopdongPdf, 'hop-dong-doi-tac.pdf', 'file', U.hoa.id, D.hopdong, {
    mime_type: 'application/pdf', size_bytes: 2 * 1024 * 1024,
  }),
  // Owned by someone outside the workspace: only the server can name them.
  [D.chiaSe]: item(D.chiaSe, 'ke-hoach-quy-4.xlsx', 'file', 'ngoai-workspace-user', undefined, {
    mime_type: XLSX_MIME, size_bytes: 90 * 1024, owner_name: 'Phạm Văn Ngoài',
  }),
}

/** Items the signed-in user may only read (no write, no share). */
export const READ_ONLY = new Set([D.huongdan])

/** Items trashed during a test; the server stops listing them. */
const trashed = new Set<string>()
export const resetFixtures = () => trashed.clear()

const children = (parent: string | undefined) =>
  Object.values(ITEMS).filter((i) => i.parent_id === parent && i.id !== D.chiaSe && !trashed.has(i.id))

const crumbs = (...ids: string[]) => ids.map((id) => ({ id, name: ITEMS[id]!.name }))
const CRUMBS: Record<string, { id: string; name: string }[]> = {
  [D.doisoat]: crumbs(D.doisoat),
  [D.y2026]: crumbs(D.doisoat, D.y2026),
  [D.thang10]: crumbs(D.doisoat, D.y2026, D.thang10),
  [D.hopdong]: crumbs(D.hopdong),
  [D.trong]: crumbs(D.trong),
}

/** The server omits `items` for an empty folder, so a listing may be `{}`. */
const listing = (parent: string | undefined, breadcrumb?: { id: string; name: string }[]) => {
  const items = children(parent)
  return { ...(items.length ? { items } : {}), ...(breadcrumb ? { breadcrumb } : {}) }
}

export const SHARES: Record<string, DriveShare[]> = {
  [D.xlsx]: [
    {
      id: '88888888-aaaa-4bbb-8ccc-000000000001', drive_item_id: D.xlsx, share_type: 'user',
      target_ngac_id: U.lan.node, target_label: `${U.lan.node}_Owners`, operations: ['write'], created_at: ts(9, 9, 13),
    },
    {
      id: '88888888-aaaa-4bbb-8ccc-000000000002', drive_item_id: D.xlsx, share_type: 'user',
      target_ngac_id: U.hoa.node, target_label: U.hoa.node, operations: ['read'], created_at: ts(9, 9, 14),
    },
  ],
}

export const QUOTA = {
  workspace_id: WS_ID, max_bytes: 5 * 1024 ** 3, used_bytes: 2.5 * 1024 ** 3, max_files: 10000, used_files: 12,
}

export interface ApiCall { method: string; path: string; body?: unknown }
export const calls: ApiCall[] = []

/**
 * A failed request with a status. `api/client` is imported at call time: the
 * tests replace that module with a factory that itself loads this file, and a
 * static import back into it would wait on itself forever.
 */
const fail = (message: string, status: number): Promise<never> =>
  import('../api/client').then(({ ApiError }) => { throw new ApiError(message, status) })

/**
 * Stand-in for `apiFetch` for the drive screens. Unknown paths reject, which
 * makes an unexpected call loud. Every call is recorded in `calls`.
 */
export function driveFixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const p = path.split('?')[0]!
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path: p, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (p === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
  if (p === `/workspaces/${WS_ID}/contacts`) return ok({ contacts: CONTACTS, total: CONTACTS.length })
  if (p === `/workspaces/${WS_ID}/drive/quota`) return ok(QUOTA)
  if (p === `/workspaces/${WS_ID}/drive` && method === 'GET') return ok(listing(undefined))
  if (p === `/workspaces/${WS_ID}/drive/folders` && method === 'POST') {
    return ok({ ...ITEMS[D.hopdong], id: 'new-folder', name: body.name })
  }
  if (p === `/workspaces/${WS_ID}/drive/files` && method === 'POST') {
    return ok({ file_id: 'new-file', upload_url: 'https://storage.test/put', object_key: 'k' })
  }
  if (p === '/drive/shared-with-me') return ok({ items: [ITEMS[D.chiaSe]] })
  if (p === '/drive/batch-access') {
    const results: Record<string, Record<string, boolean>> = {}
    const readOnlyNodes = new Set([...READ_ONLY].map((id) => ITEMS[id]!.ngac_node_id))
    for (const id of body.object_ids as string[]) {
      const allow = !readOnlyNodes.has(id)
      results[id] = Object.fromEntries((body.operations as string[]).map((op) => [op, op === 'read' || allow]))
    }
    return ok({ results })
  }

  let m = /^\/drive\/folders\/([^/]+)$/.exec(p)
  if (m) {
    const id = m[1]!
    if (!ITEMS[id] || ITEMS[id]!.item_type !== 'folder') return fail('not found', 404)
    return ok(listing(id, CRUMBS[id]))
  }
  m = /^\/drive\/items\/([^/]+)\/shares$/.exec(p)
  if (m) return ok(SHARES[m[1]!] ? { shares: SHARES[m[1]!] } : {})
  m = /^\/drive\/items\/([^/]+)\/restore$/.exec(p)
  if (m) { trashed.delete(m[1]!); return ok(ITEMS[m[1]!]) }
  m = /^\/drive\/items\/([^/]+)\/(share|move|rename)$/.exec(p)
  if (m && ITEMS[m[1]!]) return ok(ITEMS[m[1]!])
  m = /^\/drive\/items\/([^/]+)$/.exec(p)
  if (m && method === 'GET') {
    return ITEMS[m[1]!] ? ok(ITEMS[m[1]!]) : fail('not found', 404)
  }
  if (m && method === 'DELETE') { trashed.add(m[1]!); return ok({ status: 'trashed' }) }
  if (/^\/drive\/shares\/[^/]+$/.test(p) && method === 'DELETE') return ok({ status: 'revoked' })
  if (/^\/drive\/files\/[^/]+\/confirm$/.test(p)) return ok(ITEMS[D.xlsx])
  if (/^\/drive\/files\/[^/]+\/download$/.test(p)) return ok({ download_url: 'https://files.test/download?sig=1' })
  return Promise.reject(new Error(`driveFixtureApi: unexpected ${method} ${path}`))
}
