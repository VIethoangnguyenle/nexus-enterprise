import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { settingsApi } from '../api/settings'
import { useQuickReaction, DEFAULT_QUICK_REACTION } from './useQuickReaction'

function run(workspaceId: string | undefined) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
  return renderHook(() => useQuickReaction(workspaceId), { wrapper })
}

afterEach(() => vi.restoreAllMocks())

describe('useQuickReaction', () => {
  it('reads the person’s double-tap reaction for the workspace', async () => {
    const get = vi.spyOn(settingsApi, 'get').mockResolvedValue({ double_tap: '❤️' })
    const { result } = run('ws-1')
    await waitFor(() => expect(result.current).toBe('❤️'))
    expect(get).toHaveBeenCalledWith('chat', 'ws-1')
  })

  it('is off when the person turned it off', async () => {
    vi.spyOn(settingsApi, 'get').mockResolvedValue({ double_tap: '' })
    const { result } = run('ws-1')
    await waitFor(() => expect(vi.mocked(settingsApi.get)).toHaveBeenCalled())
    await waitFor(() => expect(result.current).toBe(''))
  })

  it('is 👍 when the key was never set', async () => {
    vi.spyOn(settingsApi, 'get').mockResolvedValue({})
    const { result } = run('ws-1')
    await waitFor(() => expect(result.current).toBe(DEFAULT_QUICK_REACTION))
  })

  it('falls back to 👍 when the settings cannot be read', async () => {
    vi.spyOn(settingsApi, 'get').mockRejectedValue(new Error('down'))
    const { result } = run('ws-1')
    await waitFor(() => expect(result.current).toBe('👍'))
  })

  it('gives no reaction while the answer is on its way, so an "off" is never overruled', () => {
    vi.spyOn(settingsApi, 'get').mockReturnValue(new Promise(() => {}))
    const { result } = run('ws-1')
    expect(result.current).toBe('')
  })

  it('is 👍 without a workspace, and asks nothing', () => {
    const get = vi.spyOn(settingsApi, 'get')
    const { result } = run(undefined)
    expect(result.current).toBe('👍')
    expect(get).not.toHaveBeenCalled()
  })
})
