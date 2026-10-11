import { act, cleanup, screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { CH, ME, U, WS_ID } from '../../test/chat-fixtures'
import { messagingApi } from '../../api/messaging'
import { shortcutsApi, type FeedMessage } from '../../api/shortcuts'
import type { Conversation } from '../../lib/conversations'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { MentionsView } from './MentionsView'

vi.mock('../../api/shortcuts', async (orig) => {
  const real = await orig<typeof import('../../api/shortcuts')>()
  return { ...real, shortcutsApi: { ...real.shortcutsApi, listMentions: vi.fn(), mentionsUnreadCount: vi.fn() } }
})
vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, getThread: vi.fn() } }
})
const api = vi.mocked(shortcutsApi)
const getThread = vi.mocked(messagingApi.getThread)

const stamp = (daysAgo: number, h: number, mi: number) => {
  const d = new Date()
  d.setDate(d.getDate() - daysAgo)
  d.setHours(h, mi, 0, 0)
  return { seconds: Math.floor(d.getTime() / 1000), nanos: 0 }
}
const space = (id: string, title: string): Conversation => ({
  id, kind: 'space', title, hueKey: id, unread: 0, memberCount: 3, lastActivity: 0,
})
const CONVS = new Map([[CH.vanhanh, space(CH.vanhanh, 'Vận hành chung')], [CH.doisoat, space(CH.doisoat, 'Đối soát giao dịch')]])
const selfMention = `<p>Nhờ <span data-mention="${ME.id}">@Lê Thị Hoa</span> xem lại</p>`

const feed = (...messages: FeedMessage[]) => api.listMentions.mockResolvedValue({ messages, has_more: false })
const main = (over: Partial<FeedMessage> = {}): FeedMessage => ({
  id: 'mm-1', channel_id: CH.vanhanh, sender_id: U.yen.id, sender_name: 'Phạm Hải Yến',
  content: selfMention, content_format: 'html', created_at: stamp(0, 9, 12), unread: true, ...over,
})

const view = (over: Partial<Parameters<typeof MentionsView>[0]> = {}) => {
  const props = { workspaceId: WS_ID, conversationOf: (id: string) => CONVS.get(id), onOpen: vi.fn(), onHome: vi.fn(), ...over }
  renderWithClient(<MentionsView {...props} />)
  return props
}

beforeEach(() => {
  vi.clearAllMocks()
  api.mentionsUnreadCount.mockResolvedValue(2)
})
afterEach(resetClient)

describe('MentionsView', () => {
  it('lists mentions under day dividers with the conversation, sender, time and the unread count', async () => {
    feed(main(), main({ id: 'mm-2', channel_id: CH.doisoat, sender_name: 'Nguyễn Thu Lan', unread: undefined, created_at: stamp(1, 17, 20) }))
    view()
    expect(await screen.findByRole('heading', { name: 'Hôm nay' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Hôm qua' })).toBeInTheDocument()
    const today = screen.getByRole('list', { name: 'Hôm nay' })
    expect(within(today).getByText('Vận hành chung')).toBeInTheDocument()
    expect(within(today).getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(within(today).getByText('09:12')).toBeInTheDocument()
    expect(await screen.findByText(/^2 chưa đọc · /)).toBeInTheDocument()
    expectNoIds()
  })

  it('marks unread with a bold name and a hidden label, and read plainly', async () => {
    feed(main(), main({ id: 'mm-2', sender_name: 'Nguyễn Thu Lan', unread: undefined }))
    view()
    const links = await screen.findAllByRole('link')
    expect(links[0]).toHaveAttribute('data-unread', 'true')
    expect(within(links[0]!).getByText('Chưa đọc')).toHaveClass('sr-only')
    expect(within(links[0]!).getByText('Phạm Hải Yến')).toHaveClass('font-semibold')
    expect(links[1]).not.toHaveAttribute('data-unread')
    expect(within(links[1]!).queryByText('Chưa đọc')).toBeNull()
  })

  it('washes the reader\'s own @name and leaves other people\'s plain', async () => {
    feed(main({ content: `<p><span data-mention="${ME.id}">@Lê Thị Hoa</span> và <span data-mention="${U.duc.id}">@Trần Minh Đức</span></p>` }))
    view()
    expect(await screen.findByText('@Lê Thị Hoa')).toHaveClass('bg-accent-wash')
    expect(screen.getByText('@Trần Minh Đức')).not.toHaveClass('bg-accent-wash')
  })

  it('names the thread a reply belongs to', async () => {
    feed(main({ id: 'rr-1', parent_message_id: 'root-1' }))
    getThread.mockResolvedValue({ messages: [{ id: 'root-1', channel_id: CH.vanhanh, sender_id: U.lan.id, sender_name: 'Nguyễn Thu Lan', content: 'Đối soát lệch 3 giao dịch' }] })
    view()
    expect(await screen.findByText('Trả lời trong thread của Nguyễn Thu Lan: “Đối soát lệch 3 giao dịch”')).toBeInTheDocument()
  })

  it('opens an item, turning its dot and the badge down at once, and viewing alone reads nothing', async () => {
    const user = userEvent.setup()
    feed(main())
    const props = view()
    const link = await screen.findByRole('link')
    await screen.findByText(/^2 chưa đọc/)
    expect(link).toHaveAttribute('data-unread', 'true')
    await user.click(link)
    expect(props.onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'mm-1' }))
    await waitFor(() => expect(screen.getByText(/^1 chưa đọc/)).toBeInTheDocument())
    expect(screen.getByRole('link')).not.toHaveAttribute('data-unread')
  })

  it('shows skeletons while loading', () => {
    api.listMentions.mockReturnValue(new Promise(() => {}))
    view()
    expect(screen.getByText('Đang tải').parentElement).toHaveAttribute('aria-busy', 'true')
  })

  it('says nobody mentioned the reader and offers the way home', async () => {
    const user = userEvent.setup()
    feed()
    const props = view()
    expect(await screen.findByText('Chưa ai nhắc tên bạn. Khi có người gõ @tên bạn, tin sẽ hiện ở đây.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Về Trang chủ' }))
    expect(props.onHome).toHaveBeenCalled()
  })

  it('calls the way home Về Tin nhắn on a phone, which also has a Quay lại bar', async () => {
    const user = userEvent.setup()
    feed()
    const onBack = vi.fn()
    view({ phone: true, onBack })
    expect(await screen.findByRole('button', { name: 'Về Tin nhắn' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /Quay lại/ }))
    expect(onBack).toHaveBeenCalled()
  })

  it('answers a failure with Thử lại, which asks again', async () => {
    const user = userEvent.setup()
    api.listMentions.mockRejectedValueOnce(new Error('boom'))
    view()
    expect(await screen.findByText('Không tải được tin nhắc tên bạn.')).toBeInTheDocument()
    feed(main())
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('link')).toBeInTheDocument()
  })

  it('pages with Tải thêm, 25 at a time, and closes with a line when the end is reached', async () => {
    const user = userEvent.setup()
    api.listMentions
      .mockResolvedValueOnce({ messages: [main()], has_more: true })
      .mockResolvedValueOnce({ messages: [main({ id: 'mm-9', sender_name: 'Trần Minh Đức', unread: undefined })], has_more: false })
    view()
    await screen.findByText('Phạm Hải Yến')
    expect(screen.queryByText('Đã hiện hết tin nhắc tên bạn')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }))
    expect(await screen.findByText('Trần Minh Đức')).toBeInTheDocument()
    expect(screen.getByText('Đã hiện hết tin nhắc tên bạn')).toBeInTheDocument()
    expect(api.listMentions).toHaveBeenLastCalledWith(expect.objectContaining({ limit: 25, cursor: expect.objectContaining({ beforeId: 'mm-1' }) }))
  })

  it('keeps what it has when a later page fails, and retries', async () => {
    const user = userEvent.setup()
    api.listMentions
      .mockResolvedValueOnce({ messages: [main()], has_more: true })
      .mockRejectedValueOnce(new Error('boom'))
    view()
    await screen.findByText('Phạm Hải Yến')
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }))
    expect(await screen.findByText('Không tải thêm được.')).toBeInTheDocument()
    expect(screen.getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Thử lại' })).toBeInTheDocument()
  })
})

describe('MentionsView while it is open', () => {
  // Unmount before the shared afterEach clears the cache, or the live view refetches into the next test.
  afterEach(cleanup)
  const refetch = () => act(async () => { await queryClient.invalidateQueries({ queryKey: keys.shortcuts.mentionsAll() }) })

  it('puts a new mention at the top, washed in its sender\'s colour; what was already there is not news', async () => {
    feed(main())
    view()
    await screen.findByText('Phạm Hải Yến')
    expect(screen.getByRole('link')).not.toHaveClass('rt-wash')
    feed(main({ id: 'mm-new', sender_id: U.duc.id, sender_name: 'Trần Minh Đức', created_at: stamp(0, 9, 40) }), main())
    await refetch()
    await waitFor(() => expect(screen.getAllByRole('link')).toHaveLength(2))
    const [fresh, old] = screen.getAllByRole('link')
    expect(fresh).toHaveTextContent('Trần Minh Đức')
    expect(fresh).toHaveClass('rt-wash')
    expect(fresh).toHaveAttribute('data-realtime', 'other')
    expect(old).not.toHaveClass('rt-wash')
  })

  it('does not wash an older page that "Tải thêm" brings in', async () => {
    const user = userEvent.setup()
    api.listMentions
      .mockResolvedValueOnce({ messages: [main()], has_more: true })
      .mockResolvedValueOnce({ messages: [main({ id: 'mm-old', sender_name: 'Trần Minh Đức', created_at: stamp(3, 8, 0), unread: undefined })], has_more: false })
    view()
    await screen.findByText('Phạm Hải Yến')
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }))
    const older = (await screen.findByText('Trần Minh Đức')).closest('a')!
    expect(older).not.toHaveClass('rt-wash')
  })

  it('washes nothing when a refetch brings nothing new', async () => {
    feed(main())
    view()
    await screen.findByText('Phạm Hải Yến')
    await refetch()
    expect(screen.getByRole('link')).not.toHaveClass('rt-wash')
  })
})
