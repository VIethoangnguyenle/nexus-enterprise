import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, ME, U, UUID_RE, WS_ID } from '../../test/chat-fixtures'
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

const onOpenThreads = vi.fn()
let server: ReturnType<typeof createChatStateServer>
const serve = (...args: Parameters<typeof createChatStateServer>) => {
  server = createChatStateServer(...args)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
}

function Host({ scope = 'home', news = 0 }: { scope?: FeedScope; news?: number }) {
  const nav = useChatNavigation(WS_ID)
  const overlays = useSectionOverlays(nav.sectionsApi, true)
  return (
    <>
      <PhoneHomeFeed
        scope={scope}
        conversations={scope === 'direct' ? nav.directs : nav.conversations}
        fresh={new Map()}
        overlays={overlays}
        threadsWithNews={news}
        onOpenThreads={onOpenThreads}
      />
      {overlays.node}
    </>
  )
}

beforeEach(() => {
  localStorage.clear()
  onOpenThreads.mockClear()
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

describe('Trang chủ on a phone', () => {
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

  it('has a Thread button that counts the threads with news and opens the Thread screen', async () => {
    const user = userEvent.setup()
    renderWithClient(<Host news={3} />)
    const button = await screen.findByRole('button', { name: 'Thread, 3 thread có tin mới' })
    await user.click(button)
    expect(onOpenThreads).toHaveBeenCalledTimes(1)
  })

  it('has a plain Thread button with no count when no thread has news', async () => {
    renderWithClient(<Host news={0} />)
    expect(await screen.findByRole('button', { name: 'Thread' })).toBeInTheDocument()
  })

  it('has a title, and the Thread button is only Trang chủ\'s', async () => {
    const { unmount } = renderWithClient(<Host />)
    expect(await screen.findByRole('heading', { level: 1, name: 'Trang chủ' })).toBeInTheDocument()
    unmount()
    renderWithClient(<Host scope="direct" />)
    expect(await screen.findByRole('heading', { level: 1, name: 'Trực tiếp' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Thread/ })).toBeNull()
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

describe('the unread-only pill', () => {
  const pill = () => screen.findByRole('button', { name: 'Chỉ hiện chưa đọc' })

  it('is 52 by 40 and off at first', async () => {
    renderWithClient(<Host />)
    const button = await pill()
    expect(button).toHaveAttribute('aria-pressed', 'false')
    expect(button.className).toContain('w-13')
    expect(button.className).toContain('h-10')
  })

  it('lists only the conversations with unread messages', async () => {
    const user = userEvent.setup()
    renderWithClient(<Host />)
    await waitFor(() => expect(rows()).toHaveLength(5))
    await user.click(await pill())
    expect(names().sort()).toEqual(['Phạm Hải Yến', 'Đối soát giao dịch'])
    expect(await pill()).toHaveAttribute('aria-pressed', 'true')
  })

  it('is remembered on this device for this person, and for each tab on its own', async () => {
    const user = userEvent.setup()
    const { unmount } = renderWithClient(<Host />)
    await waitFor(() => expect(rows()).toHaveLength(5))
    await user.click(await pill())
    unmount()
    expect(localStorage.getItem(`ngac-feed-unread-only:${ME.id}:home`)).toBe('1')
    renderWithClient(<Host />)
    await waitFor(() => expect(rows()).toHaveLength(2))
    resetClient()
    document.body.innerHTML = ''
    renderWithClient(<Host scope="direct" />)
    await waitFor(() => expect(rows()).toHaveLength(2))
    expect(await pill()).toHaveAttribute('aria-pressed', 'false')
  })

  it('says "Bạn đã đọc hết" when nothing is unread, and turning it off brings the list back', async () => {
    const user = userEvent.setup()
    serve({ [CH.doisoat]: { unread_count: 0 }, [CH.dmYen]: { unread_count: 0 } })
    server.state(CH.doisoat).unread_count = 0
    server.state(CH.dmYen).unread_count = 0
    localStorage.setItem(`ngac-feed-unread-only:${ME.id}:home`, '1')
    renderWithClient(<Host />)
    expect(await screen.findByText('Bạn đã đọc hết')).toBeInTheDocument()
    expect(screen.getByText('Tắt bộ lọc để xem mọi cuộc trò chuyện.')).toBeInTheDocument()
    await user.click(await pill())
    await waitFor(() => expect(rows()).toHaveLength(5))
  })

  it('still works when the device refuses to remember', async () => {
    const user = userEvent.setup()
    renderWithClient(<Host />)
    await waitFor(() => expect(rows()).toHaveLength(5))
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('full') })
    await user.click(await pill())
    expect(rows()).toHaveLength(2)
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
