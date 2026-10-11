import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from '@tanstack/react-router'
import { QueryClient } from '@tanstack/react-query'
import { routeTree } from '../routeTree.gen'
import { keys } from '../hooks/keys'

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
})
