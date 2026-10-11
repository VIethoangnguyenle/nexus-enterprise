import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { driveApi } from '../api/drive'
import { ApiError } from '../api/client'
import { useFilePreview } from './useFilePreview'

function setup() {
  // Production retry/staleTime come from the hook itself; only the backoff is shortened.
  const client = new QueryClient({ defaultOptions: { queries: { retryDelay: 1 } } })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
  return { client, wrapper }
}

afterEach(() => vi.restoreAllMocks())

describe('useFilePreview', () => {
  it('asks the drive for the file preview and caches it under the preview key', async () => {
    const answer = { kind: 'native', url: 'https://s/x', mime: 'image/png' } as const
    const spy = vi.spyOn(driveApi, 'preview').mockResolvedValue(answer)
    const { client, wrapper } = setup()
    const { result } = renderHook(() => useFilePreview('f1'), { wrapper })
    await waitFor(() => expect(result.current.data).toEqual(answer))
    expect(spy).toHaveBeenCalledWith('f1')
    expect(client.getQueryData(['drive', 'preview', 'f1'])).toEqual(answer)
  })

  it('does nothing without a file id', () => {
    const spy = vi.spyOn(driveApi, 'preview')
    const { wrapper } = setup()
    renderHook(() => useFilePreview(undefined), { wrapper })
    expect(spy).not.toHaveBeenCalled()
  })

  it.each([403, 404])('does not retry a %i', async (status) => {
    const spy = vi.spyOn(driveApi, 'preview').mockRejectedValue(new ApiError('no', status))
    const { wrapper } = setup()
    const { result } = renderHook(() => useFilePreview('f1'), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('retries a network failure once', async () => {
    const spy = vi.spyOn(driveApi, 'preview').mockRejectedValue(new Error('network'))
    const { wrapper } = setup()
    const { result } = renderHook(() => useFilePreview('f1'), { wrapper })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(spy).toHaveBeenCalledTimes(2)
  })
})
