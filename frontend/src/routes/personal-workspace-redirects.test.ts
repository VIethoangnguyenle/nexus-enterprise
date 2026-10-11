import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from '@tanstack/react-router'
import { QueryClient } from '@tanstack/react-query'
import { routeTree } from '../routeTree.gen'
import { keys } from '../hooks/keys'
import { apiFetch } from '../api/client'

const areaRendered = vi.fn()
vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
vi.mock('../components/assets/AssetsScreen', () => ({ AssetsScreen: () => { areaRendered('assets'); return null } }))
vi.mock('../components/approval/ApprovalScreen', () => ({ ApprovalScreen: () => { areaRendered('approval'); return null } }))

beforeEach(() => {
  areaRendered.mockClear()
  vi.mocked(apiFetch).mockReset()
})

/** Nothing is cached: a reload, a bookmark, a notification link or a workspace switch. */
async function landingCold(url: string, workspaces: Array<{ id: string; type?: string }>) {
  vi.mocked(apiFetch).mockImplementation((async (path: string) => {
    if (path === '/workspaces') return { workspaces }
    throw new Error(`unexpected request ${path}`)
  }) as typeof apiFetch)
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [url] }),
    context: { queryClient },
  })
  await router.load()
  return { path: router.state.location.pathname, search: router.state.location.search as Record<string, unknown> }
}

async function landingWithCache(url: string, workspaces: Array<{ id: string; type?: string }>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  queryClient.setQueryData(keys.workspaces.all(), { workspaces })
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [url] }),
    context: { queryClient },
  })
  await router.load()
  return { path: router.state.location.pathname, search: router.state.location.search as Record<string, unknown> }
}

describe('Personal workspace route guards (/assets and /approval)', () => {
  it('redirects /assets to /channels in a personal workspace, keeping ws search param', async () => {
    const to = await landingWithCache('/assets?ws=p-1', [{ id: 'p-1', type: 'personal' }])
    expect(to.path).toBe('/channels')
    expect(to.search).toEqual({ ws: 'p-1' })
  })

  it('redirects /approval to /channels in a personal workspace, keeping ws search param', async () => {
    const to = await landingWithCache('/approval?ws=p-1', [{ id: 'p-1', type: 'personal' }])
    expect(to.path).toBe('/channels')
    expect(to.search).toEqual({ ws: 'p-1' })
  })

  it('redirects /assets without search param if no ws was given', async () => {
    const to = await landingWithCache('/assets', [{ id: 'p-1', type: 'personal' }])
    expect(to.path).toBe('/channels')
    expect(to.search).toEqual({})
  })

  it('stays on /assets in an organization workspace', async () => {
    const to = await landingWithCache('/assets?ws=org-1', [{ id: 'org-1', type: 'organization' }])
    expect(to.path).toBe('/assets')
    expect(to.search).toEqual({ ws: 'org-1' })
  })

  it('stays on /approval in an organization workspace', async () => {
    const to = await landingWithCache('/approval?ws=org-1', [{ id: 'org-1', type: 'organization' }])
    expect(to.path).toBe('/approval')
    expect(to.search).toEqual({ ws: 'org-1' })
  })

  describe('with nothing cached', () => {
    it.each(['/assets', '/approval'])('redirects %s in a personal workspace before the area renders or asks for its data', async (area) => {
      const to = await landingCold(`${area}?ws=p-1`, [{ id: 'org-1', type: 'organization' }, { id: 'p-1', type: 'personal' }])
      expect(to.path).toBe('/channels')
      expect(to.search).toEqual({ ws: 'p-1' })
      expect(areaRendered).not.toHaveBeenCalled()
      expect(vi.mocked(apiFetch).mock.calls.map((c) => c[0])).toEqual(['/workspaces'])
    })

    it('redirects when no ws is named and the first workspace is personal', async () => {
      const to = await landingCold('/assets', [{ id: 'p-1', type: 'personal' }])
      expect(to.path).toBe('/channels')
      expect(to.search).toEqual({})
    })

    it.each(['/assets', '/approval'])('lets %s through in an organization workspace', async (area) => {
      const to = await landingCold(`${area}?ws=org-1`, [{ id: 'org-1', type: 'organization' }])
      expect(to.path).toBe(area)
      expect(to.search).toEqual({ ws: 'org-1' })
    })

    it('lets the area through when the workspace list cannot be fetched', async () => {
      vi.mocked(apiFetch).mockRejectedValue(new Error('offline'))
      const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
      const router = createRouter({
        routeTree,
        history: createMemoryHistory({ initialEntries: ['/assets?ws=p-1'] }),
        context: { queryClient },
      })
      await router.load()
      expect(router.state.location.pathname).toBe('/assets')
    })
  })
})
