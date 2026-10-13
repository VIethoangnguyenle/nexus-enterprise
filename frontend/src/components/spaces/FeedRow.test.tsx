import { render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CH, UUID_RE } from '../../test/chat-fixtures'
import type { Conversation, FeedLine } from '../../lib/conversations'
import { FeedRow } from './FeedRow'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())

const NOW = Date.now()
const line = (over: Partial<FeedLine> = {}): FeedLine => ({
  id: 'm1', author: 'Phạm Hải Yến', text: 'Chị xem giúp em đề nghị tạm ứng', at: NOW - 5 * 60_000, mentionNames: [], mentionsMe: false, ...over,
})
const conv = (over: Partial<Conversation> = {}): Conversation => ({
  id: CH.doisoat, kind: 'space', title: 'Đối soát giao dịch', hueKey: CH.doisoat, unread: 0, memberCount: 14,
  lastActivity: NOW - 5 * 60_000, main: line(), ...over,
})
const show = (c: Conversation, online = false) => render(<FeedRow conversation={c} online={online} />)
const rowLink = () => screen.getByRole('link', { name: /^Đối soát giao dịch/ })

describe('FeedRow', () => {
  it('shows the name, the relative time and "Người gửi: đoạn đầu" on two lines', () => {
    show(conv())
    expect(rowLink()).toHaveAttribute('href', `/channels/${CH.doisoat}`)
    expect(screen.getByText('5 phút')).toBeInTheDocument()
    const text = screen.getByText(/Phạm Hải Yến:/)
    expect(text).toHaveTextContent('Phạm Hải Yến: Chị xem giúp em đề nghị tạm ứng')
    expect(text.parentElement!.className).toContain('line-clamp-2')
  })

  it('says "Bạn:" for my own message', () => {
    show(conv({ main: line({ author: 'Bạn', text: 'Ok em' }) }))
    expect(screen.getByText(/Bạn:/)).toHaveTextContent('Bạn: Ok em')
  })

  it('falls back to the member count, or to the role of the person in a direct message', () => {
    const { unmount } = show(conv({ main: undefined, lastActivity: 0 }))
    expect(screen.getByText('14 thành viên')).toBeInTheDocument()
    unmount()
    show(conv({ kind: 'dm', title: 'Phạm Hải Yến', main: undefined, lastActivity: 0 }))
    expect(screen.getByText('Tin nhắn trực tiếp')).toBeInTheDocument()
  })

  describe('a thread reply newer than the main message', () => {
    const withReply = (over: Partial<Conversation> = {}) => conv({
      main: line({ author: 'Lê Quang Vinh', text: 'Bảng đối soát tháng 10' }),
      reply: { ...line({ id: 'r1', author: 'Nguyễn Thu Lan', text: 'em gửi lại ạ' }), parentId: 'root-1' },
      ...over,
    })

    it('adds an indented line that opens that thread, and cuts the main line to one', () => {
      show(withReply())
      const thread = screen.getByRole('link', { name: 'Mở thread: Nguyễn Thu Lan: em gửi lại ạ' })
      expect(thread).toHaveAttribute('href', `/channels/${CH.doisoat}`)
      expect(JSON.parse(thread.getAttribute('data-search')!)).toEqual({ thread: 'root-1' })
      expect(within(thread).getByText(/Nguyễn Thu Lan:/)).toBeInTheDocument()
      expect(screen.getByText(/Lê Quang Vinh:/).parentElement!.className).toContain('line-clamp-1')
      // The row still opens the conversation, not the thread.
      expect(rowLink()).not.toHaveAttribute('data-search')
    })

    it('keeps the reply link above the stretched row link so it takes its own tap', () => {
      show(withReply())
      expect(screen.getByRole('link', { name: /^Mở thread/ }).className).toContain('z-10')
      expect(rowLink().className).toContain('after:inset-0')
    })

    it('has no thread line without a reply', () => {
      show(conv())
      expect(screen.queryByRole('link', { name: /^Mở thread/ })).toBeNull()
    })
  })

  it('draws the people a message names as chips, only for the exact "@name"', () => {
    show(conv({
      main: line({ text: '@Lê Thị Hoa xem dòng 14, cc @Trần Minh', mentionNames: ['Lê Thị Hoa'] }),
    }))
    const chip = screen.getByText('@Lê Thị Hoa')
    expect(chip.className).toContain('bg-accent-wash')
    expect(screen.queryByText('@Trần Minh')).toBeNull()
    expect(screen.getByText(/cc @Trần Minh/)).toBeInTheDocument()
  })

  describe('unread', () => {
    it('is bold, with the count at the end of the line', () => {
      show(conv({ unread: 3 }))
      expect(rowLink().className).toContain('font-bold')
      expect(rowLink().closest('[data-unread]')).not.toBeNull()
      expect(screen.getByLabelText('3 tin chưa đọc')).toHaveTextContent('3')
    })

    it('puts the count after the thread line when there is one', () => {
      show(conv({
        unread: 2,
        reply: { ...line({ id: 'r1', author: 'Nguyễn Thu Lan', text: 'ok' }), parentId: 'root-1' },
      }))
      const thread = screen.getByRole('link', { name: /^Mở thread/ })
      expect(within(thread).getByLabelText('2 tin chưa đọc')).toBeInTheDocument()
      expect(screen.getAllByLabelText('2 tin chưa đọc')).toHaveLength(1)
    })

    it('shows "@" instead of the count when the newest reply names the reader', () => {
      show(conv({
        unread: 2,
        reply: { ...line({ id: 'r1', author: 'Nguyễn Thu Lan', text: '@Lê Thị Hoa ơi', mentionsMe: true }), parentId: 'root-1' },
      }))
      expect(screen.getByLabelText('nhắc tên bạn')).toBeInTheDocument()
      expect(screen.queryByLabelText('2 tin chưa đọc')).toBeNull()
    })

    it('shows "@" when the newest message is a main message that names the reader', () => {
      show(conv({ unread: 2, main: line({ id: 'm2', text: '@Lê Thị Hoa xem giúp', mentionsMe: true }) }))
      expect(screen.getByLabelText('nhắc tên bạn')).toBeInTheDocument()
      expect(screen.queryByLabelText('2 tin chưa đọc')).toBeNull()
    })

    it('shows the count, not "@", when a newer reply does not name the reader', () => {
      show(conv({
        unread: 2,
        main: line({ id: 'm2', text: '@Lê Thị Hoa xem giúp', mentionsMe: true }),
        reply: { ...line({ id: 'r1', author: 'Nguyễn Thu Lan', text: 'ok' }), parentId: 'm2' },
      }))
      expect(screen.queryByLabelText('nhắc tên bạn')).toBeNull()
      expect(screen.getByLabelText('2 tin chưa đọc')).toBeInTheDocument()
    })
  })

  describe('muted', () => {
    it('fades, says so to a screen reader, and counts nothing', () => {
      const { container } = show(conv({ level: 'off', unread: 0, mentions: 0 }))
      expect(screen.getByRole('link', { name: 'Đối soát giao dịch, đã tắt thông báo' })).toBeInTheDocument()
      expect(container.querySelector('[data-muted]')!.className).toContain('opacity-[0.66]')
      expect(screen.queryByLabelText(/chưa đọc|lần nhắc/)).toBeNull()
    })

    it('still shows "@" when someone named the reader', () => {
      show(conv({ level: 'off', mentions: 2 }))
      expect(screen.getByLabelText('2 lần nhắc tên bạn')).toHaveTextContent('@')
    })
  })

  it('never renders an identifier', () => {
    const { container } = show(conv({ unread: 1, reply: { ...line({ id: 'r1' }), parentId: 'root-1' } }))
    expect(container.textContent).not.toMatch(UUID_RE)
  })
})
