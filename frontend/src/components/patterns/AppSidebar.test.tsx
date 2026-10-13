import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, WS_ID } from '../../test/chat-fixtures'
import { apiFetch, logoutSession } from '../../api/client'
import { loadPreferences, switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { useAuthStore } from '../../stores/auth.store'
import { queryClient } from '../../lib/query-client'
import { AppSidebar } from './AppSidebar'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(), logoutSession: vi.fn() }))
const api = vi.mocked(apiFetch)
const OTHER_WS = '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c'
const HIDDEN_WS = '0c0b8e30-4d5f-4071-8cbd-3e4f5a6b7c8d'

async function renderSidebar(url = '/drive', unreadCounts: Record<string, number> = { messaging: 3 }) {
  const root = createRootRoute({ component: () => <AppSidebar workspaceName="Khối Vận hành" unreadCounts={unreadCounts} /> })
  const pages = ['/drive', '/documents/$docId', '/contacts', '/settings', '/channels', '/onboarding', '/approval', '/assets', '/workspace-select'].map((path) =>
    createRoute({ getParentRoute: () => root, path, component: () => null }))
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}
const expandMenu = () => updatePreferences({ sidebarCollapsed: false })

beforeEach(() => {
  api.mockReset()
  vi.mocked(logoutSession).mockReset()
  localStorage.clear()
  switchPreferencesTo('user-a')
  updatePreferences({ motion: 'reduce' }) // the swap is instant; the animated path has its own test
  api.mockImplementation((path: string) =>
    path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }, { id: OTHER_WS, name: 'NovaPay Kế toán' }, { id: HIDDEN_WS, name: 'Chỉ xem được' }] })
    : path === '/me/workspaces' ? Promise.resolve({ workspaces: [
        { id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: '' },
        { id: OTHER_WS, name: 'NovaPay Kế toán', role: 'member', member_count: 12, domain: '' }] })
    : path === '/auth/switch-tenant' ? Promise.resolve({ access_token: `h.${btoa(JSON.stringify({ tenant_id: OTHER_WS }))}.s` })
    : path.endsWith('/contacts') ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
    : path === '/notifications/unread-count' ? Promise.resolve({ count: 0 })
    : Promise.reject(new Error(`unexpected ${path}`)))
})
afterEach(() => {
  cleanup()
  resetClient()
  switchPreferencesTo(undefined)
})

/** A link's name as a screen reader gets it: its visible label plus the hidden count text, minus the aria-hidden pill. */
const nameOf = (a: Element) => {
  const copy = a.cloneNode(true) as Element
  copy.querySelectorAll('[aria-hidden="true"]').forEach((n) => n.remove())
  return copy.textContent
}
const mainLinks = (nav: HTMLElement) => within(nav).getAllByRole('link').map(nameOf)

describe('AppSidebar, expanded', () => {
  beforeEach(expandMenu)

  it('is one navigation landmark that names each destination and carries the unread count', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    expect(screen.getAllByRole('navigation')).toHaveLength(1)
    expect(mainLinks(nav)).toEqual(['Tin nhắn, 3 chưa đọc', 'Tài liệu', 'Phê duyệt', 'Tài sản', 'Danh bạ', 'Quản trị', 'Cài đặt'])
    expect(within(nav).getByRole('link', { name: 'Tin nhắn, 3 chưa đọc' })).toBeInTheDocument()
    expect(within(nav).getByRole('button', { name: 'Thông báo' })).toBeInTheDocument()
    expect(nav).not.toHaveAttribute('data-rail')
  })

  it('lights up Tài liệu on a document, since Văn bản lives there', async () => {
    await renderSidebar('/documents/99999999-aaaa-4bbb-8ccc-000000000001')
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    expect(within(nav).getByRole('link', { name: 'Tài liệu' })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getByRole('link', { name: /Tin nhắn/ })).not.toHaveAttribute('aria-current')
  })

  it('lists only the other workspaces the person can enter, names the open one in full, and switching re-scopes the session', async () => {
    await renderSidebar()
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    const menu = await screen.findByRole('dialog', { name: 'Đổi workspace' })
    expect(within(menu).getByRole('heading', { name: 'Khối Vận hành' })).toBeInTheDocument()
    const list = within(menu).getByRole('listbox', { name: 'Chuyển sang' })
    await waitFor(() => expect(within(list).getAllByRole('option')).toHaveLength(1))
    expect(within(list).queryByText('Chỉ xem được')).toBeNull()

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(useAuthStore.getState().tenantId).toBe(OTHER_WS))
    expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.objectContaining({ body: JSON.stringify({ tenant_id: OTHER_WS }) }))
    expect(queryClient.getQueryData(['drive', 'old'])).toBeUndefined()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Đổi workspace' })).toBeNull())
  })

  it('hides Phê duyệt, Tài sản and Quản trị in a personal workspace, and labels it Workspace cá nhân', async () => {
    api.mockImplementation((path: string) =>
      path === '/workspaces'
        ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', type: 'personal' }] })
        : path === '/me/workspaces'
          ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', role: 'owner', member_count: 1, domain: '', type: 'personal' }] })
          : path.endsWith('/contacts')
            ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
            : path === '/notifications/unread-count' ? Promise.resolve({ count: 0 })
            : Promise.reject(new Error(`unexpected ${path}`)))

    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    expect(mainLinks(nav)).toEqual(['Tin nhắn, 3 chưa đọc', 'Tài liệu', 'Danh bạ', 'Cài đặt'])
    expect(within(nav).queryByRole('link', { name: /Phê duyệt/ })).toBeNull()
    expect(within(nav).queryByRole('link', { name: /Tài sản/ })).toBeNull()
    expect(await screen.findByText('Workspace cá nhân')).toBeInTheDocument()
    expect(within(nav).getByRole('button', { name: 'Thông báo' })).toBeInTheDocument()
  })

  it('shows each workspace type line and a Tạo workspace link in the switcher', async () => {
    api.mockImplementation((path: string) =>
      path === '/workspaces'
        ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', type: 'personal' }, { id: OTHER_WS, name: 'Khối Vận hành', type: 'organization' }] })
        : path === '/me/workspaces'
          ? Promise.resolve({ workspaces: [
              { id: WS_ID, name: 'Không gian của bạn', role: 'owner', member_count: 1, domain: '', type: 'personal' },
              { id: OTHER_WS, name: 'Khối Vận hành', role: 'member', member_count: 12, domain: 'NovaPay', type: 'organization' },
            ] })
          : path.endsWith('/contacts')
            ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
            : path === '/notifications/unread-count' ? Promise.resolve({ count: 0 })
            : Promise.reject(new Error(`unexpected ${path}`)))

    await renderSidebar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    const menu = await screen.findByRole('dialog', { name: 'Đổi workspace' })
    const list = within(menu).getByRole('listbox', { name: 'Chuyển sang' })
    // WS_ID is the open one (header); the other is organization-type and carries its domain.
    expect(within(list).getAllByRole('option')).toHaveLength(1)
    expect(within(list).getByText('NovaPay')).toBeInTheDocument()
    expect(within(menu).getAllByText(/Workspace cá nhân/).length).toBeGreaterThan(0)

    // The listbox holds only options; the label and the create link sit beside it.
    expect(within(list).queryByRole('link')).toBeNull()
    const label = within(menu).getByText('Chuyển sang')
    expect(list.contains(label)).toBe(false)
    const createLink = within(menu).getByRole('link', { name: 'Tạo workspace' })
    expect(list.contains(createLink)).toBe(false)

    // It carries where the person is, so Back on that screen returns here.
    const href = new URL(createLink.getAttribute('href')!, 'http://app.test')
    expect(href.pathname).toBe('/onboarding')
    expect(href.searchParams.get('from')).toBe('/drive')
  })

  it('falls back to the headcount for a workspace type it does not know', async () => {
    api.mockImplementation((path: string) =>
      path === '/workspaces'
        ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
        : path === '/me/workspaces'
          ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: '', type: 'galaxy' }] })
          : path.endsWith('/contacts')
            ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
            : path === '/notifications/unread-count' ? Promise.resolve({ count: 0 })
            : Promise.reject(new Error(`unexpected ${path}`)))
    await renderSidebar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    const menu = await screen.findByRole('dialog', { name: 'Đổi workspace' })
    expect(within(menu).getByText(/64 thành viên/)).toBeInTheDocument()
    expect(within(menu).queryByText('Workspace tổ chức')).toBeNull()
  })

  it('keeps the person row with Đăng xuất beside the name', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    await userEvent.setup().click(within(nav).getByRole('button', { name: 'Đăng xuất' }))
    expect(logoutSession).toHaveBeenCalled()
  })
})

describe('AppSidebar, collapsed rail', () => {
  it('is the default, and shows every full label under its icon with no tooltip on the items', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    expect(nav).toHaveAttribute('data-rail', 'true')
    expect(nav).toHaveClass('w-18')
    for (const name of ['Tài liệu', 'Phê duyệt', 'Tài sản', 'Danh bạ', 'Quản trị', 'Cài đặt']) {
      const link = within(nav).getByRole('link', { name })
      expect(link).toHaveTextContent(name)
      expect(link).not.toHaveAttribute('title')
    }
    expect(within(nav).getByRole('button', { name: 'Thông báo' })).toHaveTextContent('Thông báo')
  })

  it('draws the count as a corner pill, caps it at 99+, hides it at 0, and reads it out', async () => {
    api.mockImplementation((path: string) =>
      path === '/notifications/unread-count' ? Promise.resolve({ count: 128 })
      : path === '/me/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: '' }] })
      : path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
      : path.endsWith('/contacts') ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
      : Promise.reject(new Error(`unexpected ${path}`)))
    await renderSidebar('/drive', { messaging: 3, approval: 0 })
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    const messages = within(nav).getByRole('link', { name: 'Tin nhắn, 3 chưa đọc' })
    expect(within(messages).getByText('3')).toHaveAttribute('aria-hidden', 'true')
    const bell = await within(nav).findByRole('button', { name: 'Thông báo, 128 chưa đọc' })
    expect(within(bell).getByText('99+')).toHaveAttribute('aria-hidden', 'true')
    expect(within(nav).getByRole('link', { name: 'Phê duyệt' })).toHaveTextContent(/^Phê duyệt$/)
  })

  it('marks the open module with aria-current', async () => {
    await renderSidebar('/contacts')
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    expect(within(nav).getByRole('link', { name: 'Danh bạ' })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getByRole('link', { name: 'Tài liệu' })).not.toHaveAttribute('aria-current')
  })

  it('still hides Phê duyệt and Tài sản in a personal workspace', async () => {
    api.mockImplementation((path: string) =>
      path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', type: 'personal' }] })
      : path === '/me/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Không gian của bạn', role: 'owner', member_count: 1, domain: '', type: 'personal' }] })
      : path.endsWith('/contacts') ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
      : path === '/notifications/unread-count' ? Promise.resolve({ count: 0 })
      : Promise.reject(new Error(`unexpected ${path}`)))
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    await waitFor(() => expect(within(nav).queryByRole('link', { name: 'Phê duyệt' })).toBeNull())
    expect(within(nav).queryByRole('link', { name: 'Tài sản' })).toBeNull()
  })

  it('opens the workspace menu from the letter tile, with the full name and the type line in the header', async () => {
    await renderSidebar()
    const tile = await screen.findByRole('button', { name: /^Đổi workspace, đang ở Khối Vận hành/ })
    await userEvent.setup().click(tile)
    const menu = await screen.findByRole('dialog', { name: 'Đổi workspace' })
    expect(within(menu).getByRole('heading', { name: 'Khối Vận hành' })).toBeInTheDocument()
    await waitFor(() => expect(within(menu).getByText(/Quản trị viên|admin/i)).toBeInTheDocument())
  })

  it('opens the account menu from the avatar, with the person, their status, settings and logout', async () => {
    await renderSidebar()
    const user = userEvent.setup()
    const avatar = await screen.findByRole('button', { name: /^Tài khoản: .*đang hoạt động$/ })
    await user.click(avatar)
    const menu = await screen.findByRole('menu', { name: 'Tài khoản' })
    expect(within(menu).getByText(/Đang hoạt động/)).toBeInTheDocument()
    await user.click(within(menu).getByRole('menuitem', { name: 'Đăng xuất' }))
    expect(logoutSession).toHaveBeenCalled()
  })

  it('takes the account menu to settings', async () => {
    const router = await renderSidebar()
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /^Tài khoản: / }))
    await user.click(within(await screen.findByRole('menu', { name: 'Tài khoản' })).getByRole('menuitem', { name: 'Cài đặt tài khoản' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  })
})

describe('AppSidebar, the toggle', () => {
  it('is alone on the top row, says which way it goes, and expands and collapses the menu', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    const toggle = within(nav).getByRole('button', { name: 'Mở rộng menu chính' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(toggle).toHaveAttribute('aria-keyshortcuts', 'Control+Backslash')

    const user = userEvent.setup()
    await user.click(toggle)
    expect(nav).not.toHaveAttribute('data-rail')
    expect(nav).toHaveClass('w-58')
    const back = within(nav).getByRole('button', { name: 'Thu gọn menu chính' })
    expect(back).toHaveAttribute('aria-expanded', 'true')
    expect(loadPreferences('user-a').sidebarCollapsed).toBe(false)

    await user.click(back)
    expect(nav).toHaveAttribute('data-rail', 'true')
    expect(loadPreferences('user-a').sidebarCollapsed).toBe(true)
  })

  it('follows Ctrl+\\ and ⌘\\, and remembers the choice for this person on this device', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    fireEvent.keyDown(document, { key: '\\', ctrlKey: true })
    await waitFor(() => expect(nav).not.toHaveAttribute('data-rail'))
    fireEvent.keyDown(document, { key: '\\', metaKey: true })
    await waitFor(() => expect(nav).toHaveAttribute('data-rail', 'true'))
    fireEvent.keyDown(document, { key: '\\', ctrlKey: true })
    await waitFor(() => expect(nav).not.toHaveAttribute('data-rail'))
    expect(loadPreferences('user-a').sidebarCollapsed).toBe(false)
    expect(loadPreferences('user-b').sidebarCollapsed).toBe(true)
  })

  it('swaps in steps when motion is allowed: content fades, then the width changes', async () => {
    updatePreferences({ motion: 'system' })
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    await userEvent.setup().click(within(nav).getByRole('button', { name: 'Mở rộng menu chính' }))
    expect(nav).toHaveAttribute('data-swapping', 'true')
    expect(nav).toHaveAttribute('data-rail', 'true') // still the rail while the content fades out
    await waitFor(() => expect(nav).not.toHaveAttribute('data-rail'), { timeout: 1000 })
    await waitFor(() => expect(nav).not.toHaveAttribute('data-swapping'), { timeout: 1000 })
  })

  it('shows its tooltip with the shortcut on keyboard focus, in both states', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    const user = userEvent.setup()
    await user.tab()
    expect(document.activeElement).toBe(within(nav).getByRole('button', { name: 'Mở rộng menu chính' }))
    const tip = await screen.findByRole('tooltip')
    expect(tip).toHaveTextContent('Mở rộng menu chính')
    expect(tip).toHaveTextContent('Ctrl+\\')
  })

  it('gives the letter tile and the avatar a tooltip on the rail only, never as their only name', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    const user = userEvent.setup()
    await user.tab() // toggle
    await user.tab() // workspace tile
    const tip = await screen.findByRole('tooltip')
    expect(tip).toHaveTextContent('Khối Vận hành')
    expect(within(nav).getByRole('button', { name: /^Đổi workspace, đang ở Khối Vận hành/ })).toHaveFocus()

    cleanup()
    expandMenu()
    await renderSidebar()
    const open = await screen.findByRole('navigation', { name: 'Điều hướng chính' })
    await user.tab() // toggle
    await user.tab() // workspace button
    expect(within(open).getByRole('button', { name: /Khối Vận hành/ })).toHaveFocus()
    expect(screen.queryByText(/bấm để đổi workspace/)).toBeNull()
  })
})

describe('AppSidebar, Thông báo', () => {
  const notifApi = (count = 2) => api.mockImplementation((path: string) =>
    path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
    : path === '/me/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: '' }] })
    : path.endsWith('/contacts') ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
    : path === '/notifications/unread-count' ? Promise.resolve({ count })
    : path.startsWith('/notifications?') ? Promise.resolve({ notifications: [], total: 0, unread_count: count })
    : Promise.reject(new Error(`unexpected ${path}`)))

  it('opens the panel beside the menu, marks the item expanded, and gives focus back on Esc', async () => {
    notifApi()
    await renderSidebar()
    const user = userEvent.setup()
    const bell = await screen.findByRole('button', { name: 'Thông báo, 2 chưa đọc' })
    expect(bell).toHaveAttribute('aria-expanded', 'false')
    await user.click(bell)
    expect(await screen.findByRole('dialog', { name: 'Thông báo' })).toBeInTheDocument()
    expect(bell).toHaveAttribute('aria-expanded', 'true')
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull())
    expect(bell).toHaveFocus()
    expect(bell).toHaveAttribute('aria-expanded', 'false')
  })

  it('pressing the item again closes it, and so does collapsing or expanding the menu', async () => {
    notifApi()
    await renderSidebar()
    const user = userEvent.setup()
    const bell = await screen.findByRole('button', { name: 'Thông báo, 2 chưa đọc' })
    await user.click(bell)
    await screen.findByRole('dialog', { name: 'Thông báo' })
    await user.click(bell)
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull())

    await user.click(bell)
    await screen.findByRole('dialog', { name: 'Thông báo' })
    fireEvent.keyDown(document, { key: '\\', ctrlKey: true })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thông báo' })).toBeNull())
  })
})

describe('AppSidebar, offers waiting', () => {
  beforeEach(expandMenu)

  const offer = (id: string) => ({
    id, workspace_name: 'NovaPay Kế toán', inviter_name: 'Lê Thị Hoa', role_name: '', department_name: '',
    created_at: '2026-10-01T08:00:00Z', expires_at: '2026-10-08T08:00:00Z',
  })
  /** Answers /me and /invitations on top of the default fixtures. */
  function withOffers(count: number, verified = true) {
    const base = api.getMockImplementation()!
    api.mockImplementation(((path: string, init?: RequestInit) =>
      path === '/me' ? Promise.resolve({ user: { id: 'u1', email: 'hoa@novapay.vn', email_verified: verified } })
      : path === '/invitations' ? Promise.resolve({ invitations: Array.from({ length: count }, (_, i) => offer(`o${i}`)) })
      : base(path, init)) as typeof apiFetch)
  }
  const openMenu = async () => {
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    return { user, menu: await screen.findByRole('dialog', { name: 'Đổi workspace' }) }
  }

  it('shows Lời mời with the count and a dot on the workspace button, and opens Chọn workspace', async () => {
    withOffers(3)
    const router = await renderSidebar()
    const button = await screen.findByRole('button', { name: /Khối Vận hành.*có lời mời/ })
    expect(within(button).getByTestId('invitation-dot')).toBeInTheDocument()
    const user = userEvent.setup()
    await user.click(button)
    const menu = await screen.findByRole('dialog', { name: 'Đổi workspace' })
    const row = within(menu).getByRole('link', { name: /Lời mời.*3 lời mời đang chờ/ })
    expect(row).toHaveTextContent('3')
    await user.click(row)
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
  })

  it('caps the pill at 99+', async () => {
    withOffers(120)
    await renderSidebar()
    const { menu } = await openMenu()
    expect(await within(menu).findByText('99+')).toBeInTheDocument()
  })

  it('shows neither row nor dot when there are no offers', async () => {
    withOffers(0)
    await renderSidebar()
    const { menu } = await openMenu()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/invitations'))
    expect(within(menu).queryByRole('link', { name: /Lời mời/ })).toBeNull()
    expect(screen.queryByTestId('invitation-dot')).toBeNull()
  })

  it('shows nothing and never asks when the address is not verified', async () => {
    withOffers(2, false)
    await renderSidebar()
    const { menu } = await openMenu()
    await waitFor(() => expect(api).toHaveBeenCalledWith('/me'))
    expect(within(menu).queryByRole('link', { name: /Lời mời/ })).toBeNull()
    expect(screen.queryByTestId('invitation-dot')).toBeNull()
    expect(api).not.toHaveBeenCalledWith('/invitations')
  })
})
