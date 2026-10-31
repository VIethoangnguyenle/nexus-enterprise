import { vi } from 'vitest'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { ApiError } from '../../api/client'
import { renderWithClient } from '../../test/render'
import { validateSettingsSearch } from '../../lib/settings-search'
import { validateWorkspaceSearch } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { SettingsScreen } from './SettingsScreen'
import { SettingsSectionScreen } from './SettingsSectionScreen'

/** One fake server for the settings pages' tests: what each service answers, and every write it was sent. */

export const WS = '7a1c2d3e-4f50-4a6b-8c7d-9e0f1a2b3c4d'
export const SESSION_HERE = '1b2c3d4e-5f60-4718-89ab-cdef01234567'
export const SESSION_OTHER = '2c3d4e5f-6071-4829-9abc-def012345678'
export const CAL_MINE = '3d4e5f60-7182-493a-abcd-ef0123456789'
export const CAL_TEAM = '4e5f6071-8293-4a4b-bcde-f01234567890'
/** Where a picture the server keeps is read from. */
export const AVATAR_URL = '/api/auth/avatars/0123456789abcdef0123456789abcdef'

const fresh = () => ({
  personal: false,
  canManage: true,
  role: 'owner',
  groupCreation: 'everyone' as 'everyone' | 'admins',
  ownerNames: ['Lê Thị Hoa', 'Trần Minh Đức'],
  /** Set to make every PUT of a module's settings fail. */
  settingsDown: false,
  /** The person's own record (`/me`); a PATCH of the profile changes it. */
  profile: { display_name: 'Lê Thị Hoa', title: 'Kế toán trưởng', location: '', avatar_url: '' },
  /** The department an administrator assigned here; an organization only. */
  department: 'Phòng Tài chính',
  meDown: false,
  profileFails: null as ApiError | null,
  avatarFails: null as ApiError | null,
  removeFails: null as ApiError | null,
  /** The workspace's name once a save of the details changed it; null is the default name for its kind. */
  workspaceName: null as string | null,
  description: 'Thanh toán số',
  /** More of the details answer (the type, who converted it), laid over the rest. */
  detailsExtra: {} as Record<string, unknown>,
  detailsDown: false,
  detailsFails: null as ApiError | null,
  leaveFails: null as ApiError | null,
  drive: { view: 'auto', sort: 'modified', folders_first: true, cellular_previews: true, notify_shared: true } as Record<string, unknown>,
  approval: { default_tab: 'pending', sort: 'newest', notify_pending: true, notify_result: true } as Record<string, unknown>,
  asset: { default_tab: 'overview', sort: 'name', notify_requests: true, notify_maintenance: true } as Record<string, unknown>,
  chat: { enter_to_send: false, link_preview: true, open_at: 'all', sort: 'activity', double_tap: '👍', default_level: null } as Record<string, unknown>,
  calendar: {
    tzid: 'Asia/Ho_Chi_Minh', workStart: '08:00', workEnd: '17:30', defaultReminder: 10, defaultView: 'agenda', weekStart: 1,
    visibleCalendars: [], defaultDuration: 30, notifyInvites: true, notifyChanges: true, notifyReminders: true,
  } as Record<string, unknown>,
  calendars: [
    { id: CAL_MINE, kind: 'personal', name: 'Lê Thị Hoa', color: 7, visible: true, access: 'manage', isMine: true },
    { id: CAL_TEAM, kind: 'workspace', name: 'NovaPay', color: -1, visible: true, access: 'read', isMine: false },
  ],
  push: {
    kinds: { direct: true, mention: true, thread_reply: true, incoming_call: true },
    quiet_hours: { enabled: true, start: '22:00', end: '07:00', time_zone: 'Asia/Ho_Chi_Minh' },
    pause_until: null as string | null,
  },
  sessions: [
    { session_id: SESSION_HERE, device: 'Safari · iPhone', started_at: '2026-10-18T02:00:00Z', current: true },
    { session_id: SESSION_OTHER, device: 'Chrome · Windows', started_at: '2026-10-10T01:00:00Z', current: false },
  ],
  /** Every write: method, path and body. */
  writes: [] as Array<{ method: string; path: string; body: unknown }>,
})

export let server = fresh()
export function resetSettingsServer() { server = fresh() }

const ok = (v: unknown) => Promise.resolve(structuredClone(v))
const fail = (status: number) => Promise.reject(new ApiError(`HTTP ${status}`, status))

export function settingsAnswer(path: string, init?: RequestInit): Promise<unknown> {
  const p = path.split('?')[0]!
  const method = init?.method ?? 'GET'
  const body = init?.body ? JSON.parse(String(init.body)) : undefined
  if (method !== 'GET') server.writes.push({ method, path, body })

  if (p === '/workspaces') {
    return ok({ workspaces: [{ id: WS, name: server.personal ? 'Lê Thị Hoa' : 'NovaPay', type: server.personal ? 'personal' : 'organization' }] })
  }
  if (p === '/me/workspaces') {
    return ok({ workspaces: [{ id: WS, name: 'NovaPay', role: server.role, member_count: server.personal ? 1 : 7, domain: '', type: server.personal ? 'personal' : 'organization' }] })
  }
  if (p === '/me') {
    if (server.meDown) return fail(500)
    return ok({
      user: {
        id: 'u-hoa', username: 'hoa', ngac_node_id: 'n-hoa', email: 'hoa@novapay.vn', union_id: '', ...server.profile,
        email_verified: true, needs_profile: false, language: 'vi',
      },
      current_tenant: { id: WS, name: 'NovaPay', role: server.role, open_id: '', department: server.personal ? '' : server.department },
    })
  }
  if (p === '/me/profile' && method === 'PATCH') {
    if (server.profileFails) return Promise.reject(server.profileFails)
    for (const key of ['display_name', 'title', 'location'] as const) {
      if (typeof body?.[key] === 'string') server.profile[key] = body[key]
    }
    return ok({ status: 'updated' })
  }
  if (p === '/me/avatar' && method === 'DELETE') {
    if (server.removeFails) return Promise.reject(server.removeFails)
    server.profile.avatar_url = ''
    return ok({ avatar_url: '' })
  }
  if (p === '/me/push-preferences') {
    if (method === 'PUT') {
      if (server.settingsDown) return fail(500)
      server.push = body
    }
    return ok(server.push)
  }
  if (p === `/workspaces/${WS}/settings`) return ok({ settings: {} })
  if (p === `/workspaces/${WS}/details`) {
    if (method === 'PATCH') {
      if (server.detailsFails) return Promise.reject(server.detailsFails)
      if (typeof body?.name === 'string') server.workspaceName = body.name
      if (typeof body?.description === 'string') server.description = body.description
    } else if (server.detailsDown) return fail(500)
    return ok({
      name: server.workspaceName ?? (server.personal ? 'Lê Thị Hoa' : 'NovaPay'), description: server.description, can_manage: server.canManage,
      type: server.personal ? 'personal' : 'organization', is_owner: server.role === 'owner', owner_names: server.ownerNames,
      ...server.detailsExtra,
    })
  }
  if (p === `/workspaces/${WS}/leave` && method === 'POST') {
    return server.leaveFails ? Promise.reject(server.leaveFails) : ok({ status: 'left' })
  }
  if (p === `/workspaces/${WS}/locks`) {
    if (server.personal) return fail(400)
    if (method === 'PUT') {
      if (!server.canManage) return fail(403)
      server.groupCreation = body.group_creation
    }
    return ok({ group_creation: server.groupCreation })
  }
  if (p === '/drive/settings') {
    if (method === 'PUT') {
      if (server.settingsDown) return fail(500)
      server.drive = body
    }
    return ok(server.drive)
  }
  if (p === `/workspaces/${WS}/chat-settings`) {
    if (method === 'PUT') {
      if (server.settingsDown) return fail(500)
      server.chat = { ...server.chat, ...body }
    }
    return ok(server.chat)
  }
  if (p === `/workspaces/${WS}/calendar/settings`) {
    if (method === 'PUT') {
      if (server.settingsDown) return fail(500)
      server.calendar = { ...server.calendar, ...body }
    }
    return ok(server.calendar)
  }
  if (p === `/workspaces/${WS}/calendar/calendars`) return ok({ calendars: server.calendars })
  const sub = p.match(new RegExp(`^/workspaces/${WS}/calendar/calendars/([^/]+)/subscription$`))
  if (sub && method === 'PUT') {
    const cal = server.calendars.find((c) => c.id === sub[1])
    if (!cal) return fail(404)
    Object.assign(cal, body)
    return ok(cal)
  }
  if (p === '/approval/settings') {
    if (method === 'PUT') {
      if (server.settingsDown) return fail(500)
      server.approval = { ...server.approval, ...body }
    }
    return ok(server.approval)
  }
  if (p === `/workspaces/${WS}/asset-settings`) {
    if (method === 'PUT') {
      if (server.settingsDown) return fail(500)
      server.asset = { ...server.asset, ...body }
    }
    return ok(server.asset)
  }
  if (p === '/auth/sessions') return ok({ sessions: server.sessions })
  if (p.startsWith('/auth/sessions/') && method === 'DELETE') {
    const id = decodeURIComponent(p.slice('/auth/sessions/'.length))
    if (!server.sessions.some((s) => s.session_id === id && !s.current)) return fail(404)
    server.sessions = server.sessions.filter((s) => s.session_id !== id)
    return ok({ status: 'revoked' })
  }
  if (p === `/workspaces/${WS}/admin/members`) {
    if (!server.canManage) return fail(403)
    return ok({ members: Array.from({ length: 7 }, (_, i) => ({ ngac_node_id: `n${i}`, user_id: `u${i}`, display_name: `Người ${i}`, roles: [], status: 'active' })) })
  }
  if (p === `/workspaces/${WS}/invitations`) return ok({ invitations: [{ id: 'i1' }, { id: 'i2' }] })
  if (p === `/workspaces/${WS}/departments`) return ok({ departments: [{ id: 'd1', name: 'Phòng Tài chính' }] })
  if (p === `/workspaces/${WS}/roles`) return ok({
    roles: [{ id: 'r1', name: 'Kế toán', ngac_node_id: 'nr1', kind: 'custom', member_count: 2 }],
    system_roles: [{ id: 's1', ngac_node_id: 'ns1', kind: 'owners', member_count: 1 }, { id: 's2', ngac_node_id: 'ns2', kind: 'members', member_count: 6 }],
  })
  if (p === '/approval/templates') return ok({ templates: [{ id: 't1' }, { id: 't2' }] })
  if (p === `/workspaces/${WS}/asset-types`) return ok({ types: [{ id: 'a1' }], can_manage: true })
  if (p === `/workspaces/${WS}/drive/quota`) {
    // Protobuf JSON: zero fields are left out (max_files here; used_files and used_bytes of an empty drive).
    if (server.personal) return ok({ workspace_id: WS, max_bytes: 5 * 1024 ** 3 })
    return ok({ workspace_id: WS, max_bytes: 10 * 1024 ** 3, used_bytes: 2.4 * 1024 ** 3, used_files: 312 })
  }
  return Promise.reject(new Error(`unexpected ${method} ${p}`))
}

/** A new profile picture: `PUT /me/avatar` goes through `apiUpload`, as a form with the file. */
export function settingsUpload(path: string, body: FormData, method = 'POST'): Promise<unknown> {
  server.writes.push({ method, path, body: undefined })
  if (path !== '/me/avatar' || method !== 'PUT' || !(body.get('file') instanceof File)) {
    return Promise.reject(new Error(`unexpected upload ${method} ${path}`))
  }
  if (server.avatarFails) return Promise.reject(server.avatarFails)
  server.profile.avatar_url = AVATAR_URL
  return ok({ avatar_url: AVATAR_URL })
}

/** The viewport: below 1024 (the phone's pages) or the desktop's. */
export function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

/** The settings routes, as the app mounts them, and stand-ins for where their rows lead. */
export async function renderSettings(url: string) {
  const root = createRootRoute({ component: () => <Outlet /> })
  const page = (path: string) => createRoute({ getParentRoute: () => root, path, validateSearch: validateWorkspaceSearch, component: () => <p>{`page ${path}`}</p> })
  const settings = createRoute({ getParentRoute: () => root, path: '/settings', validateSearch: validateSettingsSearch, component: () => <Outlet /> })
  const index = createRoute({ getParentRoute: () => settings, path: '/', component: SettingsScreen })
  const section = createRoute({ getParentRoute: () => settings, path: '$section', component: SettingsSectionScreen })
  const tree = root.addChildren([
    settings.addChildren([index, section]),
    page('/channels'), page('/admin'), page('/admin/users'), page('/admin/roles'), page('/admin/directory'),
    page('/approval'), page('/assets'), page('/workspace-select'),
  ])
  const router = createRouter({ routeTree: tree, history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  useAuthStore.setState({ accessToken: `h.${btoa(JSON.stringify({ tenant_id: WS }))}.s` })
  return router
}
