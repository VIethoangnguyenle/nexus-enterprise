import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { swipe } from '../../test/gesture-events'
import { CH, U, WS_ID } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import type { ActivityMessage, ConversationActivity } from '../../api/messaging'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useWebSocketStore } from '../../stores/websocket.store'
import { useToastStore } from '../primitives/Toast'
import { PhoneHomeFeed } from './PhoneHomeFeed'
import { useSectionOverlays } from './SectionOverlays'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString()
const entry = (channel_id: string, over: Partial<ActivityMessage>): ConversationActivity => ({
  channel_id,
  latest: { id: 'm', sender_id: U.yen.id, sender_name: 'yen', preview: 'xin chào', kind: 'text', created_at: ago(5), ...over },
})

let server: ReturnType<typeof createChatStateServer>
let phone = true

function Host() {
  const nav = useChatNavigation(WS_ID)
  const overlays = useSectionOverlays(nav.sectionsApi, true)
  if (nav.isLoading) return null
  return (
    <>
      <PhoneHomeFeed scope="all" conversations={nav.conversations} fresh={new Map()} overlays={overlays} />
      {overlays.node}
    </>
  )
}

beforeEach(() => {
  localStorage.clear()
  phone = true
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true, writable: true })
  server = createChatStateServer(
    { [CH.dmYen]: { unread_count: 2 } },
    0,
    { activity: [entry(CH.dmYen, { id: 'm-yen', created_at: ago(2) }), entry(CH.hopdong, { id: 'm-hd', created_at: ago(30) })] },
  )
  vi.mocked(apiFetch).mockClear()
  vi.mocked(apiFetch).mockImplementation(server.api as never)
  useToastStore.getState().clear()
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  cleanup()
  resetClient()
  useWebSocketStore.setState({ lastMessages: {}, onlineUsers: {} })
})

/** The row of a conversation: the element the finger drags is its link's wrapper. */
async function rowOf(name: string) {
  const link = await screen.findByRole('link', { name: new RegExp(`^${name}`) })
  return link.closest('[data-swipe-row]')!.lastElementChild!
}
const swipeLeft = (el: Element) => swipe(el, [{ x: 300, t: 0 }, { x: 288, t: 16 }, { x: 150, t: 400 }])
const fullSwipeRight = (el: Element) => swipe(el, [{ x: 40, t: 0 }, { x: 52, t: 16 }, { x: 340, t: 600 }])
const posted = (path: string, method = 'POST') =>
  vi.mocked(apiFetch).mock.calls.filter(([p, init]) => String(p).startsWith(path) && (init?.method ?? 'GET') === method)
const toasts = () => useToastStore.getState().toasts

describe('Chat rows on a phone: swipes', () => {
  it('a swipe left reveals Tắt thông báo · Gắn sao · Thêm, and Gắn sao stars with Hoàn tác', async () => {
    renderWithClient(<Host />)
    const row = await rowOf('Hợp đồng đối tác')
    swipeLeft(row)
    const buttons = within(row.parentElement!)
    expect(buttons.getByRole('button', { name: 'Tắt thông báo' })).toBeInTheDocument()
    expect(buttons.getByRole('button', { name: 'Thêm' })).toBeInTheDocument()
    fireEvent.click(buttons.getByRole('button', { name: 'Gắn sao' }))
    await waitFor(() => expect(posted(`/channels/${CH.hopdong}/star`, 'PUT')).toHaveLength(1))
    await waitFor(() => expect(toasts().some((t) => t.action?.label === 'Hoàn tác')).toBe(true))
  })

  it('Tắt thông báo turns the conversation off', async () => {
    renderWithClient(<Host />)
    const row = await rowOf('Hợp đồng đối tác')
    swipeLeft(row)
    fireEvent.click(within(row.parentElement!).getByRole('button', { name: 'Tắt thông báo' }))
    await waitFor(() => expect(server.state(CH.hopdong).notification_level).toBe('off'))
  })

  it('Thêm opens the row’s sheet, the long-press one', async () => {
    renderWithClient(<Host />)
    const row = await rowOf('Hợp đồng đối tác')
    swipeLeft(row)
    fireEvent.click(within(row.parentElement!).getByRole('button', { name: 'Thêm' }))
    expect(await screen.findByRole('dialog', { name: 'Tuỳ chọn cho Hợp đồng đối tác' })).toBeInTheDocument()
  })

  it('a full swipe right marks an unread conversation read, and Hoàn tác puts it back to unread', async () => {
    renderWithClient(<Host />)
    const row = await rowOf('Phạm Hải Yến')
    fullSwipeRight(row)
    await waitFor(() => expect(posted(`/channels/${CH.dmYen}/read`)).toHaveLength(1))
    expect(JSON.parse(String(posted(`/channels/${CH.dmYen}/read`)[0]![1]!.body))).toEqual({ last_message_id: 'm-yen' })
    const undo = await waitFor(() => {
      const t = toasts().find((x) => x.action?.label === 'Hoàn tác')
      expect(t).toBeDefined()
      return t!
    })
    expect(undo.message).toBe('Đã đánh dấu đã đọc “Phạm Hải Yến”')
    await act(async () => { undo.action!.onClick() })
    await waitFor(() => expect(posted('/messages/m-yen/unread')).toHaveLength(1))
  })

  it('a full swipe right on a read conversation marks it unread', async () => {
    renderWithClient(<Host />)
    const row = await rowOf('Hợp đồng đối tác')
    fullSwipeRight(row)
    await waitFor(() => expect(posted('/messages/m-hd/unread')).toHaveLength(1))
    expect(posted(`/channels/${CH.hopdong}/read`)).toHaveLength(0)
  })

  it('keeps the hidden buttons out of reach until a swipe reveals them', async () => {
    renderWithClient(<Host />)
    await rowOf('Hợp đồng đối tác')
    expect(screen.queryByRole('button', { name: 'Gắn sao' })).toBeNull()
  })

  it('on a desktop the rows do not swipe', async () => {
    phone = false
    renderWithClient(<Host />)
    const link = await screen.findByRole('link', { name: /^Hợp đồng đối tác/ })
    expect(link.closest('[data-swipe-row]')).toBeNull()
  })
})

