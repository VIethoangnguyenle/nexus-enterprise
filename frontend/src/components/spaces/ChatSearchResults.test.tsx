import { useState } from 'react'
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, ME, U, UUID_RE, WS_ID, fixtureApi } from '../../test/chat-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { ApiError, apiFetch } from '../../api/client'
import type { Message } from '../../api/messaging'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { usePeople } from '../../hooks/usePeople'
import { isoDate, presetRange, type ChatSearchFilters } from '../../lib/chat-search'
import { switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { readRecent } from '../../lib/recent-searches'
import { ChatSearchResults } from './ChatSearchResults'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const at = (n: number) => ({ seconds: Math.floor(Date.now() / 1000) - n * 3600, nanos: 0 })
const hit = (n: number, over: Partial<Message> = {}): Message => ({
  id: `5b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a${String(n).padStart(2, '0')}`, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh',
  content: `<p>Bảng đối soát tháng ${n} em để trong Tài liệu nhé</p>`, content_format: 'html', created_at: at(n), ...over,
})

/** What `/messages/search` answers, one entry per call; the last repeats. */
let answers: Array<Message[] | 'error' | { hold: Promise<Message[]> }>
let hasMore: boolean
let calls: URLSearchParams[]
const route = (path: string, init?: RequestInit): Promise<unknown> => {
  if (path.startsWith('/messages/search')) {
    calls.push(new URLSearchParams(path.split('?')[1]))
    const a = answers[Math.min(calls.length - 1, answers.length - 1)]!
    if (a === 'error') return Promise.reject(new ApiError('internal error', 500))
    if (!Array.isArray(a)) return a.hold.then((messages) => ({ messages, has_more: false }))
    return Promise.resolve({ messages: a, has_more: hasMore })
  }
  return fixtureApi(path, init)
}

function Host({ variant = 'desktop', q = 'đối soát', initial = {}, onFilters }: {
  variant?: 'desktop' | 'phone'; q?: string; initial?: ChatSearchFilters; onFilters?: (f: ChatSearchFilters) => void
}) {
  const nav = useChatNavigation(WS_ID)
  const people = usePeople(WS_ID)
  const [filters, setFilters] = useState(initial)
  if (nav.isLoading || people.isLoading) return null
  return (
    <ChatSearchResults
      variant={variant}
      q={q}
      filters={filters}
      onFilters={(f) => {
        onFilters?.(f)
        setFilters(f)
      }}
      conversations={nav.conversations}
      people={people}
      onClose={() => {}}
    />
  )
}

const render = async (props: Parameters<typeof Host>[0] = {}) => {
  const user = userEvent.setup()
  renderWithClient(<Host {...props} />)
  await screen.findByRole('toolbar', { name: 'Bộ lọc' })
  return user
}
const lastCall = () => calls[calls.length - 1]!
const chip = (name: string | RegExp) => screen.getByRole('button', { name })

beforeEach(() => {
  answers = [[hit(1), hit(2)]]
  hasMore = false
  calls = []
  localStorage.clear()
  routerState.navigate.mockClear()
  vi.mocked(apiFetch).mockImplementation(route as never)
})
afterEach(() => {
  cleanup()
  resetClient()
  switchPreferencesTo(undefined)
})

describe('the page', () => {
  it('has a header, the six chips and the messages that match, newest first, with their number', async () => {
    await render()
    expect(screen.getByRole('heading', { level: 1, name: 'Kết quả tìm kiếm' })).toBeInTheDocument()
    expect(screen.getByRole('region', { name: 'Kết quả tìm kiếm' })).toBeInTheDocument()
    const bar = screen.getByRole('toolbar', { name: 'Bộ lọc' })
    expect(within(bar).getAllByRole('button').map((b) => b.textContent)).toEqual([
      'Người gửi', 'Trong', 'Ngày', 'Có tệp', 'Có liên kết', 'Nhắc đến tôi',
    ])
    await screen.findByText('2 tin nhắn')
    expect(screen.queryByRole('button', { name: 'Xoá bộ lọc' })).toBeNull()
    expect(lastCall().get('q')).toBe('đối soát')
    expect(lastCall().get('limit')).toBe('25')
    // Nothing but the words is sent while no filter is on.
    expect([...lastCall().keys()].sort()).toEqual(['limit', 'q'])
  })

  it('shows four skeleton cards while it looks', async () => {
    let release!: (m: Message[]) => void
    answers = [{ hold: new Promise<Message[]>((r) => { release = r }) }]
    await render()
    const status = await screen.findByRole('status', { name: 'Đang tìm tin nhắn' })
    expect(status.children).toHaveLength(4)
    expect(screen.getByRole('region', { name: 'Kết quả tìm kiếm' })).toHaveAttribute('aria-busy', 'true')
    release([hit(1)])
    await screen.findByText('1 tin nhắn')
    expect(screen.queryByRole('status', { name: 'Đang tìm tin nhắn' })).toBeNull()
  })

  it('says "Hơn 25 tin nhắn" while there is another page', async () => {
    answers = [Array.from({ length: 25 }, (_, i) => hit(i + 1))]
    hasMore = true
    await render()
    expect(await screen.findByText('Hơn 25 tin nhắn')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Tải thêm' })).toBeInTheDocument()
  })

  it('writes each hit with where it was said, who said it, when, and the words found marked', async () => {
    answers = [[hit(1, { attachments: [{ drive_file_id: 'f-1', file_name: 'Bảng đối soát T10.xlsx', mime_type: 'x', size_bytes: 1 }], content: '<p>Xem https://docs.example.com/bang-doi-soat/abc123 nhé, đối soát lệch</p>' })]]
    await render()
    const link = await screen.findByRole('link', { name: /Lê Quang Vinh/ })
    expect(link).toHaveAttribute('href', `/channels/${CH.doisoat}`)
    expect(JSON.parse(link.getAttribute('data-search')!)).toEqual({ message: hit(1).id })
    expect(within(link).getByText('Đối soát giao dịch')).toBeInTheDocument()
    expect(within(link).getAllByText(/đối soát/i, { selector: 'mark' })).toHaveLength(1)
    expect(within(link).getByText('Bảng đối soát T10.xlsx')).toBeInTheDocument()
    // A link in the message is also shown as the site it leads to.
    expect(within(link).getByText('docs.example.com')).toBeInTheDocument()
    expectNoIds()
  })

  it('remembers the search when a result is opened', async () => {
    const user = await render()
    await user.click((await screen.findAllByRole('link', { name: /Lê Quang Vinh/ }))[0]!)
    expect(readRecent('chat', ME.id)).toEqual(['đối soát'])
  })

  it('lists matching people and groups above the messages while no filter narrows the search', async () => {
    const user = await render({ q: 'duc' })
    answers = [[]]
    expect(await screen.findByRole('region', { name: 'Nhóm và người' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Trần Minh Đức/ })).toBeInTheDocument()
    await user.click(chip('Có tệp'))
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Nhóm và người' })).toBeNull())
  })
})

describe('states', () => {
  it('says nothing matched, with the words', async () => {
    answers = [[]]
    await render()
    expect(await screen.findByText('Không có tin nhắn nào khớp “đối soát”. Thử từ khác hoặc viết ngắn hơn.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Xoá bộ lọc' })).toBeNull()
  })

  it('says nothing matched these filters and offers to remove them', async () => {
    answers = [[]]
    const user = await render({ initial: { hasFile: true } })
    expect(await screen.findByText('Không có tin nhắn nào khớp “đối soát” với các bộ lọc này.')).toBeInTheDocument()
    const clear = screen.getAllByRole('button', { name: 'Xoá bộ lọc' })
    await user.click(clear[clear.length - 1]!)
    await waitFor(() => expect(lastCall().has('has_file')).toBe(false))
  })

  it('says it could not search, and Thử lại asks again', async () => {
    answers = ['error', [hit(1)]]
    const user = await render()
    expect(await screen.findByText('Chưa tìm được. Kiểm tra kết nối rồi thử lại.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await screen.findByText('1 tin nhắn')
  })

  it('asks for two letters instead of searching for one', async () => {
    await render({ q: 'đ' })
    expect(await screen.findByText('Gõ ít nhất 2 ký tự để tìm tin nhắn.')).toBeInTheDocument()
    expect(calls).toHaveLength(0)
  })

  it('keeps the old list, dimmed and busy, until the new answer is in, then swaps with a fade', async () => {
    let release!: (m: Message[]) => void
    answers = [[hit(1), hit(2)], { hold: new Promise<Message[]>((r) => { release = r }) }]
    const user = await render()
    await screen.findByText('2 tin nhắn')
    await user.click(chip('Có tệp'))
    const list = await screen.findByRole('list')
    await waitFor(() => expect(list).toHaveAttribute('aria-busy', 'true'))
    expect(list).toHaveClass('opacity-50')
    expect(within(list).getAllByRole('link')).toHaveLength(2)
    release([hit(3)])
    await waitFor(() => expect(screen.getByRole('list')).not.toHaveAttribute('aria-busy'))
    expect(within(screen.getByRole('list')).getAllByRole('link')).toHaveLength(1)
  })
})

describe('the toggle chips', () => {
  it('apply at once, show a check and aria-pressed, and send their own parameter', async () => {
    const user = await render()
    await screen.findByText('2 tin nhắn')
    expect(chip('Có tệp')).toHaveAttribute('aria-pressed', 'false')
    await user.click(chip('Có tệp'))
    expect(chip('Có tệp')).toHaveAttribute('aria-pressed', 'true')
    await waitFor(() => expect(lastCall().get('has_file')).toBe('true'))
    await user.click(chip('Có liên kết'))
    await waitFor(() => expect(lastCall().get('has_link')).toBe('true'))
    await user.click(chip('Nhắc đến tôi'))
    await waitFor(() => expect(lastCall().get('mentions_me')).toBe('true'))
    expect(lastCall().get('has_file')).toBe('true')
  })

  it('switch off again with a second press', async () => {
    const user = await render({ initial: { hasLink: true } })
    await screen.findByText('2 tin nhắn')
    await user.click(chip('Có liên kết'))
    await waitFor(() => expect(lastCall().has('has_link')).toBe(false))
  })

  it('are cleared together by Xoá bộ lọc', async () => {
    const user = await render({ initial: { hasFile: true, hasLink: true, mentionsMe: true, from: U.vinh.id } })
    await screen.findByText('2 tin nhắn')
    await user.click(screen.getByRole('button', { name: 'Xoá bộ lọc' }))
    await waitFor(() => expect([...lastCall().keys()].sort()).toEqual(['limit', 'q']))
    expect(screen.queryByRole('button', { name: 'Xoá bộ lọc' })).toBeNull()
    expect(chip('Có tệp')).toHaveAttribute('aria-pressed', 'false')
  })
})

describe('Người gửi', () => {
  it('lists me first as "Bạn (name)", then the people of the directory by name and title, never an id', async () => {
    const user = await render()
    await user.click(chip('Người gửi'))
    const dialog = await screen.findByRole('dialog', { name: 'Người gửi' })
    const rows = within(dialog).getAllByRole('radio')
    expect(rows[0]).toHaveTextContent('Bạn (Lê Thị Hoa)')
    expect(rows.map((r) => r.textContent).join(' ')).toContain('Phạm Hải Yến')
    expect(within(dialog).getByRole('searchbox', { name: 'Tìm người' })).toHaveFocus()
    expectNoIds()
  })

  it('narrows the list as the name is typed, without accents', async () => {
    const user = await render()
    await user.click(chip('Người gửi'))
    const dialog = await screen.findByRole('dialog', { name: 'Người gửi' })
    await user.type(within(dialog).getByRole('searchbox', { name: 'Tìm người' }), 'duc')
    expect(within(dialog).getAllByRole('radio').map((r) => r.textContent)).toEqual([expect.stringContaining('Trần Minh Đức')])
    await user.type(within(dialog).getByRole('searchbox'), 'zzz')
    expect(within(dialog).getByText('Không có ai tên “duczzz”.')).toBeInTheDocument()
  })

  it('applies a choice at once: the chip says the name, the request carries the sender, and ✕ takes it off', async () => {
    const user = await render()
    await user.click(chip('Người gửi'))
    await user.click(await screen.findByRole('radio', { name: /Trần Minh Đức/ }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Người gửi' })).toBeNull())
    const on = chip('Người gửi: Trần Minh Đức')
    expect(on).toHaveFocus()
    await waitFor(() => expect(lastCall().get('from')).toBe(U.duc.id))
    expect(document.body.textContent).not.toContain(U.duc.id)

    await user.click(screen.getByRole('button', { name: 'Bỏ lọc Người gửi: Trần Minh Đức' }))
    await waitFor(() => expect(chip('Người gửi')).toHaveFocus())
    expect(screen.queryByRole('button', { name: /^Bỏ lọc/ })).toBeNull()
  })

  it('offers me as the sender', async () => {
    const user = await render()
    await user.click(chip('Người gửi'))
    await user.click(await screen.findByRole('radio', { name: /Bạn \(Lê Thị Hoa\)/ }))
    await waitFor(() => expect(lastCall().get('from')).toBe(ME.id))
    expect(chip('Người gửi: Lê Thị Hoa')).toBeInTheDocument()
  })

  it('closes with Esc and gives the focus back to the chip', async () => {
    const user = await render()
    await user.click(chip('Người gửi'))
    await screen.findByRole('dialog', { name: 'Người gửi' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Người gửi' })).toBeNull())
    expect(chip('Người gửi')).toHaveFocus()
    expect(chip('Người gửi')).toHaveAttribute('aria-expanded', 'false')
  })

  it('names a sender the directory no longer knows without showing an id', async () => {
    await render({ initial: { from: '99999999-aaaa-4bbb-8ccc-000000000009' } })
    expect(chip('Người gửi: Người đã rời')).toBeInTheDocument()
    expectNoIds()
  })
})

describe('a list with the keyboard', () => {
  it('walks from the search box through the rows with ↓ and ↑, and Enter chooses', async () => {
    const user = await render()
    await user.click(chip('Người gửi'))
    const dialog = await screen.findByRole('dialog', { name: 'Người gửi' })
    const search = within(dialog).getByRole('searchbox', { name: 'Tìm người' })
    await waitFor(() => expect(search).toHaveFocus())
    const rows = within(dialog).getAllByRole('radio')
    await user.keyboard('{ArrowDown}')
    expect(rows[0]).toHaveFocus()
    await user.keyboard('{ArrowDown}{ArrowDown}')
    expect(rows[2]).toHaveFocus()
    await user.keyboard('{ArrowUp}')
    expect(rows[1]).toHaveFocus()
    await user.keyboard('{ArrowUp}{ArrowUp}')
    expect(search).toHaveFocus()
    await user.keyboard('{ArrowDown}{ArrowDown}{Enter}')
    await waitFor(() => expect(lastCall().get('from')).toBe(U.yen.id))
    expect(chip('Người gửi: Phạm Hải Yến')).toBeInTheDocument()
  })
})

describe('Trong', () => {
  it('lists the conversations the person may read by name, and a choice narrows to it', async () => {
    const user = await render()
    await user.click(chip('Trong'))
    const dialog = await screen.findByRole('dialog', { name: 'Trong' })
    expect(within(dialog).getByRole('searchbox', { name: 'Tìm cuộc trò chuyện' })).toBeInTheDocument()
    const row = within(dialog).getByRole('radio', { name: /Đối soát giao dịch/ })
    expect(row).toHaveTextContent('Nhóm · 14 thành viên')
    expect(within(dialog).getByRole('radio', { name: /Phạm Hải Yến/ })).toHaveTextContent('Trò chuyện trực tiếp')
    expectNoIds()
    await user.click(row)
    await waitFor(() => expect(lastCall().get('channel_id')).toBe(CH.doisoat))
    expect(chip('Trong: Đối soát giao dịch')).toBeInTheDocument()
  })

  it('searches the list by name', async () => {
    const user = await render()
    await user.click(chip('Trong'))
    const dialog = await screen.findByRole('dialog', { name: 'Trong' })
    await user.type(within(dialog).getByRole('searchbox'), 'van hanh')
    expect(within(dialog).getAllByRole('radio').map((r) => r.textContent)).toEqual([expect.stringContaining('Vận hành chung')])
  })

  it('names a conversation that is gone from the list without showing an id', async () => {
    await render({ initial: { channelId: '99999999-aaaa-4bbb-8ccc-000000000009' } })
    expect(chip('Trong: Cuộc trò chuyện đã đóng')).toBeInTheDocument()
    expectNoIds()
  })
})

describe('Ngày', () => {
  it('offers the presets and a custom range, and a preset sends whole days in the reader’s zone', async () => {
    const user = await render()
    await user.click(chip('Ngày'))
    const dialog = await screen.findByRole('dialog', { name: 'Ngày' })
    expect(within(dialog).getAllByRole('radio').map((r) => r.textContent)).toEqual([
      'Bất kỳ lúc nào', 'Hôm nay', '7 ngày qua', '30 ngày qua', 'Tuỳ chọn khoảng…',
    ])
    await user.click(within(dialog).getByRole('radio', { name: '7 ngày qua' }))
    const week = presetRange('week', new Date())
    await waitFor(() => expect(lastCall().get('after')).toBe(week.after))
    expect(lastCall().get('before_date')).toBe(week.beforeDate)
    expect(lastCall().get('tz')).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)
    expect(chip('Ngày: 7 ngày qua')).toBeInTheDocument()
  })

  it('applies a custom range from two days, and "Bất kỳ lúc nào" takes it off', async () => {
    const user = await render()
    await user.click(chip('Ngày'))
    const dialog = await screen.findByRole('dialog', { name: 'Ngày' })
    await user.click(within(dialog).getByRole('radio', { name: 'Tuỳ chọn khoảng…' }))
    const year = new Date().getFullYear()
    expect(within(dialog).getByRole('button', { name: 'Áp dụng' })).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText('Từ ngày'), { target: { value: `${year}-01-05` } })
    fireEvent.change(within(dialog).getByLabelText('Đến ngày'), { target: { value: `${year}-01-20` } })
    await user.click(within(dialog).getByRole('button', { name: 'Áp dụng' }))
    await waitFor(() => expect(lastCall().get('after')).toBe(`${year}-01-05`))
    expect(lastCall().get('before_date')).toBe(`${year}-01-20`)
    expect(chip('Ngày: 05/01 – 20/01')).toBeInTheDocument()
    // The answer for these days was asked for once, with the zone they are days in.
    expect(lastCall().get('tz')).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone)

    await user.click(chip('Ngày: 05/01 – 20/01'))
    await user.click(await screen.findByRole('radio', { name: 'Bất kỳ lúc nào' }))
    await waitFor(() => expect(chip('Ngày')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: /^Bỏ lọc Ngày/ })).toBeNull()
  })

  it('does not apply a range that ends before it starts', async () => {
    const user = await render()
    await user.click(chip('Ngày'))
    const dialog = await screen.findByRole('dialog', { name: 'Ngày' })
    await user.click(within(dialog).getByRole('radio', { name: 'Tuỳ chọn khoảng…' }))
    fireEvent.change(within(dialog).getByLabelText('Từ ngày'), { target: { value: isoDate(new Date()) } })
    fireEvent.change(within(dialog).getByLabelText('Đến ngày'), { target: { value: '2020-01-01' } })
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Ngày bắt đầu phải trước ngày kết thúc.')
    expect(within(dialog).getByRole('button', { name: 'Áp dụng' })).toBeDisabled()
  })
})

describe('on a phone', () => {
  it('has no header; the count sits under the chips, which scroll sideways', async () => {
    await render({ variant: 'phone' })
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull()
    await screen.findByText('2 tin nhắn')
    const bar = screen.getByRole('toolbar', { name: 'Bộ lọc' })
    expect(bar).toHaveClass('flex-nowrap')
    expect(chip('Có tệp')).toBeInTheDocument()
    // The block of people and groups is for a desktop.
    expect(screen.queryByRole('region', { name: 'Nhóm và người' })).toBeNull()
  })

  it('opens a list as a bottom sheet, a modal dialog with a search box in it', async () => {
    const user = await render({ variant: 'phone' })
    await user.click(chip('Người gửi'))
    const sheet = await screen.findByRole('dialog', { name: 'Người gửi' })
    expect(sheet).toHaveAttribute('aria-modal', 'true')
    // 24px corners and a 32px handle, as a list with a search box has them.
    expect(sheet).toHaveClass('rounded-t-3xl')
    expect(within(sheet).getByRole('heading', { name: 'Người gửi' })).toBeInTheDocument()
    await user.click(within(sheet).getByRole('radio', { name: /Phạm Hải Yến/ }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Người gửi' })).toBeNull())
    await waitFor(() => expect(lastCall().get('from')).toBe(U.yen.id))
    expect(chip('Người gửi: Phạm Hải Yến')).toBeInTheDocument()
  })
})

describe('motion', () => {
  it('lifts the first cards 8px as they fade in, one after another', async () => {
    // Hold the clock still so the animation cannot leave its first frame while the test looks.
    const frozen = performance.now()
    const clock = vi.spyOn(performance, 'now').mockReturnValue(frozen)
    try {
      await render()
      const cards = (await screen.findAllByRole('link', { name: /Lê Quang Vinh/ })).map((l) => l.parentElement!)
      expect(cards[0]!.style.transform).toContain('translateY(8px)')
    } finally {
      clock.mockRestore()
    }
  })

  it('under reduced motion only fades', async () => {
    switchPreferencesTo('user-a')
    updatePreferences({ motion: 'reduce' })
    await render()
    const cards = (await screen.findAllByRole('link', { name: /Lê Quang Vinh/ })).map((l) => l.parentElement!)
    expect(cards[0]!.style.transform).toBe('')
    expect(cards[0]!.style.opacity).not.toBe('')
  })
})

it('never puts an id on the page', async () => {
  await render({ initial: { from: U.vinh.id, channelId: CH.doisoat } })
  await screen.findByText('2 tin nhắn')
  expectNoIds()
  expect(document.body.textContent).not.toMatch(UUID_RE)
})
