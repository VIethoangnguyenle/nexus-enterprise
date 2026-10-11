import type { ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../lib/query-client'
import { useToastStore } from '../components/primitives'
import { CH, WS_ID } from '../test/chat-fixtures'
import { createSectionsServer } from '../test/chat-sections-server'
import { apiFetch } from '../api/client'
import { useChatSections } from './useChatSections'

vi.mock('../api/client', async (orig) => ({ ...(await orig<typeof import('../api/client')>()), apiFetch: vi.fn() }))

const sections = [
  { id: 's1', name: 'Quyết toán quý 4', position: 0, channel_ids: [CH.doisoat] },
  { id: 's2', name: 'Nội bộ', position: 1, channel_ids: [] },
]

let server: ReturnType<typeof createSectionsServer>

function setup() {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
  return renderHook(() => useChatSections(WS_ID), { wrapper })
}

const names = (r: { current: ReturnType<typeof useChatSections> }) => r.current.sections.map((s) => s.name)
const last = <T,>(xs: T[]) => xs[xs.length - 1]
const toasts = () => useToastStore.getState().toasts

beforeEach(() => {
  server = createSectionsServer(sections)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: Infinity } })
})
afterEach(() => {
  queryClient.clear()
  useToastStore.getState().clear()
})

describe('useChatSections', () => {
  it('lists the sections in order', async () => {
    const { result } = setup()
    await waitFor(() => expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ']))
  })

  it('renames at once and keeps the name once the server agrees', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    const release = server.holdNext('/s2')
    let done!: Promise<void>
    act(() => { done = result.current.renameSection('s2', 'Nội bộ kế toán') })
    await waitFor(() => expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ kế toán']))
    release()
    await act(() => done)
    expect(server.calls.find((c) => c.method === 'PATCH')).toMatchObject({ body: { name: 'Nội bộ kế toán' } })
    await waitFor(() => expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ kế toán']))
  })

  it('rolls a rename back and rejects when the name is taken (409)', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    server.failNext('/s2', 409)
    await act(async () => {
      await expect(result.current.renameSection('s2', 'quyết toán quý 4')).rejects.toMatchObject({ status: 409 })
    })
    expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ'])
    // The popover explains it inline, so no second message appears as a toast.
    expect(toasts()).toHaveLength(0)
  })

  it('reorders at once', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    const release = server.holdNext('/s2')
    act(() => { void result.current.moveSection('s2', 'up') })
    await waitFor(() => expect(names(result)).toEqual(['Nội bộ', 'Quyết toán quý 4']))
    release()
    await waitFor(() => expect(server.calls.find((c) => c.method === 'PATCH')).toMatchObject({ body: { move: 'up' } }))
  })

  it('places a conversation at once, says so, and sends the call', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    const release = server.holdNext('/s2/channels')
    act(() => { void result.current.placeConversation({ id: CH.vanhanh, title: 'Vận hành chung' }, 's2') })
    await waitFor(() => expect(result.current.sections[1]!.channel_ids).toEqual([CH.vanhanh]))
    release()
    await waitFor(() => expect(last(toasts())?.message).toBe('Đã chuyển Vận hành chung vào “Nội bộ”'))
    expect(server.calls.find((c) => c.method === 'PUT')?.path).toBe(`/s2/channels/${CH.vanhanh}`)
  })

  it('rolls a placement back and tells the person when the call fails', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    server.failNext('/s2/channels', 403)
    await act(async () => {
      await result.current.placeConversation({ id: CH.vanhanh, title: 'Vận hành chung' }, 's2').catch(() => {})
    })
    await waitFor(() => expect(result.current.sections[1]!.channel_ids).toEqual([]))
    expect(last(toasts())).toMatchObject({ tone: 'error' })
  })

  it('takes a conversation out of its section', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    await act(() => result.current.unplaceConversation({ id: CH.doisoat, title: 'Đối soát giao dịch' }))
    await waitFor(() => expect(result.current.sections[0]!.channel_ids).toEqual([]))
    expect(server.calls.find((c) => c.method === 'DELETE')).toMatchObject({ path: `/channels/${CH.doisoat}` })
    expect(last(toasts())?.message).toBe('Đã bỏ Đối soát giao dịch khỏi “Quyết toán quý 4”')
  })

  it('creates a section last, and can put a conversation in it in the same call', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    await act(() => result.current.createSection('Dự án Q4', { id: CH.hopdong, title: 'Hợp đồng đối tác' }))
    expect(server.calls.find((c) => c.method === 'POST')).toMatchObject({ body: { name: 'Dự án Q4', channel_ids: [CH.hopdong] } })
    await waitFor(() => expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ', 'Dự án Q4']))
    expect(last(toasts())?.message).toBe('Đã tạo mục “Dự án Q4” và chuyển Hợp đồng đối tác vào')
  })

  it('rejects a duplicate name from the server without adding anything', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    server.failNext('', 409)
    await act(async () => {
      await expect(result.current.createSection('Nội bộ')).rejects.toMatchObject({ status: 409 })
    })
    expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ'])
  })

  it('deletes at once, and Hoàn tác recreates it in place with its conversations', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    await act(() => result.current.deleteSection('s1'))
    await waitFor(() => expect(names(result)).toEqual(['Nội bộ']))

    const t = last(toasts())!
    expect(t.message).toBe('Đã xoá mục “Quyết toán quý 4”, cuộc trò chuyện trở về Tin nhắn trực tiếp và Nhóm')
    expect(t.action?.label).toBe('Hoàn tác')

    await act(async () => { t.action!.onClick() })
    await waitFor(() => expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ']))
    expect(server.calls.find((c) => c.method === 'POST')).toMatchObject({
      body: { name: 'Quyết toán quý 4', position: 0, channel_ids: [CH.doisoat] },
    })
    await waitFor(() => expect(result.current.sections[0]!.channel_ids).toEqual([CH.doisoat]))
  })

  it('brings a deleted section back and says so when Hoàn tác fails', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    await act(() => result.current.deleteSection('s1'))
    const t = last(toasts())!
    server.failNext('', 403)
    await act(async () => { t.action!.onClick() })
    await waitFor(() => expect(last(toasts())).toMatchObject({ tone: 'error' }))
    expect(names(result)).toEqual(['Nội bộ'])
  })

  it('restores a section whose delete failed', async () => {
    const { result } = setup()
    await waitFor(() => expect(result.current.sections).toHaveLength(2))
    server.failNext('/s1', 500)
    await act(async () => { await result.current.deleteSection('s1').catch(() => {}) })
    expect(names(result)).toEqual(['Quyết toán quý 4', 'Nội bộ'])
    expect(toasts().some((t) => t.tone === 'error')).toBe(true)
  })
})
