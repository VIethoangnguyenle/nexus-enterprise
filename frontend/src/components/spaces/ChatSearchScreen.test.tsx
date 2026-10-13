import { useState } from 'react'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderWithClient, resetClient } from '../../test/render'
import { routerState } from '../../test/router-mock'
import { CH, ME, U, UUID_RE, WS_ID, fixtureApi } from '../../test/chat-fixtures'
import { ApiError, apiFetch } from '../../api/client'
import type { Message } from '../../api/messaging'
import { useChatNavigation } from '../../hooks/useChatNavigation'
import { usePeople } from '../../hooks/usePeople'
import { addRecent, readRecent } from '../../lib/recent-searches'
import { Pressable } from '../primitives'
import { ChatSearchScreen } from './ChatSearchScreen'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const onClose = vi.fn()
const at = (n: number) => ({ seconds: Math.floor(Date.now() / 1000) - n * 3600, nanos: 0 })
const hit = (n: number, over: Partial<Message> = {}): Message => ({
  id: `5b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a0${n}`, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh',
  content: `<p>Bảng đối soát tháng ${n} em để trong Tài liệu nhé</p>`, content_format: 'html', created_at: at(n), ...over,
})

let searchPages: Array<Message[] | 'error'>
let searchCalls: URLSearchParams[]
let created: { id: string } | null
const route = (path: string, init?: RequestInit): Promise<unknown> => {
  if (path.startsWith('/messages/search')) {
    searchCalls.push(new URLSearchParams(path.split('?')[1]))
    const page = searchPages[Math.min(searchCalls.length - 1, searchPages.length - 1)]
    if (page === 'error') return Promise.reject(new ApiError('internal error', 500))
    return Promise.resolve({ messages: page ?? [], has_more: searchCalls.length < searchPages.length })
  }
  if (path === '/dms' && init?.method === 'POST') {
    created = { id: '55555555-aaaa-4bbb-8ccc-000000000009' }
    return Promise.resolve({ id: created.id, name: 'hoa, duc', channel_type: 'dm', workspace_id: '', member_count: 2 })
  }
  return fixtureApi(path, init)
}

function Host({ close = onClose }: { close?: () => void }) {
  const nav = useChatNavigation(WS_ID)
  const people = usePeople(WS_ID)
  if (nav.isLoading || people.isLoading) return null
  return <ChatSearchScreen conversations={nav.conversations} people={people} onClose={close} />
}

const open = async () => {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  renderWithClient(<Host />)
  return screen.findByRole('dialog', { name: 'Tìm trong chat' })
}
const box = () => screen.getByRole('searchbox', { name: 'Tìm trong chat' })
const chip = (name: string) => screen.getByRole('radio', { name })

beforeEach(() => {
  searchPages = [[]]
  searchCalls = []
  created = null
  localStorage.clear()
  onClose.mockClear()
  routerState.navigate.mockClear()
  vi.mocked(apiFetch).mockImplementation(route as never)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
})

describe('the Tìm trong chat screen', () => {
  it('is a full-screen dialog with the box focused and the five chips', async () => {
    const dialog = await open()
    await waitFor(() => expect(box()).toHaveFocus())
    const chips = within(within(dialog).getByRole('radiogroup', { name: 'Lọc kết quả' })).getAllByRole('radio')
    expect(chips.map((c) => c.textContent)).toEqual(['Tất cả', 'Người', 'Nhóm', 'Tin nhắn', 'Có tệp'])
    expect(chip('Tất cả')).toBeChecked()
  })

  it('closes with the back button and with Esc', async () => {
    const user = userEvent.setup()
    await open()
    await user.click(screen.getByRole('button', { name: 'Đóng tìm kiếm' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledTimes(2)
  })

  it('clears the box and keeps the focus there', async () => {
    const user = userEvent.setup()
    await open()
    expect(screen.queryByRole('button', { name: 'Xoá' })).toBeNull()
    await user.type(box(), 'đối')
    await user.click(screen.getByRole('button', { name: 'Xoá' }))
    expect(box()).toHaveValue('')
    expect(box()).toHaveFocus()
  })

  it('hands the focus back to what opened it', async () => {
    const user = userEvent.setup()
    function WithOpener() {
      const [on, setOn] = useState(false)
      return (
        <>
          <Pressable onClick={() => setOn(true)}>Mở tìm kiếm</Pressable>
          {on && <Host close={() => setOn(false)} />}
        </>
      )
    }
    renderWithClient(<WithOpener />)
    const opener = screen.getByRole('button', { name: 'Mở tìm kiếm' })
    await user.click(opener)
    await waitFor(() => expect(box()).toHaveFocus())
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(opener).toHaveFocus()
  })

  it('keeps Tab inside the screen', async () => {
    const user = userEvent.setup()
    const dialog = await open()
    await waitFor(() => expect(box()).toHaveFocus())
    for (let i = 0; i < 12; i++) {
      await user.tab()
      expect(dialog).toContainElement(document.activeElement as HTMLElement)
    }
  })
})

describe('before anything is typed', () => {
  it('lists the recent searches of this person, and a tap fills the box', async () => {
    const user = userEvent.setup()
    addRecent(ME.id, 'tạm ứng tháng 10')
    addRecent(ME.id, 'hợp đồng')
    addRecent('someone-else', 'không phải của tôi')
    await open()
    expect(screen.getByText('Tìm gần đây')).toBeInTheDocument()
    const rows = screen.getAllByRole('button', { name: /^Tìm lại / })
    expect(rows.map((r) => r.textContent)).toEqual(['hợp đồng', 'tạm ứng tháng 10'])
    expect(screen.queryByText('không phải của tôi')).toBeNull()
    await user.click(rows[1]!)
    expect(box()).toHaveValue('tạm ứng tháng 10')
    expect(box()).toHaveFocus()
  })

  it('removes one recent search without touching the others', async () => {
    const user = userEvent.setup()
    addRecent(ME.id, 'một')
    addRecent(ME.id, 'hai')
    await open()
    await user.click(screen.getByRole('button', { name: 'Xoá “một” khỏi tìm gần đây' }))
    expect(screen.queryByRole('button', { name: 'Tìm lại một' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Tìm lại hai' })).toBeInTheDocument()
    expect(readRecent(ME.id)).toEqual(['hai'])
  })

  it('says nothing about recent searches when there are none', async () => {
    await open()
    expect(screen.queryByText('Tìm gần đây')).toBeNull()
    expect(searchCalls).toHaveLength(0)
  })
})

describe('groups and people', () => {
  it('finds a group typed without accents, from the lists already loaded', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'doi soat')
    const link = await screen.findByRole('link', { name: /Đối soát giao dịch/ })
    expect(link).toHaveAttribute('href', `/channels/${CH.doisoat}`)
    expect(within(link).getByText('Đối soát', { selector: 'mark' })).toBeInTheDocument()
    expect(within(link).getByText('Nhóm · 14 thành viên')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Hợp đồng đối tác/ })).toBeNull()
    expect(screen.getByText('Nhóm và người')).toBeInTheDocument()
  })

  it('finds people by name without accents, but never the person using the app', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'tran')
    expect(await screen.findByRole('button', { name: /Trần Minh Đức/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Trần Bảo Ngọc/ })).toBeInTheDocument()
    await user.clear(box())
    await user.type(box(), 'le thi hoa')
    await waitFor(() => expect(screen.queryByRole('button', { name: /Lê Thị Hoa/ })).toBeNull())
  })

  it('opens the direct message that exists with a person', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'hai yen')
    await user.click(await screen.findByRole('button', { name: /Phạm Hải Yến/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.dmYen } })
    expect(created).toBeNull()
  })

  it('creates the direct message with a person who has none, then opens it', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'duc')
    await user.click(await screen.findByRole('button', { name: /Trần Minh Đức/ }))
    await waitFor(() => expect(created).not.toBeNull())
    await waitFor(() =>
      expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: created!.id } }))
  })

  it('remembers the search once a result is opened', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'doi soat')
    await user.click(await screen.findByRole('link', { name: /Đối soát giao dịch/ }))
    expect(readRecent(ME.id)).toEqual(['doi soat'])
  })
})

describe('messages', () => {
  it('asks the server after a pause and shows sender, conversation, the matching words and the time', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1), hit(2, { sender_id: U.lan.id, sender_name: 'lan', channel_id: CH.dmLan, content: 'Chị ơi file đối soát ngân hàng lệch 2 dòng' })]]
    await open()
    await user.type(box(), 'đối soát')
    expect(await screen.findByRole('heading', { name: 'Tin nhắn' })).toBeInTheDocument()
    const first = await screen.findByRole('link', { name: /Lê Quang Vinh · Đối soát giao dịch/ })
    expect(within(first).getAllByText(/đối soát/i, { selector: 'mark' })).toHaveLength(1)
    expect(first).toHaveTextContent('Bảng đối soát tháng 1 em để trong Tài liệu nhé')
    expect(first).toHaveAttribute('href', `/channels/${CH.doisoat}`)
    expect(JSON.parse(first.getAttribute('data-search')!)).toEqual({ message: hit(1).id })
    // A direct message names the person only.
    const second = screen.getByRole('link', { name: /^Nguyễn Thu Lan/ })
    expect(second).not.toHaveTextContent('·')
    expect(searchCalls).toHaveLength(1)
    expect(searchCalls[0]!.get('q')).toBe('đối soát')
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('opens a reply in its thread, with the reply lit', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1, { parent_message_id: '5b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5aff' })]]
    await open()
    await user.type(box(), 'đối soát')
    const link = await screen.findByRole('link', { name: /Lê Quang Vinh/ })
    expect(JSON.parse(link.getAttribute('data-search')!)).toEqual({ thread: '5b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5aff', message: hit(1).id })
  })

  it('does not ask for a single letter, and says why', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'đ')
    await new Promise((r) => setTimeout(r, 350))
    expect(searchCalls).toHaveLength(0)
    expect(screen.getByText('Gõ ít nhất 2 ký tự để tìm tin nhắn.')).toBeInTheDocument()
  })

  it('shows a skeleton while it looks', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'đối soát')
    expect(screen.getByRole('status', { name: 'Đang tìm tin nhắn' })).toHaveAttribute('aria-busy', 'true')
    await screen.findByText(/Không tìm thấy/)
    expect(screen.queryByRole('status', { name: 'Đang tìm tin nhắn' })).toBeNull()
  })

  it('says when nothing is found, and suggests accents for a search written without them', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'khong co gi')
    expect(await screen.findByText('Không tìm thấy “khong co gi” — thử từ khác.')).toBeInTheDocument()
    expect(screen.getByText('Thử gõ có dấu.')).toBeInTheDocument()
    await user.clear(box())
    await user.type(box(), 'không có gì')
    expect(await screen.findByText('Không tìm thấy “không có gì” — thử từ khác.')).toBeInTheDocument()
    expect(screen.queryByText('Thử gõ có dấu.')).toBeNull()
  })

  it('says a search failed and tries again', async () => {
    const user = userEvent.setup()
    searchPages = ['error']
    await open()
    await user.type(box(), 'đối soát')
    expect(await screen.findByText('Chưa tìm được tin nhắn.')).toBeInTheDocument()
    searchPages = [[hit(1)]]
    searchCalls = []
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('link', { name: /Lê Quang Vinh/ })).toBeInTheDocument()
  })

  it('loads the next page on "Tải thêm"', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1)], [hit(2)]]
    await open()
    await user.type(box(), 'đối soát')
    await screen.findByRole('link', { name: /tháng 1 /i })
    await user.click(screen.getByRole('button', { name: 'Tải thêm' }))
    await waitFor(() => expect(screen.getAllByRole('link', { name: /Lê Quang Vinh/ })).toHaveLength(2))
    expect(searchCalls[1]!.get('before_id')).toBe(hit(1).id)
    expect(screen.queryByRole('button', { name: 'Tải thêm' })).toBeNull()
  })
})

describe('the chips', () => {
  it('Người shows people only and asks the server nothing', async () => {
    const user = userEvent.setup()
    await open()
    await user.click(chip('Người'))
    await user.type(box(), 'doi')
    await user.clear(box())
    await user.type(box(), 'duc')
    expect(await screen.findByRole('button', { name: /Trần Minh Đức/ })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Tin nhắn' })).toBeNull()
    await new Promise((r) => setTimeout(r, 350))
    expect(searchCalls).toHaveLength(0)
  })

  it('Nhóm shows groups only', async () => {
    const user = userEvent.setup()
    await open()
    await user.click(chip('Nhóm'))
    await user.type(box(), 'hop')
    expect(await screen.findByRole('link', { name: /Hợp đồng đối tác/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Trần/ })).toBeNull()
  })

  it('Tin nhắn shows messages only', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1)]]
    await open()
    await user.click(chip('Tin nhắn'))
    await user.type(box(), 'đối soát')
    expect(await screen.findByRole('link', { name: /Lê Quang Vinh/ })).toBeInTheDocument()
    expect(screen.queryByText('Nhóm và người')).toBeNull()
    expect(screen.queryByRole('link', { name: /^Đối soát giao dịch$/ })).toBeNull()
  })

  it('Có tệp asks for messages with a file and hides groups and people', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1)]]
    await open()
    await user.click(chip('Có tệp'))
    await user.type(box(), 'đối soát')
    expect(await screen.findByRole('link', { name: /Lê Quang Vinh/ })).toBeInTheDocument()
    expect(searchCalls[0]!.get('has_file')).toBe('true')
    expect(screen.queryByText('Nhóm và người')).toBeNull()
  })

  it('keeps the recent searches under every chip', async () => {
    const user = userEvent.setup()
    addRecent(ME.id, 'hợp đồng')
    await open()
    await user.click(chip('Người'))
    expect(screen.getByRole('button', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
  })
})
