import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from '@tanstack/react-router'
import { routeTree } from '../routeTree.gen'

/** The router with the app's real route tree, loaded at `url`; what the address bar ends up as. */
async function landing(url: string) {
  const router = createRouter({ routeTree, history: createMemoryHistory({ initialEntries: [url] }) })
  await router.load()
  return { path: router.state.location.pathname, search: router.state.location.search as Record<string, unknown> }
}

const D = '99999999-aaaa-4bbb-8ccc-000000000001'

// Văn bản became a group inside Tài liệu. The old page is gone; its address must
// still land somewhere sensible, and a document keeps its own address.
describe('/documents', () => {
  it('opens the Văn bản group of Tài liệu', async () => {
    expect(await landing('/documents')).toEqual({ path: '/drive', search: { view: 'texts' } })
  })

  it('keeps the workspace the link named', async () => {
    expect(await landing('/documents?ws=w-9')).toEqual({ path: '/drive', search: { ws: 'w-9', view: 'texts' } })
  })

  it('a document opens at its own address', async () => {
    expect((await landing(`/documents/${D}`)).path).toBe(`/documents/${D}`)
  })

  it('/drive?view=texts&group=drafts keeps its group', async () => {
    expect(await landing('/drive?view=texts&group=drafts')).toEqual({ path: '/drive', search: { view: 'texts', group: 'drafts' } })
  })

  it('/settings keeps its tab', async () => {
    expect(await landing('/settings?tab=giao-dien')).toEqual({ path: '/settings', search: { tab: 'giao-dien' } })
    expect(await landing('/settings?ws=w-9&tab=workspace')).toEqual({ path: '/settings', search: { ws: 'w-9', tab: 'workspace' } })
  })
})
