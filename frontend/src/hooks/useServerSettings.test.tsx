import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../api/client'
import { useServerSettings } from './useServerSettings'
import { keys } from './keys'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

function harness() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>
  return { client, wrapper }
}

const BAR = ['chat', 'calendar', 'drive', 'contacts']

beforeEach(() => api.mockReset())

describe('useServerSettings(workspace)', () => {
  it('reads the settings of the workspace from its own endpoint', async () => {
    api.mockResolvedValueOnce({ settings: { 'mobile.bar': BAR } })
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('workspace', 'w1'), { wrapper })
    expect(result.current.isLoading).toBe(true)
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(api).toHaveBeenCalledWith('/workspaces/w1/settings')
    expect(result.current.settings['mobile.bar']).toEqual(BAR)
  })

  it('treats a key the server leaves out as unset', async () => {
    api.mockResolvedValueOnce({ settings: {} })
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('workspace', 'w1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.settings['mobile.bar']).toBeUndefined()
  })

  it('answers {} instead of failing when the read fails, so callers use their defaults', async () => {
    api.mockRejectedValueOnce(new Error('offline'))
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('workspace', 'w1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.settings).toEqual({})
  })

  it('asks nothing without a workspace', () => {
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('workspace', ''), { wrapper })
    expect(result.current.isLoading).toBe(false)
    expect(api).not.toHaveBeenCalled()
  })

  it('keeps one cache entry per workspace', async () => {
    api.mockResolvedValueOnce({ settings: { 'mobile.bar': BAR } }).mockResolvedValueOnce({ settings: {} })
    const { wrapper } = harness()
    const a = renderHook(() => useServerSettings('workspace', 'w1'), { wrapper })
    const b = renderHook(() => useServerSettings('workspace', 'w2'), { wrapper })
    await waitFor(() => expect(a.result.current.isLoading || b.result.current.isLoading).toBe(false))
    expect(a.result.current.settings['mobile.bar']).toEqual(BAR)
    expect(b.result.current.settings['mobile.bar']).toBeUndefined()
  })

  it('saves with PUT, shows the new value at once and keeps what the server answers', async () => {
    const next = ['calendar', 'chat', 'drive', 'assets']
    api.mockResolvedValueOnce({ settings: { 'mobile.bar': BAR } })
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('workspace', 'w1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    let release!: (v: unknown) => void
    api.mockReturnValueOnce(new Promise((r) => { release = r }))
    let saved: Promise<boolean>
    act(() => { saved = result.current.save({ 'mobile.bar': next }) })
    await waitFor(() => expect(result.current.settings['mobile.bar']).toEqual(next))
    expect(api).toHaveBeenLastCalledWith('/workspaces/w1/settings', { method: 'PUT', body: JSON.stringify({ settings: { 'mobile.bar': next } }) })

    await act(async () => { release({ settings: { 'mobile.bar': next } }); expect(await saved).toBe(true) })
    expect(result.current.settings['mobile.bar']).toEqual(next)
  })

  it('puts the old value back and answers false when the server refuses', async () => {
    api.mockResolvedValueOnce({ settings: { 'mobile.bar': BAR } })
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('workspace', 'w1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    api.mockRejectedValueOnce(new Error('400'))
    let ok = true
    await act(async () => { ok = await result.current.save({ 'mobile.bar': ['chat', 'chat', 'drive', 'assets'] }) })
    expect(ok).toBe(false)
    expect(result.current.settings['mobile.bar']).toEqual(BAR)
  })
})

describe('every service merges a PUT', () => {
  const DRIVE = { view: 'auto', sort: 'modified', folders_first: true, cellular_previews: true, notify_shared: true } as const

  it('sends only the change, so a stale copy of the other keys never overwrites them', async () => {
    api.mockResolvedValueOnce({ ...DRIVE, sort: 'name' })
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('drive', 'w1', DRIVE), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(api).toHaveBeenCalledWith('/drive/settings?workspace_id=w1')

    api.mockImplementationOnce((_p, init) => Promise.resolve(JSON.parse(String(init?.body))))
    await act(async () => { expect(await result.current.save({ notify_shared: false })).toBe(true) })
    expect(api).toHaveBeenLastCalledWith('/drive/settings?workspace_id=w1', {
      method: 'PUT', body: JSON.stringify({ notify_shared: false }),
    })
    await waitFor(() => expect(result.current.settings.notify_shared).toBe(false))
    expect(result.current.settings.sort).toBe('name')
    expect(api.mock.calls.map((c) => c[1]?.method ?? 'GET')).toEqual(['GET', 'PUT'])
  })

  it('shows the defaults for what the read never answered, and sends only the change', async () => {
    api.mockRejectedValueOnce(new Error('offline'))
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('drive', 'w1', DRIVE), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.settings).toEqual(DRIVE)

    api.mockRejectedValueOnce(new Error('500'))
    await act(async () => { expect(await result.current.save({ view: 'list' })).toBe(false) })
    expect(api).toHaveBeenLastCalledWith('/drive/settings?workspace_id=w1', { method: 'PUT', body: JSON.stringify({ view: 'list' }) })
    expect(result.current.settings.view).toBe('auto')
  })

  it('reads Lịch under the key its own dialog uses, so both show one value', async () => {
    api.mockResolvedValueOnce({ tzid: 'Asia/Ho_Chi_Minh', workStart: '08:00', workEnd: '17:30', defaultReminder: 10, notifyInvites: false })
    const { client, wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('calendar', 'w1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(api).toHaveBeenCalledWith('/workspaces/w1/calendar/settings')
    expect(client.getQueryData(keys.calendar.settings('w1'))).toMatchObject({ notifyInvites: false })
  })

  it('sends Chat only the change', async () => {
    api.mockResolvedValueOnce({ enter_to_send: false, link_preview: true, open_at: 'all', sort: 'activity', double_tap: '👍', default_level: null })
    const { wrapper } = harness()
    const { result } = renderHook(() => useServerSettings('chat', 'w1'), { wrapper })
    await waitFor(() => expect(result.current.isLoading).toBe(false))
    api.mockResolvedValueOnce({ enter_to_send: true })
    await act(async () => { await result.current.save({ enter_to_send: true }) })
    expect(api).toHaveBeenLastCalledWith('/workspaces/w1/chat-settings', { method: 'PUT', body: JSON.stringify({ enter_to_send: true }) })
  })
})
