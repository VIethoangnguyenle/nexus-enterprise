import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { queryClient } from '../lib/query-client'
import { jwtFor as tenantToken } from '../test/auth-fixtures'
import { routerMockFactory, routerState } from '../test/router-mock'
import { useAuthStore } from '../stores/auth.store'

const switchTenant = vi.fn()
vi.mock('../api/auth', () => ({ authApi: { switchTenant: (id: string) => switchTenant(id) } }))
vi.mock('@tanstack/react-router', () => routerMockFactory())
const toastError = vi.fn()
vi.mock('../components/primitives/Toast', () => ({ toast: { error: (m: string) => toastError(m) } }))

let active = { workspaceId: 'ws-2', workspaces: [{ id: 'ws-1' }, { id: 'ws-2' }] }
vi.mock('./useActiveWorkspace', () => ({ useActiveWorkspace: () => active }))

const { useTenantSync } = await import('./useTenantSync')

const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
const setToken = (tenant: string) => useAuthStore.setState({ accessToken: tenantToken(tenant), tenantId: tenant })

beforeEach(() => {
  cleanup()
  vi.clearAllMocks()
  active = { workspaceId: 'ws-2', workspaces: [{ id: 'ws-1' }, { id: 'ws-2' }] }
})

describe('useTenantSync', () => {
  it('re-scopes the token to the open workspace without navigating, and stops syncing once it matches', async () => {
    setToken('ws-1')
    switchTenant.mockResolvedValue({ access_token: tenantToken('ws-2') })
    const { result } = renderHook(() => useTenantSync(), { wrapper })
    expect(result.current.syncing).toBe(true)
    await waitFor(() => expect(result.current.syncing).toBe(false))
    expect(switchTenant).toHaveBeenCalledTimes(1)
    expect(switchTenant).toHaveBeenCalledWith('ws-2')
    expect(routerState.navigate).not.toHaveBeenCalled()
  })

  it('does nothing when the token already speaks for the open workspace', () => {
    setToken('ws-2')
    const { result } = renderHook(() => useTenantSync(), { wrapper })
    expect(result.current.syncing).toBe(false)
    expect(switchTenant).not.toHaveBeenCalled()
  })

  it('does not call twice across re-renders while the switch is in flight', async () => {
    setToken('ws-1')
    let release!: (v: { access_token: string }) => void
    switchTenant.mockReturnValue(new Promise((r) => { release = r }))
    const { rerender } = renderHook(() => useTenantSync(), { wrapper })
    await waitFor(() => expect(switchTenant).toHaveBeenCalledTimes(1))
    rerender()
    rerender()
    expect(switchTenant).toHaveBeenCalledTimes(1)
    await act(async () => release({ access_token: tenantToken('ws-2') }))
  })

  it('on refusal drops to the token workspace in the URL, replacing history, and toasts', async () => {
    setToken('ws-1')
    switchTenant.mockRejectedValue(new Error('forbidden'))
    const { rerender } = renderHook(() => useTenantSync(), { wrapper })
    await waitFor(() => expect(routerState.navigate).toHaveBeenCalledTimes(1))
    const arg = routerState.navigate.mock.calls[0]![0] as { replace: boolean; search: (p: object) => object }
    expect(arg.replace).toBe(true)
    expect(arg.search({ other: 'x' })).toEqual({ other: 'x', ws: 'ws-1' })
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(useAuthStore.getState().accessToken).toBe(tenantToken('ws-1'))
    rerender()
    expect(switchTenant).toHaveBeenCalledTimes(1)
  })
})
