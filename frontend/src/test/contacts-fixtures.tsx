import { RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { ContactsScreen } from '../components/contacts/ContactsScreen'
import type { Contact } from '../hooks/useContacts'
import { validateContactsSearch } from '../lib/contacts-search'
import { validateWorkspaceSearch } from '../lib/workspace'
import { CH, CONTACTS, DMS, U, WS_ID } from './chat-fixtures'
import { renderWithClient } from './render'

/**
 * A company workspace for the tests of Danh bạ, as the API would return it: two departments called "Đối soát" in
 * different khối, people with roles, one private group with a member the directory hides, and a fake server that
 * records what was asked. Ids are UUID-shaped so a test can assert none reaches the screen.
 */

export const DM_ID = '33333333-aaaa-4bbb-8ccc-0000000000ff'
export const CALL_ID = '77777777-aaaa-4bbb-8ccc-000000000001'
export const HIDDEN_ID = '11111111-aaaa-4bbb-8ccc-0000000000ee'
const dept = (n: number) => `55555555-aaaa-4bbb-8ccc-0000000000${String(n).padStart(2, '0')}`
export const D = { kvh: dept(1), vhtt: dept(2), vhds: dept(3), htdt: dept(4), ktc: dept(5), tcds: dept(6), kt: dept(7) }

/** Khối Vận hành › Vận hành thanh toán › (Đối soát, Hỗ trợ đối tác); Khối Tài chính › (Đối soát, Kế toán): two Đối soát. */
export const DEPARTMENTS = [
  { id: D.kvh, name: 'Khối Vận hành', parent_id: '', member_count: 4 },
  { id: D.vhtt, name: 'Vận hành thanh toán', parent_id: D.kvh, member_count: 4 },
  { id: D.vhds, name: 'Đối soát', parent_id: D.vhtt, member_count: 2 },
  { id: D.htdt, name: 'Hỗ trợ đối tác', parent_id: D.vhtt, member_count: 0 },
  { id: D.ktc, name: 'Khối Tài chính', parent_id: '', member_count: 2 },
  { id: D.tcds, name: 'Đối soát', parent_id: D.ktc, member_count: 1 },
  { id: D.kt, name: 'Kế toán', parent_id: D.ktc, member_count: 1 },
]
const OWNERS = { id: 'role-owners', name: '', kind: 'owners' as const }
const MEMBERS = { id: 'role-members', name: '', kind: 'members' as const }
const PAY = { id: 'role-pay', name: 'Duyệt chi', kind: 'custom' as const }
const MATCHING = { id: 'role-match', name: 'Đối soát viên', kind: 'custom' as const }

const place = (c: Contact, over: Partial<Contact>): Contact => ({ ...structuredClone(c), ...over })
const byName = (name: string) => CONTACTS.find((c) => c.display_name === name)!
export const PEOPLE: Contact[] = [
  place(byName('Lê Thị Hoa'), { department: 'Kế toán', department_id: D.kt, roles: [MEMBERS, PAY] }),
  place(byName('Phạm Hải Yến'), { title: 'Chuyên viên vận hành', department: 'Vận hành thanh toán', department_id: D.vhtt, roles: [MEMBERS] }),
  place(byName('Nguyễn Thu Lan'), { title: 'Chuyên viên đối soát', department: 'Đối soát', department_id: D.vhds, roles: [MEMBERS, MATCHING], location: 'Hà Nội, tầng 12' }),
  place(byName('Lê Quang Vinh'), { title: 'Chuyên viên đối soát', department: 'Đối soát', department_id: D.vhds, roles: [MEMBERS, MATCHING] }),
  place(byName('Trần Minh Đức'), { department: 'Vận hành thanh toán', department_id: D.vhtt, roles: [MEMBERS, OWNERS, PAY] }),
  place(byName('Trần Bảo Ngọc'), { title: 'Chuyên viên đối soát', department: 'Đối soát', department_id: D.tcds, roles: [MEMBERS] }),
  {
    ...structuredClone(CONTACTS[0]!), user_id: '11111111-aaaa-4bbb-8ccc-0000000000aa', ngac_node_id: '22222222-aaaa-4bbb-8ccc-0000000000aa',
    username: 'bao', display_name: 'Ngô Gia Bảo', email: 'bao@novapay.vn', title: 'Cố vấn pháp lý', department: '', department_id: '', roles: [MEMBERS],
  },
]

export const GROUPS = [
  { id: CH.doisoat, name: 'Đối soát liên khối', channel_type: 'private', workspace_id: WS_ID, member_count: 3 },
  { id: CH.vanhanh, name: 'Vận hành chung', channel_type: 'workspace', workspace_id: WS_ID, member_count: 58 },
]
export const GROUP_MEMBERS = [
  { user_id: U.lan.id, username: 'lan', ngac_node_id: U.lan.node },
  { user_id: U.vinh.id, username: 'vinh', ngac_node_id: U.vinh.node },
  { user_id: HIDDEN_ID, username: 'hidden_thanh', ngac_node_id: '22222222-aaaa-4bbb-8ccc-0000000000ee' },
]

/** What the fake server answers; a test changes it, `resetServer` puts it back. */
export const server = {
  calls: [] as { method: string; path: string; body?: unknown }[],
  contacts: undefined as unknown as () => Promise<unknown>,
  departments: undefined as unknown as () => Promise<unknown>,
  workspaceType: undefined as 'personal' | undefined,
  activity: [] as unknown[],
}
export const calls = server.calls

export function resetServer() {
  server.calls.length = 0
  server.workspaceType = undefined
  server.activity = []
  server.contacts = directory()
  server.departments = () => Promise.resolve({ departments: structuredClone(DEPARTMENTS) })
}
export const said = (channelId: string, hoursAgo: number) => ({
  channel_id: channelId,
  latest: {
    id: `66666666-aaaa-4bbb-8ccc-00000000000${hoursAgo}`, sender_id: U.vinh.id, sender_name: 'vinh', preview: 'Chào chị', kind: 'text',
    created_at: new Date(Date.now() - hoursAgo * 3_600_000).toISOString(),
  },
})
export const directory = (people: Contact[] = PEOPLE, extra: object = {}) =>
  () => Promise.resolve({ contacts: structuredClone(people), total: people.length, ...extra })

export function fixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  server.calls.push({ method, path, ...(body ? { body } : {}) })
  if (path === '/workspaces') return Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành', ...(server.workspaceType ? { type: server.workspaceType } : {}) }] })
  if (path === `/workspaces/${WS_ID}/contacts/resolve`) {
    return Promise.resolve({ people: [{ user_id: HIDDEN_ID, ngac_node_id: '22222222-aaaa-4bbb-8ccc-0000000000ee', display_name: 'Nguyễn Hữu Thành', avatar_url: '' }] })
  }
  if (path.startsWith(`/workspaces/${WS_ID}/contacts`)) return server.contacts()
  if (path === `/workspaces/${WS_ID}/departments`) return server.departments()
  if (path === '/dms' && method === 'POST') return Promise.resolve({ id: DM_ID, channel_type: 'dm', name: 'dm' })
  if (path === `/channels/${DM_ID}/calls` && method === 'POST') {
    return Promise.resolve({ call: { id: CALL_ID, kind: 'audio', status: 'ringing', channel_id: DM_ID }, token: 't', url: 'wss://x' })
  }
  if (path === '/dms') return Promise.resolve({ channels: structuredClone(DMS) })
  if (path === `/workspaces/${WS_ID}/channels`) return Promise.resolve({ channels: structuredClone(GROUPS) })
  if (path === `/channels/${CH.doisoat}/members`) return Promise.resolve({ members: structuredClone(GROUP_MEMBERS) })
  if (path === '/channels/unread') return Promise.resolve({ channels: [] })
  if (path === '/channels/activity') return Promise.resolve({ channels: structuredClone(server.activity) })
  return Promise.reject(new Error(`unexpected ${method} ${path}`))
}

/** `before` is an address the person was at just before: Danh bạ is then not the first entry of the history. */
export async function renderContacts(url = '/contacts', before?: string) {
  const root = createRootRoute()
  const contacts = createRoute({ getParentRoute: () => root, path: '/contacts', validateSearch: validateContactsSearch, component: ContactsScreen })
  const channel = createRoute({ getParentRoute: () => root, path: '/channels/$channelId', validateSearch: validateWorkspaceSearch, component: () => <p>Cuộc trò chuyện</p> })
  const call = createRoute({ getParentRoute: () => root, path: '/calls/$callId', validateSearch: (s: Record<string, unknown>) => s, component: () => <p>Cuộc gọi</p> })
  const settings = createRoute({ getParentRoute: () => root, path: '/settings', validateSearch: validateWorkspaceSearch, component: () => <p>Cài đặt</p> })
  const router = createRouter({
    routeTree: root.addChildren([contacts, channel, call, settings]),
    history: createMemoryHistory(before ? { initialEntries: [before, url], initialIndex: 1 } : { initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

