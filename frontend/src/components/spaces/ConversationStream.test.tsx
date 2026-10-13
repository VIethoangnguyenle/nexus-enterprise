import { act, fireEvent, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { buildDirectory } from '../../lib/people'
import { expectNoIds } from '../../test/no-ids'
import { CONTACTS, MESSAGES, MSG, U, CH, ME } from '../../test/chat-fixtures'
import type { Message } from '../../api/messaging'
import type { MessageLifecycle } from '../../hooks/useMessageActions'
import { ConversationStream, type ConversationStreamProps } from './ConversationStream'

// The card reads its own poll from the server; these tests are about the stream around it.
vi.mock('../chat/PollCard', () => ({ PollCard: () => <div data-testid="poll-card" /> }))

afterEach(() => {
  resetClient()
  vi.useRealTimers()
})
beforeEach(() => {
  // Only the clock is frozen, so day labels do not depend on the day the suite runs.
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date(2026, 9, 9, 12, 0))
})

/** Timers too, for the long press; the clock stays where beforeEach put it. */
const fakeAllTimers = () => {
  vi.useFakeTimers({ now: new Date(2026, 9, 9, 12, 0) })
}

const people = buildDirectory(CONTACTS)
const ascending = [...MESSAGES].reverse()
const at = (h: number, m: number) => new Date(2026, 9, 9, h, m).toISOString()
const said = (id: string, sender: string, name: string, when: string, content: string, extra: Partial<Message> = {}): Message => ({
  id, channel_id: CH.doisoat, sender_id: sender, sender_name: name, content, content_format: 'plain', created_at: when, ...extra,
})

function stream(messages: Message[], props: Partial<ConversationStreamProps> = {}) {
  return (
    <ConversationStream
      channelId={CH.doisoat}
      label="Tin nhắn trong Đối soát giao dịch"
      messages={messages}
      ready
      hasMore={false}
      loadingOlder={false}
      onLoadOlder={vi.fn()}
      me={ME.id}
      people={people}
      unread={{ ready: true, lastReadId: '', count: 0 }}
      onOpenThread={vi.fn()}
      onReact={vi.fn()}
      onToggleReaction={vi.fn()}
      onPin={vi.fn()}
      onFollow={vi.fn()}
      {...props}
    />
  )
}

const article = (name: RegExp) => screen.getByRole('article', { name })

describe('ConversationStream: the log', () => {
  it('is a polite live log named for the conversation', () => {
    renderWithClient(stream(ascending))
    const log = screen.getByRole('log', { name: 'Tin nhắn trong Đối soát giao dịch' })
    expect(log).toHaveAttribute('aria-live', 'polite')
  })

  it('shows others by display name and your own message with no name', () => {
    renderWithClient(stream(ascending))
    expect(within(article(/Trần Minh Đức/)).getByText('Trần Minh Đức')).toBeInTheDocument()
    const mine = article(/Bạn/)
    expect(within(mine).getByText(/hạch toán vào 6427/)).toBeInTheDocument()
    expect(within(mine).queryByText('Lê Thị Hoa')).toBeNull()
    expectNoIds()
  })

  it('puts your messages on the right in the accent wash and others on the left in sunk', () => {
    renderWithClient(stream(ascending))
    expect(within(article(/Bạn/)).getByText(/hạch toán vào 6427/).closest('[id$="-text"]')?.className).toContain('bg-accent-wash')
    expect(within(article(/Trần Minh Đức/)).getByText(/lệch 3 giao dịch/).closest('[id$="-text"]')?.className).toContain('bg-sunk')
  })

  it('has a day separator by day', () => {
    renderWithClient(stream([said('a', U.lan.id, 'lan', new Date(2026, 9, 8, 17).toISOString(), 'hôm qua'), ...ascending]))
    expect(screen.getByRole('separator', { name: 'Hôm qua' })).toBeInTheDocument()
    expect(screen.getByRole('separator', { name: 'Hôm nay' })).toBeInTheDocument()
  })

  it('tells an empty conversation apart from a loading one', () => {
    const { rerender } = renderWithClient(stream([], { ready: false }))
    expect(screen.getByRole('log')).toHaveAttribute('aria-busy', 'true')
    rerender(stream([]))
    expect(screen.getByText(/Chưa có tin nhắn nào/)).toBeInTheDocument()
  })
})

describe('ConversationStream: grouping', () => {
  const lan = (id: string, h: number, m: number, text: string) => said(id, U.lan.id, 'lan', at(h, m), text)

  it('shows the name once for the same sender within five minutes', () => {
    renderWithClient(stream([lan('a', 9, 0, 'một'), lan('b', 9, 4, 'hai'), lan('c', 9, 20, 'ba')]))
    // Visible name on the first and the third (gap > 5 min); the second only has a hidden one for screen readers.
    const names = screen.getAllByText('Nguyễn Thu Lan')
    expect(names.filter((n) => !n.className.includes('sr-only'))).toHaveLength(2)
    expect(article(/hai/)).toBeInTheDocument()
  })

  it('does not group across a message that has a thread', () => {
    const a = said('a', U.lan.id, 'lan', at(9, 0), 'một', { reply_count: 1, last_reply_at: at(9, 1), reply_participant_ids: [U.vinh.id] })
    renderWithClient(stream([a, lan('b', 9, 2, 'hai')]))
    expect(screen.getAllByText('Nguyễn Thu Lan').filter((n) => !n.className.includes('sr-only'))).toHaveLength(2)
  })
})

describe('ConversationStream: "Tin mới"', () => {
  it('has no divider when only your own messages follow the read position', () => {
    renderWithClient(stream(ascending, { unread: { ready: true, lastReadId: MSG.t1, count: 1 } }))
    // t1 is read; t2 is mine, so nothing after it is unread.
    expect(screen.queryByRole('separator', { name: 'Tin mới' })).toBeNull()
  })

  it('is placed before the unread message, not before the read one', () => {
    const read = said('r', U.duc.id, 'duc', at(9, 0), 'đã đọc')
    const fresh = said('f', U.vinh.id, 'vinh', at(9, 30), 'chưa đọc')
    renderWithClient(stream([read, fresh], { unread: { ready: true, lastReadId: 'r', count: 1 } }))
    const divider = screen.getByRole('separator', { name: 'Tin mới' })
    const log = screen.getByRole('log')
    const order = [...log.querySelectorAll('article, [role="separator"]')]
    expect(order.indexOf(divider)).toBe(order.indexOf(article(/đã đọc/)) + 1)
    expect(order.indexOf(divider)).toBe(order.indexOf(article(/chưa đọc/)) - 1)
  })

  it('waits for the read position instead of guessing', () => {
    renderWithClient(stream(ascending, { unread: { ready: false, lastReadId: '', count: 0 } }))
    expect(screen.queryByRole('separator', { name: 'Tin mới' })).toBeNull()
  })

  it('stays where the visit began even after the read position moves', () => {
    const read = said('r', U.duc.id, 'duc', at(9, 0), 'đã đọc')
    const fresh = said('f', U.vinh.id, 'vinh', at(9, 30), 'chưa đọc')
    const view = renderWithClient(stream([read, fresh], { unread: { ready: true, lastReadId: 'r', count: 1 } }))
    expect(screen.getByRole('separator', { name: 'Tin mới' })).toBeInTheDocument()
    view.rerender(stream([read, fresh], { unread: { ready: true, lastReadId: 'f', count: 0 } }))
    expect(screen.getByRole('separator', { name: 'Tin mới' })).toBeInTheDocument()
  })

  it('goes when you send a message', async () => {
    const read = said('r', U.duc.id, 'duc', at(9, 0), 'đã đọc')
    const fresh = said('f', U.vinh.id, 'vinh', at(9, 30), 'chưa đọc')
    const mine = said('m', ME.id, 'hoa', at(9, 40), 'trả lời', { _clientKey: 'm' } as Partial<Message>)
    const view = renderWithClient(stream([read, fresh], { unread: { ready: true, lastReadId: 'r', count: 1 } }))
    view.rerender(stream([read, fresh, mine], { unread: { ready: true, lastReadId: 'r', count: 1 } }))
    await vi.waitFor(() => expect(screen.queryByRole('separator', { name: 'Tin mới' })).toBeNull())
  })
})

describe('ConversationStream: thread summary', () => {
  it('summarises the thread from the server fields and opens it', async () => {
    const user = userEvent.setup()
    const onOpenThread = vi.fn()
    renderWithClient(stream(ascending, { onOpenThread }))
    const summary = within(article(/Trần Minh Đức/)).getByRole('button', { name: /^Thread, 2 trả lời/ })
    expect(summary).toHaveAccessibleName('Thread, 2 trả lời, lần cuối 09:20, 1 trả lời mới')
    expect(summary).toHaveTextContent('2 trả lời')
    expect(summary).toHaveTextContent('1 mới')
    await user.click(summary)
    expect(onOpenThread).toHaveBeenCalledWith(MSG.t1)
  })

  it('shows the repliers by name, newest first', () => {
    renderWithClient(stream(ascending))
    const summary = within(article(/Trần Minh Đức/)).getByRole('button', { name: /^Thread/ })
    const faces = [...summary.querySelectorAll('[title]')].map((e) => e.getAttribute('title'))
    expect(faces).toEqual(['Lê Quang Vinh', 'Nguyễn Thu Lan'])
  })

  it('shows "mới" only while following', () => {
    const quiet = { ...ascending[0]!, following: false, thread_unread_count: 4 }
    renderWithClient(stream([quiet, ascending[1]!]))
    const summary = within(article(/Trần Minh Đức/)).getByRole('button', { name: /^Thread/ })
    expect(summary).not.toHaveTextContent('mới')
    expect(summary).toHaveTextContent('2 trả lời')
  })

  it('has no summary row on a message nobody replied to', () => {
    renderWithClient(stream(ascending))
    expect(within(article(/Bạn/)).queryByRole('button', { name: /^Thread/ })).toBeNull()
  })

  it('marks the open thread', () => {
    renderWithClient(stream(ascending, { activeThreadId: MSG.t1 }))
    expect(within(article(/Trần Minh Đức/)).getByRole('button', { name: /^Thread/ })).toHaveAttribute('aria-expanded', 'true')
  })
})

describe('ConversationStream: actions', () => {
  it('replies in a thread from the hover bar', async () => {
    const user = userEvent.setup()
    const onOpenThread = vi.fn()
    renderWithClient(stream(ascending, { onOpenThread }))
    await user.click(within(article(/Bạn/)).getByRole('button', { name: 'Trả lời trong thread' }))
    expect(onOpenThread).toHaveBeenCalledWith(MSG.t2)
  })

  it('follows and unfollows from the "Thêm" menu', async () => {
    const user = userEvent.setup()
    const onFollow = vi.fn()
    renderWithClient(stream(ascending, { onFollow }))
    await user.click(within(article(/Trần Minh Đức/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Bỏ theo dõi thread' }))
    expect(onFollow).toHaveBeenCalledWith(expect.objectContaining({ id: MSG.t1 }), false)
  })

  it('opens the action sheet on a long press and offers only what exists', () => {
    fakeAllTimers()
    const onOpenThread = vi.fn()
    renderWithClient(stream(ascending, { onOpenThread }))
    const row = within(article(/Trần Minh Đức/)).getByText(/lệch 3 giao dịch/)
    fireEvent.pointerDown(row, { pointerType: 'touch', clientX: 10, clientY: 10 })
    act(() => { vi.advanceTimersByTime(399) })
    expect(screen.queryByRole('dialog', { name: 'Thao tác với tin' })).toBeNull()
    act(() => { vi.advanceTimersByTime(2) })
    const sheet = screen.getByRole('dialog', { name: 'Thao tác với tin' })
    expect(within(sheet).getByRole('group', { name: 'Cảm xúc nhanh' })).toBeInTheDocument()
    for (const name of ['Trả lời trong thread', 'Bỏ theo dõi thread', 'Gắn sao tin', 'Sao chép', 'Trích dẫn khi trả lời', 'Ghim']) {
      expect(within(sheet).getByRole('button', { name })).toBeInTheDocument()
    }
    expect(within(sheet).queryByRole('button', { name: /Sửa tin|Xoá tin|chưa đọc/ })).toBeNull()
  })

  // Text selection is off on touch screens (the long press opens the sheet),
  // so the sheet is where a phone copies a message.
  it('copies the message text from the sheet', async () => {
    fakeAllTimers()
    const writeText = vi.fn((_text: string) => Promise.resolve())
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    renderWithClient(stream(ascending))
    fireEvent.pointerDown(within(article(/Trần Minh Đức/)).getByText(/lệch 3 giao dịch/), { pointerType: 'touch', clientX: 1, clientY: 1 })
    act(() => { vi.advanceTimersByTime(401) })
    const sheet = screen.getByRole('dialog', { name: 'Thao tác với tin' })
    await act(async () => { fireEvent.click(within(sheet).getByRole('button', { name: 'Sao chép' })) })
    expect(writeText).toHaveBeenCalledWith(expect.stringMatching(/lệch 3 giao dịch/))
    expect(writeText.mock.calls[0]![0]).not.toMatch(/</)
  })

  it('does not open the sheet when the finger moves (a scroll) or for a mouse', () => {
    fakeAllTimers()
    renderWithClient(stream(ascending))
    const text = within(article(/Trần Minh Đức/)).getByText(/lệch 3 giao dịch/)
    fireEvent.pointerDown(text, { pointerType: 'touch', clientX: 10, clientY: 10 })
    fireEvent.pointerMove(text, { pointerType: 'touch', clientX: 10, clientY: 60 })
    act(() => { vi.advanceTimersByTime(600) })
    fireEvent.pointerDown(text, { pointerType: 'mouse', clientX: 10, clientY: 10 })
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByRole('dialog', { name: 'Thao tác với tin' })).toBeNull()
  })

  it('sends a quick reaction from the sheet', () => {
    fakeAllTimers()
    const onToggleReaction = vi.fn()
    renderWithClient(stream(ascending, { onToggleReaction }))
    fireEvent.pointerDown(within(article(/Trần Minh Đức/)).getByText(/lệch 3 giao dịch/), { pointerType: 'touch', clientX: 1, clientY: 1 })
    act(() => { vi.advanceTimersByTime(401) })
    fireEvent.click(within(screen.getByRole('dialog', { name: 'Thao tác với tin' })).getByRole('button', { name: '👍' }))
    expect(onToggleReaction).toHaveBeenCalledWith(MSG.t1, '👍', false)
  })
})

describe('ConversationStream: scrolling', () => {
  /** jsdom has no layout; give the log a size and a position. */
  function size(el: HTMLElement, v: { scrollHeight: number; clientHeight: number; scrollTop: number }) {
    for (const [k, val] of Object.entries(v)) Object.defineProperty(el, k, { configurable: true, writable: true, value: val })
  }

  it('asks for older messages at once when it opens near the top (a long unread run)', () => {
    const onLoadOlder = vi.fn()
    renderWithClient(stream(ascending, { hasMore: true, onLoadOlder }))
    expect(onLoadOlder).toHaveBeenCalledTimes(1)
  })

  it('asks when the reader scrolls up to the top', () => {
    const onLoadOlder = vi.fn()
    const view = renderWithClient(stream(ascending, { hasMore: false, onLoadOlder }))
    const log = screen.getByRole('log')
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 900 })
    fireEvent.scroll(log)
    view.rerender(stream(ascending, { hasMore: true, onLoadOlder }))
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 100 })
    fireEvent.scroll(log)
    expect(onLoadOlder).toHaveBeenCalledTimes(1)
  })

  it('does not ask while the reader moves away from the top, as a jump to the latest does', () => {
    const onLoadOlder = vi.fn()
    const view = renderWithClient(stream(ascending, { hasMore: false, onLoadOlder }))
    const log = screen.getByRole('log')
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 0 })
    fireEvent.scroll(log)
    view.rerender(stream(ascending, { hasMore: true, onLoadOlder }))
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 100 })
    fireEvent.scroll(log)
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 400 })
    fireEvent.scroll(log)
    expect(onLoadOlder).not.toHaveBeenCalled()
  })

  it('does not ask while loading, when there are none, or far from the top', () => {
    const onLoadOlder = vi.fn()
    const view = renderWithClient(stream(ascending, { hasMore: true, loadingOlder: true, onLoadOlder }))
    const log = screen.getByRole('log')
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 10 })
    fireEvent.scroll(log)
    view.rerender(stream(ascending, { hasMore: false, onLoadOlder }))
    fireEvent.scroll(log)
    view.rerender(stream(ascending, { hasMore: true, onLoadOlder }))
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 900 })
    fireEvent.scroll(log)
    expect(onLoadOlder).not.toHaveBeenCalled()
  })

  it('keeps the line being read in place when older messages arrive', () => {
    const view = renderWithClient(stream(ascending, { hasMore: true }))
    const log = screen.getByRole('log')
    size(log, { scrollHeight: 1000, clientHeight: 600, scrollTop: 0 })
    // A render with the same oldest message records the height ...
    view.rerender(stream(ascending, { hasMore: true }))
    // ... and a page of older ones grows the content by 700px.
    const older = said('o', U.lan.id, 'lan', at(7, 0), 'cũ hơn')
    Object.defineProperty(log, 'scrollHeight', { configurable: true, value: 1700 })
    view.rerender(stream([older, ...ascending], { hasMore: true }))
    expect(log.scrollTop).toBe(700)
  })

  it('offers "Tin mới nhất" when far from the end, and scrolls there', async () => {
    const user = userEvent.setup()
    renderWithClient(stream(ascending))
    const log = screen.getByRole('log')
    const scrollTo = vi.fn()
    log.scrollTo = scrollTo as unknown as typeof log.scrollTo
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 100 })
    fireEvent.scroll(log)
    await user.click(await screen.findByRole('button', { name: /Tin mới nhất/ }))
    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ top: 3000 }))
  })

  it('after jumping past "Tin mới", offers "Nhảy tới tin chưa đọc" instead, and goes back to it', async () => {
    const user = userEvent.setup()
    const read = said('r', U.duc.id, 'duc', at(9, 0), 'đã đọc')
    const fresh = said('f', U.vinh.id, 'vinh', at(9, 30), 'chưa đọc')
    renderWithClient(stream([read, fresh], { unread: { ready: true, lastReadId: 'r', count: 1 } }))
    const log = screen.getByRole('log')
    const scrollTo = vi.fn()
    log.scrollTo = scrollTo as unknown as typeof log.scrollTo
    const divider = log.querySelector('[data-unread-divider]') as HTMLElement
    Object.defineProperty(divider, 'offsetTop', { configurable: true, value: 100 })

    // Opened at the divider, far from the end: the pill says "Tin mới nhất".
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 52 })
    fireEvent.scroll(log)
    await user.click(await screen.findByRole('button', { name: /Tin mới nhất/ }))
    // Passing the divider on the way down is not reading it ...
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 90 })
    fireEvent.scroll(log)
    // ... and at the end the way back is offered.
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 2400 })
    fireEvent.scroll(log)
    await user.click(await screen.findByRole('button', { name: 'Nhảy tới tin chưa đọc' }))
    expect(scrollTo).toHaveBeenLastCalledWith(expect.objectContaining({ top: 52 }))
    expect(screen.queryByRole('button', { name: 'Nhảy tới tin chưa đọc' })).toBeNull()
  })

  it('has no pill at the end', () => {
    renderWithClient(stream(ascending))
    const log = screen.getByRole('log')
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 2400 })
    fireEvent.scroll(log)
    expect(screen.queryByRole('button', { name: /Tin mới nhất|Nhảy tới tin chưa đọc/ })).toBeNull()
  })

  it('reports whether the end is on screen', () => {
    const onAtEndChange = vi.fn()
    renderWithClient(stream(ascending, { onAtEndChange }))
    const log = screen.getByRole('log')
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 100 })
    fireEvent.scroll(log)
    expect(onAtEndChange).toHaveBeenLastCalledWith(false)
    size(log, { scrollHeight: 3000, clientHeight: 600, scrollTop: 2400 })
    fireEvent.scroll(log)
    expect(onAtEndChange).toHaveBeenLastCalledWith(true)
  })
})

describe('ConversationStream: realtime attribution', () => {
  const fromYen = said('44444444-aaaa-4bbb-8ccc-0000000000aa', U.yen.id, 'yen', new Date().toISOString(), 'Em đã gửi bản scan phụ lục số 3')
  const fromMe = said('44444444-aaaa-4bbb-8ccc-0000000000bb', ME.id, 'hoa', new Date().toISOString(), 'Chị nhận rồi nhé')

  it('does not wash history that was there on load', () => {
    renderWithClient(stream(ascending))
    for (const card of screen.getAllByRole('article')) expect(card.className).not.toContain('rt-wash')
  })

  it('does not wash older pages that load later', () => {
    const view = renderWithClient(stream(ascending))
    const older = said('o', U.lan.id, 'lan', at(7, 0), 'cũ hơn')
    view.rerender(stream([older, ...ascending]))
    expect(article(/cũ hơn/).className).not.toContain('rt-wash')
  })

  it('washes a message from someone else in their hue and labels it', () => {
    vi.setSystemTime(new Date(2026, 9, 9, 12, 0))
    const view = renderWithClient(stream(ascending))
    const later = { ...fromYen, created_at: new Date(2026, 9, 9, 12, 1).toISOString() }
    view.rerender(stream([...ascending, later]))
    const card = article(/Phạm Hải Yến/)
    expect(card.className).toContain('rt-wash')
    expect(card.getAttribute('style')).toMatch(/--pw: var\(--color-person-\d-wash\)/)
    expect(within(card).getByText('vừa gửi')).toBeInTheDocument()
  })

  it('gives your own new message only the insert, no wash and no label', () => {
    const view = renderWithClient(stream(ascending))
    const later = { ...fromMe, created_at: new Date(2026, 9, 9, 12, 1).toISOString() }
    view.rerender(stream([...ascending, later]))
    const card = article(/Bạn.*Chị nhận rồi/s)
    expect(card.className).not.toContain('rt-wash')
    expect(within(card).queryByText('vừa gửi')).toBeNull()
  })

  it('washes the summary row only for a followed thread that someone else replied to', () => {
    const base = { ...ascending[0]!, following: true }
    const view = renderWithClient(stream([base, ascending[1]!]))
    const bumped = { ...base, reply_count: 3, reply_participant_ids: [U.ngoc.id, U.vinh.id, U.lan.id] }
    view.rerender(stream([bumped, ascending[1]!]))
    const summary = within(article(/Trần Minh Đức/)).getByRole('button', { name: /^Thread/ })
    expect(summary.className).toContain('rt-wash')

    const quiet = { ...ascending[0]!, following: false, reply_count: 5, reply_participant_ids: [U.yen.id] }
    view.rerender(stream([quiet, ascending[1]!]))
    expect(within(article(/Trần Minh Đức/)).getByRole('button', { name: /^Thread/ }).className).not.toContain('rt-wash')
  })
})

// --- message lifecycle: edit, delete, quote, mark unread ----------------------

const abilitiesFor = (manager = false) => (m: Message) => ({
  edit: m.sender_id === ME.id && !m.linked_entity_type && !m.deleted,
  remove: !m.deleted && (m.sender_id === ME.id || manager),
  markUnread: !m.deleted && m.sender_id !== ME.id,
})

function lifecycleStub(over: Partial<MessageLifecycle> = {}, manager = false): MessageLifecycle {
  return {
    me: ME.id, abilities: abilitiesFor(manager), editing: null, startEdit: vi.fn(), cancelEdit: vi.fn(), saveEdit: vi.fn(),
    deleting: null, deleteBusy: false, askDelete: vi.fn(), cancelDelete: vi.fn(), confirmDelete: vi.fn(), markUnread: vi.fn(),
    ...over,
  }
}

const mineMsg = (extra: Partial<Message> = {}) =>
  said('44444444-aaaa-4bbb-8ccc-0000000000b1', ME.id, 'hoa', at(10, 0), 'Tin của tôi', extra)
const theirMsg = (extra: Partial<Message> = {}) =>
  said('44444444-aaaa-4bbb-8ccc-0000000000b2', U.lan.id, 'lan', at(10, 5), 'Tin của Lan', extra)

describe('ConversationStream: edited and deleted messages', () => {
  it('ends an edited message with "đã sửa", for the reader too', () => {
    renderWithClient(stream([theirMsg({ edited_at: { seconds: 1760000000, nanos: 0 } })]))
    expect(within(article(/Nguyễn Thu Lan/)).getByText('đã sửa')).toBeInTheDocument()
  })

  it('has no "đã sửa" on a message that was never edited', () => {
    renderWithClient(stream([theirMsg()]))
    expect(screen.queryByText('đã sửa')).toBeNull()
  })

  it('shows a deleted root with a thread as the placeholder, keeping the thread summary and dropping text and reactions', () => {
    const gone = theirMsg({
      deleted: true, content: '', reply_count: 2, last_reply_at: at(10, 20), reply_participant_ids: [U.vinh.id],
      reactions: [{ emoji: '👍', count: 1, user_ids: [U.hoa.id] }],
    })
    renderWithClient(stream([gone]))
    const a = article(/Nguyễn Thu Lan/)
    expect(within(a).getByText('Tin nhắn đã bị xoá')).toBeInTheDocument()
    expect(within(a).getByRole('button', { name: /Thread, 2 trả lời/ })).toBeInTheDocument()
    expect(within(a).queryByText('👍')).toBeNull()
    expect(within(a).queryByRole('button', { name: 'Thêm' })).toBeNull()
  })

  it('does not open the sheet on a deleted placeholder', () => {
    fakeAllTimers()
    renderWithClient(stream([theirMsg({ deleted: true, content: '', reply_count: 1 })], { lifecycle: lifecycleStub() }))
    fireEvent.pointerDown(screen.getByText('Tin nhắn đã bị xoá'), { pointerType: 'touch', clientX: 1, clientY: 1 })
    act(() => { vi.advanceTimersByTime(600) })
    expect(screen.queryByRole('dialog', { name: 'Thao tác với tin' })).toBeNull()
  })

  it('shows the line a manager leaves when deleting as a quiet system line, not a bubble', () => {
    const log = said('44444444-aaaa-4bbb-8ccc-0000000000c1', U.duc.id, 'duc', at(10, 9), 'Trần Minh Đức đã xoá một tin nhắn của Nguyễn Thu Lan', {
      message_type: 'system', linked_entity_type: 'message_deleted',
    })
    renderWithClient(stream([log]))
    expect(screen.getByText('Trần Minh Đức đã xoá một tin nhắn của Nguyễn Thu Lan')).toBeInTheDocument()
    expect(screen.queryByRole('article', { name: /Trần Minh Đức/ })).toBeNull()
    expectNoIds()
  })
})

describe('ConversationStream: a quote', () => {
  const quoting = (quoted: Message['quoted']) => theirMsg({ quoted })

  it('shows the quoted name and the opening of the text inside the bubble', () => {
    renderWithClient(stream([quoting({ id: 'q1', sender_id: U.vinh.id, sender_name: 'vinh', text: 'Em thấy kỳ trước để 6427' })]))
    const a = article(/Nguyễn Thu Lan/)
    expect(within(a).getByText('Lê Quang Vinh')).toBeInTheDocument()
    expect(within(a).getByText('Em thấy kỳ trước để 6427')).toBeInTheDocument()
    expectNoIds()
  })

  it('calls your own quoted message "Bạn"', () => {
    renderWithClient(stream([quoting({ id: 'q1', sender_id: ME.id, sender_name: 'hoa', text: 'Vẫn 6427 nhé' })]))
    expect(within(article(/Nguyễn Thu Lan/)).getByText('Bạn')).toBeInTheDocument()
  })

  it('says a deleted original is deleted instead of showing its text', () => {
    renderWithClient(stream([quoting({ id: 'q1', sender_id: U.vinh.id, sender_name: 'vinh', text: '', deleted: true })]))
    expect(within(article(/Nguyễn Thu Lan/)).getByText('Tin nhắn đã bị xoá')).toBeInTheDocument()
  })

  it('scrolls to the original when it is loaded, and is plain text when it is not', async () => {
    const user = userEvent.setup()
    const original = said('44444444-aaaa-4bbb-8ccc-0000000000d1', U.vinh.id, 'vinh', at(9, 50), 'Bản gốc')
    const reply = quoting({ id: original.id, sender_id: U.vinh.id, sender_name: 'vinh', text: 'Bản gốc' })
    const { unmount } = renderWithClient(stream([original, reply]))
    const target = document.querySelector<HTMLElement>(`[data-message-id="${original.id}"]`)!
    const scrollIntoView = vi.fn()
    target.scrollIntoView = scrollIntoView
    await user.click(within(article(/Nguyễn Thu Lan/)).getByRole('button', { name: /Lê Quang Vinh/ }))
    expect(scrollIntoView).toHaveBeenCalledWith(expect.objectContaining({ block: 'center' }))
    unmount()

    renderWithClient(stream([reply]))
    expect(within(article(/Nguyễn Thu Lan/)).queryByRole('button', { name: /Lê Quang Vinh/ })).toBeNull()
  })
})

describe('ConversationStream: edit, delete and mark unread', () => {
  it('puts a pencil on your own text message and not on someone else’s', () => {
    renderWithClient(stream([theirMsg(), mineMsg()], { lifecycle: lifecycleStub() }))
    expect(within(article(/Bạn/)).getByRole('button', { name: 'Sửa tin' })).toBeInTheDocument()
    expect(within(article(/Nguyễn Thu Lan/)).queryByRole('button', { name: 'Sửa tin' })).toBeNull()
  })

  it('offers no pencil on a card', () => {
    renderWithClient(stream([mineMsg({ linked_entity_type: 'poll', linked_entity_id: 'p' })], { lifecycle: lifecycleStub() }))
    expect(screen.queryByRole('button', { name: 'Sửa tin' })).toBeNull()
  })

  it('opens the message for editing from the pencil', async () => {
    const user = userEvent.setup()
    const startEdit = vi.fn()
    const m = mineMsg()
    renderWithClient(stream([m], { lifecycle: lifecycleStub({ startEdit }) }))
    await user.click(within(article(/Bạn/)).getByRole('button', { name: 'Sửa tin' }))
    expect(startEdit).toHaveBeenCalledWith(expect.objectContaining({ id: m.id }))
  })

  it('lists, in the design order, what "Thêm" holds for your own message', async () => {
    const user = userEvent.setup()
    renderWithClient(stream([mineMsg()], { lifecycle: lifecycleStub() }))
    await user.click(within(article(/Bạn/)).getByRole('button', { name: 'Thêm' }))
    const menu = await screen.findByRole('menu', { name: 'Thêm thao tác cho tin' })
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Theo dõi thread', 'Gắn sao tin', 'Sao chép văn bản', 'Trích dẫn khi trả lời', 'Sửa tin', 'Xoá tin',
    ])
    expect(within(menu).getByRole('separator')).toBeInTheDocument()
  })

  it('lists mark-unread, and no edit or delete, for someone else’s message when you do not manage', async () => {
    const user = userEvent.setup()
    renderWithClient(stream([theirMsg()], { lifecycle: lifecycleStub() }))
    await user.click(within(article(/Nguyễn Thu Lan/)).getByRole('button', { name: 'Thêm' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual([
      'Theo dõi thread', 'Gắn sao tin', 'Sao chép văn bản', 'Trích dẫn khi trả lời', 'Đánh dấu chưa đọc',
    ])
  })

  it('lets a manager delete someone else’s message', async () => {
    const user = userEvent.setup()
    const askDelete = vi.fn()
    renderWithClient(stream([theirMsg()], { lifecycle: lifecycleStub({ askDelete }, true) }))
    await user.click(within(article(/Nguyễn Thu Lan/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Xoá tin' }))
    expect(askDelete).toHaveBeenCalledWith(expect.objectContaining({ sender_id: U.lan.id }))
  })

  it('marks a message unread from the menu', async () => {
    const user = userEvent.setup()
    const markUnread = vi.fn()
    renderWithClient(stream([theirMsg()], { lifecycle: lifecycleStub({ markUnread }) }))
    await user.click(within(article(/Nguyễn Thu Lan/)).getByRole('button', { name: 'Thêm' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Đánh dấu chưa đọc' }))
    expect(markUnread).toHaveBeenCalledTimes(1)
  })

  it('has the same actions in the long-press sheet: edit and delete for yours, mark unread for others', () => {
    fakeAllTimers()
    const { unmount } = renderWithClient(stream([mineMsg()], { lifecycle: lifecycleStub() }))
    fireEvent.pointerDown(within(article(/Bạn/)).getByText('Tin của tôi'), { pointerType: 'touch', clientX: 1, clientY: 1 })
    act(() => { vi.advanceTimersByTime(401) })
    let sheet = screen.getByRole('dialog', { name: 'Thao tác với tin' })
    for (const name of ['Sao chép', 'Ghim', 'Sửa tin', 'Xoá tin']) expect(within(sheet).getByRole('button', { name })).toBeInTheDocument()
    expect(within(sheet).queryByRole('button', { name: 'Đánh dấu chưa đọc' })).toBeNull()
    expect(within(sheet).getByRole('button', { name: 'Xoá tin' })).toHaveClass('text-danger')
    unmount()

    renderWithClient(stream([theirMsg()], { lifecycle: lifecycleStub() }))
    fireEvent.pointerDown(within(article(/Nguyễn Thu Lan/)).getByText('Tin của Lan'), { pointerType: 'touch', clientX: 1, clientY: 1 })
    act(() => { vi.advanceTimersByTime(401) })
    sheet = screen.getByRole('dialog', { name: 'Thao tác với tin' })
    expect(within(sheet).getByRole('button', { name: 'Đánh dấu chưa đọc' })).toBeInTheDocument()
    expect(within(sheet).queryByRole('button', { name: /Sửa tin|Xoá tin/ })).toBeNull()
  })

  it('runs a sheet action once and closes the sheet', () => {
    fakeAllTimers()
    const askDelete = vi.fn()
    renderWithClient(stream([mineMsg()], { lifecycle: lifecycleStub({ askDelete }) }))
    fireEvent.pointerDown(within(article(/Bạn/)).getByText('Tin của tôi'), { pointerType: 'touch', clientX: 1, clientY: 1 })
    act(() => { vi.advanceTimersByTime(401) })
    fireEvent.click(within(screen.getByRole('dialog', { name: 'Thao tác với tin' })).getByRole('button', { name: 'Xoá tin' }))
    expect(askDelete).toHaveBeenCalledTimes(1)
    // The sheet closes after its exit animation. Run its timers, then allow the
    // wall clock enough room for a busy machine (the default second is not).
    act(() => { vi.advanceTimersByTime(1000) })
    return vi.waitFor(() => expect(screen.queryByRole('dialog', { name: 'Thao tác với tin' })).toBeNull(), { timeout: 5000 })
  })

  it('edits in place on a wide screen: the bubble becomes the editor', () => {
    const m = mineMsg({ content: '<p>Tin của tôi</p>', content_format: 'html' })
    renderWithClient(stream([m], { lifecycle: lifecycleStub({ editing: { message: m } }) }))
    const a = article(/Bạn/)
    expect(within(a).getByRole('textbox', { name: 'Sửa tin' })).toHaveTextContent('Tin của tôi')
    expect(within(a).getByText('Esc huỷ · Enter lưu')).toBeInTheDocument()
  })

  it('leaves editing to the composer on a phone', () => {
    const original = window.matchMedia
    window.matchMedia = ((q: string) => ({
      matches: q.includes('max-width: 767'), media: q, addEventListener: vi.fn(), removeEventListener: vi.fn(),
    })) as unknown as typeof window.matchMedia
    try {
      const m = mineMsg()
      renderWithClient(stream([m], { lifecycle: lifecycleStub({ editing: { message: m } }) }))
      expect(screen.queryByRole('textbox', { name: 'Sửa tin' })).toBeNull()
      expect(within(article(/Bạn/)).getByText('Tin của tôi')).toBeInTheDocument()
    } finally {
      window.matchMedia = original
    }
  })

  it('asks before deleting, in an alert dialog that names a colleague’s message', () => {
    renderWithClient(stream([theirMsg()], { lifecycle: lifecycleStub({ deleting: theirMsg() }, true) }))
    const dialog = screen.getByRole('alertdialog', { name: 'Xoá tin nhắn?' })
    expect(dialog).toHaveTextContent('Tin của Nguyễn Thu Lan')
    expectNoIds()
  })

  it('offers none of it without lifecycle actions', async () => {
    const user = userEvent.setup()
    renderWithClient(stream([mineMsg()]))
    expect(screen.queryByRole('button', { name: 'Sửa tin' })).toBeNull()
    await user.click(within(article(/Bạn/)).getByRole('button', { name: 'Thêm' }))
    expect(within(await screen.findByRole('menu')).queryByRole('menuitem', { name: /Xoá tin|Sửa tin|chưa đọc/ })).toBeNull()
  })
})
