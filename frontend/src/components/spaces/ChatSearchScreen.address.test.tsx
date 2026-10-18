import { act, cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { renderShell, viewport, answer, resetServer } from '../patterns/phone-shell-harness'
import { CH, U, WS_ID } from '../../test/chat-fixtures'
import { apiFetch } from '../../api/client'
import type { Message } from '../../api/messaging'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useChatOverlay } from '../../hooks/useChatOverlay'
import { usePeople } from '../../hooks/usePeople'
import { Pressable } from '../primitives'
import { ChatSearchScreen } from './ChatSearchScreen'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const hit: Message = {
  id: '5b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a01', channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh',
  content: 'Bảng đối soát tháng 10', content_format: 'plain', created_at: { seconds: Math.floor(Date.now() / 1000) - 3600, nanos: 0 },
}
let searches: URLSearchParams[]
const server = (path: string, init?: RequestInit): Promise<unknown> => {
  if (path.startsWith('/messages/search')) {
    searches.push(new URLSearchParams(path.split('?')[1]))
    return Promise.resolve({ messages: [hit], has_more: false })
  }
  return answer(path, init)
}

/** The Chat list's own wiring of the layer: the real overlay hook and the real address. */
function Host() {
  const { overlay, openSearch, close } = useChatOverlay()
  const nav = useChatNavigation(WS_ID)
  const people = usePeople(WS_ID)
  return (
    <>
      <Pressable onClick={openSearch}>Tìm trong chat</Pressable>
      {overlay?.kind === 'search' && !nav.isLoading && !people.isLoading && (
        <ChatSearchScreen conversations={nav.conversations} people={people} onClose={close} />
      )}
    </>
  )
}

const location = (router: Awaited<ReturnType<typeof renderShell>>) => router.state.location.search as Record<string, unknown>
const box = () => screen.getByRole('searchbox', { name: 'Tìm trong chat' })

beforeEach(() => {
  searches = []
  localStorage.clear()
  resetServer()
  viewport(true)
  vi.mocked(apiFetch).mockImplementation(server as typeof apiFetch)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('the words and filters of the phone search are in the address', () => {
  it('writes the words on Enter and each filter on its chip, all replacing the entry the screen opened on', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Host />, '/channels')
    await user.click(screen.getByRole('button', { name: 'Tìm trong chat' }))
    await screen.findByRole('dialog', { name: 'Tìm trong chat' })
    await user.type(box(), 'đối soát{Enter}')
    await waitFor(() => expect(location(router)).toEqual({ search: 1, q: 'đối soát' }))
    await screen.findByRole('region', { name: 'Kết quả tìm kiếm' })
    await user.click(screen.getByRole('button', { name: 'Có tệp' }))
    await waitFor(() => expect(location(router)).toEqual({ search: 1, q: 'đối soát', file: 1 }))
    await waitFor(() => expect(searches.some((s) => s.get('has_file') === 'true')).toBe(true))
    await user.click(screen.getByRole('button', { name: 'Người gửi' }))
    await user.click(await screen.findByRole('radio', { name: /Lê Quang Vinh/ }))
    await waitFor(() => expect(location(router)).toEqual({ search: 1, q: 'đối soát', file: 1, from: U.vinh.id }))
  })

  it('goes back past all of it in one step, and forward brings the results and their filters back', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Host />, '/channels')
    await user.click(screen.getByRole('button', { name: 'Tìm trong chat' }))
    await screen.findByRole('dialog', { name: 'Tìm trong chat' })
    await user.type(box(), 'đối soát{Enter}')
    await screen.findByRole('region', { name: 'Kết quả tìm kiếm' })
    await user.click(screen.getByRole('button', { name: 'Có liên kết' }))
    await waitFor(() => expect(location(router)).toMatchObject({ link: 1 }))

    await act(async () => router.history.back())
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tìm trong chat' })).toBeNull())
    expect(location(router)).toEqual({})

    await act(async () => router.history.forward())
    await screen.findByRole('dialog', { name: 'Tìm trong chat' })
    expect(await screen.findByRole('button', { name: 'Có liên kết' })).toHaveAttribute('aria-pressed', 'true')
    expect(box()).toHaveValue('đối soát')
  })

  it('keeps them across a reload: the address alone brings the results and the chips back', async () => {
    await renderShell(<Host />, `/channels?search=1&q=${encodeURIComponent('đối soát')}&from=${U.vinh.id}&in=${CH.doisoat}&file=1&me=1`)
    await screen.findByRole('region', { name: 'Kết quả tìm kiếm' })
    expect(box()).toHaveValue('đối soát')
    expect(await screen.findByRole('button', { name: 'Người gửi: Lê Quang Vinh' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Trong: Đối soát giao dịch' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Có tệp' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Nhắc đến tôi' })).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(searches).toHaveLength(1))
    expect(searches[0]!.get('from')).toBe(U.vinh.id)
    expect(searches[0]!.get('channel_id')).toBe(CH.doisoat)
    expect(searches[0]!.get('mentions_me')).toBe('true')
  })

  it('takes the words and filters along when the screen closes', async () => {
    const user = userEvent.setup()
    const router = await renderShell(<Host />, `/channels?search=1&q=${encodeURIComponent('đối soát')}&file=1&tab=direct`)
    await screen.findByRole('region', { name: 'Kết quả tìm kiếm' })
    await user.click(screen.getByRole('button', { name: 'Đóng tìm kiếm' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Tìm trong chat' })).toBeNull())
    expect(location(router)).toEqual({ tab: 'direct' })
  })
})
