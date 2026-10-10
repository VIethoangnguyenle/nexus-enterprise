import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { CONTACTS, WS_ID } from '../../test/chat-fixtures'
import { notificationServer, seed, type NotificationServer } from '../../test/notification-fixtures'
import { apiFetch } from '../../api/client'
import { resetArrivals } from '../../lib/notification-arrivals'
import { validateWorkspaceSearch } from '../../lib/workspace'
import { switchPreferencesTo } from '../../lib/preferences'
import { useAuthStore } from '../../stores/auth.store'
import { useNotificationUi } from '../../stores/notification.store'
import { AppSidebar } from './AppSidebar'
import { MobileNav } from './MobileNav'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(), logoutSession: vi.fn() }))
const api = vi.mocked(apiFetch)

const OTHER_WS = '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c'
const tokenFor = (tenant: string) => `h.${btoa(JSON.stringify({ tenant_id: tenant }))}.s`

let server: NotificationServer
let offers = 0

function answer(path: string, init?: RequestInit): Promise<unknown> {
  if (path === '/approval/pending') return Promise.resolve({ items: [], total: 0 })
  if (path === '/invitations') {
    return Promise.resolve({
      invitations: Array.from({ length: offers }, (_, i) => ({
        id: `aaaaaaaa-aaaa-4bbb-8ccc-00000000000${i}`, workspace_name: `Workspace ${i}`, inviter_name: 'Lê Quang Vinh',
        role_name: '', department_name: '', created_at: '', expires_at: '',
      })),
    })
  }
  if (path === '/workspaces') return Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }, { id: OTHER_WS, name: 'NovaPay Kế toán' }] })
  if (path === '/me/workspaces') {
    return Promise.resolve({ workspaces: [
      { id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: '' },
      { id: OTHER_WS, name: 'NovaPay Kế toán', role: 'member', member_count: 12, domain: '' },
    ] })
  }
  if (path.endsWith('/contacts')) return Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
  return server.handler(path, init)
}

function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
}

async function mount(component: () => React.JSX.Element, url = '/channels') {
  const root = createRootRoute({ component })
  const paths = ['/channels', '/drive', '/approval', '/assets', '/contacts', '/admin', '/settings', '/workspace-select']
  const pages = paths.map((path) =>
    createRoute({ getParentRoute: () => root, path, validateSearch: validateWorkspaceSearch, component: () => null }))
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  useAuthStore.setState({ accessToken: tokenFor(WS_ID), tenantId: WS_ID })
  return router
}

beforeEach(() => {
  server = notificationServer()
  offers = 0
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
  localStorage.clear()
  switchPreferencesTo('user-a')
  useNotificationUi.setState({ listVisible: false, fresh: {}, burst: null, announcement: '', openRequest: 0 })
  window.history.replaceState({}, '', '/channels')
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetArrivals()
  resetClient()
  switchPreferencesTo(undefined)
})

describe('desktop: the sidebar row', () => {
  const sidebar = () => mount(() => <AppSidebar workspaceName="Khối Vận hành" />)

  it('is the first row of the bottom group, above Quản trị, with the unread count', async () => {
    viewport(false)
    await sidebar()
    const row = await screen.findByRole('button', { name: 'Thông báo, 3 chưa đọc' })
    expect(row).toHaveTextContent('Thông báo3')
    const group = row.parentElement!
    expect(group.firstElementChild).toBe(row)
    expect(within(group).getAllByRole('link').map((a) => a.textContent)).toEqual(['Quản trị', 'Cài đặt'])
    // Still five destinations above it: the main list did not grow.
    expect(within(screen.getByRole('navigation', { name: 'Phân hệ' })).getAllByRole('link')).toHaveLength(5)
  })

  it('shows no count when everything is read', async () => {
    viewport(false)
    server.rows = seed().map((n) => ({ ...n, read: true }))
    await sidebar()
    const row = await screen.findByRole('button', { name: 'Thông báo' })
    await waitFor(() => expect(api).toHaveBeenCalledWith('/notifications/unread-count'))
    expect(row).toHaveTextContent(/^Thông báo$/)
  })

  it('opens a 380px panel beside the sidebar, and Esc closes it and returns focus to the row', async () => {
    viewport(false)
    await sidebar()
    const user = userEvent.setup()
    const row = await screen.findByRole('button', { name: 'Thông báo, 3 chưa đọc' })
    expect(row).toHaveAttribute('aria-expanded', 'false')
    await user.click(row)
    const panel = await screen.findByRole('dialog', { name: 'Thông báo' })
    expect(row).toHaveAttribute('aria-expanded', 'true')
    expect(panel.className).toContain('w-95')
    await within(panel).findByText('Hôm nay')
    expectNoIds(document.body)

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull())
    expect(row).toHaveFocus()
  })

  it('closes on a click outside, and on the Close button', async () => {
    viewport(false)
    await sidebar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Thông báo, 3 chưa đọc' }))
    await user.click(await screen.findByRole('button', { name: 'Đóng thông báo' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull())

    await user.click(screen.getByRole('button', { name: 'Thông báo, 3 chưa đọc' }))
    await screen.findByRole('dialog', { name: 'Thông báo' })
    await user.click(document.body)
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull())
  })

  it('opens when a toast asks for the list ("Mở")', async () => {
    viewport(false)
    await sidebar()
    await screen.findByRole('button', { name: 'Thông báo, 3 chưa đọc' })
    useNotificationUi.getState().requestOpen()
    await screen.findByRole('dialog', { name: 'Thông báo' })
  })

  it('has no panel and asks for nothing on a narrow screen, where the Thêm sheet carries it', async () => {
    viewport(true)
    await sidebar()
    await screen.findByRole('button', { name: 'Thông báo' })
    expect(api).not.toHaveBeenCalledWith('/notifications/unread-count')
    useNotificationUi.getState().requestOpen()
    expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull()
  })
})

describe('desktop: the workspace switcher', () => {
  it('lists the offers waiting, and the row leads to the picker', async () => {
    viewport(false)
    offers = 2
    const router = await mount(() => <AppSidebar workspaceName="Khối Vận hành" />)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    const row = await screen.findByRole('link', { name: 'Lời mời đang chờ (2)' })
    await user.click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
  })

  it('has no such row when no offer is waiting', async () => {
    viewport(false)
    await mount(() => <AppSidebar workspaceName="Khối Vận hành" />)
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    await screen.findByRole('listbox', { name: 'Không gian làm việc' })
    await waitFor(() => expect(api).toHaveBeenCalledWith('/invitations'))
    expect(screen.queryByText(/Lời mời đang chờ/)).toBeNull()
  })
})

describe('phone and tablet: the dot and the sheet', () => {
  const bar = () => mount(() => <MobileNav />)

  it('puts a dot, not a number, on Thêm and adds no icon to the bar', async () => {
    viewport(true)
    await bar()
    const more = await screen.findByRole('button', { name: 'Thêm, 3 thông báo chưa đọc' })
    expect(within(more).getByTestId('more-dot')).toBeInTheDocument()
    expect(more).not.toHaveTextContent(/\d/)
    const nav = screen.getByRole('navigation', { name: 'Điều hướng di động' })
    expect(nav.querySelectorAll('svg')).toHaveLength(4)
    expect(within(nav).queryByRole('button', { name: /^Thông báo/ })).toBeNull()
  })

  it('shows no dot when everything is read', async () => {
    viewport(true)
    server.rows = seed().map((n) => ({ ...n, read: true }))
    await bar()
    const more = await screen.findByRole('button', { name: 'Thêm' })
    await waitFor(() => expect(api).toHaveBeenCalledWith('/notifications/unread-count'))
    expect(within(more).queryByTestId('more-dot')).toBeNull()
  })

  it('puts "Thông báo" with its count at the top of the sheet, and swaps the sheet to the list', async () => {
    viewport(true)
    await bar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Thêm, 3 thông báo chưa đọc' }))
    const sheet = await screen.findByRole('dialog', { name: 'Thêm' })
    const nav = within(sheet).getByRole('navigation', { name: 'Thêm' })
    const first = nav.firstElementChild as HTMLElement
    expect(first).toHaveAccessibleName('Thông báo, 3 chưa đọc')
    expect(first).toHaveTextContent('Thông báo3')

    await user.click(first)
    await within(sheet).findByText('Hôm nay')
    expect(within(sheet).getByRole('heading', { name: 'Thông báo' })).toBeInTheDocument()
    expectNoIds(document.body)

    await user.click(within(sheet).getByRole('button', { name: 'Quay lại' }))
    await within(sheet).findByRole('navigation', { name: 'Thêm' })
  })

  it('closes the sheet on Esc, once, from the list, and returns focus to Thêm', async () => {
    viewport(true)
    await bar()
    const user = userEvent.setup()
    const more = await screen.findByRole('button', { name: 'Thêm, 3 thông báo chưa đọc' })
    await user.click(more)
    const sheet = await screen.findByRole('dialog', { name: 'Thêm' })
    await user.click(within(sheet).getByRole('button', { name: 'Thông báo, 3 chưa đọc' }))
    await within(sheet).findByText('Hôm nay')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thêm' })).toBeNull())
    expect(more).toHaveFocus()
  })

  it('opens a link from the list, closing the sheet', async () => {
    viewport(true)
    const router = await bar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Thêm, 3 thông báo chưa đọc' }))
    const sheet = await screen.findByRole('dialog', { name: 'Thêm' })
    await user.click(within(sheet).getByRole('button', { name: 'Thông báo, 3 chưa đọc' }))
    await user.click(await within(sheet).findByRole('link', { name: /MacBook Pro 14 inch/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/assets'))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thêm' })).toBeNull())
  })

  it('opens straight onto the list when a toast asks for it ("Mở")', async () => {
    viewport(true)
    await bar()
    await screen.findByRole('button', { name: 'Thêm, 3 thông báo chưa đọc' })
    useNotificationUi.getState().requestOpen()
    const sheet = await screen.findByRole('dialog', { name: 'Thêm' })
    await within(sheet).findByText('Hôm nay')
  })

  it('lists the offers waiting in the sheet\'s workspace switcher', async () => {
    viewport(true)
    offers = 3
    const router = await bar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Thêm, 3 thông báo chưa đọc' }))
    const sheet = await screen.findByRole('dialog', { name: 'Thêm' })
    await user.click(within(sheet).getByRole('button', { name: /Đổi workspace/ }))
    await user.click(await within(sheet).findByRole('link', { name: 'Lời mời đang chờ (3)' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
  })
})
