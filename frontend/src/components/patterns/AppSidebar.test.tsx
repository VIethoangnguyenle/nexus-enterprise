import '@testing-library/jest-dom/vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import { switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { AppSidebar } from './AppSidebar'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

async function renderSidebar(url = '/drive') {
  const root = createRootRoute({ component: () => <AppSidebar workspaceName="Khối Vận hành" unreadCounts={{ messaging: 3 }} /> })
  const pages = ['/drive', '/documents/$docId', '/contacts', '/settings', '/channels'].map((path) =>
    createRoute({ getParentRoute: () => root, path, component: () => null }))
  const router = createRouter({ routeTree: root.addChildren(pages), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
}

beforeEach(() => {
  localStorage.clear()
  switchPreferencesTo('user-a')
  api.mockImplementation((path: string) =>
    path === '/workspaces' ? Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] })
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
})
