import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Message } from '../../api/messaging'
import { notificationLevelApi } from '../../api/notification-levels'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { EMPTY_DIRECTORY } from '../../lib/people'
import type { Conversation } from '../../lib/conversations'
import { UnreadThreadsSection } from './UnreadThreadsSection'

vi.mock('../../api/notification-levels', async (orig) => {
  const real = await orig<typeof import('../../api/notification-levels')>()
  return { ...real, notificationLevelApi: { ...real.notificationLevelApi, unreadThreads: vi.fn() } }
})
const api = vi.mocked(notificationLevelApi)

const conv = (id: string, title: string): Conversation =>
  ({ id, kind: 'space', title, hueKey: id, unread: 0, memberCount: 3, lastActivity: 0 })
const CONVS = [conv('0a8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a01', 'Đối soát giao dịch'), conv('0a8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a02', 'Hợp đồng đối tác')]
const root = (n: number, over: Partial<Message> = {}): Message => ({
  id: `1b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a${String(n).padStart(2, '0')}`,
  channel_id: CONVS[0]!.id,
  sender_id: 'u1',
  sender_name: 'Lê Quang Vinh',
  content: `<p>Ảnh sao kê số ${n}</p>`,
  content_format: 'html',
  reply_count: 4,
  thread_unread_count: n,
  last_reply_at: { seconds: 1760000000 + n, nanos: 0 },
  reply_participant_ids: [],
  ...over,
})
const serve = (...roots: Message[]) => api.unreadThreads.mockResolvedValue({ messages: roots })
const mount = (onOpenThread = vi.fn(), max?: number) => {
  renderWithClient(<UnreadThreadsSection conversations={CONVS} people={EMPTY_DIRECTORY} onOpenThread={onOpenThread} max={max} />)
  return onOpenThread
}

beforeEach(() => localStorage.clear())
afterEach(() => { resetClient(); vi.clearAllMocks() })

describe('UnreadThreadsSection', () => {
  it('renders nothing when no thread has news', async () => {
    serve()
    mount()
    await waitFor(() => expect(api.unreadThreads).toHaveBeenCalled())
    expect(screen.queryByText('Thread có tin mới')).not.toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('lists a thread: group, author and root text, new replies, with no identifier on screen', async () => {
    serve(root(2))
    mount()
    const row = await screen.findByRole('button', { name: /^Thread của Lê Quang Vinh trong Đối soát giao dịch, 2 trả lời mới/ })
    expect(within(row).getByText('Đối soát giao dịch')).toBeInTheDocument()
    expect(within(row).getByText('Lê Quang Vinh:')).toBeInTheDocument()
    expect(within(row).getByText(/Ảnh sao kê số 2/)).toBeInTheDocument()
    expect(within(row).getByText('2 trả lời mới')).toBeInTheDocument()
    expectNoIds()
  })

  it('opens the thread in its group on a tap', async () => {
    serve(root(2))
    const open = mount()
    await userEvent.setup().click(await screen.findByRole('button', { name: /^Thread của/ }))
    expect(open).toHaveBeenCalledWith(CONVS[0]!.id, root(2).id)
  })

  it('shows the first three and folds the rest behind "Xem thêm N thread"', async () => {
    serve(root(1), root(2), root(3), root(4), root(5))
    mount()
    const list = await screen.findByRole('list', { name: 'Thread có tin mới' })
    expect(within(list).getAllByRole('button', { name: /^Thread của/ })).toHaveLength(3)
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Xem thêm 2 thread' }))
    expect(within(list).getAllByRole('button', { name: /^Thread của/ })).toHaveLength(5)
    await user.click(screen.getByRole('button', { name: 'Ẩn bớt' }))
    expect(within(list).getAllByRole('button', { name: /^Thread của/ })).toHaveLength(3)
  })

  it('shows two on a phone', async () => {
    serve(root(1), root(2), root(3))
    mount(undefined, 2)
    await screen.findByRole('button', { name: 'Xem thêm 1 thread' })
    expect(screen.getAllByRole('button', { name: /^Thread của/ })).toHaveLength(2)
  })

  it('counts the threads in the header and skips ones from conversations it does not know', async () => {
    serve(root(1), root(2, { channel_id: 'elsewhere' }))
    mount()
    await screen.findByRole('button', { name: /^Thread của/ })
    expect(screen.getAllByRole('button', { name: /^Thread của/ })).toHaveLength(1)
    expect(screen.getByLabelText('1 thread')).toBeInTheDocument()
  })

  it('collapses, and remembers it on this device', async () => {
    serve(root(1))
    const user = userEvent.setup()
    mount()
    await user.click(await screen.findByRole('button', { name: 'Thu gọn' }))
    expect(screen.queryByRole('button', { name: /^Thread của/ })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Mở rộng' })).toHaveAttribute('aria-expanded', 'false')
    resetClient()
    document.body.innerHTML = ''
    serve(root(1))
    mount()
    expect(await screen.findByRole('button', { name: 'Mở rộng' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Thread của/ })).not.toBeInTheDocument()
  })

  it('still works when storage throws', async () => {
    // Only this section's key is blocked; the auth store persists through the same Storage.
    const real = { get: Storage.prototype.getItem, set: Storage.prototype.setItem }
    const blocked = (k: string) => k === 'ngac-home-threads-collapsed'
    const get = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(function (this: Storage, k: string) {
      if (blocked(k)) throw new Error('denied')
      return real.get.call(this, k)
    })
    const set = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(function (this: Storage, k: string, v: string) {
      if (blocked(k)) throw new Error('denied')
      real.set.call(this, k, v)
    })
    try {
      serve(root(1))
      mount()
      await userEvent.setup().click(await screen.findByRole('button', { name: 'Thu gọn' }))
      expect(screen.getByRole('button', { name: 'Mở rộng' })).toBeInTheDocument()
    } finally {
      get.mockRestore()
      set.mockRestore()
    }
  })

  it('hides itself when the list cannot be loaded', async () => {
    api.unreadThreads.mockRejectedValue(new Error('500'))
    mount()
    await waitFor(() => expect(api.unreadThreads).toHaveBeenCalled())
    expect(screen.queryByText('Thread có tin mới')).not.toBeInTheDocument()
  })
})
