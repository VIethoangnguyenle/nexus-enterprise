import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { WS_ID, assetFixtureApi, resetFixtures } from '../../test/asset-fixtures'
import { validateAssetsSearch } from '../../lib/assets-search'
import { apiFetch } from '../../api/client'
import type { AssetSettings } from '../../api/settings'
import { queryClient } from '../../lib/query-client'
import { settingsQueryKey } from '../../hooks/useServerSettings'
import { AssetsScreen } from './AssetsScreen'

vi.mock('../../api/client', async (orig) => {
  const { assetFixtureApi } = await import('../../test/asset-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(assetFixtureApi) }
})
const api = vi.mocked(apiFetch)

async function renderAssets(url = '/assets') {
  const root = createRootRoute()
  const route = createRoute({
    getParentRoute: () => root, path: '/assets', validateSearch: validateAssetsSearch, component: AssetsScreen,
  })
  const settings = createRoute({ getParentRoute: () => root, path: '/settings/$section', component: () => null })
  const router = createRouter({
    routeTree: root.addChildren([route, settings]), history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

const seed = (s: AssetSettings) => queryClient.setQueryData(settingsQueryKey('asset', WS_ID), s)
const section = (r: Awaited<ReturnType<typeof renderAssets>>) => (r.state.location.search as { section?: string }).section

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  api.mockImplementation(assetFixtureApi)
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
})

describe('AssetsScreen: the desktop gear to Cài đặt Tài sản', () => {
  it('on a wide screen the header ends with the gear, which opens Cài đặt Tài sản in the same workspace', async () => {
    const user = userEvent.setup()
    const router = await renderAssets(`/assets?ws=${WS_ID}&section=requests`)
    const gear = await screen.findByRole('button', { name: 'Cài đặt Tài sản' })
    expect(gear).toHaveAttribute('title', 'Cài đặt Tài sản')
    await user.click(gear)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/assets'))
    // Only the workspace follows; the tab stays behind.
    expect(router.state.location.search).toEqual({ ws: WS_ID })
  })

  it('a phone has no gear: its ⋯ already leads there', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    await renderAssets()
    await screen.findAllByRole('heading', { name: 'Tài sản', level: 1 })
    expect(screen.queryByRole('button', { name: 'Cài đặt Tài sản' })).toBeNull()
  })
})

describe('AssetsScreen: Cài đặt Tài sản → Tab mở đầu', () => {
  it('Danh sách tài sản: opened without a tab, it moves to the list in place', async () => {
    seed({ default_tab: 'inventory' })
    const router = await renderAssets()
    await waitFor(() => expect(section(router)).toBe('list'))
    expect(await screen.findByRole('tab', { name: 'Danh sách', selected: true })).toBeInTheDocument()
    // In place: going back does not land on Tổng quan first.
    expect(router.history.length).toBe(1)
  })

  it('Yêu cầu: opens on the requests', async () => {
    seed({ default_tab: 'requests' })
    const router = await renderAssets()
    await waitFor(() => expect(section(router)).toBe('requests'))
  })

  it('Tổng quan (the default): stays on Tổng quan', async () => {
    seed({ default_tab: 'overview' })
    const router = await renderAssets()
    await screen.findByRole('tab', { name: 'Tổng quan', selected: true })
    expect(section(router)).toBeUndefined()
  })

  it('a tab in the address wins, and choosing Tổng quan afterwards is left alone', async () => {
    const user = userEvent.setup()
    seed({ default_tab: 'inventory' })
    const router = await renderAssets('/assets?section=requests')
    await screen.findByRole('tab', { name: /Yêu cầu/, selected: true })
    expect(section(router)).toBe('requests')
    await user.click(screen.getByRole('tab', { name: 'Tổng quan' }))
    await waitFor(() => expect(section(router)).toBeUndefined())
    expect(screen.getByRole('tab', { name: 'Tổng quan', selected: true })).toBeInTheDocument()
  })
})
