import '@testing-library/jest-dom/vitest'
import { act, cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { resetClient } from '../../test/render'
import { CH, ME, U, WS_ID } from '../../test/chat-fixtures'
import {
  CALL_ID, D, DEPARTMENTS, DM_ID, GROUPS, PEOPLE, directory, fixtureApi, renderContacts, resetServer, said, server,
} from '../../test/contacts-fixtures'
import { addRecent, readRecent } from '../../lib/recent-searches'
import { clearResolvers } from '../../lib/people-resolver'
import { expectNoIds } from '../../test/no-ids'
import { apiFetch, ApiError } from '../../api/client'
import { switchPreferencesTo } from '../../lib/preferences'
import { useToastStore } from '../primitives'
import { useWebSocketStore } from '../../stores/websocket.store'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

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
  cleanup() // unmount first, so nothing mounted refetches into the cleared cache
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  resetClient()
  useToastStore.getState().clear()
})

const asked = (re: RegExp) => server.calls.filter((c) => c.method === 'GET' && re.test(c.path)).map((c) => c.path)
const departmentCalls = () => asked(/\/departments$/)
const channelCalls = () => asked(/\/channels$|\/members$/)
/** What a row says to a reader: its text without the decorative avatar and icons. */
const textOf = (el: Element) => {
  const copy = el.cloneNode(true) as Element
  copy.querySelectorAll('[aria-hidden="true"]').forEach((n) => n.remove())
  return (copy.textContent ?? '').replace(/\s+/g, ' ').trim()
}
const nav = () => screen.getByRole('navigation', { name: 'Danh mục' })
const category = (name: RegExp | string) => within(nav()).getByRole('button', { name })
const middle = () => screen.getByRole('region', { name: 'Danh sách' })
const crumbs = () => screen.getByRole('navigation', { name: 'Đường dẫn' })
/** The rows of a section, in the order shown, as what they say. */
const rows = (heading: RegExp | string) => {
  const section = screen.getByRole('region', { name: heading })
  return within(section).getAllByRole('button').map(textOf)
}
const searchBox = () => screen.getByRole('combobox', { name: /^Tìm người/ })

describe('Danh bạ: the categories', () => {
  it('lists Phòng ban, Vai trò and Nhóm của tôi, with Phòng của tôi as a shortcut, and no leftover categories', async () => {
    await renderContacts()
    expect(await screen.findByRole('heading', { level: 1, name: 'Danh bạ' })).toBeInTheDocument()
    await within(nav()).findByRole('button', { name: /Kế toán/ })

    const names = within(nav()).getAllByRole('button').map(textOf)
    expect(names[0]).toMatch(/^Phòng ban/)
    expect(names[1]).toMatch(/^Vai trò/)
    expect(names[2]).toMatch(/^Nhóm của tôi/)
    // The shortcut opens the caller's own department.
    expect(names[3]).toMatch(/^Kế toán/)
    expect(nav().textContent).toContain('Phòng của tôi')
    expect(document.body.textContent).not.toMatch(/Tất cả thành viên|Bên ngoài|Danh bạ chia sẻ/)
    expect(category(/^Phòng ban/)).toHaveAttribute('aria-current', 'page')
    expectNoIds()
  })

  it('asks for no department and no group until a category is opened', async () => {
    const user = userEvent.setup()
    await renderContacts('/contacts?cat=roles')
    await within(nav()).findByRole('button', { name: /^Vai trò/ })
    await screen.findByRole('heading', { level: 2, name: 'Vai trò' })
    expect(departmentCalls()).toEqual([])
    expect(channelCalls()).toEqual([])

    await user.click(category(/^Nhóm của tôi/))
    await screen.findByRole('button', { name: /Đối soát liên khối/ })
    expect(channelCalls()).toEqual([`/workspaces/${WS_ID}/channels`])
    expect(departmentCalls()).toEqual([])

    await user.click(category(/^Phòng ban/))
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    expect(departmentCalls()).toHaveLength(1)
  })

  it('asks for the members of a group only when that group is opened', async () => {
    const user = userEvent.setup()
    await renderContacts('/contacts?cat=groups')
    await user.click(await screen.findByRole('button', { name: /Đối soát liên khối/ }))
    await screen.findByRole('heading', { level: 2, name: 'Đối soát liên khối' })
    expect(asked(/\/members$/)).toEqual([`/channels/${CH.doisoat}/members`])
  })

  it('shows rows shaped like the real ones, not a spinner, while a category loads', async () => {
    server.departments = () => new Promise(() => {})
    await renderContacts()
    const busy = await screen.findByRole('status')
    expect(busy).toHaveAttribute('aria-busy', 'true')
    expect(busy.querySelector('[data-skeleton-mark="dept"]')).not.toBeNull()
    expect(busy.querySelector('[data-skeleton-mark="person"]')).not.toBeNull()
  })

  it('moves between categories with the keyboard and keeps the place in the address', async () => {
    const user = userEvent.setup()
    const router = await renderContacts()
    await within(nav()).findByRole('button', { name: /^Vai trò/ })
    category(/^Vai trò/).focus()
    await user.keyboard('{Enter}')
    await screen.findByRole('heading', { level: 2, name: 'Vai trò' })
    expect(router.state.location.search).toMatchObject({ cat: 'roles' })
    expect(category(/^Vai trò/)).toHaveAttribute('aria-current', 'page')
  })
})

describe('Danh bạ: Phòng ban', () => {
  it('starts at the top of the organisation: the khối first, then people with no department', async () => {
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    expect(await screen.findByText('7 người · 2 khối')).toBeInTheDocument()

    const khoi = rows(/^Khối · 2$/)
    expect(khoi[0]).toMatch(/^Khối Tài chính.*2 phòng, nhóm bên dưới.*2 người/)
    expect(khoi[1]).toMatch(/^Khối Vận hành.*1 phòng, nhóm bên dưới.*4 người/)
    expect(rows(/^Chưa thuộc phòng ban nào · 1$/)).toEqual([expect.stringMatching(/^Ngô Gia Bảo/)])
    expect(within(crumbs()).getByText('Toàn Khối Vận hành')).toHaveAttribute('aria-current', 'page')
    expectNoIds()
  })

  it('goes down one level at a time, breadcrumbs lead back, and the last segment is the page', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await screen.findByRole('button', { name: /^Khối Vận hành/ }))
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.click(await screen.findByRole('button', { name: /^Vận hành thanh toán/ }))

    expect(await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })).toBeInTheDocument()
    expect(screen.getByText('4 người · 2 phòng con')).toBeInTheDocument()
    expect(within(crumbs()).getByText('Vận hành thanh toán')).toHaveAttribute('aria-current', 'page')
    // Sub-departments by name: Đối soát before Hỗ trợ đối tác.
    const below = rows(/^Phòng ban · 2$/)
    expect(below[0]).toMatch(/^Đối soát.*2 người/)
    expect(below[1]).toMatch(/^Hỗ trợ đối tác.*0 người/)
    // The people who sit directly in this department, by given name: Đức before Yến.
    const people = rows(/^Thành viên · 2$/)
    expect(people[0]).toMatch(/^Trần Minh Đức/)
    expect(people[1]).toMatch(/^Phạm Hải Yến/)

    await user.click(within(crumbs()).getByRole('button', { name: 'Khối Vận hành' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
    await user.click(within(crumbs()).getByRole('button', { name: 'Toàn Khối Vận hành' }))
    expect(await screen.findByText('7 người · 2 khối')).toBeInTheDocument()
  })

  it('keeps the people of two departments with one name apart, and tells their paths apart', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await screen.findByRole('button', { name: /^Khối Tài chính/ }))
    await user.click(await screen.findByRole('button', { name: /^Đối soát/ }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Đối soát' })).toBeInTheDocument()
    expect(within(crumbs()).getByRole('button', { name: 'Khối Tài chính' })).toBeInTheDocument()
    expect(rows(/^Thành viên · 1$/)).toEqual([expect.stringMatching(/^Trần Bảo Ngọc/)])

    await user.click(await screen.findByRole('button', { name: /^Trần Bảo Ngọc/ }))
    const profile = await screen.findByRole('complementary', { name: /Hồ sơ/ })
    expect(within(profile).getByRole('button', { name: 'Khối Tài chính' })).toBeInTheDocument()
    expect(within(profile).queryByRole('button', { name: 'Khối Vận hành' })).toBeNull()
  })

  it('says a department with nobody in it is empty, and goes back to its parent', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.htdt}`)
    expect(await screen.findByText('Hỗ trợ đối tác chưa có ai')).toBeInTheDocument()
    expect(screen.getByText('Khi quản trị viên xếp người vào phòng này, họ sẽ hiện ở đây.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Về Vận hành thanh toán' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })).toBeInTheDocument()
    expect(router.state.location.search).toMatchObject({ dept: D.vhtt })
  })

  it('opens the caller\'s own department from the shortcut', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await user.click(await within(nav()).findByRole('button', { name: /^Kế toán/ }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Kế toán' })).toBeInTheDocument()
    expect(rows(/^Thành viên · 1$/)).toEqual([expect.stringMatching(/^Lê Thị Hoa/)])
  })

  it('words the Phòng của tôi shortcut with the department and the path above it, never its own name twice', async () => {
    await renderContacts()
    const mine = await within(nav()).findByRole('button', { name: /^Kế toán/ })
    await waitFor(() => expect(textOf(mine)).toBe('Kế toánKhối Tài chính1'))
  })

  it('leaves the Phòng của tôi sub-label out when the department has nothing above it', async () => {
    // A limited caller is served their own department with the parent blanked.
    server.departments = () => Promise.resolve({
      departments: DEPARTMENTS.filter((d) => d.id === D.kt).map((d) => ({ ...d, parent_id: '' })),
    })
    await renderContacts()
    const mine = await within(nav()).findByRole('button', { name: /^Kế toán/ })
    await waitFor(() => expect(textOf(mine)).toBe('Kế toán1'))
  })

  it('marks a department an administrator hid, and nobody else sees the mark', async () => {
    server.departments = () => Promise.resolve({
      departments: DEPARTMENTS.map((d) => (d.id === D.ktc ? { ...d, hidden: true } : d)),
    })
    await renderContacts()
    const row = await screen.findByRole('button', { name: /^Khối Tài chính/ })
    expect(within(row).getByText('Ẩn khỏi danh bạ')).toBeInTheDocument()
    expect(within(screen.getByRole('button', { name: /^Khối Vận hành/ })).queryByText('Ẩn khỏi danh bạ')).toBeNull()
  })

  it('names the part of the tree a limited caller sees, and does not hint at what is hidden', async () => {
    server.contacts = directory(PEOPLE.filter((p) => p.department_id === D.kt), { visibility: 'limited' })
    server.departments = () => Promise.resolve({ departments: [{ id: D.kt, name: 'Kế toán', parent_id: D.ktc, member_count: 1 }] })
    await renderContacts()
    expect(await within(nav()).findByText('Chỉ phòng của bạn')).toBeInTheDocument()
    expect(await within(crumbs()).findByText('Phạm vi của bạn')).toHaveAttribute('aria-current', 'page')
    expect(searchBox()).toHaveAccessibleName('Tìm người trong phạm vi bạn được xem')
    expect(await within(middle()).findByRole('button', { name: /^Kế toán/ })).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/Ẩn khỏi|giới hạn|hạn chế/i)
  })

  it('when the departments cannot be loaded says so under Phòng ban, and search and Vai trò still work', async () => {
    const user = userEvent.setup()
    server.departments = () => Promise.reject(new ApiError('boom', 500))
    await renderContacts()
    const alert = await within(nav()).findByRole('alert')
    expect(alert).toHaveTextContent('Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm.')

    await user.type(searchBox(), 'lan')
    expect(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
    await user.click(category(/^Vai trò/))
    expect(await screen.findByRole('button', { name: /^Quản trị viên/ })).toBeInTheDocument()
  })

  it('Thử lại under Phòng ban asks for the departments again', async () => {
    const user = userEvent.setup()
    server.departments = () => Promise.reject(new ApiError('boom', 500))
    await renderContacts()
    await within(nav()).findByRole('alert')

    server.departments = () => Promise.resolve({ departments: structuredClone(DEPARTMENTS) })
    await user.click(within(nav()).getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
    expect(within(nav()).queryByRole('alert')).toBeNull()
  })

  it('asks again when Phòng ban is opened again after a failure', async () => {
    const user = userEvent.setup()
    server.departments = () => Promise.reject(new ApiError('boom', 500))
    await renderContacts()
    await within(nav()).findByRole('alert')
    await user.click(category(/^Vai trò/))
    server.departments = () => Promise.resolve({ departments: structuredClone(DEPARTMENTS) })
    await user.click(category(/^Phòng ban/))
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
  })
})

describe('Danh bạ: Vai trò', () => {
  it('lists Quản trị viên first, then the roles an administrator made; never Thành viên', async () => {
    await renderContacts('/contacts?cat=roles')
    await screen.findByRole('button', { name: /Quản trị viên/ })
    const names = within(middle()).getAllByRole('button').map(textOf)
    expect(names[0]).toMatch(/^Quản trị viên.*1 người/)
    expect(names[1]).toMatch(/^Duyệt chi.*2 người/)
    expect(names[2]).toMatch(/^Đối soát viên.*2 người/)
    expect(names).toHaveLength(3)
    expect(within(middle()).queryByText('Thành viên')).toBeNull()
    expect(within(middle()).getByText('3 vai trò')).toBeInTheDocument()
  })

  it('opens a role to its holders, with their title and path, and no permissions', async () => {
    const user = userEvent.setup()
    await renderContacts('/contacts?cat=roles')
    await user.click(await screen.findByRole('button', { name: /^Duyệt chi/ }))

    expect(await screen.findByRole('heading', { level: 2, name: 'Duyệt chi' })).toBeInTheDocument()
    expect(screen.getByText('2 người')).toBeInTheDocument()
    expect(within(crumbs()).getByRole('button', { name: 'Vai trò' })).toBeInTheDocument()
    const holders = rows(/^Thành viên · 2$/)
    expect(holders[0]).toMatch(/^Trần Minh Đức.*Trưởng phòng.*Khối Vận hành.*Vận hành thanh toán/)
    expect(holders[1]).toMatch(/^Lê Thị Hoa.*Kế toán trưởng.*Khối Tài chính.*Kế toán/)
    expect(document.body.textContent).not.toMatch(/quyền|permission/i)
  })

  it('says the owners role manages the workspace', async () => {
    const user = userEvent.setup()
    await renderContacts('/contacts?cat=roles')
    await user.click(await screen.findByRole('button', { name: /^Quản trị viên/ }))
    expect(await screen.findByText('1 người · quản lý workspace')).toBeInTheDocument()
  })

  it('has no Vai trò in a personal workspace', async () => {
    server.workspaceType = 'personal'
    await renderContacts()
    await within(nav()).findByRole('button', { name: /^Thành viên/ })
    expect(within(nav()).queryByRole('button', { name: /^Vai trò/ })).toBeNull()
  })
})

describe('Danh bạ: Nhóm của tôi', () => {
  it('lists the groups the caller is in, not #general', async () => {
    await renderContacts('/contacts?cat=groups')
    const group = await screen.findByRole('button', { name: /Đối soát liên khối/ })
    expect(group).toHaveTextContent('3 thành viên')
    expect(screen.queryByRole('button', { name: /Vận hành chung/ })).toBeNull()
  })

  it('opens a group to its members with title and path, and a member the directory hides is named but not opened', async () => {
    const user = userEvent.setup()
    const router = await renderContacts('/contacts?cat=groups')
    await user.click(await screen.findByRole('button', { name: /Đối soát liên khối/ }))

    expect(await screen.findByRole('heading', { level: 2, name: 'Đối soát liên khối' })).toBeInTheDocument()
    expect(screen.getByText('3 thành viên')).toBeInTheDocument()
    const members = await screen.findByRole('region', { name: /^Thành viên · 3$/ })
    expect(await within(members).findByRole('button', { name: /Nguyễn Hữu Thành/ })).toBeInTheDocument()
    const named = within(members).getAllByRole('button').map(textOf)
    expect(named.find((t) => t.startsWith('Nguyễn Thu Lan'))).toMatch(/Chuyên viên đối soát.*Khối Vận hành.*Đối soát/)

    // The hidden member opens the small card with one sentence, not a profile.
    await user.click(within(members).getByRole('button', { name: /Nguyễn Hữu Thành/ }))
    expect(await screen.findByText('Hồ sơ này không còn hiển thị với bạn.')).toBeInTheDocument()
    expect(screen.queryByRole('complementary', { name: /Hồ sơ/ })).toBeNull()
    expect(router.state.location.search).not.toHaveProperty('person')
    expect(document.body.textContent).not.toContain('hidden_thanh')
  })

  it('Mở nhóm goes to the conversation', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=groups&chat=${CH.doisoat}`)
    await user.click(await screen.findByRole('button', { name: 'Mở nhóm' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/channels/${CH.doisoat}`))
  })

  it('leaves the category out when the caller is in no group', async () => {
    api.mockImplementation((path, init) =>
      path === `/workspaces/${WS_ID}/channels` ? Promise.resolve({ channels: [GROUPS[1]] }) : fixtureApi(path, init))
    await renderContacts('/contacts?cat=groups')
    expect(await screen.findByText('Bạn chưa ở nhóm nào')).toBeInTheDocument()
    await waitFor(() => expect(within(nav()).queryByRole('button', { name: /^Nhóm của tôi/ })).toBeNull())
  })
})

describe('Danh bạ: tìm kiếm', () => {
  it('finds people across the whole workspace by name, title or email, ignoring accents, with the path of each', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.type(searchBox(), 'doi soat')

    expect(await screen.findByText('Kết quả trong cả Khối Vận hành')).toBeInTheDocument()
    expect(screen.getByText('3 người khớp')).toBeInTheDocument()
    const found = rows(/^Người · 3$/)
    expect(found).toHaveLength(3)
    const ngoc = found.find((t) => t.startsWith('Trần Bảo Ngọc'))!
    expect(ngoc).toMatch(/Khối Tài chính, Đối soát/)
    const lan = found.find((t) => t.startsWith('Nguyễn Thu Lan'))!
    expect(lan).toMatch(/Khối Vận hành, Vận hành thanh toán, Đối soát/)
    expect(document.querySelector('mark')?.textContent).toMatch(/đối soát/i)
  })

  it('Esc and the clear button return to the level that was open before', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhtt}`)
    await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })
    await user.type(searchBox(), 'lan')
    expect(await screen.findByText('Kết quả trong cả Khối Vận hành')).toBeInTheDocument()

    await user.keyboard('{Escape}')
    expect(await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })).toBeInTheDocument()
    expect(searchBox()).toHaveValue('')

    await user.type(searchBox(), 'lan')
    await user.click(await screen.findByRole('button', { name: 'Xoá tìm kiếm' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })).toBeInTheDocument()
  })

  it('says nobody matches and offers to clear', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.type(searchBox(), 'Hưng')
    expect(await screen.findByText('Không có ai khớp “Hưng” trong cả Khối Vận hành')).toBeInTheDocument()
    // The ✕ in the box and the button of this state do the same.
    const clearers = screen.getAllByRole('button', { name: 'Xoá tìm kiếm' })
    await user.click(clearers[clearers.length - 1]!)
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
  })

  it('keeps the search in the address, so a link or a reload lands on it', async () => {
    const router = await renderContacts('/contacts?q=lan')
    expect(searchBox()).toHaveValue('lan')
    expect(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
    fireEvent.change(searchBox(), { target: { value: 'vinh' } })
    await waitFor(() => expect(router.state.location.search).toMatchObject({ q: 'vinh' }))
  })
})

describe('Danh bạ: đang trực tuyến', () => {
  it('filters the level that is open and counts who is online there', async () => {
    const user = userEvent.setup()
    useWebSocketStore.setState({ onlineUsers: { [U.yen.id]: 'yen' } })
    await renderContacts(`/contacts?cat=org&dept=${D.vhtt}`)
    await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })
    const chip = screen.getByRole('button', { name: /Đang trực tuyến/ })
    expect(chip).toHaveTextContent('1')
    await user.click(chip)
    expect(chip).toHaveAttribute('aria-pressed', 'true')
    expect(rows(/^Thành viên đang trực tuyến · 1$/)).toEqual([expect.stringMatching(/^Phạm Hải Yến/)])
  })

  it('offers to show everyone when nobody is online', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await screen.findByRole('heading', { level: 2, name: 'Đối soát' })
    await user.click(screen.getByRole('button', { name: /Đang trực tuyến/ }))
    expect(await screen.findByText('Không ai ở Đối soát đang trực tuyến')).toBeInTheDocument()
    expect(screen.getByText('Bạn vẫn nhắn được; họ thấy tin khi quay lại.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Hiện mọi người' }))
    expect(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
  })

  it('says in words who is online and who is not', async () => {
    useWebSocketStore.setState({ onlineUsers: { [U.duc.id]: 'duc' } })
    await renderContacts(`/contacts?cat=org&dept=${D.vhtt}`)
    const duc = await screen.findByRole('button', { name: /^Trần Minh Đức/ })
    expect(within(duc).getByText('Trực tuyến')).toBeInTheDocument()
    expect(within(screen.getByRole('button', { name: /^Phạm Hải Yến/ })).getByText('Ngoại tuyến')).toBeInTheDocument()
  })
})

describe('Danh bạ: states', () => {
  it('says it could not load the directory, and retries', async () => {
    const user = userEvent.setup()
    server.contacts = () => Promise.reject(new ApiError('boom', 500))
    await renderContacts()
    expect(await screen.findByText('Không tải được danh bạ')).toBeInTheDocument()
    server.contacts = directory()
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
  })

  it('shows rows to come, not "nobody here", until the workspace and its people are known', async () => {
    let release: () => void = () => {}
    api.mockImplementation((path, init) => (path === '/workspaces'
      ? new Promise((resolve) => { release = () => resolve({ workspaces: [{ id: WS_ID, name: 'Khối Vận hành' }] }) })
      : fixtureApi(path, init)))
    await renderContacts()
    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true')
    expect(screen.queryByText(/chưa có ai khác/)).toBeNull()
    release()
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
    expect(screen.queryByText(/chưa có ai khác/)).toBeNull()
  })

  it('says an empty workspace is empty', async () => {
    server.contacts = directory([])
    await renderContacts()
    expect(await screen.findByText(/chưa có ai khác/)).toBeInTheDocument()
  })

  it('follows the directory\'s pages, so nobody beyond the first page is missing', async () => {
    const pages: string[] = []
    api.mockImplementation((path, init) => {
      if (path.startsWith(`/workspaces/${WS_ID}/contacts?`)) {
        pages.push(path)
        return Promise.resolve(path.includes('cursor=p2')
          ? { contacts: structuredClone(PEOPLE.slice(3)), total: 7 }
          : { contacts: structuredClone(PEOPLE.slice(0, 3)), total: 7, next_cursor: 'p2' })
      }
      return fixtureApi(path, init)
    })
    await renderContacts()
    expect(await screen.findByText('7 người · 2 khối')).toBeInTheDocument()
    expect(pages).toHaveLength(2)
  })

  it('shows a person with no display name as an unknown member, never by username', async () => {
    server.contacts = directory([{ ...structuredClone(PEOPLE[1]!), display_name: '', username: 'handle_yen' }, structuredClone(PEOPLE[2]!)])
    await renderContacts(`/contacts?cat=org&dept=${D.vhtt}`)
    expect(await screen.findByRole('button', { name: /^Thành viên/ })).toBeInTheDocument()
    expect(document.body.textContent).not.toContain('handle_yen')
  })
})

describe('Danh bạ: hồ sơ', () => {
  const openLan = async (user: ReturnType<typeof userEvent.setup>) => {
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    return screen.findByRole('complementary', { name: /Hồ sơ/ })
  }

  it('opens beside the list with the details that exist, the whole path, roles and colleagues', async () => {
    const profile = await openLan(userEvent.setup())
    expect(within(profile).getByRole('heading', { name: 'Nguyễn Thu Lan' })).toBeInTheDocument()
    expect(within(profile).getByText('Chuyên viên đối soát')).toBeInTheDocument()
    expect(within(profile).getByText('lan@novapay.vn')).toBeInTheDocument()
    expect(within(profile).getByText('Hà Nội, tầng 12')).toBeInTheDocument()
    for (const step of ['Khối Vận hành', 'Vận hành thanh toán', 'Đối soát']) {
      expect(within(profile).getByRole('button', { name: step })).toBeInTheDocument()
    }
    expect(within(profile).getByRole('button', { name: 'Đối soát viên' })).toBeInTheDocument()
    expect(within(profile.querySelector('dl') as HTMLElement).queryByText('Thành viên')).toBeNull()
    expect(within(profile).getByText('Lê Quang Vinh')).toBeInTheDocument()
    expect(within(profile).getByRole('link', { name: 'Gửi email' })).toHaveAttribute('href', 'mailto:lan@novapay.vn')
    expect(profile.textContent).not.toContain('@lan')
    expectNoIds()
  })

  it('keeps the person in the address, and a link with ?person= opens the profile', async () => {
    const router = await renderContacts(`/contacts?person=${U.lan.id}`)
    const profile = await screen.findByRole('complementary', { name: /Hồ sơ/ })
    expect(within(profile).getByRole('heading', { name: 'Nguyễn Thu Lan' })).toBeInTheDocument()
    expect(router.state.location.search).toMatchObject({ person: U.lan.id })
    // The default category is shown beside it.
    expect(await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })).toBeInTheDocument()
  })

  it('a path segment opens that department, and a role opens its holders', async () => {
    const user = userEvent.setup()
    const profile = await openLan(user)
    await user.click(within(profile).getByRole('button', { name: 'Vận hành thanh toán' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Vận hành thanh toán' })).toBeInTheDocument()
    await user.click(within(screen.getByRole('complementary', { name: /Hồ sơ/ })).getByRole('button', { name: 'Đối soát viên' }))
    expect(await screen.findByRole('heading', { level: 2, name: 'Đối soát viên' })).toBeInTheDocument()
  })

  it('starts a direct message and goes to it', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    await user.click(await screen.findByRole('button', { name: 'Nhắn tin' }))
    await waitFor(() => expect(router.state.location.pathname).toBe(`/channels/${DM_ID}`))
    expect(server.calls.find((c) => c.path === '/dms' && c.method === 'POST')?.body).toEqual({
      target_user_id: U.lan.id, target_ngac_node_id: U.lan.node,
    })
  })

  it('Gọi is enabled for someone offline: it opens the direct message with their user id, starts a call in it and goes to the call', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    const call = await screen.findByRole('button', { name: 'Gọi' })
    expect(call).toBeEnabled()
    await user.click(call)

    await waitFor(() => expect(router.state.location.pathname).toBe(`/calls/${CALL_ID}`))
    expect(server.calls.find((c) => c.path === '/dms' && c.method === 'POST')?.body).toEqual({
      target_user_id: U.lan.id, target_ngac_node_id: U.lan.node,
    })
    expect(server.calls.find((c) => c.path === `/channels/${DM_ID}/calls`)?.body).toEqual({ kind: 'audio' })
    expect(router.state.location.search).toMatchObject({ answer: 'audio' })
  })

  it('tells why a message or a call could not start, and stays', async () => {
    const user = userEvent.setup()
    api.mockImplementation((path, init) => (path === '/dms' && init?.method === 'POST' ? Promise.reject(new ApiError('nope', 403)) : fixtureApi(path, init)))
    const router = await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    await user.click(await screen.findByRole('button', { name: 'Gọi' }))
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => /Bạn chưa có quyền/.test(t.message))).toBe(true))
    expect(router.state.location.pathname).toBe('/contacts')
    expect(screen.getByRole('button', { name: 'Gọi' })).toBeEnabled()
  })

  it('offers Sửa hồ sơ, not messaging, on one\'s own profile', async () => {
    const user = userEvent.setup()
    const router = await renderContacts(`/contacts?cat=org&dept=${D.kt}`)
    await user.click(await screen.findByRole('button', { name: /^Lê Thị Hoa/ }))
    const profile = await screen.findByRole('complementary', { name: /Hồ sơ/ })
    expect(within(profile).queryByRole('button', { name: 'Nhắn tin' })).toBeNull()
    expect(within(profile).queryByRole('button', { name: 'Gọi' })).toBeNull()
    await user.click(within(profile).getByRole('button', { name: 'Sửa hồ sơ' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  })

  it('closes with Esc or the button, and focus goes back to the row', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const row = await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })
    await user.click(row)
    await screen.findByRole('complementary', { name: /Hồ sơ/ })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: /Hồ sơ/ })).toBeNull())
    expect(row).toHaveFocus()

    await user.click(row)
    await user.click(await screen.findByRole('button', { name: 'Đóng hồ sơ' }))
    await waitFor(() => expect(screen.queryByRole('complementary', { name: /Hồ sơ/ })).toBeNull())
    await waitFor(() => expect(row).toHaveFocus())
  })

  it('marks the chosen row', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    const row = await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })
    await user.click(row)
    expect(row).toHaveAttribute('aria-current', 'true')
  })
})

describe('Danh bạ: personal workspace', () => {
  it('lists the members flat, with no department, role, shortcut or colleagues', async () => {
    server.workspaceType = 'personal'
    const user = userEvent.setup()
    await renderContacts()
    await within(nav()).findByRole('button', { name: /^Thành viên/ })
    const names = within(nav()).getAllByRole('button').map(textOf)
    expect(names[0]).toMatch(/^Thành viên.*7 người trong không gian này/)
    expect(names[1]).toMatch(/^Nhóm của tôi/)
    expect(nav().textContent).not.toMatch(/Phòng của tôi|Phòng ban|Vai trò/)

    const list = rows(/^Thành viên · 7$/)
    expect(list[0]).toMatch(/^Ngô Gia Bảo/)
    expect(list[1]).toMatch(/^Trần Minh Đức/)
    expect(screen.queryByText('Vận hành thanh toán')).toBeNull()
    expect(departmentCalls()).toEqual([])

    await user.click(screen.getByRole('button', { name: /^Nguyễn Thu Lan/ }))
    const profile = await screen.findByRole('complementary', { name: /Hồ sơ/ })
    expect(within(profile).queryByText('Phòng ban')).toBeNull()
    expect(within(profile).queryByText('Vai trò')).toBeNull()
    expect(within(profile).queryByRole('region', { name: 'Cùng phòng' })).toBeNull()
    expect(within(profile).getByText('Email')).toBeInTheDocument()
  })
})

describe('Danh bạ: suggestions in the empty search box', () => {
  it('offers recent searches and the people one messaged lately, latest first, and never an id', async () => {
    const user = userEvent.setup()
    addRecent('contacts', ME.id, 'kế toán')
    server.activity = [said(CH.dmLan, 3), said(CH.dmYen, 1)]
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.click(searchBox())
    const list = await screen.findByRole('listbox', { name: 'Gợi ý tìm kiếm' })
    expect(within(within(list).getByRole('group', { name: 'Tìm gần đây' })).getByRole('option', { name: 'Tìm lại kế toán' })).toBeInTheDocument()
    const messaged = await within(list).findByRole('group', { name: 'Nhắn gần đây' })
    expect(within(messaged).getAllByRole('option').map((o) => o.querySelector('.font-medium')?.textContent))
      .toEqual(['Phạm Hải Yến', 'Nguyễn Thu Lan'])
    expectNoIds()
  })

  it('forgets a recent search with Delete, and the rest with Xoá tất cả until Hoàn tác', async () => {
    const user = userEvent.setup()
    ;['ba', 'hai', 'một'].forEach((w) => addRecent('contacts', ME.id, w))
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.click(searchBox())
    await user.keyboard('{ArrowDown}{Delete}')
    await waitFor(() => expect(readRecent('contacts', ME.id)).toEqual(['hai', 'ba']))
    await user.click(await screen.findByRole('button', { name: 'Xoá tất cả' }))
    await waitFor(() => expect(readRecent('contacts', ME.id)).toEqual([]))
    const [toast] = useToastStore.getState().toasts
    expect(toast).toMatchObject({ message: 'Đã xoá 2 lần tìm gần đây', action: { label: 'Hoàn tác' } })
    act(() => toast!.action!.onClick())
    expect(readRecent('contacts', ME.id)).toEqual(['hai', 'ba'])
  })

  it('runs a recent search again', async () => {
    const user = userEvent.setup()
    addRecent('contacts', ME.id, 'Thu Lan')
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.click(searchBox())
    await user.click(await screen.findByRole('option', { name: 'Tìm lại Thu Lan' }))
    expect(searchBox()).toHaveValue('Thu Lan')
    expect(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })).toBeInTheDocument()
  })

  it('opens the profile of a suggested person', async () => {
    const user = userEvent.setup()
    server.activity = [said(CH.dmYen, 1)]
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.click(searchBox())
    await user.click(await screen.findByRole('option', { name: /Phạm Hải Yến/ }))
    const panel = await screen.findByRole('complementary', { name: /Hồ sơ/ })
    expect(within(panel).getByRole('heading', { name: 'Phạm Hải Yến' })).toBeInTheDocument()
  })

  it('keeps a search when Enter is pressed or a profile is opened from the results', async () => {
    const user = userEvent.setup()
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.type(searchBox(), 'vinh{Enter}')
    expect(readRecent('contacts', ME.id)).toEqual(['vinh'])
    await user.clear(searchBox())
    await user.type(searchBox(), 'Thu Lan')
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    expect(readRecent('contacts', ME.id)).toEqual(['Thu Lan', 'vinh'])
  })

  it('asks nothing about direct messages until the box is focused', async () => {
    const user = userEvent.setup()
    server.activity = [said(CH.dmYen, 1)]
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    const chat = () => server.calls.filter((c) => c.method === 'GET' && ['/dms', '/channels/activity', '/channels/unread', `/workspaces/${WS_ID}/channels`].includes(c.path))
    expect(chat()).toEqual([])
    await user.click(searchBox())
    expect(await screen.findByRole('option', { name: /Phạm Hải Yến/ })).toBeInTheDocument()
    expect(chat().map((c) => c.path).sort()).toEqual(['/channels/activity', '/dms'])
  })

  it('does not keep a search on the Enter that ends a word being composed', async () => {
    await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    const box = searchBox()
    box.focus()
    fireEvent.change(box, { target: { value: 'Đức' } })
    fireEvent.keyDown(box, { key: 'Enter', isComposing: true })
    expect(readRecent('contacts', ME.id)).toEqual([])
    fireEvent.keyDown(box, { key: 'Enter' })
    expect(readRecent('contacts', ME.id)).toEqual(['Đức'])
  })
})

describe('Danh bạ: typing and focus', () => {
  it('keeps the rows that still match while typing: a person\'s row is the same element, not a new one', async () => {
    const user = userEvent.setup()
    server.workspaceType = 'personal'
    await renderContacts('/contacts?cat=members')
    const row = await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ })
    await user.type(searchBox(), 'La')
    await screen.findByText(/người khớp/)
    expect(screen.getByRole('button', { name: /^Nguyễn Thu Lan/ })).toBe(row)
  })

  it('writes the search into the address with the person that is opened, even before the typing settles', async () => {
    const user = userEvent.setup()
    const router = await renderContacts()
    await screen.findByRole('heading', { level: 2, name: 'Khối Vận hành' })
    await user.type(searchBox(), 'lan')
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    expect(router.state.location.search).toMatchObject({ q: 'lan', person: U.lan.id })
  })

  it('moves focus to the name of a profile that opens beside the list', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}`)
    await user.click(await screen.findByRole('button', { name: /^Nguyễn Thu Lan/ }))
    const profile = await screen.findByRole('complementary', { name: /Hồ sơ/ })
    await waitFor(() => expect(within(profile).getByRole('heading', { name: 'Nguyễn Thu Lan' })).toHaveFocus())
  })

  it('closing a profile that was opened by a link, with no row to go back to, lands on the heading of the list', async () => {
    const user = userEvent.setup()
    await renderContacts(`/contacts?cat=org&dept=${D.vhds}&person=${U.lan.id}`)
    await screen.findByRole('complementary', { name: /Hồ sơ/ })
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: /Hồ sơ/ })).toBeNull())
    await waitFor(() => expect(screen.getByRole('heading', { level: 2, name: 'Đối soát' })).toHaveFocus())
  })
})

describe('Danh bạ: one directory request per load', () => {
  it('asks for the directory once, however many parts of the screen read it', async () => {
    await renderContacts()
    await within(nav()).findByRole('button', { name: /Kế toán/ })
    await screen.findByRole('region', { name: 'Danh sách' })
    await new Promise((r) => setTimeout(r, 50))
    expect(asked(/\/contacts\?/)).toHaveLength(1)
  })
})
