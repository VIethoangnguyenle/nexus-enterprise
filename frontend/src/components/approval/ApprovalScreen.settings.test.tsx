import { cleanup, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { REQUESTS, WS_ID, approvalFixtureApi, resetFixtures } from '../../test/approval-fixtures'

import { validateApprovalSearch } from '../../lib/approval-search'
import { apiFetch } from '../../api/client'
import type { ApprovalSettings } from '../../api/settings'
import { queryClient } from '../../lib/query-client'
import { settingsQueryKey } from '../../hooks/useServerSettings'
import { useWebSocketStore } from '../../stores/websocket.store'
import { ApprovalScreen } from './ApprovalScreen'

vi.mock('../../api/client', async (orig) => {
  const { approvalFixtureApi } = await import('../../test/approval-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(approvalFixtureApi) }
})
const api = vi.mocked(apiFetch)
const ORIGINAL = structuredClone(REQUESTS)

async function renderApproval(url = '/approval') {
  const root = createRootRoute()
  const route = createRoute({
    getParentRoute: () => root, path: '/approval', validateSearch: validateApprovalSearch, component: ApprovalScreen,
  })
  const router = createRouter({ routeTree: root.addChildren([route]), history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

function viewport(narrow: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: narrow && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
}

const seed = (s: ApprovalSettings) => queryClient.setQueryData(settingsQueryKey('approval', WS_ID), s)

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  for (const [id, r] of Object.entries(structuredClone(ORIGINAL))) REQUESTS[id] = r
  api.mockImplementation(approvalFixtureApi)
  useWebSocketStore.setState({ approvalActivity: {} })
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  resetClient()
})

/** The titles of Chờ tôi duyệt, top to bottom. */
async function pendingTitles(): Promise<string[]> {
  const table = await screen.findByRole('table')
  return waitFor(() => {
    const titles = [...table.querySelectorAll<HTMLElement>('[data-row]')].map((t) => t.textContent ?? '')
    expect(titles.length).toBeGreaterThan(1)
    return titles
  })
}

/** The server's order of Chờ tôi duyệt in the fixtures: the request of 9 October, then the one of 8 October. */
async function servedTitles(): Promise<string[]> {
  viewport(false)
  await renderApproval()
  const titles = await pendingTitles()
  cleanup()
  resetClient()
  return titles
}

describe('ApprovalScreen: Cài đặt Phê duyệt → Sắp xếp', () => {
  it('below 1024px Mới nhất trước puts the newest request first; Cũ nhất trước the oldest', async () => {
    const [newer, older] = await servedTitles()
    viewport(true)
    seed({ sort: 'oldest' })
    await renderApproval()
    expect(await pendingTitles()).toEqual([older, newer])
    cleanup()
    resetClient()

    seed({ sort: 'newest' })
    await renderApproval()
    expect(await pendingTitles()).toEqual([newer, older])
  })

  it('a wide screen keeps the server order, whatever the setting', async () => {
    const served = await servedTitles()
    viewport(false)
    seed({ sort: 'oldest' })
    await renderApproval()
    expect(await pendingTitles()).toEqual(served)
  })
})
