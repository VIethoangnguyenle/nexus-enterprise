import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { NGAC_OPS } from '../api/access'

const batchCheckAccess = vi.fn()
vi.mock('../api/access', async (orig) => ({
  ...(await orig<typeof import('../api/access')>()),
  batchCheckAccess: (...a: unknown[]) => batchCheckAccess(...a),
}))

const { usePermissions, useObjectPermissions } = await import('./usePermissions')

const allow = (...ops: string[]) => Object.fromEntries(NGAC_OPS.map((op) => [op, ops.includes(op)]))

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const Wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  return { client, Wrapper }
}

beforeEach(() => {
  batchCheckAccess.mockReset()
})

describe('usePermissions', () => {
  it('fetches several objects in one batch call and denies until answered', async () => {
    batchCheckAccess.mockResolvedValue({
      results: { a: allow('read', 'write'), b: allow('read') },
    })
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => usePermissions(['a', 'b']), { wrapper: Wrapper })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.permsMap.a!.write).toBe(false)

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(batchCheckAccess).toHaveBeenCalledTimes(1)
    expect([...batchCheckAccess.mock.calls[0]![0]].sort()).toEqual(['a', 'b'])
    expect(result.current.permsMap.a).toMatchObject({ read: true, write: true, share: false })
    expect(result.current.permsMap.b).toMatchObject({ read: true, write: false })
  })

  it('shares the answer between components instead of asking again', async () => {
    batchCheckAccess.mockResolvedValue({ results: { a: allow('read') } })
    const { Wrapper } = wrapper()
    const first = renderHook(() => useObjectPermissions('a'), { wrapper: Wrapper })
    await waitFor(() => expect(first.result.current.isLoading).toBe(false))
    const second = renderHook(() => useObjectPermissions('a'), { wrapper: Wrapper })

    expect(second.result.current.read).toBe(true)
    expect(batchCheckAccess).toHaveBeenCalledTimes(1)
  })

  it('treats an object the server did not answer for as denied', async () => {
    batchCheckAccess.mockResolvedValue({ results: {} })
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useObjectPermissions('ghost'), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(NGAC_OPS.every((op) => result.current[op] === false)).toBe(true)
  })

  it('denies when the check itself fails', async () => {
    batchCheckAccess.mockRejectedValue(new Error('down'))
    const { Wrapper } = wrapper()
    const { result } = renderHook(() => useObjectPermissions('a'), { wrapper: Wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.write).toBe(false)
  })

  it('asks for nothing when there is nothing to ask about', () => {
    const { Wrapper } = wrapper()
    renderHook(() => usePermissions([]), { wrapper: Wrapper })
    expect(batchCheckAccess).not.toHaveBeenCalled()
  })
})
