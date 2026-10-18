import { beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { queryClient } from '../lib/query-client'
import { jwtFor as tenantToken } from '../test/auth-fixtures'
import { routerMockFactory, routerState } from '../test/router-mock'
import { useAuthStore } from '../stores/auth.store'
import { validateChannelSearch } from '../routes/_workspace/channels.$channelId'
import { validateCallSearch } from '../routes/_workspace/calls.$callId'

const switchTenant = vi.fn()
vi.mock('../api/auth', () => ({ authApi: { switchTenant: (id: string) => switchTenant(id) } }))
vi.mock('@tanstack/react-router', async () => ({ ...(await vi.importActual<object>('@tanstack/react-router')), ...routerMockFactory() }))
vi.mock('../components/primitives/Toast', () => ({ toast: { error: vi.fn() } }))
vi.mock('./useWorkspaces', () => ({
  useWorkspaces: () => ({ data: { workspaces: [{ id: 'ws-a', name: 'A' }, { id: 'ws-b', name: 'B' }] }, isLoading: false, isError: false }),
}))

const { useTenantSync } = await import('./useTenantSync')
const { useActiveWorkspace } = await import('./useActiveWorkspace')

const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>

/** What the router hands a route for a link: the query string, through the route's own validator. */
function searchOf(url: string, validate: (s: Record<string, unknown>) => object): object {
  return validate(Object.fromEntries(new URL(url, 'https://nexus.test').searchParams))
}

beforeEach(() => {
  cleanup()
  vi.clearAllMocks()
  routerState.search = {}
})

// A push or a call ring for a DM names the DM's workspace (`?ws=`). Following
// it from a session signed in to another workspace re-scopes the session to
// that workspace and the conversation opens there.
describe('following a link to a direct message of another workspace', () => {
  const follow = async (path: string, validate: (s: Record<string, unknown>) => object) => {
    useAuthStore.setState({ accessToken: tenantToken('ws-a'), tenantId: 'ws-a' })
    switchTenant.mockResolvedValue({ access_token: tenantToken('ws-b') })
    routerState.search = searchOf(path, validate) as Record<string, unknown>
    return renderHook(() => ({ sync: useTenantSync(), active: useActiveWorkspace() }), { wrapper })
  }

  it('a push to a DM switches the session to the DM\'s workspace and keeps the message to focus', async () => {
    const { result } = await follow('/channels/dm-1?ws=ws-b&message=msg-1', validateChannelSearch)
    expect(routerState.search).toEqual({ ws: 'ws-b', message: 'msg-1' })
    expect(result.current.active.workspaceId).toBe('ws-b')
    await waitFor(() => expect(switchTenant).toHaveBeenCalledWith('ws-b'))
    await waitFor(() => expect(result.current.sync.syncing).toBe(false))
    expect(switchTenant).toHaveBeenCalledTimes(1)
  })

  it('a call ring switches the session to the DM\'s workspace and keeps how to answer', async () => {
    const { result } = await follow('/calls/call-1?ws=ws-b&answer=video', validateCallSearch)
    expect(routerState.search).toEqual({ ws: 'ws-b', answer: 'video' })
    expect(result.current.active.workspaceId).toBe('ws-b')
    await waitFor(() => expect(switchTenant).toHaveBeenCalledWith('ws-b'))
  })

  it('a link without a workspace stays in the one the session speaks for (the old links)', async () => {
    const { result } = await follow('/channels/dm-1', validateChannelSearch)
    expect(result.current.active.workspaceId).toBe('ws-a')
    expect(switchTenant).not.toHaveBeenCalled()
  })

  it('a link to a workspace the person does not have is ignored', async () => {
    const { result } = await follow('/channels/dm-1?ws=ws-gone', validateChannelSearch)
    expect(result.current.active.workspaceId).toBe('ws-a')
    expect(switchTenant).not.toHaveBeenCalled()
  })
})
