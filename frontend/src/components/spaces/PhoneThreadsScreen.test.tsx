import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, CONTACTS, U, UUID_RE } from '../../test/chat-fixtures'
import { createChatStateServer } from '../../test/chat-state-server'
import { apiFetch } from '../../api/client'
import type { Message } from '../../api/messaging'
import { buildDirectory } from '../../lib/people'
import { PhoneThreadsScreen, type ThreadsChip } from './PhoneThreadsScreen'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const people = buildDirectory(CONTACTS)
const conversations = [
  { id: CH.doisoat, kind: 'space' as const, title: 'Đối soát giao dịch', hueKey: CH.doisoat },
  { id: CH.vanhanh, kind: 'space' as const, title: 'Vận hành chung', hueKey: CH.vanhanh },
]
const root = (n: number, over: Partial<Message> = {}): Message => ({
  id: `1b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a0${n}`, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh',
  content: `<p>Ảnh sao kê số ${n}</p>`, content_format: 'html', reply_count: 4, thread_unread_count: 0, following: true,
  last_reply_at: { seconds: Math.floor(Date.now() / 1000) - 600, nanos: 0 }, reply_participant_ids: [U.lan.id],
  created_at: { seconds: 1760000000 + n, nanos: 0 },
  ...over,
})

const onOpen = vi.fn()
const onClose = vi.fn()
let server: ReturnType<typeof createChatStateServer>
const show = (initial: ThreadsChip, feeds: Parameters<typeof createChatStateServer>[2] = {}) => {
  server = createChatStateServer({}, 0, feeds)
  vi.mocked(apiFetch).mockImplementation(server.api as never)
  return renderWithClient(
    <PhoneThreadsScreen initial={initial} conversations={conversations} people={people} onOpen={onOpen} onClose={onClose} />,
  )
}

beforeEach(() => {
  onOpen.mockClear()
  onClose.mockClear()
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

const chips = () => within(screen.getByRole('radiogroup', { name: 'Lọc thread' }))

describe('the Thread screen', () => {
  it('is a full-screen dialog with the chips Có tin mới and Đang theo dõi', async () => {
    show('unread', { threads: [root(1, { thread_unread_count: 2 })] })
    const dialog = await screen.findByRole('dialog', { name: 'Thread' })
    expect(within(dialog).getByRole('heading', { name: 'Thread' })).toBeInTheDocument()
    expect(chips().getByRole('radio', { name: /^Có tin mới/ })).toBeChecked()
    expect(chips().getByRole('radio', { name: 'Đang theo dõi' })).not.toBeChecked()
  })

  it('opens on Có tin mới with the followed threads that have unread replies', async () => {
    show('unread', { threads: [root(1, { thread_unread_count: 2 }), root(2, { thread_unread_count: 1, channel_id: CH.vanhanh })] })
    const row = await screen.findByRole('button', { name: /^Thread của Lê Quang Vinh trong Đối soát giao dịch/ })
    expect(row).toHaveAccessibleName('Thread của Lê Quang Vinh trong Đối soát giao dịch, 4 trả lời, 2 trả lời mới')
    expect(within(row).getByText('Ảnh sao kê số 1', { exact: false })).toBeInTheDocument()
    expect(within(row).getByText('2 mới')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /trong Vận hành chung/ })).toBeInTheDocument()
    expect(chips().getByRole('radio', { name: 'Có tin mới 2 thread' })).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('switches to every followed thread, read ones included', async () => {
    const user = userEvent.setup()
    show('unread', { threads: [root(1, { thread_unread_count: 2 })], followed: [root(1, { thread_unread_count: 2 }), root(2)] })
    await screen.findByRole('button', { name: /^Thread của/ })
    await user.click(chips().getByRole('radio', { name: 'Đang theo dõi' }))
    await waitFor(() => expect(screen.getAllByRole('button', { name: /^Thread của/ })).toHaveLength(2))
    expect(server.calls.some((c) => c.path === '/threads/followed')).toBe(true)
    // The read thread has no "mới" badge.
    const read = screen.getByRole('button', { name: 'Thread của Lê Quang Vinh trong Đối soát giao dịch, 4 trả lời' })
    expect(within(read).queryByText(/mới$/)).toBeNull()
  })

  it('opens straight on Đang theo dõi when asked, and does not ask for what it does not show twice', async () => {
    show('following', { followed: [root(2)] })
    expect(await screen.findByRole('button', { name: /^Thread của/ })).toBeInTheDocument()
    expect(chips().getByRole('radio', { name: 'Đang theo dõi' })).toBeChecked()
  })

  it('opens the conversation on that thread when a row is chosen', async () => {
    const user = userEvent.setup()
    show('unread', { threads: [root(1, { thread_unread_count: 2 })] })
    await user.click(await screen.findByRole('button', { name: /^Thread của/ }))
    expect(onOpen).toHaveBeenCalledWith(CH.doisoat, root(1).id)
  })

  it('says each chip is empty in its own words', async () => {
    const user = userEvent.setup()
    show('unread')
    expect(await screen.findByText('Không có thread nào có tin mới.')).toBeInTheDocument()
    await user.click(chips().getByRole('radio', { name: 'Đang theo dõi' }))
    expect(await screen.findByText('Bạn chưa theo dõi thread nào.')).toBeInTheDocument()
  })

  it('leaves out a thread of a conversation the list does not hold', async () => {
    show('unread', { threads: [root(1, { thread_unread_count: 1, channel_id: CH.hopdong })] })
    expect(await screen.findByText('Không có thread nào có tin mới.')).toBeInTheDocument()
  })

  it('offers to try again when the followed threads do not load', async () => {
    const user = userEvent.setup()
    server = createChatStateServer({}, 0, { followed: [root(2)] })
    let failing = true
    vi.mocked(apiFetch).mockImplementation((async (path: string, init?: RequestInit) => {
      if (path.startsWith('/threads/followed') && failing) throw new Error('offline')
      return server.api(path, init)
    }) as never)
    renderWithClient(
      <PhoneThreadsScreen initial="following" conversations={conversations} people={people} onOpen={onOpen} onClose={onClose} />,
    )
    expect(await screen.findByText('Không tải được danh sách thread.')).toBeInTheDocument()
    failing = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('button', { name: /^Thread của/ })).toBeInTheDocument()
  })

  it('goes back with the arrow and with Esc', async () => {
    const user = userEvent.setup()
    show('unread')
    await screen.findByRole('dialog', { name: 'Thread' })
    await user.click(screen.getByRole('button', { name: 'Đóng danh sách thread' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(2)
  })
})
