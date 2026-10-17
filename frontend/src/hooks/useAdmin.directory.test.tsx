import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminApi } from '../api/admin'
import { keys } from './keys'
import {
  useDeleteDepartment, useDepartments, useDirectoryScopeRules, useHideDepartment, useLimitSubject, useUnhideDepartment, useUnlimitSubject,
} from './useAdmin'

vi.mock('../api/admin', async (orig) => {
  const real = await orig<typeof import('../api/admin')>()
  return {
    ...real,
    adminApi: {
      ...real.adminApi,
      listDepartments: vi.fn(),
      deleteDepartment: vi.fn(),
      getDirectoryScope: vi.fn(),
      hideDepartment: vi.fn(),
      unhideDepartment: vi.fn(),
      limitSubject: vi.fn(),
      unlimitSubject: vi.fn(),
    },
  }
})
const api = vi.mocked(adminApi)

let client: QueryClient
const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  Object.values(api).forEach((f) => vi.isMockFunction(f) && f.mockReset())
})

describe('departments for a category', () => {
  it('are not fetched until the category is open', async () => {
    api.listDepartments.mockResolvedValue({ departments: [] })
    const { result, rerender } = renderHook(({ on }) => useDepartments('w1', on), { wrapper, initialProps: { on: false } })
    await new Promise((r) => setTimeout(r, 20))
    expect(api.listDepartments).not.toHaveBeenCalled()
    expect(result.current.fetchStatus).toBe('idle')
    rerender({ on: true })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.listDepartments).toHaveBeenCalledTimes(1)
  })

  it('keep the hidden mark an administrator gets', async () => {
    api.listDepartments.mockResolvedValue({ departments: [{ id: 'd', name: 'Pháp chế', parent_id: '', member_count: 2, hidden: true }] })
    const { result } = renderHook(() => useDepartments('w1'), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.flat[0]?.hidden).toBe(true)
  })
})

describe('directory scope rules', () => {
  it('are read as the server sends them, including "not ready" for a workspace with no directory yet', async () => {
    api.getDirectoryScope.mockResolvedValue({ ready: false, hidden_departments: [], limited: [] })
    const { result } = renderHook(() => useDirectoryScopeRules('w1'), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.ready).toBe(false)
    expect(result.current.data?.limited).toEqual([])
  })

  it('are not fetched without a workspace', async () => {
    renderHook(() => useDirectoryScopeRules(''), { wrapper })
    await new Promise((r) => setTimeout(r, 20))
    expect(api.getDirectoryScope).not.toHaveBeenCalled()
  })
})

describe('changing the rules', () => {
  const stale = () => {
    const spy = vi.spyOn(client, 'invalidateQueries')
    return () => spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))
  }

  it.each([
    ['hide', () => useHideDepartment('w1'), (m: { mutate: (v: string) => void }) => m.mutate('d'), () => api.hideDepartment],
    ['unhide', () => useUnhideDepartment('w1'), (m: { mutate: (v: string) => void }) => m.mutate('d'), () => api.unhideDepartment],
  ])('%s refreshes the rules, the departments and the directory', async (_n, hook, run, fn) => {
    const calls = stale()
    fn().mockResolvedValue({ status: 'ok' })
    const { result } = renderHook(hook, { wrapper })
    run(result.current as never)
    await waitFor(() => expect(fn()).toHaveBeenCalledWith('w1', 'd'))
    await waitFor(() => expect(calls()).toEqual(expect.arrayContaining([
      JSON.stringify(keys.admin.directoryScope('w1')),
      JSON.stringify(keys.admin.departments('w1')),
      JSON.stringify(keys.contacts.all('w1')),
    ])))
  })

  it('limit and unlimit pass the kind and the subject, and refresh the same three', async () => {
    const calls = stale()
    api.limitSubject.mockResolvedValue({ status: 'ok' })
    api.unlimitSubject.mockResolvedValue({ status: 'ok' })
    const { result } = renderHook(() => ({ limit: useLimitSubject('w1'), unlimit: useUnlimitSubject('w1') }), { wrapper })
    result.current.limit.mutate({ kind: 'role', id: 'r' })
    await waitFor(() => expect(api.limitSubject).toHaveBeenCalledWith('w1', 'role', 'r'))
    result.current.unlimit.mutate({ kind: 'department', id: 'd' })
    await waitFor(() => expect(api.unlimitSubject).toHaveBeenCalledWith('w1', 'department', 'd'))
    await waitFor(() => expect(calls()).toEqual(expect.arrayContaining([
      JSON.stringify(keys.admin.directoryScope('w1')),
      JSON.stringify(keys.admin.departments('w1')),
      JSON.stringify(keys.contacts.all('w1')),
    ])))
  })
})

describe('deleting a department', () => {
  it('also refreshes the rules: a hidden or limited department leaves its list with it, and the directory loses it', async () => {
    const spy = vi.spyOn(client, 'invalidateQueries')
    api.deleteDepartment.mockResolvedValue(undefined as never)
    const { result } = renderHook(() => useDeleteDepartment('w1'), { wrapper })
    result.current.mutate('d')
    await waitFor(() => expect(api.deleteDepartment).toHaveBeenCalledWith('w1', 'd'))
    await waitFor(() => expect(spy.mock.calls.map((c) => JSON.stringify((c[0] as { queryKey: unknown }).queryKey))).toEqual(
      expect.arrayContaining([JSON.stringify(keys.admin.directoryScope('w1')), JSON.stringify(keys.contacts.all('w1'))]),
    ))
  })
})
