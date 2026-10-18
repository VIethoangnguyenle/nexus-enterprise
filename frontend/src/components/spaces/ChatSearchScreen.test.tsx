import { useState } from 'react'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
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
import { switchPreferencesTo, updatePreferences } from '../../lib/preferences'
import { useToastStore } from '../primitives'
import { isDeviceBack } from '../../lib/device-back'
import { expectNoIds } from '../../test/no-ids'
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
let activity: unknown[]
/** What a conversation last said, `hoursAgo` hours ago, as `/channels/activity` returns it. */
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

function Host({ close = onClose, unreadable }: { close?: () => void; unreadable?: string }) {
  const nav = useChatNavigation(WS_ID)
  const people = usePeople(WS_ID)
  if (nav.isLoading || people.isLoading) return null
  // A conversation the caller lost read on is missing from what the server lists for them.
  const conversations = unreadable ? nav.conversations.filter((c) => c.id !== unreadable) : nav.conversations
  return <ChatSearchScreen conversations={conversations} people={people} onClose={close} />
}

const open = async () => {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  renderWithClient(<Host />)
  const dialog = await screen.findByRole('dialog', { name: 'Tìm trong chat' })
  // The screen is ready once its box has the focus: that is also when the suggestions show.
  await waitFor(() => expect(screen.getByRole('searchbox', { name: 'Tìm trong chat' })).toHaveFocus())
  return dialog
}
const box = () => screen.getByRole('searchbox', { name: 'Tìm trong chat' })
const openOnResults = async (search: Record<string, unknown>) => {
  routerState.search = { search: 1, ...search }
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
  }))
  renderWithClient(<Host />)
  await screen.findByRole('dialog', { name: 'Tìm trong chat' })
}

beforeEach(() => {
  searchPages = [[]]
  searchCalls = []
  created = null
  activity = []
  localStorage.clear()
  useToastStore.getState().clear()
  onClose.mockClear()
  routerState.navigate.mockClear()
  routerState.search = { search: 1 }
  routerState.history = { back: routerState.history.back }
  routerState.historyState = {}
  routerState.historyState = {}
  vi.mocked(apiFetch).mockImplementation(route as never)
})
afterEach(() => {
  vi.unstubAllGlobals()
  cleanup()
  resetClient()
  switchPreferencesTo(undefined)
})

describe('the Tìm trong chat screen', () => {
  it('is a full-screen dialog with the box focused, ← where ☰ was, and no avatar to fall back on', async () => {
    const dialog = await open()
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(within(dialog).getByRole('button', { name: 'Đóng tìm kiếm' })).toBeInTheDocument()
    // The old chips are gone: the filters belong to the results.
    expect(within(dialog).queryByRole('radiogroup')).toBeNull()
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
    for (let i = 0; i < 12; i++) {
      await user.tab()
      expect(dialog).toContainElement(document.activeElement as HTMLElement)
    }
  })
})

describe('turning the pill into the search box', () => {
  it('turns ☰ out and ← in, the pill staying where it is', async () => {
    await open()
    const back = screen.getByRole('button', { name: 'Đóng tìm kiếm' }).parentElement!
    const menu = back.previousElementSibling as HTMLElement
    await waitFor(() => expect(menu.style.transform).toContain('rotate(-90deg)'))
    await waitFor(() => expect(back.style.opacity).toBe('1'))
    expect(menu).toHaveAttribute('aria-hidden', 'true')
  })

  it('under reduced motion changes at once, without turning anything', async () => {
    switchPreferencesTo('user-a')
    updatePreferences({ motion: 'reduce' })
    await open()
    const back = screen.getByRole('button', { name: 'Đóng tìm kiếm' }).parentElement!
    const menu = back.previousElementSibling as HTMLElement
    await waitFor(() => expect(back.style.opacity).toBe('1'))
    expect(menu.style.transform).not.toContain('rotate(-90deg)')
    expect(back.style.transform).not.toContain('rotate(90deg)')
  })
})

describe('reached again by the device back', () => {
  /** The layer under it: opacity in the render that mounts it (the lists are fetched by a first mount). */
  const coverOpacityAtMount = async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true, addEventListener: () => {}, removeEventListener: () => {} }))
    const first = renderWithClient(<Host />)
    await screen.findByRole('dialog', { name: 'Tìm trong chat' })
    first.unmount()
    renderWithClient(<Host />)
    return (screen.getByRole('dialog', { name: 'Tìm trong chat' }).firstElementChild as HTMLElement).style.opacity
  }

  it('fades in when the app opens it', async () => {
    routerState.historyState = { __TSR_index: 3 }
    isDeviceBack(routerState.history, 3)
    expect(await coverOpacityAtMount()).toBe('0')
  })

  it('is on screen at once when the device back returns to it', async () => {
    routerState.historyState = { __TSR_index: 3 }
    isDeviceBack(routerState.history, 3)
    routerState.historyState = { __TSR_index: 2 }
    isDeviceBack(routerState.history, 2)
    expect(await coverOpacityAtMount()).not.toBe('0')
  })
})

describe('before anything is typed', () => {
  it('lists the recent searches of this person, and a tap fills the box', async () => {
    const user = userEvent.setup()
    addRecent('chat', ME.id, 'tạm ứng tháng 10')
    addRecent('chat', ME.id, 'hợp đồng')
    addRecent('chat', 'someone-else', 'không phải của tôi')
    await open()
    expect(await screen.findByText('Tìm gần đây')).toBeInTheDocument()
    const rows = screen.getAllByRole('button', { name: /^Tìm lại / })
    expect(rows.map((r) => r.textContent)).toEqual(['hợp đồng', 'tạm ứng tháng 10'])
    expect(screen.queryByText('không phải của tôi')).toBeNull()
    await user.click(rows[1]!)
    expect(box()).toHaveValue('tạm ứng tháng 10')
    expect(box()).toHaveFocus()
  })

  it('removes one recent search without touching the others, with a 44px ✕', async () => {
    const user = userEvent.setup()
    addRecent('chat', ME.id, 'một')
    addRecent('chat', ME.id, 'hai')
    await open()
    const x = screen.getByRole('button', { name: 'Xoá “một” khỏi Tìm gần đây' })
    expect(x).toHaveClass('w-11', 'h-11')
    await user.click(x)
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Tìm lại một' })).toBeNull())
    expect(screen.getByRole('button', { name: 'Tìm lại hai' })).toBeInTheDocument()
    expect(readRecent('chat', ME.id)).toEqual(['hai'])
  })

  it('clears them all at once and brings them back with Hoàn tác', async () => {
    const user = userEvent.setup()
    addRecent('chat', ME.id, 'một')
    addRecent('chat', ME.id, 'hai')
    await open()
    await user.click(screen.getByRole('button', { name: 'Xoá tất cả' }))
    await waitFor(() => expect(screen.queryByText('Tìm gần đây')).toBeNull())
    expect(readRecent('chat', ME.id)).toEqual([])
    const [toast] = useToastStore.getState().toasts
    expect(toast).toMatchObject({ message: 'Đã xoá 2 lần tìm gần đây', action: { label: 'Hoàn tác' } })
    act(() => toast!.action!.onClick())
    expect(await screen.findByRole('button', { name: 'Tìm lại hai' })).toBeInTheDocument()
    expect(readRecent('chat', ME.id)).toEqual(['hai', 'một'])
  })

  it('says how to search when there is nothing to suggest', async () => {
    await open()
    expect(screen.queryByText('Tìm gần đây')).toBeNull()
    expect(screen.getByText('Tìm tin nhắn, người và nhóm.')).toBeInTheDocument()
    expect(screen.getByText('Sau khi tìm, lọc theo người gửi, cuộc trò chuyện hay ngày.')).toBeInTheDocument()
    expect(searchCalls).toHaveLength(0)
  })

  it('lists the conversations something was said in lately, newest first, and a tap opens one', async () => {
    const user = userEvent.setup()
    activity = [said(CH.doisoat, 3, 'Bảng đối soát tháng 10'), said(CH.hopdong, 1, 'Ok mai chốt nhé')]
    await open()
    const group = await screen.findByRole('group', { name: 'Trò chuyện gần đây' })
    const rows = within(group).getAllByRole('button')
    expect(rows.map((r) => r.querySelector('.font-medium')?.textContent)).toEqual(['Hợp đồng đối tác', 'Đối soát giao dịch'])
    expect(within(group).getByText(/Ok mai chốt nhé/)).toBeInTheDocument()
    await user.click(rows[0]!)
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.hopdong } })
    expect(document.body.textContent).not.toMatch(UUID_RE)
  })

  it('never suggests a conversation the person can no longer read', async () => {
    activity = [said(CH.doisoat, 1, 'Bảng đối soát tháng 10'), said(CH.hopdong, 2, 'Ok mai chốt nhé')]
    renderWithClient(<Host unreadable={CH.doisoat} />)
    const group = await screen.findByRole('group', { name: 'Trò chuyện gần đây' })
    expect(within(group).queryByText('Đối soát giao dịch')).toBeNull()
    expect(within(group).getByText('Hợp đồng đối tác')).toBeInTheDocument()
  })

  it('shows the suggestions as the screen appears, without waiting for the box to take the focus', async () => {
    addRecent('chat', ME.id, 'hợp đồng')
    renderWithClient(<Host />)
    await screen.findByRole('dialog', { name: 'Tìm trong chat' })
    expect(screen.getByRole('button', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
  })

  it('keeps the suggestions up when the keyboard is put away', async () => {
    addRecent('chat', ME.id, 'hợp đồng')
    await open()
    box().blur()
    expect(screen.getByRole('button', { name: 'Tìm lại hợp đồng' })).toBeInTheDocument()
  })
})

describe('words typed', () => {
  it('suggests people, groups, messages and "Xem tất cả" at once, as buttons', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1)]]
    await open()
    await user.type(box(), 'doi')
    const spaces = await screen.findByRole('group', { name: 'Nhóm và trò chuyện' })
    expect(within(spaces).getByRole('button', { name: /^Đối soát giao dịch, Nhóm · 14 thành viên/ })).toBeInTheDocument()
    expect(within(spaces).getByText('Đối', { selector: 'mark' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Xem tất cả kết quả cho “doi”' })).toBeInTheDocument()
    expectNoIds()
  })

  it('finds people by name without accents, but never the person using the app', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'tran')
    expect(await screen.findByRole('button', { name: /^Trần Minh Đức/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Trần Bảo Ngọc/ })).toBeInTheDocument()
    await user.clear(box())
    await user.type(box(), 'le thi hoa')
    await waitFor(() => expect(screen.queryByRole('button', { name: /^Lê Thị Hoa/ })).toBeNull())
  })

  it('opens the direct message that exists with a person', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'hai yen')
    await user.click(await screen.findByRole('button', { name: /^Phạm Hải Yến/ }))
    expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: CH.dmYen } })
    expect(created).toBeNull()
  })

  it('creates the direct message with a person who has none, then opens it', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'duc')
    await user.click(await screen.findByRole('button', { name: /^Trần Minh Đức/ }))
    await waitFor(() => expect(created).not.toBeNull())
    await waitFor(() =>
      expect(routerState.navigate).toHaveBeenCalledWith({ to: '/channels/$channelId', params: { channelId: created!.id } }))
  })

  it('opens a message in its conversation with the message lit, and remembers the search', async () => {
    const user = userEvent.setup()
    searchPages = [[hit(1)]]
    await open()
    await user.type(box(), 'đối soát')
    await user.click(await screen.findByRole('button', { name: /^Tin của Lê Quang Vinh trong Đối soát giao dịch: / }))
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels/$channelId', params: { channelId: CH.doisoat }, search: { message: hit(1).id },
    })
    expect(readRecent('chat', ME.id)).toEqual(['đối soát'])
  })

  it('does not ask the server for a single letter', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'đ')
    await new Promise((r) => setTimeout(r, 350))
    expect(searchCalls).toHaveLength(0)
  })
})

describe('Enter', () => {
  it('puts the words in the address, replacing the entry and keeping its mark, and puts the keyboard away', async () => {
    const user = userEvent.setup()
    routerState.historyState = { chatOverlay: true }
    await open()
    await user.type(box(), 'đối soát{Enter}')
    expect(routerState.navigate).toHaveBeenCalledWith({
      to: '/channels', search: { search: 1, q: 'đối soát' }, replace: true, state: { chatOverlay: true },
    })
    expect(box()).not.toHaveFocus()
    expect(readRecent('chat', ME.id)).toEqual(['đối soát'])
  })

  it('keeps the filters already in the address', async () => {
    const user = userEvent.setup()
    await openOnResults({ q: 'cũ', from: U.vinh.id, file: 1 })
    await user.click(box())
    await user.clear(box())
    await user.type(box(), 'mới{Enter}')
    expect(routerState.navigate).toHaveBeenLastCalledWith({
      to: '/channels', search: { search: 1, q: 'mới', from: U.vinh.id, file: 1 }, replace: true, state: undefined,
    })
  })

  it('submits from the "Xem tất cả" button too', async () => {
    const user = userEvent.setup()
    await open()
    await user.type(box(), 'tạm ứng')
    await user.click(await screen.findByRole('button', { name: 'Xem tất cả kết quả cho “tạm ứng”' }))
    expect(routerState.navigate).toHaveBeenCalledWith(expect.objectContaining({ search: { search: 1, q: 'tạm ứng' } }))
  })
})

describe('the results, in the same screen', () => {
  it('shows them under the pill when the address has words, with the box holding them', async () => {
    searchPages = [[hit(1), hit(2)]]
    await openOnResults({ q: 'đối soát' })
    expect(await screen.findByRole('region', { name: 'Kết quả tìm kiếm' })).toBeInTheDocument()
    expect(box()).toHaveValue('đối soát')
    expect(screen.getByRole('toolbar', { name: 'Bộ lọc' })).toBeInTheDocument()
    expect(await screen.findByText('2 tin nhắn')).toBeInTheDocument()
    // No suggestions cover them until the box is used again.
    expect(screen.queryByText('Tìm gần đây')).toBeNull()
    expect(screen.queryByRole('heading', { level: 1 })).toBeNull()
  })

  it('reads the filters from the address and a chip writes one back, replacing the entry', async () => {
    const user = userEvent.setup()
    await openOnResults({ q: 'đối soát', from: U.vinh.id })
    await screen.findByRole('toolbar', { name: 'Bộ lọc' })
    expect(await screen.findByRole('button', { name: 'Người gửi: Lê Quang Vinh' })).toBeInTheDocument()
    await waitFor(() => expect(searchCalls.some((c) => c.get('from') === U.vinh.id)).toBe(true))
    await user.click(screen.getByRole('button', { name: 'Có tệp' }))
    expect(routerState.navigate).toHaveBeenLastCalledWith({
      to: '/channels', search: { search: 1, q: 'đối soát', from: U.vinh.id, file: 1 }, replace: true, state: undefined,
    })
  })

  it('covers the results with the suggestions again when the box is used', async () => {
    const user = userEvent.setup()
    addRecent('chat', ME.id, 'hợp đồng')
    await openOnResults({ q: 'đối soát' })
    await screen.findByRole('region', { name: 'Kết quả tìm kiếm' })
    await user.click(box())
    expect(await screen.findByRole('button', { name: 'Xem tất cả kết quả cho “đối soát”' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Kết quả tìm kiếm' })).toBeNull()
  })
})
