import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../api/client'
import { clearResolvers } from '../lib/people-resolver'
import { displayName } from '../lib/people'
import { usePeople } from './usePeople'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
vi.mock('./useWorkspaceType', () => ({
  useWorkspaceTypeOf: () => ({ type: 'organization', isPersonal: false, isLoading: false }),
}))
const api = vi.mocked(apiFetch)

const HIDDEN = '00000000-0000-4000-8000-0000000000ff'
const VISIBLE = '00000000-0000-4000-8000-000000000001'
const visible = {
  user_id: VISIBLE, ngac_node_id: '00000000-0000-4000-9000-000000000001', username: 'duc', display_name: 'Trần Minh Đức', email: '', title: '',
  department: '', department_id: '', roles: [], location: '', avatar_url: '', is_online: false,
}

const createWrapper = () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

let resolveCalls: unknown[]
beforeEach(() => {
  clearResolvers()
  resolveCalls = []
  api.mockReset()
  api.mockImplementation((url: string, init?: RequestInit) => {
    if (url.endsWith('/contacts/resolve')) {
      resolveCalls.push(JSON.parse(String(init?.body)))
      return Promise.resolve({ people: [{ user_id: HIDDEN, ngac_node_id: '00000000-0000-4000-9000-0000000000ff', display_name: 'Phạm Ẩn', avatar_url: '' }] })
    }
    return Promise.resolve({ contacts: [visible], total: 1, visibility: 'limited' })
  })
})

describe('usePeople: people the directory hides', () => {
  it('names a hidden person from the resolver without listing them, once the directory has loaded', async () => {
    const { result } = renderHook(
      () => {
        const dir = usePeople('w1')
        return { dir, name: displayName(dir, HIDDEN) }
      },
      { wrapper: createWrapper() },
    )
    // Until the directory is known, nobody is asked about: a name the directory has would be fetched for nothing.
    expect(resolveCalls).toEqual([])
    await waitFor(() => expect(result.current.name).toBe('Phạm Ẩn'))
    expect(resolveCalls).toEqual([{ user_ids: [HIDDEN] }])
    expect(result.current.dir.isVisible(HIDDEN)).toBe(false)
    expect(result.current.dir.isVisible(VISIBLE)).toBe(true)
    expect(result.current.dir.list.map((p) => p.userId)).toEqual([VISIBLE])
  })

  it('never asks about someone the directory lists', async () => {
    const { result } = renderHook(() => { const dir = usePeople('w1'); return displayName(dir, VISIBLE) }, { wrapper: createWrapper() })
    await waitFor(() => expect(result.current).toBe('Trần Minh Đức'))
    await new Promise((r) => setTimeout(r, 120))
    expect(resolveCalls).toEqual([])
  })

  it('answers a direct id lookup of a hidden person, by user id and by node id, with no call site changed', async () => {
    const NODE = '00000000-0000-4000-9000-0000000000ff'
    const { result } = renderHook(() => usePeople('w1'), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.ready).toBe(true))
    expect(result.current.byUserId.get(HIDDEN)).toBeUndefined() // the first look only asks
    expect(result.current.byNodeId.get(NODE)).toBeUndefined()
    await waitFor(() => expect(result.current.byUserId.get(HIDDEN)?.name).toBe('Phạm Ẩn'))
    expect(result.current.byNodeId.get(NODE)?.userId).toBe(HIDDEN)
    expect(resolveCalls).toEqual([{ user_ids: [HIDDEN], node_ids: [NODE] }])
    // Answered: looking again asks nothing more.
    await new Promise((r) => setTimeout(r, 120))
    expect(resolveCalls).toHaveLength(1)
  })

  it('is not ready while the contacts load, nor when they fail, and then asks about nobody', async () => {
    api.mockReset()
    api.mockImplementation((url: string) =>
      url.endsWith('/contacts/resolve') ? Promise.resolve({ people: [] }) : Promise.reject(new Error('down')))
    const { result } = renderHook(() => { const d = usePeople('w1'); d.byUserId.get(HIDDEN); return d }, { wrapper: createWrapper() })
    expect(result.current.ready).toBe(false)
    await new Promise((r) => setTimeout(r, 150))
    expect(result.current.ready).toBe(false)
    expect(api.mock.calls.some((c) => String(c[0]).endsWith('/resolve'))).toBe(false)
  })
})
