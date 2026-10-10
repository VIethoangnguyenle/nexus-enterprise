import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { driveApi, type DriveListing } from '../api/drive'
import { keys } from './keys'
import { ApiError } from '../api/client'
import { useToastStore } from '../components/primitives'
import { useDeleteItemPermanently, useMoveItem, useRenameItem, useRestoreItem, useTrashItem } from './useDrive'

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)

const WS = 'ws-1'
const ITEM = 'item-1'

beforeEach(() => {
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: Infinity } })
  vi.spyOn(driveApi, 'renameItem').mockResolvedValue({} as never)
  vi.spyOn(driveApi, 'moveItem').mockResolvedValue({} as never)
  vi.spyOn(driveApi, 'trashItem').mockResolvedValue({} as never)
  vi.spyOn(driveApi, 'restoreItem').mockResolvedValue({} as never)
})
afterEach(() => {
  vi.restoreAllMocks()
  queryClient.clear()
})

/** What a change to an item must refresh besides the workspace's listings. */
function expectRefreshes(spy: { mock: { calls: unknown[][] } }) {
  const keysSeen = spy.mock.calls.map((c) => (c[0] as { queryKey: unknown }).queryKey)
  expect(keysSeen).toContainEqual(keys.drive.all(WS))
  expect(keysSeen).toContainEqual(keys.drive.sharedWithMe())
  expect(keysSeen).toContainEqual(keys.drive.item(ITEM))
}

describe('drive mutations refresh every place the item shows', () => {
  it.each([
    ['rename', () => useRenameItem(WS), { itemId: ITEM, newName: 'x' }],
    ['move', () => useMoveItem(WS), { itemId: ITEM, targetFolderId: 'f' }],
    ['restore', () => useRestoreItem(WS), ITEM],
  ] as const)('%s', async (_name, hook, vars) => {
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(hook as () => { mutateAsync: (v: unknown) => Promise<unknown> }, { wrapper })
    await act(() => result.current.mutateAsync(vars))
    expectRefreshes(spy)
  })

  it('delete', async () => {
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useTrashItem(WS), { wrapper })
    await act(() => result.current.mutateAsync(ITEM))
    expectRefreshes(spy)
  })

  it('delete takes the item out of the shared-with-me list at once, and puts it back on failure', async () => {
    const listing: DriveListing = { items: [{ id: ITEM } as never, { id: 'other' } as never] }
    queryClient.setQueryData(keys.drive.sharedWithMe(), listing)
    let fail!: (e: Error) => void
    vi.spyOn(driveApi, 'trashItem').mockImplementation(() => new Promise((_, rej) => { fail = rej }))
    const { result } = renderHook(() => useTrashItem(WS), { wrapper })
    act(() => { result.current.mutate(ITEM) })
    await waitFor(() =>
      expect(queryClient.getQueryData<DriveListing>(keys.drive.sharedWithMe())?.items?.map((i) => i.id)).toEqual(['other']),
    )
    await act(async () => { fail(new Error('boom')) })
    await waitFor(() =>
      expect(queryClient.getQueryData<DriveListing>(keys.drive.sharedWithMe())?.items?.map((i) => i.id)).toEqual([ITEM, 'other']),
    )
  })
})

describe('permanent delete of a folder that holds text documents', () => {
  it('says what to do, and leaves the view as it was', async () => {
    useToastStore.getState().clear()
    const listing: DriveListing = { items: [{ id: ITEM } as never] }
    queryClient.setQueryData(keys.drive.sharedWithMe(), listing)
    vi.spyOn(driveApi, 'deleteItem').mockRejectedValue(
      new ApiError('conflict', 409, { error: 'x', reason: 'folder_has_documents' }),
    )
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useDeleteItemPermanently(WS), { wrapper })

    await act(async () => { await result.current.mutateAsync(ITEM).catch(() => undefined) })

    const messages = useToastStore.getState().toasts.map((t) => t.message)
    expect(messages).toEqual([expect.stringContaining('trong thư mục còn văn bản')])
    expect(messages[0]).not.toContain('người khác thay đổi')
    expect(queryClient.getQueryData<DriveListing>(keys.drive.sharedWithMe())?.items?.map((i) => i.id)).toEqual([ITEM])
    expect(spy).not.toHaveBeenCalled()
  })

  it('refreshes the listings when the delete goes through', async () => {
    vi.spyOn(driveApi, 'deleteItem').mockResolvedValue({} as never)
    const spy = vi.spyOn(queryClient, 'invalidateQueries')
    const { result } = renderHook(() => useDeleteItemPermanently(WS), { wrapper })
    await act(() => result.current.mutateAsync(ITEM))
    expectRefreshes(spy)
  })
})
