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
  const router = createRouter({ routeTree: root.addChildren([route]), history: createMemoryHistory({ initialEntries: [url] }) })
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
  vi.restoreAllMocks()
  resetClient()
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
