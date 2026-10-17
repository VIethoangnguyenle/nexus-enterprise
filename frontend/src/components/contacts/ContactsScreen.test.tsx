import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CH, CONTACTS, DMS, ME, U, WS_ID } from '../../test/chat-fixtures'
import { addRecent, readRecent } from '../../lib/recent-searches'
import { expectNoIds } from '../../test/no-ids'
import { apiFetch, ApiError } from '../../api/client'
import { savePreferences, switchPreferencesTo, DEFAULT_PREFERENCES } from '../../lib/preferences'
import { useToastStore } from '../primitives'
import { useWebSocketStore } from '../../stores/websocket.store'
import { ContactsScreen } from './ContactsScreen'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

const DM_ID = '33333333-aaaa-4bbb-8ccc-0000000000ff'
const calls: { method: string; path: string; body?: unknown }[] = []
let contactsResult: () => Promise<unknown>
let workspaceType: 'personal' | undefined
let activity: unknown[]
/** The direct message `channelId` last said something `hoursAgo` hours ago. */
const said = (channelId: string, hoursAgo: number) => ({
  channel_id: channelId,
  latest: {
    id: `66666666-aaaa-4bbb-8ccc-00000000000${hoursAgo}`, sender_id: U.vinh.id, sender_name: 'vinh', preview: 'Chào chị', kind: 'text',
    created_at: new Date(Date.now() - hoursAgo * 3_600_000).toISOString(),
  },
})

function fixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, ...(body ? { body } : {}) })
  if (path === '/workspaces') return Promise.resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành', ...(workspaceType ? { type: workspaceType } : {}) }] })
  if (path.startsWith(`/workspaces/${WS_ID}/contacts`)) return contactsResult()
  if (path === '/dms' && method === 'POST') return Promise.resolve({ id: DM_ID, channel_type: 'dm', name: 'dm' })
  // What "Nhắn gần đây" is made of: the direct messages and when each last said something.
  if (path === '/dms') return Promise.resolve({ channels: structuredClone(DMS) })
  if (path === `/workspaces/${WS_ID}/channels`) return Promise.resolve({ channels: [] })
  if (path === '/channels/unread') return Promise.resolve({ channels: [] })
  if (path === '/channels/activity') return Promise.resolve({ channels: structuredClone(activity) })
  return Promise.reject(new Error(`unexpected ${method} ${path}`))
}

async function renderContacts() {
  const root = createRootRoute()
  const contacts = createRoute({ getParentRoute: () => root, path: '/contacts', component: ContactsScreen })
  const channel = createRoute({ getParentRoute: () => root, path: '/channels/$channelId', component: () => <p>Cuộc trò chuyện</p> })
  const router = createRouter({
    routeTree: root.addChildren([contacts, channel]),
    history: createMemoryHistory({ initialEntries: ['/contacts'] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  calls.length = 0
  workspaceType = undefined
  activity = []
  localStorage.clear()
  switchPreferencesTo(undefined)
  useWebSocketStore.setState({ onlineUsers: {} })
  contactsResult = () => Promise.resolve({ contacts: structuredClone(CONTACTS), total: CONTACTS.length })
  api.mockImplementation(fixtureApi)
})
afterEach(() => {
  cleanup() // unmount first, so nothing mounted refetches into the cleared cache
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
})

const table = () => screen.getByRole('table', { name: /Người trong/ })
const names = () => within(table()).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)

describe('ContactsScreen: the directory', () => {
  it('lists everyone by name with title, department and email, and never an id', async () => {
    await renderContacts()
    expect(await screen.findByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    await within(table()).findByText('Trần Minh Đức')

    expect(names()).toEqual(['Lê Quang Vinh', 'Lê Thị Hoa', 'Nguyễn Thu Lan', 'Phạm Hải Yến', 'Trần Bảo Ngọc', 'Trần Minh Đức'])
    expect(screen.getByText('6 người trong Khối Vận hành')).toBeInTheDocument()
    expect(within(table()).getAllByText('Vận hành thanh toán').length).toBeGreaterThan(0)
    expect(within(table()).getByText('duc@novapay.vn')).toBeInTheDocument()
    expectNoIds()
  })

  it('shows a person with no display name as an unknown member, never by their username', async () => {
    contactsResult = () => Promise.resolve({
      contacts: [{ ...structuredClone(CONTACTS[1]!), display_name: '', username: 'handle_yen' }, structuredClone(CONTACTS[2]!)], total: 2,
    })
    await renderContacts()
    await within(table()).findByText('Nguyễn Thu Lan')
    expect(within(table()).getByText('Thành viên')).toBeInTheDocument()
    expect(document.body.textContent).not.toContain('handle_yen')
  })

  it('follows the directory\'s pages, so nobody beyond the first page is missing', async () => {
    const asked: string[] = []
    api.mockImplementation((path, init) => {
      if (path.startsWith(`/workspaces/${WS_ID}/contacts`)) {
        asked.push(path)
        return Promise.resolve(path.includes('cursor=p2')
          ? { contacts: structuredClone(CONTACTS.slice(3)), total: 6 }
          : { contacts: structuredClone(CONTACTS.slice(0, 3)), total: 6, next_cursor: 'p2' })
      }
      return fixtureApi(path, init)
    })
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    expect(names()).toHaveLength(6)
    expect(asked).toHaveLength(2)
    expect(screen.getByText('6 người trong Khối Vận hành')).toBeInTheDocument()
  })

  it('says whether each person is online, in words, from the live presence', async () => {
    useWebSocketStore.setState({ onlineUsers: { [U.lan.id]: 'lan' } })
    await renderContacts()
    await within(table()).findByText('Nguyễn Thu Lan')

    const row = (name: string) => within(table()).getByText(name).closest('[role="row"]') as HTMLElement
    expect(within(row('Nguyễn Thu Lan')).getByText('Trực tuyến')).toBeInTheDocument()
    expect(within(row('Lê Quang Vinh')).getByText('Ngoại tuyến')).toBeInTheDocument()
    // The signed-in person is online by definition.
    expect(within(row('Lê Thị Hoa')).getByText('Trực tuyến')).toBeInTheDocument()
  })

  it('finds people by name, title or email, ignoring accents', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')

    await user.type(screen.getByRole('combobox', { name: 'Tên, chức danh hoặc email' }), 'duc')
    await waitFor(() => expect(names()).toEqual(['Trần Minh Đức']))
  })

  it('filters by department from the chip, and clears it again', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')

    await user.click(screen.getByRole('button', { name: 'Phòng ban' }))
    const list = await screen.findByRole('listbox', { name: 'Phòng ban' })
    expect(within(list).getAllByRole('option').map((o) => o.textContent)).toEqual([
      'Tất cả phòng ban', 'Đối soát2', 'Kiểm soát1', 'Vận hành thanh toán2',
    ])
    await user.click(within(list).getByRole('option', { name: /Đối soát/ }))
    await waitFor(() => expect(names()).toEqual(['Lê Quang Vinh', 'Nguyễn Thu Lan']))
    expect(screen.getByRole('button', { name: 'Phòng ban: Đối soát' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Phòng ban: Đối soát' }))
    await user.click(await screen.findByRole('option', { name: 'Tất cả phòng ban' }))
    await waitFor(() => expect(names()).toHaveLength(6))
  })

  it('shows only who is online when asked', async () => {
    const user = userEvent.setup()
    useWebSocketStore.setState({ onlineUsers: { [U.yen.id]: 'yen' } })
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')

    await user.click(screen.getByRole('button', { name: 'Đang trực tuyến' }))
    await waitFor(() => expect(names()).toEqual(['Lê Thị Hoa', 'Phạm Hải Yến']))
  })

  it('says nobody matches, and offers to clear the filter', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')

    await user.type(screen.getByRole('combobox', { name: 'Tên, chức danh hoặc email' }), 'Hưng')
    expect(await screen.findByText(/Không có ai khớp “Hưng”/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Xoá bộ lọc' }))
    await waitFor(() => expect(names()).toHaveLength(6))
  })

  it('has no call button: nothing here can place a call', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(table()).findByRole('button', { name: 'Nguyễn Thu Lan' }))
    await screen.findByRole('complementary', { name: 'Hồ sơ' })
    expect(screen.queryByRole('button', { name: /gọi/i })).toBeNull()
    expect(screen.queryByText(/^Gọi/)).toBeNull()
  })
})

describe('ContactsScreen: suggestions in the empty search box', () => {
  const searchBox = () => screen.getByRole('combobox', { name: 'Tên, chức danh hoặc email' })

  it('offers recent searches and the people one messaged lately, latest first, and never an id', async () => {
    const user = userEvent.setup()
    addRecent('contacts', ME.id, 'kế toán')
    activity = [said(CH.dmLan, 3), said(CH.dmYen, 1)]
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    await user.click(searchBox())
    const list = await screen.findByRole('listbox', { name: 'Gợi ý tìm kiếm' })
    expect(within(within(list).getByRole('group', { name: 'Tìm gần đây' })).getByRole('option', { name: 'kế toán' })).toBeInTheDocument()
    const messaged = await within(list).findByRole('group', { name: 'Nhắn gần đây' })
    expect(within(messaged).getAllByRole('option').map((o) => o.querySelector('.font-medium')?.textContent))
      .toEqual(['Phạm Hải Yến', 'Nguyễn Thu Lan'])
    expectNoIds()
  })

  it('runs a recent search again', async () => {
    const user = userEvent.setup()
    addRecent('contacts', ME.id, 'Thu Lan')
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    await user.click(searchBox())
    await user.click(await screen.findByRole('option', { name: 'Thu Lan' }))
    expect(searchBox()).toHaveValue('Thu Lan')
    await waitFor(() => expect(names()).toEqual(['Nguyễn Thu Lan']))
  })

  it('opens the profile of a suggested person', async () => {
    const user = userEvent.setup()
    activity = [said(CH.dmYen, 1)]
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    await user.click(searchBox())
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    const panel = await screen.findByRole('complementary', { name: 'Hồ sơ' })
    expect(within(panel).getByText('Phạm Hải Yến')).toBeInTheDocument()
  })

  it('keeps a search when Enter is pressed or a profile is opened from the results', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    await user.type(searchBox(), 'vinh{Enter}')
    expect(readRecent('contacts', ME.id)).toEqual(['vinh'])
    await user.clear(searchBox())
    await user.type(searchBox(), 'Thu Lan')
    await user.click(await within(table()).findByRole('button', { name: 'Nguyễn Thu Lan' }))
    expect(readRecent('contacts', ME.id)).toEqual(['Thu Lan', 'vinh'])
  })

  it('asks nothing about direct messages until the box is focused', async () => {
    const user = userEvent.setup()
    activity = [said(CH.dmYen, 1)]
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    const chat = () => calls.filter((c) => c.method === 'GET' && ['/dms', '/channels/activity', '/channels/unread', `/workspaces/${WS_ID}/channels`].includes(c.path))
    expect(chat()).toEqual([])
    await user.click(searchBox())
    expect(await screen.findByRole('option', { name: /Phạm Hải Yến/ })).toBeInTheDocument()
    expect(chat().map((c) => c.path).sort()).toEqual(['/channels/activity', '/dms'])
  })

  it('does not keep a search on the Enter that ends a word being composed', async () => {
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    const box = searchBox()
    box.focus()
    fireEvent.change(box, { target: { value: 'Đức' } })
    fireEvent.keyDown(box, { key: 'Enter', isComposing: true })
    expect(readRecent('contacts', ME.id)).toEqual([])
    fireEvent.keyDown(box, { key: 'Enter' })
    expect(readRecent('contacts', ME.id)).toEqual(['Đức'])
  })

  it('on a phone, covers the list without losing it, hides the filters, and closes on Esc', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    addRecent('contacts', ME.id, 'kế toán')
    await renderContacts()
    await user.click(await screen.findByRole('searchbox', { name: 'Tên, chức danh hoặc email' }))
    expect(screen.getByRole('button', { name: 'Tìm lại kế toán' })).toBeInTheDocument()
    // The list stays where it was under the suggestions, so going back lands at the same scroll.
    expect(document.querySelector('[role="table"]')).not.toBeNull()
    expect(screen.queryByRole('table')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Đang trực tuyến' })).toBeNull()
    await user.keyboard('{Escape}')
    expect(screen.queryByRole('button', { name: 'Tìm lại kế toán' })).toBeNull()
    expect(screen.getByRole('button', { name: 'Đang trực tuyến' })).toBeInTheDocument()
    vi.unstubAllGlobals()
  })

  it('on a phone, covers the list until Quay lại, and the box stays a plain search box', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: query.includes('max-width'), media: query, addEventListener: () => {}, removeEventListener: () => {},
    }))
    addRecent('contacts', ME.id, 'kế toán')
    await renderContacts()
    const box = await screen.findByRole('searchbox', { name: 'Tên, chức danh hoặc email' })
    await user.click(box)
    expect(screen.getByRole('button', { name: 'Tìm lại kế toán' })).toBeInTheDocument()
    expect(screen.queryByRole('table')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Quay lại danh sách' }))
    expect(screen.queryByRole('button', { name: 'Tìm lại kế toán' })).toBeNull()
    expect(await screen.findByRole('table')).toBeInTheDocument()
    vi.unstubAllGlobals()
  })
})

describe('ContactsScreen: states', () => {
  it('shows skeleton rows while loading', async () => {
    contactsResult = () => new Promise(() => {})
    await renderContacts()
    await waitFor(() => expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true'))
    expect(screen.getByText('Đang tải…')).toBeInTheDocument()
  })

  it('says it could not load, and retries', async () => {
    const user = userEvent.setup()
    contactsResult = () => Promise.reject(new ApiError('boom', 500))
    await renderContacts()
    expect(await screen.findByText(/Không tải được danh bạ/)).toBeInTheDocument()

    contactsResult = () => Promise.resolve({ contacts: structuredClone(CONTACTS), total: CONTACTS.length })
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await within(await screen.findByRole('table')).findByText('Trần Minh Đức')
  })

  it('says an empty workspace is empty', async () => {
    contactsResult = () => Promise.resolve({ contacts: [], total: 0 })
    await renderContacts()
    expect(await screen.findByText(/chưa có ai khác/)).toBeInTheDocument()
  })
})

describe('ContactsScreen: a person\'s profile', () => {
  it('opens beside the list with the details that exist, colleagues, and no identifier', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(table()).findByRole('button', { name: 'Nguyễn Thu Lan' }))

    const panel = await screen.findByRole('complementary', { name: 'Hồ sơ' })
    expect(within(panel).getByRole('heading', { name: 'Nguyễn Thu Lan' })).toBeInTheDocument()
    expect(within(panel).getByText('Chuyên viên · Đối soát')).toBeInTheDocument()
    expect(within(panel).getByText('lan@novapay.vn')).toBeInTheDocument()
    expect(within(panel).queryByText('Nơi làm việc')).toBeNull() // empty details are left out
    expect(within(panel).getByText('Lê Quang Vinh')).toBeInTheDocument() // same department
    expect(within(panel).getByRole('link', { name: 'Gửi email' })).toHaveAttribute('href', 'mailto:lan@novapay.vn')
    expect(panel.textContent).not.toContain('@lan') // no username handle
    expectNoIds()
  })

  it('starts a direct message and goes to it', async () => {
    const user = userEvent.setup()
    const router = await renderContacts()
    await user.click(await within(table()).findByRole('button', { name: 'Nguyễn Thu Lan' }))
    await user.click(await screen.findByRole('button', { name: 'Nhắn tin' }))

    await waitFor(() => expect(router.state.location.pathname).toBe(`/channels/${DM_ID}`))
    expect(calls.find((c) => c.path === '/dms' && c.method === 'POST')?.body).toEqual({
      target_user_id: U.lan.id, target_ngac_node_id: U.lan.node,
    })
  })

  it('tells why a message could not start, and stays', async () => {
    const user = userEvent.setup()
    api.mockImplementation((path, init) =>
      path === '/dms' ? Promise.reject(new ApiError('nope', 403)) : fixtureApi(path, init))
    const router = await renderContacts()
    await user.click(await within(table()).findByRole('button', { name: 'Nguyễn Thu Lan' }))
    await user.click(await screen.findByRole('button', { name: 'Nhắn tin' }))

    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => /Bạn chưa có quyền/.test(t.message))).toBe(true))
    expect(router.state.location.pathname).toBe('/contacts')
  })

  it('offers no message button on one\'s own profile', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(table()).findByRole('button', { name: 'Lê Thị Hoa' }))
    const panel = await screen.findByRole('complementary', { name: 'Hồ sơ' })
    expect(within(panel).queryByRole('button', { name: 'Nhắn tin' })).toBeNull()
  })

  it('closes with Esc', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(table()).findByRole('button', { name: 'Nguyễn Thu Lan' }))
    await screen.findByRole('complementary', { name: 'Hồ sơ' })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Hồ sơ' })).toBeNull())
  })
})

describe('ContactsScreen: layout', () => {
  it('switches to cards and remembers it for this person', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')

    await user.click(screen.getByRole('button', { name: 'Thẻ' }))
    const list = await screen.findByRole('list', { name: /Người trong/ })
    expect(within(list).getAllByRole('listitem')).toHaveLength(6)
    expect(screen.queryByRole('table')).toBeNull()
    expectNoIds()

    act(() => switchPreferencesTo(undefined))
    expect(JSON.parse(localStorage.getItem('ngac-prefs:last')!).contactsView).toBe('cards')
  })

  it('starts in the layout stored for the person', async () => {
    savePreferences(undefined, { ...DEFAULT_PREFERENCES, contactsView: 'cards' })
    switchPreferencesTo(undefined)
    await renderContacts()
    expect(await screen.findByRole('list', { name: /Người trong/ })).toBeInTheDocument()
  })
})

describe('ContactsScreen: personal workspace', () => {
  it('shows no department filter, column, panel row or colleagues', async () => {
    workspaceType = 'personal'
    const user = userEvent.setup()
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')

    expect(screen.queryByRole('button', { name: /Phòng ban/ })).toBeNull()
    expect(screen.queryByRole('columnheader', { name: 'Phòng ban' })).toBeNull()
    expect(screen.queryByText('Vận hành thanh toán')).toBeNull()

    await user.click(within(table()).getByText('Lê Quang Vinh'))
    const panel = await screen.findByRole('complementary', { name: 'Hồ sơ' })
    expect(within(panel).queryByText('Phòng ban')).toBeNull()
    expect(within(panel).queryByText('Đối soát')).toBeNull()
    expect(within(panel).queryByText('Cùng phòng')).toBeNull()
  })

  it('keeps them in a company workspace', async () => {
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    expect(screen.getByRole('button', { name: 'Phòng ban' })).toBeInTheDocument()
    expect(screen.getAllByRole('columnheader').some((h) => h.textContent === 'Phòng ban')).toBe(true)
  })
})

describe('ContactsScreen: a company workspace where nobody has a department yet', () => {
  it('hides the empty department column and the filter with no options', async () => {
    contactsResult = () => {
      const contacts = structuredClone(CONTACTS).map((c) => ({ ...c, department: '' }))
      return Promise.resolve({ contacts, total: contacts.length })
    }
    await renderContacts()
    await within(table()).findByText('Trần Minh Đức')
    expect(screen.queryByRole('button', { name: /Phòng ban/ })).toBeNull()
    expect(screen.queryByRole('columnheader', { name: 'Phòng ban' })).toBeNull()
  })
})
