import { fireEvent, screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, MSG, UUID_RE } from '../../test/chat-fixtures'
import { SpaceView } from './SpaceView'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => {
  const { fixtureApi } = await import('../../test/chat-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(fixtureApi) }
})

beforeEach(async () => {
  const { apiFetch } = await import('../../api/client')
  vi.mocked(apiFetch).mockClear()
  routerState.params = { channelId: CH.doisoat }
  routerState.pathname = `/channels/${CH.doisoat}`
})
afterEach(resetClient)

const noIds = () => expect(document.body.textContent).not.toMatch(UUID_RE)

describe('SpaceView on a phone', () => {
  it('goes back to Trang chủ, the conversation list, since there is no drawer', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: 'Về danh sách trò chuyện' }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels' })
  })

  it('marks its search action so the phone top bar can reach it', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    expect(await screen.findByRole('button', { name: 'Tìm trong nhóm' })).toHaveAttribute('data-module-search')
  })
})

describe('SpaceView', () => {
  it('shows the space by name with its member count, and its messages in a log', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    expect(await screen.findByRole('heading', { name: 'Đối soát giao dịch' })).toBeInTheDocument()
    expect(await screen.findByText('14 thành viên')).toBeInTheDocument()
    expect(await screen.findByText(/lệch 3 giao dịch/)).toBeInTheDocument()
    expect(screen.getByRole('log', { name: 'Tin nhắn trong Đối soát giao dịch' })).toBeInTheDocument()
    expect(screen.getByText('Enter để gửi · Shift+Enter xuống dòng')).toBeInTheDocument()
    expect(screen.queryByText(/mở một chủ đề/)).toBeNull()
    noIds()
  })

  it('renders no identifier anywhere: stream, thread, members, files, tasks', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await screen.findByText(/lệch 3 giao dịch/)
    noIds()

    // Thread panel
    await user.click(screen.getByRole('button', { name: /^Thread, 2 trả lời/ }))
    const thread = await screen.findByRole('complementary', { name: 'Thread' })
    await within(thread).findByText(/1\.284\.500/)
    noIds()

    // Thread list
    await user.click(screen.getByRole('button', { name: /^Thread, 1 thread có tin mới/ }))
    const list = await screen.findByRole('complementary', { name: 'Thread của nhóm' })
    await within(list).findByText(/Biên bản đối soát tháng 10/)
    noIds()

    // Members panel
    await user.click(screen.getByRole('button', { name: '14 thành viên' }))
    const members = await screen.findByRole('complementary', { name: 'Thành viên' })
    await within(members).findByText('Trần Minh Đức')
    expect(within(members).getByText('Lê Thị Hoa (bạn)')).toBeInTheDocument()
    expect(within(members).getByText('Trưởng phòng · Vận hành thanh toán')).toBeInTheDocument()
    noIds()

    // Files tab
    await user.click(screen.getByRole('tab', { name: 'Tệp' }))
    await screen.findByText('doi-soat-08-10.xlsx')
    expect(screen.getByText('412 KB')).toBeInTheDocument()
    noIds()

    // Tasks tab
    await user.click(screen.getByRole('tab', { name: /Công việc/ }))
    await screen.findByText('Cập nhật biên bản đối soát 08/10')
    const tasks = screen.getByRole('list', { name: 'Công việc' })
    await waitFor(() => expect(within(tasks).getByText('Lê Quang Vinh')).toBeInTheDocument())
    expect(within(tasks).getByText('Cần làm')).toBeInTheDocument()
    noIds()
  })

  it('closes the side panel with Escape', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: /^Thread, 2 trả lời/ }))
    await screen.findByRole('complementary', { name: 'Thread' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Thread' })).toBeNull())
  })
})

describe('SpaceView threads', () => {
  it('badges the Thread button with the threads that have news', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    const button = await screen.findByRole('button', { name: 'Thread, 1 thread có tin mới' })
    expect(button).toHaveTextContent('1')
  })

  it('opens a thread from the list, can go back to the list, and Esc closes the panel', async () => {
    const user = userEvent.setup()
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await user.click(await screen.findByRole('button', { name: /^Thread, 1 thread có tin mới/ }))
    const list = await screen.findByRole('complementary', { name: 'Thread của nhóm' })
    await user.click(await within(list).findByRole('button', { name: /lệch 3 giao dịch/ }))
    const thread = await screen.findByRole('complementary', { name: 'Thread' })
    await within(thread).findByText(/1\.284\.500/)
    await user.click(within(thread).getByRole('button', { name: 'Quay lại danh sách Thread' }))
    expect(await screen.findByRole('complementary', { name: 'Thread của nhóm' })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Thread của nhóm' })).toBeNull())
  })

  const readCalls = async () => {
    const { apiFetch } = await import('../../api/client')
    return vi.mocked(apiFetch).mock.calls.filter(([p, i]) => p === `/channels/${CH.doisoat}/read` && i?.method === 'POST')
  }

  it('marks the channel read once the newest message settles at the end of the stream', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await screen.findByText(/lệch 3 giao dịch/)
    await waitFor(async () => expect(await readCalls()).toHaveLength(1), { timeout: 3000 })
  })

  it('does not mark it read while the reader is away from the end', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await screen.findByText(/lệch 3 giao dịch/)
    const log = screen.getByRole('log')
    for (const [k, v] of Object.entries({ scrollHeight: 3000, clientHeight: 600, scrollTop: 100 })) {
      Object.defineProperty(log, k, { configurable: true, writable: true, value: v })
    }
    fireEvent.scroll(log)
    await new Promise((r) => setTimeout(r, 1300))
    expect(await readCalls()).toHaveLength(0)
  })
})

describe('SpaceView and push', () => {
  it('tells the server this conversation is on screen while it is open, and stops when it closes', async () => {
    const { useWebSocketStore } = await import('../../stores/websocket.store')
    const release = vi.fn()
    const viewChannel = vi.fn(() => release)
    const original = useWebSocketStore.getState().viewChannel
    useWebSocketStore.setState({ viewChannel })
    try {
      const { unmount } = renderWithClient(<SpaceView channelId={CH.doisoat} />)
      await screen.findByRole('heading', { name: 'Đối soát giao dịch' })
      expect(viewChannel).toHaveBeenCalledWith(CH.doisoat)
      unmount()
      expect(release).toHaveBeenCalled()
    } finally {
      useWebSocketStore.setState({ viewChannel: original })
    }
  })
})

describe('SpaceView focus target', () => {
  const flashed = () => [...document.querySelectorAll('[data-flash]')]

  it('opens the thread panel of the root a link names', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} initialThreadId={MSG.t1} />)
    const thread = await screen.findByRole('complementary', { name: 'Thread' })
    await within(thread).findByText(/1\.284\.500/)
  })

  it('opens no panel without a thread id', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await screen.findByText(/lệch 3 giao dịch/)
    expect(screen.queryByRole('complementary', { name: 'Thread' })).toBeNull()
  })

  it('outlines the message of the stream a link names, once it is on screen', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} focusMessageId={MSG.t2} />)
    await waitFor(() => expect(flashed()).toHaveLength(1))
    expect(flashed()[0]).toHaveAttribute('data-message-id', MSG.t2)
    expect(screen.queryByRole('complementary', { name: 'Thread' })).toBeNull()
  })

  it('outlines a reply inside the thread panel, not in the stream', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} initialThreadId={MSG.t1} focusMessageId={MSG.r2} />)
    const thread = await screen.findByRole('complementary', { name: 'Thread' })
    await waitFor(() => expect(flashed()).toHaveLength(1))
    expect(thread).toContainElement(flashed()[0] as HTMLElement)
    expect(flashed()[0]).toHaveTextContent(/không phải lỗi hệ thống/)
  })

  it('outlines nothing for a message that is not in the conversation', async () => {
    renderWithClient(<SpaceView channelId={CH.doisoat} focusMessageId="44444444-aaaa-4bbb-8ccc-0000000000ff" />)
    await screen.findByText(/lệch 3 giao dịch/)
    expect(flashed()).toHaveLength(0)
  })

  it('moves to the next target when the link changes inside the same conversation', async () => {
    const { rerender } = renderWithClient(<SpaceView channelId={CH.doisoat} />)
    await screen.findByText(/lệch 3 giao dịch/)
    rerender(<SpaceView channelId={CH.doisoat} initialThreadId={MSG.t1} />)
    expect(await screen.findByRole('complementary', { name: 'Thread' })).toBeInTheDocument()
  })
})
