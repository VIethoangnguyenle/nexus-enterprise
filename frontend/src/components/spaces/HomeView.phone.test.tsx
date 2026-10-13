import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, U, UUID_RE } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import type { ActivityMessage, Message } from '../../api/messaging'
import { useNavCollapseStore } from '../../stores/nav-collapse.store'
import { HomeView } from './HomeView'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

function viewport(phone: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: phone && query.includes('max-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }))
}

const ago = (minutes: number) => new Date(Date.now() - minutes * 60_000).toISOString()
const msg = (over: Partial<ActivityMessage>): ActivityMessage => ({
  id: 'm', sender_id: U.yen.id, sender_name: 'yen', preview: 'xin chào', kind: 'text', created_at: ago(5), ...over,
})
const thread = (n: number, over: Partial<Message> = {}): Message => ({
  id: `1b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a0${n}`, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh',
  content: `<p>Ảnh sao kê số ${n}</p>`, content_format: 'html', reply_count: 4, thread_unread_count: n, following: true,
  last_reply_at: { seconds: Math.floor(Date.now() / 1000) - 600, nanos: 0 }, reply_participant_ids: [],
  ...over,
})

let server: ReturnType<typeof createChatStateServer>
const serve = (...args: Parameters<typeof createChatStateServer>) => {
  server = createChatStateServer(...args)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
}

beforeEach(() => {
  routerState.search = {}
  routerState.historyState = {}
  routerState.navigate.mockClear()
  routerState.back.mockClear()
  localStorage.clear()
  serve({}, 0, {
    activity: [
      { channel_id: CH.dmYen, latest: msg({ preview: 'Chị xem giúp em đề nghị tạm ứng' }) },
      {
        channel_id: CH.doisoat,
        latest: msg({ id: 'a', sender_id: U.vinh.id, preview: 'Bảng đối soát tháng 10', created_at: ago(30) }),
        latest_reply: msg({ id: 'b', sender_id: U.lan.id, preview: 'Em sửa xong rồi ạ', parent_message_id: 'root-1', created_at: ago(2) }),
      },
    ],
    threads: [thread(1), thread(2)],
    followed: [thread(1), thread(2), thread(3, { thread_unread_count: 0 })],
  })
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
  useNavCollapseStore.setState({ collapsed: {} })
})

const show = async (tab?: string, layer: Record<string, unknown> = {}) => {
  viewport(true)
  routerState.search = { ...(tab ? { tab } : {}), ...layer }
  renderWithClient(<HomeView />)
  await screen.findByRole('heading', { level: 1 })
}

describe('Chat below 1024px', () => {
  it('opens on Trang chủ: previews and the thread line from the first paint, no desktop filter chips', async () => {
    await show()
    expect(screen.getByRole('heading', { level: 1, name: 'Trang chủ' })).toBeInTheDocument()
    expect(await screen.findByText(/Chị xem giúp em đề nghị tạm ứng/)).toBeInTheDocument()
    expect(screen.getByText(/Em sửa xong rồi ạ/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /^Mở thread: Nguyễn Thu Lan/ })).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Lọc' })).toBeNull()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('draws no "Trò chuyện mới" menu of its own: the + beside the bar does that', async () => {
    await show()
    await screen.findByText(/Chị xem giúp em/)
    expect(screen.queryByRole('button', { name: 'Trò chuyện mới' })).toBeNull()
  })

  it('shows the tab named in the address', async () => {
    await show('direct')
    expect(screen.getByRole('heading', { level: 1, name: 'Trực tiếp' })).toBeInTheDocument()
    cleanup()
    await show('sections')
    expect(screen.getByRole('heading', { level: 1, name: 'Mục' })).toBeInTheDocument()
    expect(await screen.findByRole('list', { name: 'Nhóm' })).toBeInTheDocument()
    cleanup()
    await show('more')
    expect(screen.getByRole('heading', { level: 1, name: 'Thêm' })).toBeInTheDocument()
  })

  it('falls back to Trang chủ for a tab it does not know', async () => {
    await show('nonsense')
    expect(screen.getByRole('heading', { level: 1, name: 'Trang chủ' })).toBeInTheDocument()
  })

  it('opens the conversation on a thread from the Thread screen', async () => {
    const user = userEvent.setup()
    await show(undefined, { threads: 'new' })
    const dialog = await screen.findByRole('dialog', { name: 'Thread' })
    const rows = await within(dialog).findAllByRole('button', { name: /^Thread của/ })
    expect(rows).toHaveLength(2)
    await user.click(rows[0]!)
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.doisoat }, search: { thread: thread(1).id },
    })
  })

  it('asks the address for the Thread screen, as a new history entry', async () => {
    const user = userEvent.setup()
    await show()
    await user.click(await screen.findByRole('button', { name: 'Thread, 2 thread có tin mới' }))
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels', search: { threads: 'new' }, state: { chatOverlay: true },
    })
  })

  it('opens the Thread screen on Đang theo dõi from Thêm', async () => {
    const user = userEvent.setup()
    await show('more')
    await user.click(await screen.findByRole('button', { name: /Thread đang theo dõi/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels', search: { tab: 'more', threads: 'followed' }, state: { chatOverlay: true },
    })
    cleanup()
    await show('more', { threads: 'followed' })
    const dialog = await screen.findByRole('dialog', { name: 'Thread' })
    expect(within(dialog).getByRole('radio', { name: 'Đang theo dõi' })).toBeChecked()
    await waitFor(() => expect(within(dialog).getAllByRole('button', { name: /^Thread của/ })).toHaveLength(3))
  })

  it('shows no Thread screen for a value the address does not know', async () => {
    await show(undefined, { threads: 'everything' })
    expect(screen.queryByRole('dialog', { name: 'Thread' })).toBeNull()
  })

  it('closes the Thread screen with the system back when it pushed its entry', async () => {
    const user = userEvent.setup()
    routerState.historyState = { chatOverlay: true }
    await show(undefined, { threads: 'new' })
    await user.click(await screen.findByRole('button', { name: 'Đóng danh sách thread' }))
    expect(routerState.back).toHaveBeenCalledTimes(1)
    expect(routerState.navigate).not.toHaveBeenCalled()
  })

  it('closes the Thread screen by replacing the address when the page was opened on it', async () => {
    const user = userEvent.setup()
    await show('direct', { threads: 'new' })
    await user.click(await screen.findByRole('button', { name: 'Đóng danh sách thread' }))
    expect(routerState.back).not.toHaveBeenCalled()
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels', search: { tab: 'direct' }, replace: true })
  })

  describe('pull down to reload', () => {
    const scroller = () => document.querySelector<HTMLElement>('.overscroll-y-contain')!
    const touch = (y: number) => ({ touches: [{ clientX: 10, clientY: y }], changedTouches: [{ clientX: 10, clientY: y }] })
    const reads = () => server.calls.filter((c) => c.path === '/channels/unread').length

    it('asks for the lists again after a pull from the top', async () => {
      await show()
      await screen.findByText(/Chị xem giúp em/)
      const before = reads()
      fireEvent.touchStart(scroller(), touch(100))
      fireEvent.touchMove(scroller(), touch(300))
      fireEvent.touchEnd(scroller(), touch(300))
      await waitFor(() => expect(reads()).toBeGreaterThan(before))
      expect(server.calls.filter((c) => c.path === '/channels/activity').length).toBeGreaterThan(1)
    })

    it('leaves the lists alone for a short pull', async () => {
      await show()
      await screen.findByText(/Chị xem giúp em/)
      const before = reads()
      fireEvent.touchStart(scroller(), touch(100))
      fireEvent.touchMove(scroller(), touch(130))
      fireEvent.touchEnd(scroller(), touch(130))
      expect(reads()).toBe(before)
    })
  })
})

describe('Chat at desktop width', () => {
  it('keeps the filtered feed, its chips, and the threads block', async () => {
    viewport(false)
    renderWithClient(<HomeView />)
    expect(await screen.findByRole('group', { name: 'Lọc' })).toBeInTheDocument()
    expect(await screen.findByRole('list', { name: 'Cuộc trò chuyện' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { level: 1, name: 'Trực tiếp' })).toBeNull()
    expect(await screen.findByRole('heading', { name: 'Thread có tin mới' })).toBeInTheDocument()
  })

  it('quotes the server preview in the desktop rows too', async () => {
    viewport(false)
    renderWithClient(<HomeView />)
    expect(await screen.findByText(/Chị xem giúp em đề nghị tạm ứng/)).toBeInTheDocument()
  })
})
