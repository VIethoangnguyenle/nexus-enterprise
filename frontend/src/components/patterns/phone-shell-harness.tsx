import type { ReactNode } from 'react'
import { screen } from '@testing-library/react'
import { vi } from 'vitest'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient } from '../../test/render'
import { CH, CONTACTS, WS_ID, fixtureApi } from '../../test/chat-fixtures'
import { validateChannelsSearch } from '../../lib/nav-destinations'
import { validateSettingsSearch } from '../../lib/settings-search'
import { validateWorkspaceSearch } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'

/** What the tests of the phone shell (search bar, menu, sheets, Chat bar) share: one fake server and one router. */

export const OTHER_WS = '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c'
export const NOT_MINE_WS = '0c0b8e30-4d5f-4071-8cbd-3e4f5a6b7c8d'
export const tokenFor = (tenant: string) => `h.${btoa(JSON.stringify({ tenant_id: tenant }))}.s`

type UnreadRow = {
  channel_id: string
  unread_count: number
  last_read_message_id: string
  notification_level?: 'all' | 'main' | 'off'
  mention_count?: number
}

export const server = {
  pending: 2,
  unread: [] as UnreadRow[],
  offers: 0,
  addressVerified: true,
  switchError: null as unknown,
  notificationsUnread: 0,
  mentionsUnread: 0,
  personal: false,
  notifications: [] as Array<Record<string, unknown>>,
}

export function resetServer() {
  server.pending = 2
  server.unread = []
  server.offers = 0
  server.addressVerified = true
  server.switchError = null
  server.notificationsUnread = 0
  server.mentionsUnread = 0
  server.personal = false
  server.notifications = []
}

export const unreadRow = (channel_id: string, unread_count: number, over: Partial<UnreadRow> = {}): UnreadRow =>
  ({ channel_id, unread_count, last_read_message_id: '', ...over })

export const GROUP = CH.doisoat
export const GROUP_2 = CH.vanhanh
export const DIRECT = CH.dmYen

export function answer(path: string, init?: RequestInit): Promise<unknown> {
  const p = path.split('?')[0]!
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))
  if (p === '/me') return ok({ user: { id: 'u1', email: 'hoa@novapay.vn', email_verified: server.addressVerified } })
  if (p === '/invitations') {
    return ok({ invitations: Array.from({ length: server.offers }, (_, i) => ({
      id: `o${i}`, workspace_name: 'NovaPay Kế toán', inviter_name: 'Lê Thị Hoa', role_name: '', department_name: '',
      created_at: '2026-10-01T08:00:00Z', expires_at: '2026-10-08T08:00:00Z',
    })) })
  }
  if (p === '/channels/unread') return ok({ channels: server.unread })
  if (p === '/channels/activity') return ok({ channels: [] })
  if (p === '/approval/pending') return ok({ items: [], total: server.pending })
  if (p === '/notifications/unread-count') return ok({ count: server.notificationsUnread })
  if (p === '/notifications') {
    return ok({ notifications: server.notifications, total: server.notifications.length, unread_count: server.notificationsUnread })
  }
  if (p === '/messages/mentions/unread-count') return ok({ count: server.mentionsUnread })
  // The graph can reach more workspaces than the person may enter.
  if (p === '/workspaces') {
    if (server.personal) return ok({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', type: 'personal' }] })
    return ok({ workspaces: [
      { id: WS_ID, name: 'Khối Vận hành', type: 'organization' },
      { id: OTHER_WS, name: 'NovaPay Kế toán', type: 'organization' },
      { id: NOT_MINE_WS, name: 'Chỉ xem được', type: 'organization' },
    ] })
  }
  if (p === '/me/workspaces') {
    if (server.personal) {
      return ok({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', role: 'owner', member_count: 1, domain: '', type: 'personal' }] })
    }
    return ok({ workspaces: [
      { id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: 'novapay.vn', type: 'organization' },
      { id: OTHER_WS, name: 'NovaPay Kế toán', role: 'member', member_count: 12, domain: '', type: 'organization' },
    ] })
  }
  if (p === '/auth/switch-tenant') {
    if (server.switchError) return Promise.reject(server.switchError)
    return ok({ access_token: tokenFor(JSON.parse(String(init?.body)).tenant_id) })
  }
  if (p.endsWith('/contacts')) return ok({ contacts: CONTACTS, total: CONTACTS.length })
  if (p === `/workspaces/${WS_ID}/channels` || p === '/dms' || p.endsWith('/chat-sections')) return fixtureApi(path, init)
  return Promise.reject(new Error(`unexpected ${p}`))
}

/** The viewport: the bar's widths (below 1024) or the desktop's. */
export function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

/** `ui` mounted at the root of a router that knows every destination of the app shell. */
export async function renderShell(ui: ReactNode, url = '/channels') {
  const root = createRootRoute({ component: () => <>{ui}<Outlet /></> })
  const page = (path: string, validateSearch: (s: Record<string, unknown>) => object = validateWorkspaceSearch) =>
    createRoute({ getParentRoute: () => root, path, validateSearch, component: () => null })
  const pages = [
    page('/channels', validateChannelsSearch),
    page('/channels/$channelId'),
    page('/drive'), page('/approval'), page('/assets'), page('/contacts'), page('/admin'),
    page('/settings', validateSettingsSearch),
    page('/documents/$docId'), page('/workspace-select'), page('/onboarding'),
  ]
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  useAuthStore.setState({ accessToken: tokenFor(WS_ID) })
  return router
}

export const dialogOf = (name: string | RegExp) => screen.findByRole('dialog', { name })
