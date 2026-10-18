import { act, cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../test/render'
import { CH, U, fixtureApi } from '../test/chat-fixtures'
import { apiFetch } from '../api/client'
import { parseChatSearch } from '../lib/chat-search'
import { validateWorkspaceSearch } from '../lib/workspace'
import { SearchRoute } from './_workspace/-search-route'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))

const server = (path: string, init?: RequestInit): Promise<unknown> =>
  path.startsWith('/messages/search') ? Promise.resolve({ messages: [], has_more: false }) : fixtureApi(path, init)

async function start() {
  const root = createRootRoute({ component: Outlet })
  const home = createRoute({ getParentRoute: () => root, path: '/channels', component: () => <p>Trang chủ</p> })
  const one = createRoute({ getParentRoute: () => root, path: '/channels/$channelId', component: () => <p>Cuộc trò chuyện</p> })
  const search = createRoute({
    getParentRoute: () => root, path: '/channels/search', component: SearchRoute,
    validateSearch: (s: Record<string, unknown>) => ({ ...validateWorkspaceSearch(s), ...parseChatSearch(s) }),
  })
  const router = createRouter({
    routeTree: root.addChildren([home, one, search]),
    history: createMemoryHistory({ initialEntries: [`/channels/${CH.doisoat}`, '/channels/search?q=%C4%91%E1%BB%91i+so%C3%A1t'], initialIndex: 1 }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  localStorage.clear()
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: false, media: q, addEventListener: () => {}, removeEventListener: () => {} }))
  vi.mocked(apiFetch).mockImplementation(server as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('filters on the desktop results page', () => {
  it('change the address without adding history: back leaves the page for where the search was opened', async () => {
    const user = userEvent.setup()
    const router = await start()
    await user.click(await screen.findByRole('button', { name: 'Có tệp' }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ file: 1 }))
    await user.click(screen.getByRole('button', { name: 'Người gửi' }))
    await user.click(await screen.findByRole('radio', { name: /Lê Quang Vinh/ }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ file: 1, from: U.vinh.id }))
    await user.click(screen.getByRole('button', { name: 'Có liên kết' }))
    await waitFor(() => expect(router.state.location.search).toMatchObject({ link: 1 }))

    await act(async () => router.history.back())
    await waitFor(() => expect(router.state.location.pathname).toBe(`/channels/${CH.doisoat}`))
    expect(router.history.canGoBack()).toBe(false)
  })

  it('come back from the address alone on a reload', async () => {
    const router = await start()
    await act(async () => router.navigate({ to: '/channels/search', search: { q: 'đối soát', file: 1, me: 1 }, replace: true }))
    expect(await screen.findByRole('button', { name: 'Có tệp' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Nhắc đến tôi' })).toHaveAttribute('aria-pressed', 'true')
  })
})
