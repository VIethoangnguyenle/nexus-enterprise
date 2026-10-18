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
import { ChatSearchBox } from './ChatSearchBox'

vi.mock('@tanstack/react-router', async () => (await import('../../test/router-mock')).routerMockFactory())
vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))

const at = (n: number) => ({ seconds: Math.floor(Date.now() / 1000) - n * 3600, nanos: 0 })
const hit = (n: number, over: Partial<Message> = {}): Message => ({
  id: `5b8f6c1e-2b3d-4e5f-8a9b-1c2d3e4f5a0${n}`, channel_id: CH.doisoat, sender_id: U.vinh.id, sender_name: 'vinh',
  content: `<p>Bảng đối soát tháng ${n} em để trong Tài liệu nhé</p>`, content_format: 'html', created_at: at(n), ...over,
})

let searchResult: Message[] | 'error'
let searchCalls: URLSearchParams[]
let activity: unknown[]
const said = (channelId: string, hoursAgo: number, preview: string) => ({
  channel_id: channelId,
  latest: {
    id: `66666666-aaaa-4bbb-8ccc-00000000000${hoursAgo}`, sender_id: U.vinh.id, sender_name: 'vinh', preview, kind: 'text',
    created_at: new Date(Date.now() - hoursAgo * 3_600_000).toISOString(),
  },
})
const route = (path: string, init?: RequestInit): Promise<unknown> => {
  if (path.split('?')[0] === '/channels/activity') return Promise.resolve({ channels: structuredClone(activity) })
  if (path.startsWith('/messages/search')) {
    searchCalls.push(new URLSearchParams(path.split('?')[1]))
    if (searchResult === 'error') return Promise.reject(new ApiError('internal error', 500))
    return Promise.resolve({ messages: searchResult, has_more: false })
  }
  return fixtureApi(path, init)
}

function Host() {
  const nav = useChatNavigation(WS_ID)
  const people = usePeople(WS_ID)
  if (nav.isLoading || people.isLoading) return null
  return (
    <>
      <Pressable>Ở ngoài</Pressable>
      <ChatSearchBox conversations={nav.conversations} people={people} />
    </>
  )
}

const box = () => screen.getByRole('combobox', { name: 'Tìm trong chat' })
const open = async () => {
  const user = userEvent.setup()
  renderWithClient(<Host />)
  await screen.findByRole('combobox', { name: 'Tìm trong chat' })
  return user
}
const option = (name: RegExp | string) => screen.getByRole('option', { name })

beforeEach(() => {
  searchResult = []
  searchCalls = []
  activity = []
  localStorage.clear()
  routerState.navigate.mockClear()
  routerState.pathname = '/channels'
  routerState.search = {}
  vi.mocked(apiFetch).mockImplementation(route as never)
})
afterEach(() => {
  cleanup()
  resetClient()
})

describe('the pill at the head of the list', () => {
  it('rests as a search box with a "/" hint, and opens a surface when it is used', async () => {
    const user = await open()
    expect(box()).toHaveAttribute('placeholder', 'Tìm trong chat')
    expect(box()).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByText('/')).toBeInTheDocument()
    await user.click(box())
    expect(box()).toHaveAttribute('aria-expanded', 'true')
    // Nothing recent and nothing said lately: the surface still opens to say how to search.
    expect(screen.getByText('Tìm tin nhắn, người và nhóm.')).toBeInTheDocument()
  })

  it('suggests Tìm gần đây and the conversations something was said in lately, and a choice opens it', async () => {
    addRecent('chat', ME.id, 'hợp đồng')
    activity = [said(CH.doisoat, 3, 'Bảng đối soát tháng 10'), said(CH.hopdong, 1, 'Ok mai chốt nhé')]
    const user = await open()
    await user.click(box())
    const recent = screen.getByRole('group', { name: 'Tìm gần đây' })
    expect(within(recent).getByRole('option', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
    const lately = await screen.findByRole('group', { name: 'Trò chuyện gần đây' })
    expect(within(lately).getAllByRole('option').map((o) => o.querySelector('.font-medium')?.textContent))
      .toEqual(['Hợp đồng đối tác', 'Đối soát giao dịch'])
    await user.click(within(lately).getAllByRole('option')[0]!)
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.hopdong } })
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('forgets a recent search from the surface', async () => {
    addRecent('chat', ME.id, 'một')
    addRecent('chat', ME.id, 'hai')
    const user = await open()
    await user.click(box())
    await user.keyboard('{ArrowDown}{Delete}')
    await waitFor(() => expect(readRecent('chat', ME.id)).toEqual(['một']))
  })

  it('puts ✕ where the "/" was once words are typed, and it clears them', async () => {
    const user = await open()
    await user.type(box(), 'lan')
    await user.click(screen.getByRole('button', { name: 'Xoá chữ đã nhập' }))
    expect(box()).toHaveValue('')
    expect(box()).toHaveFocus()
    expect(screen.queryByRole('button', { name: 'Xoá chữ đã nhập' })).toBeNull()
  })
})

describe('the / key and Esc', () => {
  it('takes the focus from anywhere in Chat, except while typing somewhere else', async () => {
    const user = await open()
    await user.click(screen.getByRole('button', { name: 'Ở ngoài' }))
    await user.keyboard('/')
    expect(box()).toHaveFocus()
    // The "/" itself is not typed into the box.
    expect(box()).toHaveValue('')
    const other = document.createElement('input')
    document.body.append(other)
    other.focus()
    await user.keyboard('/')
    expect(other).toHaveFocus()
    expect(other).toHaveValue('/')
    other.remove()
  })

  it('is not taken below 1024px, where the column is not on screen', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    const user = await open()
    await user.keyboard('/')
    expect(box()).not.toHaveFocus()
    vi.unstubAllGlobals()
  })

  it('does nothing while a dialog is open', async () => {
    const user = await open()
    const dialog = document.createElement('div')
    dialog.setAttribute('aria-modal', 'true')
    document.body.append(dialog)
    await user.keyboard('/')
    expect(box()).not.toHaveFocus()
    dialog.remove()
  })

  it('steps back with Esc: close the suggestions, clear the words, leave for where the focus was', async () => {
    const user = await open()
    const outside = screen.getByRole('button', { name: 'Ở ngoài' })
    await user.click(outside)
    await user.keyboard('/')
    await user.keyboard('lan')
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(box()).toHaveValue('lan')
    expect(box()).toHaveFocus()

    await user.keyboard('{Escape}')
    expect(box()).toHaveValue('')
    expect(box()).toHaveFocus()
    // Emptying the box by Esc does not bring the suggestions back; the next Esc leaves.
    expect(screen.queryByRole('listbox')).toBeNull()

    await user.keyboard('{Escape}')
    expect(outside).toHaveFocus()
  })

  it('leaves the box on the third Esc even when nothing had the focus before "/"', async () => {
    const user = await open()
    expect(document.body).toHaveFocus()
    await user.keyboard('/')
    expect(box()).toHaveFocus()
    // Nothing typed: the first Esc folds the suggestions, the next leaves the box.
    await user.keyboard('{Escape}')
    expect(box()).toHaveFocus()
    expect(box()).toHaveAttribute('aria-expanded', 'false')
    await user.keyboard('{Escape}')
    expect(document.activeElement?.tagName).toBe('BODY')
  })

  it('types a slash as text inside the box', async () => {
    const user = await open()
    await user.click(box())
    await user.keyboard('a/b')
    expect(box()).toHaveValue('a/b')
  })

  it('stays in the box after the last Esc when it was opened with the pointer', async () => {
    const user = await open()
    await user.click(box())
    await user.keyboard('{Escape}{Escape}')
    expect(box()).toHaveFocus()
  })
})

describe('words typed', () => {
  it('suggests people, then groups and conversations, then always "Xem tất cả kết quả"', async () => {
    const user = await open()
    await user.type(box(), 'lan')
    const people = await screen.findByRole('group', { name: 'Người' })
    // The person asking is never listed.
    expect(within(people).getAllByRole('option').map((o) => o.getAttribute('aria-label')))
      .toEqual([expect.stringMatching(/^Nguyễn Thu Lan, /)])
    expect(within(people).getByText('Lan', { selector: 'mark' })).toBeInTheDocument()
    // The direct message with someone already listed above is not repeated, so no group is left to show.
    expect(screen.queryByRole('group', { name: 'Nhóm và trò chuyện' })).toBeNull()
    const all = option('Xem tất cả kết quả cho “lan”')
    expect(all).toHaveTextContent('Enter')
    const options = screen.getAllByRole('option')
    expect(options[options.length - 1]).toBe(all)
  })

  it('finds groups written without accents', async () => {
    const user = await open()
    await user.type(box(), 'doi soat')
    const spaces = await screen.findByRole('group', { name: 'Nhóm và trò chuyện' })
    expect(within(spaces).getByRole('option', { name: 'Đối soát giao dịch, Nhóm · 14 thành viên' })).toBeInTheDocument()
  })

  it('asks the server for messages after a pause, with a skeleton meanwhile, and shows who said it where', async () => {
    searchResult = [hit(1)]
    const user = await open()
    await user.type(box(), 'đối soát')
    const group = await screen.findByRole('group', { name: 'Tin nhắn' })
    expect(group).toHaveAttribute('aria-busy', 'true')
    const row = await within(group).findByRole('option', { name: /^Tin của Lê Quang Vinh trong Đối soát giao dịch: / })
    expect(row).toHaveTextContent('Lê Quang Vinh · Đối soát giao dịch')
    expect(searchCalls).toHaveLength(1)
    expect(searchCalls[0]!.get('q')).toBe('đối soát')
    expect(searchCalls[0]!.get('limit')).toBe('3')
    await user.click(row)
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.doisoat }, search: { message: hit(1).id },
    })
    expect(readRecent('chat', ME.id)).toEqual(['đối soát'])
  })

  it('leaves the messages out when the server fails, without saying so among the suggestions', async () => {
    searchResult = 'error'
    const user = await open()
    await user.type(box(), 'đối soát')
    await waitFor(() => expect(searchCalls.length).toBeGreaterThan(0))
    await waitFor(() => expect(screen.queryByRole('group', { name: 'Tin nhắn' })).toBeNull())
    expect(screen.getByRole('group', { name: 'Nhóm và trò chuyện' })).toBeInTheDocument()
    expect(screen.queryByText(/Chưa tìm được/)).toBeNull()
  })

  it('does not ask for one letter', async () => {
    const user = await open()
    await user.type(box(), 'đ')
    await new Promise((r) => setTimeout(r, 350))
    expect(searchCalls).toHaveLength(0)
    expect(screen.queryByRole('group', { name: 'Tin nhắn' })).toBeNull()
    expect(option('Xem tất cả kết quả cho “đ”')).toBeInTheDocument()
  })

  it('offers only "Xem tất cả" and how to search when nothing matches', async () => {
    const user = await open()
    await user.type(box(), 'zzzzqq')
    await waitFor(() => expect(screen.getByText('Không có gợi ý. Nhấn Enter để tìm trong mọi tin nhắn.')).toBeInTheDocument())
    expect(screen.getAllByRole('option')).toHaveLength(1)
  })

  it('opens the direct message with a person, creating nothing when it exists', async () => {
    const user = await open()
    await user.type(box(), 'hai yen')
    await user.click(await screen.findByRole('option', { name: /^Phạm Hải Yến, / }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.dmYen } })
    expect(readRecent('chat', ME.id)).toEqual(['hai yen'])
  })
})

describe('Enter', () => {
  it('opens the results with the words in the address, keeps the words in the box and folds the surface', async () => {
    const user = await open()
    await user.type(box(), 'đối soát')
    await user.keyboard('{Enter}')
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/search', search: { q: 'đối soát' }, replace: false })
    expect(box()).toHaveValue('đối soát')
    expect(screen.queryByRole('listbox')).toBeNull()
    expect(readRecent('chat', ME.id)).toEqual(['đối soát'])
  })

  it('does the same from the last row, marked with the arrow keys', async () => {
    const user = await open()
    await user.type(box(), 'lan')
    await user.keyboard('{ArrowUp}{Enter}')
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/search', search: { q: 'lan' }, replace: false })
  })

  it('chooses the marked row instead of searching', async () => {
    const user = await open()
    await user.type(box(), 'doi soat')
    await screen.findByRole('group', { name: 'Nhóm và trò chuyện' })
    await user.keyboard('{ArrowDown}{Enter}')
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.doisoat } })
  })

  it('does not search when the Enter only ends a word being composed', async () => {
    const user = await open()
    await user.type(box(), 'đối')
    box().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', isComposing: true, bubbles: true }))
    expect(routerState.navigate).not.toHaveBeenCalled()
    void user
  })

  it('refines the words on the results page without losing the filters, replacing the entry', async () => {
    routerState.pathname = '/channels/search'
    routerState.search = { q: 'đối soát', from: U.vinh.id, file: 1 }
    const user = await open()
    expect(box()).toHaveValue('đối soát')
    await user.clear(box())
    await user.type(box(), 'tạm ứng{Enter}')
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/search', search: { q: 'tạm ứng', from: U.vinh.id, file: 1 }, replace: true,
    })
  })
})
