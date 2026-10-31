import '@testing-library/jest-dom/vitest'
import type { ReactNode } from 'react'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { MenuItem } from '../primitives'
import { ModuleHeader } from './ModuleHeader'
import { ModuleMenu } from './ModuleMenu'

const mockUseBottomBarLayout = vi.fn(() => true)
vi.mock('../../hooks/usePhone', () => ({ useBottomBarLayout: () => mockUseBottomBarLayout() }))

const WS = '5d1c2e3f-4a5b-4c6d-8e7f-9a0b1c2d3e4f'

async function renderAt(ui: ReactNode, url = `/approval?ws=${WS}`) {
  const root = createRootRoute({ component: () => <>{ui}<Outlet /></> })
  const keepWs = (s: Record<string, unknown>) => (typeof s.ws === 'string' ? { ws: s.ws } : {})
  const page = (path: string) => createRoute({ getParentRoute: () => root, path, validateSearch: keepWs, component: () => null })
  const router = createRouter({
    routeTree: root.addChildren([page('/approval'), page('/settings'), page('/settings/$section')]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  render(<RouterProvider router={router} />)
  return { router, user: userEvent.setup() }
}

beforeEach(() => mockUseBottomBarLayout.mockReturnValue(true))
afterEach(() => cleanup())

describe('the ⋯ of a module', () => {
  it('opens a menu whose first row is Cài đặt <phân hệ>, going to that settings page in the same workspace', async () => {
    const { router, user } = await renderAt(
      <ModuleMenu section="approval" label="Phê duyệt">
        <MenuItem>Đã lưu trữ</MenuItem>
      </ModuleMenu>,
    )
    await user.click(await screen.findByRole('button', { name: 'Thêm tuỳ chọn Phê duyệt' }))
    const menu = await screen.findByRole('menu', { name: 'Tuỳ chọn Phê duyệt' })
    const rows = within(menu).getAllByRole('menuitem')
    expect(rows.map((r) => r.textContent)).toEqual(['Cài đặt Phê duyệt', 'Đã lưu trữ'])
    await user.click(rows[0]!)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/approval'))
    expect(router.state.location.search).toEqual({ ws: WS })
    await waitFor(() => expect(screen.queryByRole('menu')).toBeNull())
  })

  it('draws nothing when there is neither a settings page nor another row', async () => {
    await renderAt(<ModuleMenu label="Danh bạ" />)
    await waitFor(() => expect(screen.queryByRole('button', { name: /Thêm tuỳ chọn/ })).toBeNull())
  })
})

describe('ModuleHeader', () => {
  it.each([
    ['approval', 'Phê duyệt', '/settings/approval'],
    ['assets', 'Tài sản', '/settings/assets'],
  ])('gives %s a ⋯ with Cài đặt %s first', async (moduleId, title, to) => {
    const { router, user } = await renderAt(<ModuleHeader moduleId={moduleId} title={title} />)
    await user.click(await screen.findByRole('button', { name: `Thêm tuỳ chọn ${title}` }))
    const menu = await screen.findByRole('menu', { name: `Tuỳ chọn ${title}` })
    await user.click(within(menu).getAllByRole('menuitem')[0]!)
    await waitFor(() => expect(router.state.location.pathname).toBe(to))
  })

  it('gives Danh bạ no ⋯: it has no settings page', async () => {
    await renderAt(<ModuleHeader moduleId="contacts" title="Danh bạ" />)
    expect(await screen.findByRole('heading', { name: 'Danh bạ' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Thêm tuỳ chọn/ })).toBeNull()
  })
})
