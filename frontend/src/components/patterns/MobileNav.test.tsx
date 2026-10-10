import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, WS_ID } from '../../test/chat-fixtures'
import { apiFetch, logoutSession } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { validateWorkspaceSearch } from '../../lib/workspace'
import { useAuthStore } from '../../stores/auth.store'
import { useDriveStore } from '../../stores/drive.store'
import { MobileNav } from './MobileNav'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

const OTHER_WS = '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c'
const NOT_MINE_WS = '0c0b8e30-4d5f-4071-8cbd-3e4f5a6b7c8d'
const tokenFor = (tenant: string) => `h.${btoa(JSON.stringify({ tenant_id: tenant }))}.s`

let pending = 2
let switchError: unknown = null

function answer(path: string, init?: RequestInit) {
  if (path === '/approval/pending') return Promise.resolve({ items: [], total: pending })
  // The graph can reach more workspaces than the person may enter.
  if (path === '/workspaces') {
    return Promise.resolve({ workspaces: [
      { id: WS_ID, name: 'Khối Vận hành' }, { id: OTHER_WS, name: 'NovaPay Kế toán' }, { id: NOT_MINE_WS, name: 'Chỉ xem được' },
    ] })
  }
  if (path === '/me/workspaces') {
    return Promise.resolve({ workspaces: [
      { id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: 'novapay.vn' },
      { id: OTHER_WS, name: 'NovaPay Kế toán', role: 'member', member_count: 12, domain: '' },
    ] })
  }
  if (path === '/auth/switch-tenant') {
    if (switchError) return Promise.reject(switchError)
    return Promise.resolve({ access_token: tokenFor(JSON.parse(String(init?.body)).tenant_id) })
  }
  if (path.endsWith('/contacts')) return Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
  return Promise.reject(new Error(`unexpected ${path}`))
}

async function renderBar(url = '/channels') {
  const root = createRootRoute({ component: MobileNav })
  const pages = ['/channels', '/drive', '/approval', '/assets', '/contacts', '/admin', '/settings', '/documents/$docId'].map((path) =>
    createRoute({ getParentRoute: () => root, path, validateSearch: validateWorkspaceSearch, component: () => null }))
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  useAuthStore.setState({ accessToken: tokenFor(WS_ID) })
  return router
}

/** The viewport: the bar's widths (below 1024) or the desktop's. */
function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

beforeEach(() => {
  viewport(true)
  pending = 2
  switchError = null
  api.mockReset()
  api.mockImplementation(answer as typeof apiFetch)
  vi.mocked(logoutSession).mockClear()
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
  useDriveStore.setState({ selectedItemId: null, expandedFolders: new Set() })
})

const openSheet = async () => {
  const user = userEvent.setup()
  await user.click(await screen.findByRole('button', { name: 'Thêm' }))
  const dialog = await screen.findByRole('dialog', { name: 'Thêm' })
  return { user, dialog }
}

describe('MobileNav on a desktop width', () => {
  it('draws no bar and never asks for the pending count', async () => {
    viewport(false)
    await renderBar()
    await waitFor(() => expect(screen.queryByRole('navigation', { name: 'Điều hướng di động' })).toBeNull())
    expect(api).not.toHaveBeenCalledWith('/approval/pending')
  })

  it('asks for the pending count once the viewport is narrow', async () => {
    await renderBar()
    await screen.findByRole('link', { name: 'Phê duyệt, 2 chờ bạn' })
    expect(api).toHaveBeenCalledWith('/approval/pending')
  })
})

describe('MobileNav bar', () => {
  it('has three labelled tabs and Thêm, nothing else', async () => {
    await renderBar()
    const nav = await screen.findByRole('navigation', { name: 'Điều hướng di động' })
    await screen.findByRole('link', { name: 'Phê duyệt, 2 chờ bạn' })
    expect(within(nav).getAllByRole('link').map((a) => a.textContent)).toEqual(['Tin nhắn', 'Tài liệu', '2Phê duyệt'])
    expect(within(nav).getAllByRole('button').map((b) => b.textContent)).toEqual(['Thêm'])
  })

  it('shows the number waiting on the person on Phê duyệt only, capped at 99+', async () => {
    pending = 120
    await renderBar()
    const link = await screen.findByRole('link', { name: 'Phê duyệt, 120 chờ bạn' })
    expect(link).toHaveTextContent('99+')
    expect(screen.getByRole('link', { name: 'Tin nhắn' })).not.toHaveTextContent(/\d/)
  })

  it('shows no badge when nothing waits', async () => {
    pending = 0
    await renderBar()
    const link = await screen.findByRole('link', { name: 'Phê duyệt' })
    await waitFor(() => expect(api).toHaveBeenCalledWith('/approval/pending'))
    expect(link).not.toHaveTextContent(/\d/)
  })

  it('marks the current tab, and Tài liệu on a document', async () => {
    await renderBar('/documents/99999999-aaaa-4bbb-8ccc-000000000001')
    expect(await screen.findByRole('link', { name: 'Tài liệu' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Tin nhắn' })).not.toHaveAttribute('aria-current')
  })

  it('lights Thêm when the screen is one that lives in it', async () => {
    await renderBar('/assets')
    expect(await screen.findByRole('button', { name: 'Thêm' })).toHaveAttribute('data-current', 'true')
    for (const link of screen.getAllByRole('link')) expect(link).not.toHaveAttribute('aria-current')
  })

  it('draws no floating avatar: the bar is the only fixed thing on the screen', async () => {
    await renderBar()
    await screen.findByRole('navigation', { name: 'Điều hướng di động' })
    expect(Array.from(document.body.querySelectorAll('.fixed'))).toHaveLength(1)
    expect(document.body.querySelector('img, [data-avatar]')).toBeNull()
  })
})

describe('Thêm sheet', () => {
  it('lists the rest in order, with the person and Đăng xuất', async () => {
    await renderBar()
    const { dialog } = await openSheet()
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    const nav = within(dialog).getByRole('navigation', { name: 'Thêm' })
    expect(within(nav).getAllByRole('link').map((a) => a.textContent)).toEqual(['Tài sản', 'Danh bạ', 'Quản trị', 'Cài đặt'])
    expect(within(nav).getByRole('button', { name: 'Đăng xuất' })).toBeInTheDocument()
    expect(await within(dialog).findByText(/Lê Thị Hoa/)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: /Đổi workspace, đang ở Khối Vận hành/ })).toBeInTheDocument()
  })

  it('puts focus on the first row, and gives it back to Thêm on Esc', async () => {
    await renderBar()
    const { user, dialog } = await openSheet()
    await waitFor(() => expect(within(dialog).getByRole('button', { name: /Đổi workspace/ })).toHaveFocus())
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('button', { name: 'Thêm' })).toHaveFocus()
  })

  it('keeps Tab inside the sheet', async () => {
    await renderBar()
    const { user, dialog } = await openSheet()
    await waitFor(() => expect(dialog.contains(document.activeElement)).toBe(true))
    for (let i = 0; i < 12; i++) {
      await user.tab()
      expect(dialog.contains(document.activeElement)).toBe(true)
    }
  })

  it('closes from the Đóng button', async () => {
    await renderBar()
    const { user, dialog } = await openSheet()
    await user.click(within(dialog).getByRole('button', { name: 'Đóng Thêm' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('goes to the chosen destination and closes', async () => {
    const router = await renderBar()
    const { user, dialog } = await openSheet()
    await user.click(within(dialog).getByRole('link', { name: 'Danh bạ' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/contacts'))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('signs out through the one logout path', async () => {
    await renderBar()
    const { user, dialog } = await openSheet()
    await user.click(within(dialog).getByRole('button', { name: 'Đăng xuất' }))
    expect(logoutSession).toHaveBeenCalledTimes(1)
  })
})

describe('switching workspace from the sheet', () => {
  const openList = async () => {
    const opened = await openSheet()
    await opened.user.click(within(opened.dialog).getByRole('button', { name: /Đổi workspace, đang ở/ }))
    const list = await within(opened.dialog).findByRole('listbox', { name: 'Workspace của bạn' })
    return { ...opened, list }
  }

  it('offers only the workspaces the person can enter, the open one ticked', async () => {
    await renderBar()
    const { list } = await openList()
    const options = within(list).getAllByRole('option')
    expect(options.map((o) => o.getAttribute('aria-selected'))).toEqual(['true', 'false'])
    expect(options[0]).toHaveTextContent('Khối Vận hành')
    expect(options[1]).toHaveTextContent('NovaPay Kế toán')
    expect(within(list).queryByText('Chỉ xem được')).toBeNull()
  })

  it('re-scopes the token, drops the old data, resets Tài liệu and opens the workspace', async () => {
    const router = await renderBar('/drive')
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })
    useDriveStore.setState({ selectedItemId: 'item-1', expandedFolders: new Set(['f1']) })
    const { user, list, dialog } = await openList()

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.objectContaining({ method: 'POST', body: JSON.stringify({ tenant_id: OTHER_WS }) }))
    expect(useAuthStore.getState().accessToken).toBe(tokenFor(OTHER_WS))
    expect(useAuthStore.getState().tenantId).toBe(OTHER_WS)
    expect(queryClient.getQueryData(['drive', 'old'])).toBeUndefined()
    expect(useDriveStore.getState().selectedItemId).toBeNull()
    expect(useDriveStore.getState().expandedFolders.size).toBe(0)
    expect(router.state.location.pathname).toBe('/drive')
    expect(router.state.location.search).toEqual({ ws: OTHER_WS })
    expect(dialog).not.toBeInTheDocument()
  })

  it('leaves everything as it was when the server refuses', async () => {
    switchError = Object.assign(new Error('forbidden'), { status: 403 })
    await renderBar('/drive')
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })
    const { user, list } = await openList()

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.anything()))
    await waitFor(() => expect(within(list).getAllByRole('option')[1]).not.toBeDisabled())
    expect(useAuthStore.getState().accessToken).toBe(tokenFor(WS_ID))
    expect(queryClient.getQueryData(['drive', 'old'])).toBeDefined()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('does nothing but close when the open workspace is chosen', async () => {
    await renderBar()
    const { user, list } = await openList()
    await user.click(within(list).getAllByRole('option')[0]!)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(api).not.toHaveBeenCalledWith('/auth/switch-tenant', expect.anything())
  })
})
