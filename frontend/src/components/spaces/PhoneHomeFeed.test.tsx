import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, U, UUID_RE, WS_ID } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import type { ActivityMessage, ConversationActivity } from '../../api/messaging'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { useWebSocketStore } from '../../stores/websocket.store'
import { PhoneHomeFeed, type FeedScope } from './PhoneHomeFeed'
import { useSectionOverlays } from './SectionOverlays'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString()
const msg = (over: Partial<ActivityMessage>): ActivityMessage => ({
  id: 'm', sender_id: U.yen.id, sender_name: 'yen', preview: 'xin chào', kind: 'text', created_at: ago(5), ...over,
})
const entry = (channel_id: string, latest: Partial<ActivityMessage>, reply?: Partial<ActivityMessage>): ConversationActivity => ({
  channel_id, latest: msg(latest), ...(reply ? { latest_reply: msg({ id: 'r', parent_message_id: 'root-1', ...reply }) } : {}),
})

let server: ReturnType<typeof createChatStateServer>
const serve = (...args: Parameters<typeof createChatStateServer>) => {
  server = createChatStateServer(...args)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
}

function Host({ scope = 'all' }: { scope?: FeedScope }) {
  const nav = useChatNavigation(WS_ID)
  const overlays = useSectionOverlays(nav.sectionsApi, true)
  // As in Chat itself: the feed is mounted once the lists are in (a skeleton stands in until then).
  if (nav.isLoading) return null
  return (
    <>
      <PhoneHomeFeed scope={scope} conversations={nav.conversations} fresh={new Map()} overlays={overlays} />
      {overlays.node}
    </>
  )
}

beforeEach(() => {
  localStorage.clear()
  serve()
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  cleanup()
  resetClient()
  useWebSocketStore.setState({ lastMessages: {}, onlineUsers: {} })
})

const rows = () => within(screen.getByRole('list', { name: 'Cuộc trò chuyện' })).getAllByRole('listitem')
const names = () => rows().map((li) => within(li).getAllByRole('link')[0]!.getAttribute('aria-label'))

describe('Tất cả on a phone', () => {
  it('draws every row at full size the moment it mounts, so coming back to it by the device back is not a flash', async () => {
    renderWithClient(<Host />)
    await waitFor(() => expect(rows().length).toBeGreaterThan(0))
    // A row that grows in from nothing (opacity 0, height 0) is a blank list for the first frames after the back.
    for (const li of rows()) {
      expect(li.style.opacity).not.toBe('0')
      expect(li.style.height).not.toBe('0px')
    }
  })

  it('shows the previews the moment it opens, with no live event at all', async () => {
    serve({}, 0, {
      activity: [
        entry(CH.dmYen, { preview: 'Chị xem giúp em đề nghị tạm ứng', created_at: ago(5) }),
        entry(CH.hopdong, { sender_id: U.vinh.id, preview: 'Bảng đối soát tháng 10', created_at: ago(40) }),
      ],
    })
    renderWithClient(<Host />)
    expect(await screen.findByText(/Chị xem giúp em đề nghị tạm ứng/)).toHaveTextContent('Phạm Hải Yến: Chị xem giúp em đề nghị tạm ứng')
    expect(screen.getByText(/Bảng đối soát tháng 10/)).toHaveTextContent('Lê Quang Vinh: Bảng đối soát tháng 10')
    expect(screen.getByText('5 phút')).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('orders by the newest activity, a thread reply included, with its own line', async () => {
    serve({}, 0, {
      activity: [
        entry(CH.dmYen, { created_at: ago(10) }),
        entry(CH.hopdong, { preview: 'Bảng đối soát', created_at: ago(60) }, { sender_id: U.lan.id, preview: 'Em gửi lại ạ', created_at: ago(2) }),
      ],
    })
    renderWithClient(<Host />)
    await screen.findByText(/Em gửi lại ạ/)
    expect(names().slice(0, 2)).toEqual(['Hợp đồng đối tác', 'Phạm Hải Yến'])
    const thread = screen.getByRole('link', { name: 'Mở thread: Nguyễn Thu Lan: Em gửi lại ạ' })
    expect(JSON.parse(thread.getAttribute('data-search')!)).toEqual({ thread: 'root-1' })
  })

  it('moves a conversation to the top when a live message arrives, newer than the server preview', async () => {
    serve({}, 0, { activity: [entry(CH.dmYen, { created_at: ago(10) }), entry(CH.hopdong, { preview: 'cũ', created_at: ago(60) })] })
    renderWithClient(<Host />)
    await screen.findByText(/cũ$/)
    expect(names()[0]).toBe('Phạm Hải Yến')
    act(() => {
      useWebSocketStore.setState({
        lastMessages: {
          [CH.hopdong]: { id: 'live', content: 'Tin vừa đến', timestamp: new Date().toISOString(), senderName: 'lan', senderId: U.lan.id },
        },
      })
    })
    await screen.findByText(/Tin vừa đến/)
    expect(names()[0]).toBe('Hợp đồng đối tác')
    expect(screen.queryByText(/cũ$/)).toBeNull()
  })

  it('says it is empty when there is no conversation at all', async () => {
    serve()
    vi.mocked(apiFetch).mockImplementation((async (path: string, init?: RequestInit) => {
      if (path.includes('/channels') && !path.includes('unread') && !path.includes('activity')) return { channels: [] }
      if (path === '/dms') return { channels: [] }
      return server.api(path, init)
    }) as never)
    renderWithClient(<Host />)
    expect(await screen.findByText(/Chưa có cuộc trò chuyện nào/)).toBeInTheDocument()
  })
})

describe('Trực tiếp on a phone', () => {
  it('lists the direct messages only', async () => {
    renderWithClient(<Host scope="direct" />)
    await waitFor(() => expect(rows()).toHaveLength(2))
    expect(names().sort()).toEqual(['Nguyễn Thu Lan', 'Phạm Hải Yến'])
  })
})

describe('Nhóm on a phone', () => {
  it('lists the groups only', async () => {
    renderWithClient(<Host scope="spaces" />)
    await waitFor(() => expect(rows()).toHaveLength(3))
    expect(names()).not.toContain('Phạm Hải Yến')
    expect(names()).not.toContain('Nguyễn Thu Lan')
  })
})

describe('Chưa đọc on a phone', () => {
  it('lists only the conversations with unread messages', async () => {
    renderWithClient(<Host scope="unread" />)
    await waitFor(() => expect(rows()).toHaveLength(2))
    expect(names().sort()).toEqual(['Phạm Hải Yến', 'Đối soát giao dịch'])
  })

  it('says "Bạn đã đọc hết" when nothing is unread', async () => {
    serve({ [CH.doisoat]: { unread_count: 0 }, [CH.dmYen]: { unread_count: 0 } })
    server.state(CH.doisoat).unread_count = 0
    server.state(CH.dmYen).unread_count = 0
    renderWithClient(<Host scope="unread" />)
    expect(await screen.findByText('Bạn đã đọc hết')).toBeInTheDocument()
    expect(screen.getByText('Chọn Tất cả để xem mọi cuộc trò chuyện.')).toBeInTheDocument()
  })
})

describe('long press on a row', () => {
  it('opens the sheet of the conversation, over a row that is a link', async () => {
    renderWithClient(<Host />)
    await waitFor(() => expect(rows()).toHaveLength(5))
    vi.useFakeTimers({ shouldAdvanceTime: true })
    fireEvent.pointerDown(rows()[0]!, { pointerType: 'touch', clientX: 5, clientY: 5 })
    act(() => { vi.advanceTimersByTime(401) })
    expect(await screen.findByRole('dialog', { name: /^Tuỳ chọn cho / })).toBeInTheDocument()
  })
})
