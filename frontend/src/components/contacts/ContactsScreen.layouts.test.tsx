import '@testing-library/jest-dom/vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { CH, ME, U } from '../../test/chat-fixtures'
import {
  CALL_ID, D, DM_ID, PEOPLE, directory, fixtureApi, renderContacts, resetServer, server,
} from '../../test/contacts-fixtures'
import { addRecent } from '../../lib/recent-searches'
import { clearResolvers } from '../../lib/people-resolver'
import { expectNoIds } from '../../test/no-ids'
import { apiFetch } from '../../api/client'
import { switchPreferencesTo } from '../../lib/preferences'
import { useToastStore } from '../primitives'
import { useWebSocketStore } from '../../stores/websocket.store'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

/** A viewport of `width` px: the media queries the app asks (`max-width`) answer for it. */
function stubViewport(width: number) {
  vi.stubGlobal('matchMedia', (query: string) => {
    const max = /max-width:\s*([\d.]+)px/.exec(query)
    return {
      matches: max ? width <= Number(max[1]) : false, media: query,
      addEventListener: () => {}, removeEventListener: () => {}, addListener: () => {}, removeListener: () => {},
    }
  })
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetServer()
  localStorage.clear()
  switchPreferencesTo(undefined)
  clearResolvers()
  useWebSocketStore.setState({ onlineUsers: {} })
  api.mockImplementation(fixtureApi)
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  resetClient()
  useToastStore.getState().clear()
})

const asked = (re: RegExp) => server.calls.filter((c) => c.method === 'GET' && re.test(c.path)).map((c) => c.path)
const departmentCalls = () => asked(/\/departments$/)
const channelCalls = () => asked(/\/channels$|\/members$/)
const chips = () => screen.getByRole('group', { name: 'Danh mục' })
const tiles = () => screen.getByRole('navigation', { name: 'Danh mục' })
const layer = (name: string | RegExp) => screen.findByRole('dialog', { name })
const textOf = (el: Element) => {
  const copy = el.cloneNode(true) as Element
  copy.querySelectorAll('[aria-hidden="true"]').forEach((n) => n.remove())
  return (copy.textContent ?? '').replace(/\s+/g, ' ').trim()
}

describe('Danh bạ on a tablet (768 to 1023)', () => {
  beforeEach(() => stubViewport(820))

  it('has the title and one chip per category instead of the list panel, in the same order', async () => {
    await renderContacts()
    expect(await screen.findByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Danh mục' })).toBeNull()
    await within(chips()).findByRole('button', { name: 'Phòng của tôi' })
    expect(within(chips()).getAllByRole('button').map((b) => b.textContent)).toEqual(['Phòng ban', 'Vai trò', 'Nhóm của tôi', 'Phòng của tôi'])
    expect(within(chips()).getByRole('button', { name: 'Phòng ban' })).toHaveAttribute('aria-pressed', 'true')
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
  })

  it('Phòng của tôi, the last chip, opens the caller\'s department', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(chips()).findByRole('button', { name: 'Phòng của tôi' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Kế toán' })).toBeInTheDocument()
    expect(within(chips()).getByRole('button', { name: 'Phòng của tôi' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('a personal workspace has Thành viên and Nhóm của tôi', async () => {
    server.workspaceType = 'personal'
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Thành viên' })
    expect(within(chips()).getAllByRole('button').map((b) => b.textContent)).toEqual(['Thành viên', 'Nhóm của tôi'])
  })

  it('opens a category on a press and asks for its data only then, as on a desktop', async () => {
    const user = userEvent.setup()
    await renderContacts('/contacts?cat=roles')
    await screen.findByRole('heading', { level: 2, name: 'Vai trò' })
    expect(departmentCalls()).toEqual([])
    expect(channelCalls()).toEqual([])
    await user.click(within(chips()).getByRole('button', { name: 'Phòng ban' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
    expect(departmentCalls()).toHaveLength(1)
    expect(channelCalls()).toEqual([])
  })

  it('shows the profile as a right sheet on a scrim: modal, named, Esc closes it and focus goes back to the row', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const row = await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })
    await user.click(row)
    const sheet = await layer('Hồ sơ Nguyễn Thu Lan')
    expect(sheet).toHaveAttribute('aria-modal', 'true')
    expect(within(sheet).getByRole('button', { name: 'Nhắn tin' })).toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: /Hồ sơ/ })).toBeNull()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(row).toHaveFocus()
  })

  it('closes the sheet from the scrim and from the close button', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    await layer(/Hồ sơ/)
    await user.click(document.querySelector('.bg-scrim') as HTMLElement)
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(router.state.location.search).not.toHaveProperty('person')

    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    await user.click(within(await layer(/Hồ sơ/)).getByRole('button', { name: 'Đóng hồ sơ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('opens the profile of a link with ?person=', async () => {
    await renderContacts(`/contacts?person=${U.lan.id}`)
    expect(await layer('Hồ sơ Nguyễn Thu Lan')).toBeInTheDocument()
  })

  it('says a failed department tree in the column, with Thử lại, as there is no list panel to say it under', async () => {
    const user = userEvent.setup()
    const { ApiError } = await import('../../api/client')
    server.departments = () => Promise.reject(new ApiError('boom', 500))
    await renderContacts()
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm.')
    server.departments = () => Promise.resolve({ departments: [] })
    await user.click(within(alert).getByRole('button', { name: 'Thử lại' }))
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())
  })
})

describe('Danh bạ on a phone (below 768): the stack', () => {
  beforeEach(() => stubViewport(375))

  it('first screen: the title and a tile per category with the shortcut to the caller\'s department, no people, nothing asked for', async () => {
    await renderContacts()
    expect(await screen.findByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    await within(tiles()).findByRole('button', { name: /^Kế toán/ })
    const names = within(tiles()).getAllByRole('button').map(textOf)
    expect(names[0]).toMatch(/^Phòng ban/)
    expect(names[1]).toMatch(/^Vai trò.*3 vai trò/)
    expect(names[2]).toMatch(/^Nhóm của tôi/)
    expect(names[3]).toMatch(/^Kế toán/)
    expect(tiles()).toHaveTextContent('Phòng của tôi')
    expect(screen.queryByRole('button', { name: /^Trần Minh Đức/ })).toBeNull()
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(document.body.textContent).not.toMatch(/Tất cả thành viên|Bên ngoài|Danh bạ chia sẻ/)
    expect(departmentCalls()).toEqual([])
    expect(channelCalls()).toEqual([])
    expectNoIds()
  })

  it('a personal workspace has two tiles and no shortcut', async () => {
    server.workspaceType = 'personal'
    await renderContacts()
    await within(tiles()).findByRole('button', { name: /^Thành viên/ })
    const names = within(tiles()).getAllByRole('button').map(textOf)
    expect(names).toHaveLength(2)
    expect(names[0]).toMatch(/^Thành viên.*7 người trong không gian này/)
    expect(names[1]).toMatch(/^Nhóm của tôi/)
    expect(tiles()).not.toHaveTextContent('Phòng của tôi')
  })

  it('a tile opens its category on its own screen, and only that category is asked for', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(tiles()).findByRole('button', { name: /^Phòng ban/ }))

    const screenOfDepartments = await layer('Phòng ban')
    expect(await within(screenOfDepartments).findByRole('button', { name: /^Khối Vận hành/ })).toBeInTheDocument()
    expect(departmentCalls()).toHaveLength(1)
    expect(channelCalls()).toEqual([])
  })

  it('goes down a level at a time with the level\'s name and where it is on the bar, and back goes up one level, not home', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(tiles()).findByRole('button', { name: /^Phòng ban/ }))
    await user.click(await within(await layer('Phòng ban')).findByRole('button', { name: /^Khối Vận hành/ }))
    await user.click(await within(await layer('Khối Vận hành')).findByRole('button', { name: /^Vận hành thanh toán/ }))

    const level3 = await layer('Vận hành thanh toán')
    expect(within(level3).getByText('Khối Vận hành')).toBeInTheDocument()

    await user.click(within(level3).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Khối Vận hành')).toBeInTheDocument()
    await user.click(within(await layer('Khối Vận hành')).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Phòng ban')).toBeInTheDocument()
    await user.click(within(await layer('Phòng ban')).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
  })

  it('a link into a department climbs one level at a time too, without leaving the page', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const level = await layer('Đối soát')
    // The path above it, the last segment whole.
    expect(within(level).getByText('Khối Vận hành')).toBeInTheDocument()
    expect(within(level).getByText('Vận hành thanh toán')).toBeInTheDocument()

    await user.click(within(level).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Vận hành thanh toán')).toBeInTheDocument()
    await user.click(within(await layer('Vận hành thanh toán')).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Khối Vận hành')).toBeInTheDocument()
    await user.click(within(await layer('Khối Vận hành')).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Phòng ban')).toBeInTheDocument()
    await user.click(within(await layer('Phòng ban')).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
  })

  it('the system back does the same: one level of the stack at a time', async () => {
    const user = userEvent.setup()
    const router = await renderContacts()
    await user.click(await within(tiles()).findByRole('button', { name: /^Phòng ban/ }))
    await user.click(await within(await layer('Phòng ban')).findByRole('button', { name: /^Khối Vận hành/ }))
    await layer('Khối Vận hành')

    act(() => router.history.back())
    expect(await layer('Phòng ban')).toBeInTheDocument()
    act(() => router.history.back())
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('Esc is one step back as well', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhtt}`)
    await layer('Vận hành thanh toán')
    await user.keyboard('{Escape}')
    expect(await layer('Khối Vận hành')).toBeInTheDocument()
  })

  it('a person opens a full-screen profile with three actions and a back on the cover, and back returns to the list', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const list = await layer('Đối soát')
    await user.click(await within(list).findByRole('button', { name: /^Nguyễn Thu Lan/ }))

    const profile = await layer('Hồ sơ Nguyễn Thu Lan')
    for (const name of ['Nhắn tin', 'Gọi']) expect(within(profile).getByRole('button', { name })).toBeInTheDocument()
    expect(within(profile).getByRole('link', { name: 'Gửi email' })).toBeInTheDocument()
    expect(router.state.location.search).toMatchObject({ person: U.lan.id })
    expectNoIds()

    await user.click(within(profile).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: /Hồ sơ/ })).toBeNull())
    expect(await layer('Đối soát')).toBeInTheDocument()
    expect(router.state.location.search).not.toHaveProperty('person')
  })

  it('the system back closes the profile', async () => {
    const user = userEvent.setup()
    const router = await renderContacts()
    await user.click(await within(tiles()).findByRole('button', { name: /^Kế toán/ }))
    await user.click(await within(await layer('Kế toán')).findByRole('button', { name: /^Lê Thị Hoa/ }))
    await layer(/Hồ sơ/)
    act(() => router.history.back())
    await waitFor(() => expect(screen.queryByRole('dialog', { name: /Hồ sơ/ })).toBeNull())
    expect(await layer('Kế toán')).toBeInTheDocument()
  })

  it('a link with ?person= opens the profile over the first screen, and back goes to it', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?person=${U.lan.id}`)
    const profile = await layer('Hồ sơ Nguyễn Thu Lan')
    await user.click(within(profile).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
  })

  it('Gọi works from the profile: the direct message, then the call', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?person=${U.lan.id}`)
    const profile = await layer('Hồ sơ Nguyễn Thu Lan')
    await user.click(within(profile).getByRole('button', { name: 'Gọi' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/calls/${CALL_ID}`))
    expect(server.calls.find((c) => c.path === '/dms' && c.method === 'POST')?.body).toMatchObject({ target_user_id: U.lan.id })
    expect(server.calls.find((c) => c.path === `/channels/${DM_ID}/calls`)?.body).toEqual({ kind: 'audio' })
  })

  it('the top bar\'s search opens the search screen, which finds people across the workspace', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await within(tiles()).findByRole('button', { name: /^Kế toán/ })
    act(() => (document.querySelector('[data-module-search]') as HTMLElement).click())

    const search = await layer('Tìm trong danh bạ')
    expect(within(search).getByText('Cả Khối Vận hành')).toBeInTheDocument()
    await user.type(within(search).getByRole('searchbox', { name: /^Tìm người/ }), 'doi soat')
    expect(await within(search).findByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
    expect(within(search).getByRole('button', { name: /^Trần Bảo Ngọc/ })).toBeInTheDocument()

    await user.click(within(search).getByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('a limited caller\'s search box says it searches the scope they may see', async () => {
    server.contacts = directory(PEOPLE.filter((p) => p.department_id === D.kt), { visibility: 'limited' })
    await renderContacts('/contacts?cat=org')
    const list = await layer('Phòng ban')
    expect(await within(list).findByRole('searchbox', { name: 'Tìm người trong phạm vi bạn được xem' })).toBeInTheDocument()
  })

  it('opens a group from Nhóm của tôi and can go to its conversation', async () => {
    const user = userEvent.setup()
    const router = await renderContacts()
    await user.click(await within(tiles()).findByRole('button', { name: /^Nhóm của tôi/ }))
    await user.click(await within(await layer('Nhóm của tôi')).findByRole('button', { name: /Đối soát liên khối/ }))
    const group = await layer('Đối soát liên khối')
    expect(await within(group).findByRole('button', { name: /Nguyễn Hữu Thành/ })).toBeInTheDocument()
    await user.click(within(group).getByRole('button', { name: 'Mở nhóm' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/channels/${CH.doisoat}`))
  })

  it('keeps the search suggestions on the search screen: they cover the list until Quay lại, and the box stays a plain search box', async () => {
    const user = userEvent.setup()
    addRecent('contacts', ME.id, 'kế toán')
    server.workspaceType = 'personal'
    await renderContacts('/contacts?cat=members')
    const list = await layer('Thành viên')
    await user.click(await within(list).findByRole('searchbox', { name: /^Tìm người/ }))
    expect(await within(list).findByRole('button', { name: 'Tìm lại kế toán' })).toBeInTheDocument()
    await user.click(within(list).getByRole('button', { name: 'Quay lại danh sách' }))
    expect(within(list).queryByRole('button', { name: 'Tìm lại kế toán' })).toBeNull()
    expect(within(list).getByRole('button', { name: /^Trần Minh Đức/ })).toBeInTheDocument()
  })
})

describe('Danh bạ on a phone: search, focus and the way back', () => {
  beforeEach(() => stubViewport(375))

  it('typing in the search box of a level keeps the box, its focus, what was typed and the level\'s name on the bar', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const level = await layer('Đối soát')
    const box = within(level).getByRole('searchbox', { name: /^Tìm người/ })
    await user.click(box)
    await user.keyboard('la')
    const after = within(await layer('Đối soát')).getByRole('searchbox', { name: /^Tìm người/ })
    expect(after).toBe(box)
    expect(box).toHaveFocus()
    expect(box).toHaveValue('la')
    expect(within(await layer('Đối soát')).getByRole('heading', { level: 2, name: 'Đối soát' })).toBeInTheDocument()
  })

  it('the search screen lists nobody until something is typed, and says what to type', async () => {
    const user = userEvent.setup()
    await renderContacts('/contacts?cat=search')
    const search = await layer('Tìm trong danh bạ')
    expect(screen.getAllByText('Gõ tên, chức danh hoặc email để tìm người.').length).toBeGreaterThan(0)
    expect(document.body.textContent).not.toContain('Nguyễn Thu Lan')
    // One way back, in the bar: no second arrow beside the box.
    expect(within(search).queryByRole('button', { name: 'Quay lại danh sách' })).toBeNull()
    expect(within(search).getAllByRole('button', { name: 'Quay lại' })).toHaveLength(1)
    await user.keyboard('lan')
    expect(await within(search).findByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
  })

  it('after going down a level, focus is on the new level\'s back button', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(tiles()).findByRole('button', { name: /^Phòng ban/ }))
    await user.click(await within(await layer('Phòng ban')).findByRole('button', { name: /^Khối Vận hành/ }))
    const level = await layer('Khối Vận hành')
    await waitFor(() => expect(within(level).getByRole('button', { name: 'Quay lại' })).toHaveFocus())
  })

  it('back never leaves Danh bạ: when the entry before is another page, it goes up one level instead', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.vhds}`, '/channels/' + CH.doisoat)
    await user.click(within(await layer('Đối soát')).getByRole('button', { name: 'Quay lại' }))
    expect(await layer('Vận hành thanh toán')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/contacts')
  })
})
