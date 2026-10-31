import { describe, expect, it, vi } from 'vitest'

vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  createFileRoute: () => (options: Record<string, unknown>) => ({ options }),
}))

type Options = { beforeLoad?: (ctx: unknown) => void; component?: unknown }
const optionsOf = (route: unknown) => (route as { options: Options }).options

const workspace = optionsOf((await import('./_workspace')).Route)
const index = optionsOf((await import('./index')).Route)
const { requireSignedInToWorkspace } = await import('../lib/auth-guards')

/*
 * A signed-out visitor opening the site used to be routed through the workspace
 * shell: its code was downloaded, it asked for the workspaces without a session,
 * and the 401 that came back emptied the cache the sign-in screen was reading.
 * The decision now happens before any of it loads.
 */
describe('entering the app', () => {
  it('the workspace turns a signed-out person away before it loads', () => {
    expect(workspace.beforeLoad).toBe(requireSignedInToWorkspace)
  })

  it('the front page forwards to chat from the router, without rendering anything first', () => {
    expect(index.component).toBeUndefined()
    let to: string | undefined
    try {
      index.beforeLoad!({})
    } catch (e) {
      to = (e as { options?: { to?: string } }).options?.to
    }
    expect(to).toBe('/channels')
  })
})
