import '@testing-library/jest-dom/vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { useAuthStore } from '../../stores/auth.store'
import { queryClient } from '../../lib/query-client'
import { AppSidebar } from './AppSidebar'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)
const OTHER_WS = '0b9a7d2f-3c4e-4f60-9bac-2d3e4f5a6b7c'
const HIDDEN_WS = '0c0b8e30-4d5f-4071-8cbd-3e4f5a6b7c8d'

async function renderSidebar(url = '/drive') {
  const root = createRootRoute({ component: () => <AppSidebar workspaceName="Khối Vận hành" unreadCounts={{ messaging: 3 }} /> })
  const pages = ['/drive', '/documents/$docId', '/contacts', '/settings', '/channels'].map((path) =>
    createRoute({ getParentRoute: () => root, path, component: () => null }))
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
}

beforeEach(() => {
  api.mockReset()
  localStorage.clear()
  switchPreferencesTo('user-a')
  api.mockImplementation((path: string) =>
    path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }, { id: OTHER_WS, name: 'NovaPay Kế toán' }, { id: HIDDEN_WS, name: 'Chỉ xem được' }] })
    : path === '/me/workspaces' ? Promise.resolve({ workspaces: [
        { id: WS_ID, name: 'Khối Vận hành', role: 'admin', member_count: 64, domain: '' },
        { id: OTHER_WS, name: 'NovaPay Kế toán', role: 'member', member_count: 12, domain: '' }] })
    : path === '/auth/switch-tenant' ? Promise.resolve({ access_token: `h.${btoa(JSON.stringify({ tenant_id: OTHER_WS }))}.s` })
    : path.endsWith('/contacts') ? Promise.resolve({ contacts: CONTACTS, total: CONTACTS.length })
    : Promise.reject(new Error(`unexpected ${path}`)))
})
afterEach(() => {
  cleanup()
  resetClient()
  switchPreferencesTo(undefined)
})

describe('AppSidebar', () => {
  it('names each destination and shows the unread count', async () => {
    await renderSidebar()
    const nav = await screen.findByRole('navigation', { name: 'Phân hệ' })
    expect(within(nav).getAllByRole('link').map((a) => a.textContent)).toEqual([
      'Tin nhắn3', 'Tài liệu', 'Phê duyệt', 'Tài sản', 'Danh bạ',
    ])
  })

  it('lights up Tài liệu on a document, since Văn bản lives there', async () => {
    await renderSidebar('/documents/99999999-aaaa-4bbb-8ccc-000000000001')
    const nav = await screen.findByRole('navigation', { name: 'Phân hệ' })
    expect(within(nav).getByRole('link', { name: 'Tài liệu' })).toHaveAttribute('aria-current', 'page')
    expect(within(nav).getByRole('link', { name: /Tin nhắn/ })).not.toHaveAttribute('aria-current')
  })

  it('collapses to a rail of icons that keep their names, and expands again', async () => {
    await renderSidebar()
    const aside = await screen.findByRole('complementary', { name: 'Điều hướng chính' })
    expect(aside).not.toHaveAttribute('data-rail')

    act(() => updatePreferences({ sidebarCollapsed: true }))
    await waitFor(() => expect(aside).toHaveAttribute('data-rail', 'true'))
    const nav = within(aside).getByRole('navigation', { name: 'Phân hệ' })
    for (const [name, label] of [['Tin nhắn', 'Tin nhắn, 3 chưa đọc'], ['Tài liệu', 'Tài liệu'], ['Phê duyệt', 'Phê duyệt'], ['Tài sản', 'Tài sản'], ['Danh bạ', 'Danh bạ']]) {
      const link = within(nav).getByRole('link', { name: label })
      expect(link).toHaveAttribute('title', name) // the tooltip
      expect(link.textContent).not.toContain(name) // icon only
    }
    expect(within(aside).getByRole('link', { name: 'Cài đặt' })).toHaveAttribute('title', 'Cài đặt')

    act(() => updatePreferences({ sidebarCollapsed: false }))
    await waitFor(() => expect(aside).not.toHaveAttribute('data-rail'))
    expect(within(nav).getByRole('link', { name: 'Tài liệu' })).toHaveTextContent('Tài liệu')
  })

  it('lists only workspaces the person can enter, and switching re-scopes the session', async () => {
    await renderSidebar()
    queryClient.setQueryData(['drive', 'old'], { items: ['x'] })
    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: /Khối Vận hành/ }))
    const list = await screen.findByRole('listbox', { name: 'Không gian làm việc' })
    await waitFor(() => expect(within(list).getAllByRole('option')).toHaveLength(2))
    expect(within(list).queryByText('Chỉ xem được')).toBeNull()

    await user.click(within(list).getByRole('option', { name: /NovaPay Kế toán/ }))

    await waitFor(() => expect(useAuthStore.getState().tenantId).toBe(OTHER_WS))
    expect(api).toHaveBeenCalledWith('/auth/switch-tenant', expect.objectContaining({ body: JSON.stringify({ tenant_id: OTHER_WS }) }))
    expect(queryClient.getQueryData(['drive', 'old'])).toBeUndefined()
    await waitFor(() => expect(screen.queryByRole('listbox')).toBeNull())
  })
})
