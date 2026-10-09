/**
 * Fixture data for the chat area. Every id is a real-looking UUID so tests can
 * assert that none of them ever reaches the screen.
 */
import type { Channel, Message, ChatTask } from '../api/messaging'
import type { Contact } from '../hooks/useContacts'

export const UUID_RE = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i

export const WS_ID = '0a8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a6b'

export const U = {
  hoa: { id: '11111111-aaaa-4bbb-8ccc-000000000001', node: '22222222-aaaa-4bbb-8ccc-000000000001' },
  yen: { id: '11111111-aaaa-4bbb-8ccc-000000000002', node: '22222222-aaaa-4bbb-8ccc-000000000002' },
  lan: { id: '11111111-aaaa-4bbb-8ccc-000000000003', node: '22222222-aaaa-4bbb-8ccc-000000000003' },
  vinh: { id: '11111111-aaaa-4bbb-8ccc-000000000004', node: '22222222-aaaa-4bbb-8ccc-000000000004' },
  duc: { id: '11111111-aaaa-4bbb-8ccc-000000000005', node: '22222222-aaaa-4bbb-8ccc-000000000005' },
  ngoc: { id: '11111111-aaaa-4bbb-8ccc-000000000006', node: '22222222-aaaa-4bbb-8ccc-000000000006' },
}

export const ME = { id: U.hoa.id, username: 'hoa', ngac_node_id: U.hoa.node }

const contact = (
  key: keyof typeof U, username: string, display: string, title: string, department: string,
): Contact => ({
  user_id: U[key].id,
  ngac_node_id: U[key].node,
  username,
  display_name: display,
  email: `${username}@novapay.vn`,
  title,
  department,
  location: '',
  avatar_url: '',
  is_online: false,
})

export const CONTACTS: Contact[] = [
  contact('hoa', 'hoa', 'Lê Thị Hoa', 'Kế toán trưởng', ''),
  contact('yen', 'yen', 'Phạm Hải Yến', 'Chuyên viên', 'Vận hành thanh toán'),
  contact('lan', 'lan', 'Nguyễn Thu Lan', 'Chuyên viên', 'Đối soát'),
  contact('vinh', 'vinh', 'Lê Quang Vinh', 'Chuyên viên', 'Đối soát'),
  contact('duc', 'duc', 'Trần Minh Đức', 'Trưởng phòng', 'Vận hành thanh toán'),
  contact('ngoc', 'ngoc', 'Trần Bảo Ngọc', 'Chuyên viên', 'Kiểm soát'),
]

export const CH = {
  doisoat: '33333333-aaaa-4bbb-8ccc-000000000001',
  vanhanh: '33333333-aaaa-4bbb-8ccc-000000000002',
  hopdong: '33333333-aaaa-4bbb-8ccc-000000000003',
  dmYen: '33333333-aaaa-4bbb-8ccc-000000000004',
  dmLan: '33333333-aaaa-4bbb-8ccc-000000000005',
}

const space = (id: string, name: string, members: number): Channel => ({
  id, name, channel_type: 'workspace', workspace_id: WS_ID, member_count: members,
})

export const SPACES: Channel[] = [
  space(CH.doisoat, 'Đối soát giao dịch', 14),
  space(CH.vanhanh, 'Vận hành chung', 58),
  space(CH.hopdong, 'Hợp đồng đối tác', 9),
]

export const DMS: Channel[] = [
  { id: CH.dmYen, name: 'hoa, yen', channel_type: 'dm', workspace_id: '', member_count: 2 },
  { id: CH.dmLan, name: 'lan, hoa', channel_type: 'dm', workspace_id: '', member_count: 2 },
]

export const UNREAD = [
  { channel_id: CH.doisoat, unread_count: 3, last_read_message_id: '44444444-aaaa-4bbb-8ccc-000000000000' },
  { channel_id: CH.dmYen, unread_count: 1, last_read_message_id: '44444444-aaaa-4bbb-8ccc-000000000000' },
]

const ts = (h: number, m: number) => ({ seconds: Math.floor(new Date(2026, 9, 9, h, m).getTime() / 1000), nanos: 0 })

export const MSG = {
  t1: '44444444-aaaa-4bbb-8ccc-000000000001',
  t2: '44444444-aaaa-4bbb-8ccc-000000000002',
  r1: '44444444-aaaa-4bbb-8ccc-000000000003',
  r2: '44444444-aaaa-4bbb-8ccc-000000000004',
}

/** Newest first, as GET /channels/:id/messages returns them. */
export const MESSAGES: Message[] = [
  {
    id: MSG.t2, channel_id: CH.doisoat, sender_id: U.hoa.id, sender_name: 'hoa',
    content: 'Phí chuyển khoản liên ngân hàng hạch toán vào 6427. Mọi người cập nhật biên bản trước 15:00 nhé.',
    content_format: 'plain', created_at: ts(9, 24), reply_count: 0,
    reactions: [{ emoji: '👍', count: 4, user_ids: [U.hoa.id, U.lan.id, U.vinh.id, U.duc.id] }],
  },
  {
    id: MSG.t1, channel_id: CH.doisoat, sender_id: U.duc.id, sender_name: 'duc',
    content: 'File đối soát ngày 08/10 đã lên Tài liệu, lệch 3 giao dịch so với sao kê ngân hàng.',
    content_format: 'plain', created_at: ts(9, 12), reply_count: 2, reactions: [],
  },
]

/** GET /messages/:id/thread: parent first, then replies oldest first. */
export const THREAD: Message[] = [
  MESSAGES[1]!,
  {
    id: MSG.r1, channel_id: CH.doisoat, sender_id: U.lan.id, sender_name: 'lan', parent_message_id: MSG.t1,
    content: 'Tổng chênh lệch 1.284.500 ₫, cả ba đều chuyển sau 22:00.', content_format: 'plain', created_at: ts(9, 14),
  },
  {
    id: MSG.r2, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh', parent_message_id: MSG.t1,
    content: 'Đã đối chiếu xong, chênh lệch do phí liên ngân hàng, không phải lỗi hệ thống.', content_format: 'plain', created_at: ts(9, 20),
  },
]

export const MEMBERS = [
  { user_id: U.hoa.id, username: 'hoa', ngac_node_id: U.hoa.node },
  { user_id: U.duc.id, username: 'duc', ngac_node_id: U.duc.node },
  { user_id: U.vinh.id, username: 'vinh', ngac_node_id: U.vinh.node },
  { user_id: U.lan.id, username: 'lan', ngac_node_id: U.lan.node },
  { user_id: U.yen.id, username: 'yen', ngac_node_id: U.yen.node },
]

export const TASKS: ChatTask[] = [
  {
    id: '55555555-aaaa-4bbb-8ccc-000000000001', message_id: '44444444-aaaa-4bbb-8ccc-000000000009', channel_id: CH.doisoat,
    title: 'Cập nhật biên bản đối soát 08/10', assignee_id: U.vinh.id, assignee_name: 'vinh', status: 'todo',
    due_date: '2026-10-10', created_by: U.hoa.id, created_at: '2026-10-09T02:00:00Z', updated_at: '2026-10-09T02:00:00Z',
  },
]

export const DRIVE_ROOT = {
  items: [{
    id: '66666666-aaaa-4bbb-8ccc-000000000001', workspace_id: WS_ID, drive_context: 'channel', drive_context_id: CH.doisoat,
    parent_id: '', item_type: 'folder', name: 'Đối soát giao dịch', mime_type: '', size_bytes: 0, object_key: '',
    ngac_node_id: '77777777-aaaa-4bbb-8ccc-000000000001', owner_id: U.hoa.id, status: 'active',
    created_at: '2026-10-01T02:00:00Z', updated_at: '2026-10-01T02:00:00Z',
  }],
}

export const DRIVE_FOLDER = {
  items: [{
    id: '66666666-aaaa-4bbb-8ccc-000000000002', workspace_id: WS_ID, drive_context: 'workspace', drive_context_id: '',
    parent_id: DRIVE_ROOT.items[0]!.id, item_type: 'file', name: 'doi-soat-08-10.xlsx',
    mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size_bytes: 412 * 1024,
    object_key: 'k', ngac_node_id: '77777777-aaaa-4bbb-8ccc-000000000002', owner_id: U.duc.id, status: 'active',
    created_at: '2026-10-09T02:12:00Z', updated_at: '2026-10-09T02:12:00Z',
  }],
}

/**
 * Stand-in for `apiFetch`: answers each path the chat area calls with the
 * fixtures above. Unknown paths reject, which makes an unexpected call loud.
 */
export function fixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const p = path.split('?')[0]!
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))
  if (p === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
  if (p === `/workspaces/${WS_ID}/channels` && method === 'GET') return ok({ channels: SPACES })
  if (p === `/workspaces/${WS_ID}/contacts`) return ok({ contacts: CONTACTS, total: CONTACTS.length })
  if (p === '/dms' && method === 'GET') return ok({ channels: DMS })
  if (p === '/channels/unread') return ok({ channels: UNREAD })
  if (p === `/channels/${CH.doisoat}/messages` && method === 'GET') return ok({ messages: MESSAGES, has_more: false })
  if (p === `/channels/${CH.doisoat}/members` && method === 'GET') return ok({ members: MEMBERS })
  if (p === `/channels/${CH.doisoat}/tasks`) return ok({ tasks: TASKS })
  if (p === `/channels/${CH.doisoat}/pins`) return ok({ pins: [] })
  if (p === `/messages/${MSG.t1}/thread`) return ok({ messages: THREAD })
  if (p === `/workspaces/${WS_ID}/drive`) return ok(DRIVE_ROOT)
  if (p === `/drive/folders/${DRIVE_ROOT.items[0]!.id}`) return ok(DRIVE_FOLDER)
  if (p.endsWith('/read') && method === 'POST') return ok({})
  return Promise.reject(new Error(`fixtureApi: unexpected ${method} ${path}`))
}
