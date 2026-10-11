import { screen, within, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { CH, U, WS_ID } from '../../test/chat-fixtures'
import { shortcutsApi, type FeedMessage } from '../../api/shortcuts'
import type { Conversation } from '../../lib/conversations'
import { useToastStore } from '../primitives'
import { StarredView } from './StarredView'

vi.mock('../../api/shortcuts', async (orig) => {
  const real = await orig<typeof import('../../api/shortcuts')>()
  return {
    ...real,
    shortcutsApi: { ...real.shortcutsApi, listStarred: vi.fn(), unstarMessage: vi.fn(), unstarConversation: vi.fn(), starMessage: vi.fn() },
  }
})
vi.mock('../../api/messaging', async (orig) => {
  const real = await orig<typeof import('../../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, getThread: vi.fn() } }
})
const api = vi.mocked(shortcutsApi)

const stamp = (daysAgo: number, h: number, mi: number) => {
  const d = new Date()
  d.setDate(d.getDate() - daysAgo)
  d.setHours(h, mi, 0, 0)
  return { seconds: Math.floor(d.getTime() / 1000), nanos: 0 }
}
const space = (id: string, title: string, unread = 0): Conversation => ({
  id, kind: 'space', title, hueKey: id, unread, memberCount: 3, lastActivity: 0,
})
const VANHANH = space(CH.vanhanh, 'Vận hành chung', 4)
const DOISOAT = space(CH.doisoat, 'Đối soát giao dịch')
const byId = new Map([[VANHANH.id, VANHANH], [DOISOAT.id, DOISOAT]])
const star = (over: Partial<FeedMessage> = {}): FeedMessage => ({
  id: 'sm-1', channel_id: CH.doisoat, sender_id: U.lan.id, sender_name: 'Nguyễn Thu Lan', content: 'Dạ xong rồi ạ',
  created_at: stamp(1, 17, 20), starred: true, starred_at: stamp(0, 8, 0), ...over,
})

const view = (over: Partial<Parameters<typeof StarredView>[0]> = {}) => {
  const props = {
    workspaceId: WS_ID, starredConversations: [VANHANH], conversationOf: (id: string) => byId.get(id),
    onOpen: vi.fn(), onOpenConversation: vi.fn(), onHome: vi.fn(), ...over,
  }
  renderWithClient(<StarredView {...props} />)
  return props
}

beforeEach(() => {
  vi.clearAllMocks()
  useToastStore.getState().clear()
  api.unstarMessage.mockResolvedValue({})
  api.unstarConversation.mockResolvedValue({})
  api.starMessage.mockResolvedValue({})
})
afterEach(resetClient)

describe('StarredView', () => {
  it('has a section of conversations and a section of messages, with a count each', async () => {
    api.listStarred.mockResolvedValue({ messages: [star()], has_more: false })
    view()
    const convs = await screen.findByRole('list', { name: 'Cuộc trò chuyện' })
    expect(within(convs).getByText('Vận hành chung')).toBeInTheDocument()
    expect(within(convs).getByLabelText('4 tin chưa đọc')).toBeInTheDocument()
    const msgs = screen.getByRole('list', { name: 'Tin nhắn' })
    expect(within(msgs).getByText('Đối soát giao dịch')).toBeInTheDocument()
    expect(within(msgs).getByText('Hôm qua 17:20')).toBeInTheDocument()
    expect(within(msgs).getByText('Dạ xong rồi ạ')).toBeInTheDocument()
    expectNoIds()
  })

  it('shows a starred card in plain ink with no unread dot', async () => {
    api.listStarred.mockResolvedValue({ messages: [star({ unread: true })], has_more: false })
    view()
    const link = (await screen.findAllByRole('link')).find((a) => within(a).queryByText('Nguyễn Thu Lan'))!
    expect(link).not.toHaveAttribute('data-unread')
    expect(within(link).queryByText('Chưa đọc')).toBeNull()
  })

  it('opens a message and a conversation', async () => {
    const user = userEvent.setup()
    api.listStarred.mockResolvedValue({ messages: [star()], has_more: false })
    const props = view()
    await user.click(await screen.findByText('Dạ xong rồi ạ'))
    expect(props.onOpen).toHaveBeenCalledWith(expect.objectContaining({ id: 'sm-1' }))
    await user.click(screen.getByText('Vận hành chung'))
    expect(props.onOpenConversation).toHaveBeenCalledWith(VANHANH)
  })

  it('unstars a message from its star button, which sits outside the link, and offers Hoàn tác', async () => {
    const user = userEvent.setup()
    api.listStarred.mockResolvedValue({ messages: [star()], has_more: false })
    view()
    const button = await screen.findByRole('button', { name: 'Gắn sao tin của Nguyễn Thu Lan' })
    expect(button).toHaveAttribute('aria-pressed', 'true')
    expect(button).toHaveAttribute('title', 'Bỏ gắn sao')
    expect(button.closest('a')).toBeNull()
    expect(button).toHaveClass('w-11', 'h-11')
    await user.click(button)
    await waitFor(() => expect(api.unstarMessage).toHaveBeenCalledWith('sm-1'))
    expect(screen.getByText('Chưa gắn sao tin nào. Trỏ vào một tin, chọn Thêm rồi Gắn sao tin.')).toBeInTheDocument()
    expect(useToastStore.getState().toasts[0]).toMatchObject({ message: 'Đã bỏ gắn sao tin', action: { label: 'Hoàn tác' } })
  })

  it('unstars a conversation from its star button', async () => {
    const user = userEvent.setup()
    api.listStarred.mockResolvedValue({ messages: [], has_more: false })
    view()
    const button = await screen.findByRole('button', { name: 'Gắn sao Vận hành chung' })
    expect(button).toHaveAttribute('aria-pressed', 'true')
    await user.click(button)
    await waitFor(() => expect(api.unstarConversation).toHaveBeenCalledWith(CH.vanhanh))
    expect(useToastStore.getState().toasts[0]).toMatchObject({ message: 'Đã bỏ gắn sao “Vận hành chung”' })
  })

  it('gives each section its own empty line', async () => {
    api.listStarred.mockResolvedValue({ messages: [star()], has_more: false })
    view({ starredConversations: [] })
    expect(await screen.findByText('Chưa gắn sao cuộc trò chuyện nào. Mở menu ⋯ của một cuộc trò chuyện rồi chọn Gắn sao.')).toBeInTheDocument()
    expect(screen.queryByText(/^Chưa gắn sao tin nào/)).toBeNull()
  })

  it('has one large empty state when nothing is starred, with the way home', async () => {
    const user = userEvent.setup()
    api.listStarred.mockResolvedValue({ messages: [], has_more: false })
    const props = view({ starredConversations: [] })
    expect(await screen.findByText('Chưa gắn sao gì. Gắn sao cuộc trò chuyện bằng menu ⋯ của hàng, gắn sao tin bằng Thêm → Gắn sao tin.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Về Trang chủ' }))
    expect(props.onHome).toHaveBeenCalled()
  })

  it('shows skeletons while loading and Thử lại on failure', async () => {
    const user = userEvent.setup()
    api.listStarred.mockRejectedValueOnce(new Error('boom'))
    view()
    expect(screen.getByText('Đang tải').parentElement).toHaveAttribute('aria-busy', 'true')
    expect(await screen.findByText('Không tải được mục đã gắn sao.')).toBeInTheDocument()
    api.listStarred.mockResolvedValue({ messages: [star()], has_more: false })
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByText('Dạ xong rồi ạ')).toBeInTheDocument()
  })

  it('pages the messages with Tải thêm', async () => {
    const user = userEvent.setup()
    api.listStarred
      .mockResolvedValueOnce({ messages: [star()], has_more: true })
      .mockResolvedValueOnce({ messages: [star({ id: 'sm-2', content: 'Tin thứ hai' })], has_more: false })
    view()
    await screen.findByText('Dạ xong rồi ạ')
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }))
    expect(await screen.findByText('Tin thứ hai')).toBeInTheDocument()
    expect(api.listStarred).toHaveBeenLastCalledWith(expect.objectContaining({ limit: 25, cursor: expect.objectContaining({ beforeId: 'sm-1' }) }))
  })
})
