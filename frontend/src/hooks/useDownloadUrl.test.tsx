import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { driveApi } from '../api/drive'
import { useToastStore } from '../components/primitives'
import { ApiError } from '../api/client'
import { keys } from './keys'
import { useDownloadFile, useDownloadUrl } from './useDownloadUrl'

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)

const FILE = '66666666-aaaa-4bbb-8ccc-000000000002'

let clicked: { href: string; download: string }[]

beforeEach(() => {
  // No retries: a failure should show at once, not after a backoff.
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: 30_000 } })
  clicked = []
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
    clicked.push({ href: this.href, download: this.download })
  })
})

afterEach(() => {
  vi.restoreAllMocks()
  queryClient.clear()
  useToastStore.getState().clear()
})

describe('useDownloadUrl', () => {
  it('fetches the presigned URL once and caches it under the drive key', async () => {
    const spy = vi.spyOn(driveApi, 'getDownloadUrl').mockResolvedValue({ download_url: 'https://files.test/a.png?sig=1' })

    const first = renderHook(() => useDownloadUrl(FILE), { wrapper })
    await waitFor(() => expect(first.result.current.data).toBe('https://files.test/a.png?sig=1'))
    const second = renderHook(() => useDownloadUrl(FILE), { wrapper })

    expect(second.result.current.data).toBe('https://files.test/a.png?sig=1')
    expect(spy).toHaveBeenCalledTimes(1)
    expect(queryClient.getQueryData(keys.drive.downloadUrl(FILE))).toBe('https://files.test/a.png?sig=1')
  })

  it('does not ask for a URL without a file, or when told not to', () => {
    const spy = vi.spyOn(driveApi, 'getDownloadUrl')
    renderHook(() => useDownloadUrl(undefined), { wrapper })
    renderHook(() => useDownloadUrl(FILE, false), { wrapper })
    expect(spy).not.toHaveBeenCalled()
  })

  it('surfaces a failed fetch as an error state, for the caller to render', async () => {
    vi.spyOn(driveApi, 'getDownloadUrl').mockRejectedValue(new ApiError('gone', 404))
    const { result } = renderHook(() => useDownloadUrl(FILE), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
  })
})

describe('useDownloadFile', () => {
  it('fetches a fresh URL on every click and downloads under the file name', async () => {
    const spy = vi.spyOn(driveApi, 'getDownloadUrl')
      .mockResolvedValueOnce({ download_url: 'https://files.test/x?sig=1' })
      .mockResolvedValueOnce({ download_url: 'https://files.test/x?sig=2' })
    const { result } = renderHook(() => useDownloadFile(), { wrapper })

    await act(() => result.current.download({ id: FILE, name: 'doi-soat-08-10.xlsx' }))
    await act(() => result.current.download({ id: FILE, name: 'doi-soat-08-10.xlsx' }))

    expect(spy).toHaveBeenCalledTimes(2)
    expect(clicked).toEqual([
      { href: 'https://files.test/x?sig=1', download: 'doi-soat-08-10.xlsx' },
      { href: 'https://files.test/x?sig=2', download: 'doi-soat-08-10.xlsx' },
    ])
  })

  it('reports whether a download is running', async () => {
    let release: (v: { download_url: string }) => void = () => {}
    vi.spyOn(driveApi, 'getDownloadUrl').mockReturnValue(new Promise((r) => { release = r }))
    const { result } = renderHook(() => useDownloadFile(), { wrapper })

    let done: Promise<void> | undefined
    act(() => { done = result.current.download({ id: FILE, name: 'a.pdf' }) })
    await waitFor(() => expect(result.current.isDownloading).toBe(true))
    await act(async () => { release({ download_url: 'https://files.test/a' }); await done })
    await waitFor(() => expect(result.current.isDownloading).toBe(false))
  })

  it('tells the user what failed and does not download', async () => {
    vi.spyOn(driveApi, 'getDownloadUrl').mockRejectedValue(new ApiError('denied', 403))
    const { result } = renderHook(() => useDownloadFile(), { wrapper })

    await act(() => result.current.download({ id: FILE, name: 'bao-cao.pdf' }))

    expect(clicked).toEqual([])
    const [t] = useToastStore.getState().toasts
    expect(t?.tone).toBe('error')
    expect(t?.message).toMatch(/tải xuống/)
  })
})
