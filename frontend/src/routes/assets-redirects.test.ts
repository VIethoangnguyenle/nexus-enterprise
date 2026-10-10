import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from '@tanstack/react-router'
import { routeTree } from '../routeTree.gen'

/** The router with the app's real route tree, loaded at `url`; what the address bar ends up as. */
async function landing(url: string) {
  const router = createRouter({ routeTree, history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  return { path: router.state.location.pathname, search: router.state.location.search as Record<string, unknown> }
}

const A = '77777777-aaaa-4bbb-8ccc-000000000001'

// The old /assets pages were a layout of their own with a sub-page each. They
// became tabs of one screen inside the workspace shell; a saved or shared link
// to any of them must still land somewhere sensible.
describe('old /assets URLs', () => {
  it.each([
    ['/assets/dashboard', {}],
    ['/assets/list', { section: 'list' }],
    ['/assets/requests', { section: 'requests' }],
    ['/assets/types', { section: 'types' }],
    ['/assets/request/new', { section: 'requests', compose: 'request' }],
    [`/assets/${A}`, { section: 'list', asset: A }],
  ])('%s opens the matching tab of /assets', async (from, search) => {
    const to = await landing(from)
    expect(to.path).toBe('/assets')
    expect(to.search).toEqual(search)
  })

  it('keeps the workspace the link named', async () => {
    const to = await landing('/assets/requests?ws=w-9')
    expect(to).toEqual({ path: '/assets', search: { ws: 'w-9', section: 'requests' } })
  })

  it('/assets itself is the overview, under the workspace shell, with its own state in the URL', async () => {
    const to = await landing('/assets?section=list&state=assigned&page=2&asset=' + A)
    expect(to).toEqual({ path: '/assets', search: { section: 'list', state: 'assigned', page: 2, asset: A } })
  })
})
