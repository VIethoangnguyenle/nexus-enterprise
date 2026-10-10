import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import {
  D, RID, U, UUID_RE, WS_ID, adminFixtureApi, calls, mode, resetFixtures,
} from '../../test/admin-fixtures'
import { validateAdminSearch } from '../../lib/admin-search'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { OverviewScreen } from './OverviewScreen'
import { RolesScreen } from './RolesScreen'
import { UsersScreen } from './UsersScreen'

vi.mock('../../api/client', async (orig) => {
  const { adminFixtureApi } = await import('../../test/admin-fixtures')
  return { ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(adminFixtureApi) }
})

const api = vi.mocked(apiFetch)

/** The admin routes on a real in-memory router, so the URL is the state under test. */
async function renderAdmin(url = '/admin') {
  const root = createRootRoute()
  const admin = createRoute({ getParentRoute: () => root, path: '/admin', validateSearch: validateAdminSearch, component: Outlet })
  const children = [
    createRoute({ getParentRoute: () => admin, path: '/', component: OverviewScreen }),
    createRoute({ getParentRoute: () => admin, path: '/users', component: UsersScreen }),
    createRoute({ getParentRoute: () => admin, path: '/roles', component: RolesScreen }),
  ]
  const router = createRouter({
    routeTree: root.addChildren([admin.addChildren(children)]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  resetFixtures()
  api.mockImplementation(adminFixtureApi)
})
afterEach(() => {
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
  vi.unstubAllGlobals()
})

/** No identifier in anything a person reads: text, names, tooltips, placeholders, values. */
function noIds() {
  expect(document.body.textContent).not.toMatch(UUID_RE)
  for (const el of document.body.querySelectorAll('[aria-label],[title],[alt],[placeholder],[aria-valuetext]')) {
    for (const attr of ['aria-label', 'title', 'alt', 'placeholder']) {
      expect(el.getAttribute(attr) ?? '').not.toMatch(UUID_RE)
    }
  }
  for (const el of document.body.querySelectorAll('input,textarea')) {
    expect((el as HTMLInputElement).value).not.toMatch(UUID_RE)
  }
  // Nothing the system names for itself reaches the screen either.
  expect(document.body.textContent).not.toMatch(/\b(uuid_|Role_|Dept_|PC_|U_)\w*|_Owners|_Members/)
}

const search = (r: Awaited<ReturnType<typeof renderAdmin>>) => r.state.location.search as Record<string, unknown>
const writes = () => calls.filter((c) => c.method !== 'GET')
const table = () => screen.getByRole('table')
const rowNames = () =>
  within(table()).getAllByRole('row').slice(1).map((r) => r.querySelector('[data-row]')?.textContent)
const toastTexts = () => useToastStore.getState().toasts.map((t) => t.message)

// ---------------------------------------------------------------------------
// Tổng quan
// ---------------------------------------------------------------------------

describe('Tổng quan', () => {
  it('shows the figures and the organisation as one tree with head counts', async () => {
    await renderAdmin()
    expect(await screen.findByRole('heading', { level: 1, name: 'Quản trị' })).toBeInTheDocument()
    const tree = await screen.findByRole('tree', { name: 'Phòng ban' })

    const khoi = within(tree).getByRole('treeitem', { name: /Khối Vận hành/ })
    expect(khoi).toHaveTextContent('64')
    expect(within(tree).getByRole('treeitem', { name: /Vận hành thanh toán/ })).toHaveAttribute('aria-level', '2')
    expect(within(tree).getByRole('treeitem', { name: /Hành chính/ })).toHaveAttribute('aria-level', '1')

    // Thành viên, Phòng ban, Vai trò: counted from the lists.
    const figures = await screen.findByText('Vai trò', { selector: 'span' })
    expect(figures.parentElement).toHaveTextContent('5')
    expect(screen.getByText('2 hệ thống, 3 tuỳ chỉnh')).toBeInTheDocument()
    expect(screen.getByText('3 cấp')).toBeInTheDocument()
    noIds()
  })

  it('draws no activity feed: nothing records who changed what, and a made-up one would be a lie', async () => {
    await renderAdmin()
    await screen.findByRole('tree', { name: 'Phòng ban' })
    expect(screen.queryByText(/Hoạt động quản trị/)).toBeNull()
    expect(screen.queryByRole('list', { name: /Nhật ký|Hoạt động/ })).toBeNull()
  })

  it('opens a department in the panel and keeps it in the URL', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin()
    const tree = await screen.findByRole('tree', { name: 'Phòng ban' })
    await user.click(within(tree).getByRole('treeitem', { name: /Đối soát/ }))

    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    expect(within(panel).getByRole('heading', { name: 'Đối soát' })).toBeInTheDocument()
    expect(search(router).dept).toBe(D.doisoat)
    // Who is in it, by name: Lan and Vinh sit in Đối soát.
    const list = await within(panel).findByRole('list', { name: 'Thành viên của phòng' })
    expect(within(list).getByText('Nguyễn Thu Lan')).toBeInTheDocument()
    expect(within(list).getByText('Lê Quang Vinh')).toBeInTheDocument()
    // Where it sits is chosen by path, not typed.
    expect(within(panel).getByRole('button', { name: 'Thuộc phòng' })).toHaveTextContent('Khối Vận hành › Vận hành thanh toán')
    noIds()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Phòng ban' })).toBeNull())
    expect(search(router).dept).toBeUndefined()
  })

  it('lands on the department named in the URL after a reload', async () => {
    await renderAdmin(`/admin?dept=${D.ketoan}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    expect(within(panel).getByRole('heading', { name: 'Kế toán' })).toBeInTheDocument()
  })

  it('renames a department with the field the API reads', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.hotro}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(within(panel).getByRole('button', { name: 'Đổi tên' }))
    const dialog = await screen.findByRole('dialog', { name: 'Đổi tên phòng ban' })
    const field = within(dialog).getByRole('textbox', { name: 'Tên' })
    expect(field).toHaveValue('Hỗ trợ đối tác')
    await user.clear(field)
    await user.type(field, 'Đối tác')
    await user.click(within(dialog).getByRole('button', { name: 'Lưu' }))

    await waitFor(() => expect(writes()).toContainEqual({ method: 'PUT', path: `/workspaces/${WS_ID}/departments/${D.hotro}`, body: { name: 'Đối tác' } }))
    expect(toastTexts()).toContain('Đã đổi tên phòng ban')
  })

  it('does not carry a half-typed name from one department to the next', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(`/admin?dept=${D.hotro}`)
    let panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(within(panel).getByRole('button', { name: 'Đổi tên' }))
    const dialog = await screen.findByRole('dialog', { name: 'Đổi tên phòng ban' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Tên' }), ' XYZ')
    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    await act(() => router.navigate({ to: '/admin', search: { dept: D.kiemsoat } }))
    panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    expect(within(panel).getByRole('heading', { name: 'Kiểm soát nội bộ' })).toBeInTheDocument()
    await user.click(within(panel).getByRole('button', { name: 'Đổi tên' }))
    expect(await screen.findByRole('textbox', { name: 'Tên' })).toHaveValue('Kiểm soát nội bộ')
  })

  it('asks before deleting, in a dialog that really opens, and Esc closes only the dialog', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(`/admin?dept=${D.hotro}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(within(panel).getByRole('button', { name: 'Xoá phòng' }))

    const dialog = await screen.findByRole('dialog', { name: 'Xoá phòng Hỗ trợ đối tác?' })
    expect(writes()).toEqual([])
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByRole('complementary', { name: 'Phòng ban' })).toBeInTheDocument()
    expect(search(router).dept).toBe(D.hotro)

    await user.click(within(panel).getByRole('button', { name: 'Xoá phòng' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Xoá phòng' }))
    await waitFor(() => expect(writes()).toContainEqual({ method: 'DELETE', path: `/workspaces/${WS_ID}/departments/${D.hotro}` }))
    await waitFor(() => expect(search(router).dept).toBeUndefined())
    expect(dialog).toBeDefined()
  })

  it('creates a department under the one chosen from the tree', async () => {
    const user = userEvent.setup()
    await renderAdmin()
    await screen.findByRole('tree', { name: 'Phòng ban' })
    await user.click(screen.getByRole('button', { name: /Phòng ban$/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Phòng ban mới' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Tên phòng ban' }), 'Thanh toán quốc tế')
    await user.click(within(dialog).getByRole('button', { name: 'Thuộc phòng' }))
    await user.type(within(dialog).getByRole('searchbox', { name: 'Tìm phòng ban' }), 'đối soát')
    await user.click(await within(dialog).findByRole('button', { name: /Đối soát/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Tạo phòng ban' }))

    await waitFor(() => expect(writes()).toContainEqual({
      method: 'POST', path: `/workspaces/${WS_ID}/departments`, body: { name: 'Thanh toán quốc tế', parent_id: D.doisoat },
    }))
  })

  it('moves a department under another by choosing its parent by name', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.kiemsoat}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(within(panel).getByRole('button', { name: 'Thuộc phòng' }))
    await user.type(within(panel).getByRole('searchbox', { name: 'Tìm phòng ban' }), 'hành chính')
    await user.click(await within(panel).findByRole('button', { name: /Hành chính/ }))
    await waitFor(() => expect(writes()).toContainEqual({
      method: 'PUT', path: `/workspaces/${WS_ID}/departments/${D.kiemsoat}/move`, body: { new_parent_id: D.hanhchinh },
    }))
  })

  it('does not offer a department\'s own subtree as its parent', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.vanhanh}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(within(panel).getByRole('button', { name: 'Thuộc phòng' }))
    await user.type(within(panel).getByRole('searchbox', { name: 'Tìm phòng ban' }), 'đối soát')
    expect(await within(panel).findByText('Không có phòng ban nào khớp.')).toBeInTheDocument()
  })

  it('adds a person to a department by picking them, never by id', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin?dept=${D.hotro}`)
    const panel = await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.type(within(panel).getByRole('combobox', { name: 'Thêm thành viên vào phòng' }), 'vinh')
    await user.click(await within(panel).findByRole('option', { name: /Lê Quang Vinh/ }))
    await waitFor(() => expect(writes()).toContainEqual({
      method: 'PUT', path: `/workspaces/${WS_ID}/members/${U.vinh.node}/department`, body: { department_id: D.hotro },
    }))
  })

  describe('states', () => {
    it('shows a skeleton while the tree loads', async () => {
      api.mockImplementation((path, init) =>
        String(path).endsWith('/departments') ? new Promise(() => {}) : adminFixtureApi(path, init),
      )
      await renderAdmin()
      expect(await screen.findByLabelText('Đang tải sơ đồ tổ chức')).toHaveAttribute('aria-busy', 'true')
    })

    it('says so when there is no department yet, with a way to make one', async () => {
      mode.empty = true
      await renderAdmin()
      expect(await screen.findByText(/Chưa có phòng ban nào/)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Tạo phòng ban' })).toBeInTheDocument()
    })

    it('says what failed and retries', async () => {
      const { ApiError } = await import('../../api/client')
      let failing = true
      api.mockImplementation((path, init) =>
        failing && String(path).endsWith('/departments') ? Promise.reject(new ApiError('boom', 500)) : adminFixtureApi(path, init),
      )
      const user = userEvent.setup()
      await renderAdmin()
      expect(await screen.findByText(/Không tải được sơ đồ tổ chức/)).toBeInTheDocument()
      failing = false
      await user.click(screen.getByRole('button', { name: 'Thử lại' }))
      expect(await screen.findByRole('tree', { name: 'Phòng ban' })).toBeInTheDocument()
    })

    it('says in words that this person may not administer the workspace', async () => {
      mode.forbidden = true
      await renderAdmin()
      expect(await screen.findByText(/Bạn chưa có quyền quản trị Khối Vận hành/)).toBeInTheDocument()
      expect(screen.queryByRole('tree')).toBeNull()
      expect(screen.queryByRole('button', { name: /Mời thành viên/ })).toBeNull()
    })
  })
})

// ---------------------------------------------------------------------------
// Người dùng
// ---------------------------------------------------------------------------

describe('Người dùng', () => {
  it('lists people by name, with department, role pills (two, then +N) and a word for the standing', async () => {
    await renderAdmin('/admin/users')
    await within(await screen.findByRole('table', { name: 'Thành viên' })).findByText('Nguyễn Thu Lan')
    // The server lists people by name.
    expect(rowNames()).toEqual(['Lê Quang Vinh', 'Lê Thị Hoa', 'Nguyễn Thu Lan', 'Phạm Hải Yến', 'Trần Bảo Ngọc', 'Trần Minh Đức'])
    const yen = within(table()).getByText('Phạm Hải Yến').closest('[role="row"]') as HTMLElement
    expect(yen).toHaveTextContent('Vận hành thanh toán')
    expect(yen).toHaveTextContent('+1')
    const hoa = within(table()).getByText('Lê Thị Hoa').closest('[role="row"]') as HTMLElement
    expect(hoa).toHaveTextContent('Chủ sở hữu')
    expect(within(table()).getByText('Đã khoá')).toBeInTheDocument()
    expect(within(table()).getAllByText('Đang hoạt động').length).toBe(5)
    const vinh = within(table()).getByText('Lê Quang Vinh').closest('[role="row"]') as HTMLElement
    expect(vinh).toHaveTextContent('Thành viên')
    expect(screen.getByText('Khối Vận hành · 6 thành viên, 2 lời mời')).toBeInTheDocument()
    noIds()
  })

  it('searches by name or email without regard to accents', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/users')
    await within(await screen.findByRole('table')).findByText('Nguyễn Thu Lan')
    await user.type(screen.getByRole('searchbox', { name: 'Tìm theo tên hoặc email' }), 'tran minh')
    expect(rowNames()).toEqual(['Trần Minh Đức'])
    await user.clear(screen.getByRole('searchbox'))
    await user.type(screen.getByRole('searchbox'), 'lan@novapay')
    expect(rowNames()).toEqual(['Nguyễn Thu Lan'])
    await user.clear(screen.getByRole('searchbox'))
    await user.type(screen.getByRole('searchbox'), 'không ai tên vậy')
    expect(await screen.findByText(/Không có ai khớp với bộ lọc/)).toBeInTheDocument()
  })

  it('filters by department (with the ones beneath it) and by role', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/users')
    await within(await screen.findByRole('table')).findByText('Nguyễn Thu Lan')

    await user.click(screen.getByRole('button', { name: 'Lọc theo phòng ban' }))
    await user.click(await screen.findByRole('menuitem', { name: /^Khối Vận hành › Vận hành thanh toán$/ }))
    // Vận hành thanh toán holds Đức and Yến, and Đối soát beneath it holds Lan and Vinh.
    expect(rowNames().sort()).toEqual(['Lê Quang Vinh', 'Nguyễn Thu Lan', 'Phạm Hải Yến', 'Trần Minh Đức'].sort())

    await user.click(screen.getByRole('button', { name: 'Lọc theo vai trò' }))
    await user.click(await screen.findByRole('menuitem', { name: 'Kế toán' }))
    expect(rowNames().sort()).toEqual(['Nguyễn Thu Lan', 'Phạm Hải Yến'])
  })

  it('opens a person in the panel by clicking the row, keeps it in the URL, and Esc closes it', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin('/admin/users')
    await user.click(await within(await screen.findByRole('table')).findByRole('button', { name: 'Nguyễn Thu Lan' }))
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    expect(within(panel).getByRole('heading', { name: 'Nguyễn Thu Lan' })).toBeInTheDocument()
    expect(search(router).member).toBe(U.lan.node)
    expect(within(panel).getByRole('list', { name: 'Vai trò của thành viên' })).toHaveTextContent('Kế toán')
    expect(within(panel).getByRole('button', { name: 'Phòng ban' })).toHaveTextContent('Khối Vận hành › Vận hành thanh toán › Đối soát')
    noIds()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('complementary', { name: 'Thành viên' })).toBeNull())
    expect(search(router).member).toBeUndefined()
  })

  it('rows are real buttons that the arrow keys and Enter drive', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/users')
    const first = await within(await screen.findByRole('table')).findByRole('button', { name: 'Lê Quang Vinh' })
    first.focus()
    await user.keyboard('{ArrowDown}')
    expect(document.activeElement).toHaveTextContent('Lê Thị Hoa')
    await user.keyboard('{Enter}')
    expect(await screen.findByRole('complementary', { name: 'Thành viên' })).toBeInTheDocument()
  })

  it('gives a role to a person by picking it, sending only the two ids the route needs', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin/users?member=${U.vinh.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    await user.click(within(panel).getByRole('button', { name: 'Thêm vai trò' }))
    await user.type(within(panel).getByRole('searchbox', { name: 'Tìm vai trò' }), 'kiểm')
    await user.click(await within(panel).findByRole('button', { name: /Kiểm soát nội bộ/ }))

    await waitFor(() => expect(writes()).toContainEqual({ method: 'PUT', path: `/workspaces/${WS_ID}/members/${U.vinh.node}/roles/${RID.kiemsoat}` }))
    expect(toastTexts()).toContain('Đã gán vai trò Kiểm soát nội bộ cho Lê Quang Vinh')
    // The pill appears, by name.
    expect(await within(panel).findByRole('button', { name: 'Bỏ vai trò Kiểm soát nội bộ' })).toBeInTheDocument()
  })

  it('takes a role away in place and offers to undo it', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin/users?member=${U.lan.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    await user.click(await within(panel).findByRole('button', { name: 'Bỏ vai trò Kế toán' }))

    await waitFor(() => expect(writes()).toContainEqual({ method: 'DELETE', path: `/workspaces/${WS_ID}/members/${U.lan.node}/roles/${RID.ketoan}` }))
    const undo = useToastStore.getState().toasts.find((t) => t.message === 'Đã bỏ vai trò Kế toán của Nguyễn Thu Lan')
    expect(undo?.action?.label).toBe('Hoàn tác')
    act(() => undo!.action!.onClick())
    await waitFor(() => expect(writes()).toContainEqual({ method: 'PUT', path: `/workspaces/${WS_ID}/members/${U.lan.node}/roles/${RID.ketoan}` }))
  })

  it('never offers to remove a built-in role from here', async () => {
    await renderAdmin(`/admin/users?member=${U.hoa.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    const pills = within(panel).getByRole('list', { name: 'Vai trò của thành viên' })
    expect(within(pills).getByText('Chủ sở hữu')).toBeInTheDocument()
    expect(within(pills).queryByRole('button', { name: /Bỏ vai trò Chủ sở hữu/ })).toBeNull()
  })

  it('changes a person\'s department from the tree and says what happened', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin/users?member=${U.vinh.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    await user.click(within(panel).getByRole('button', { name: 'Phòng ban' }))
    await user.type(within(panel).getByRole('searchbox', { name: 'Tìm phòng ban' }), 'hành chính')
    await user.click(await within(panel).findByRole('button', { name: /Hành chính/ }))

    await waitFor(() => expect(writes()).toContainEqual({
      method: 'PUT', path: `/workspaces/${WS_ID}/members/${U.vinh.node}/department`, body: { department_id: D.hanhchinh },
    }))
    expect(toastTexts()).toContain('Đã chuyển Lê Quang Vinh sang phòng Hành chính')
  })

  it('takes a person out of every department with "none"', async () => {
    const user = userEvent.setup()
    await renderAdmin(`/admin/users?member=${U.vinh.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    await user.click(within(panel).getByRole('button', { name: 'Phòng ban' }))
    await user.click(within(panel).getByRole('button', { name: 'Chưa thuộc phòng ban nào' }))
    await waitFor(() => expect(writes()).toContainEqual({
      method: 'PUT', path: `/workspaces/${WS_ID}/members/${U.vinh.node}/department`, body: { department_id: '' },
    }))
  })

  it('asks before removing someone from the workspace', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(`/admin/users?member=${U.vinh.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    await user.click(within(panel).getByRole('button', { name: 'Xoá khỏi workspace' }))
    const dialog = await screen.findByRole('dialog', { name: 'Xoá Lê Quang Vinh khỏi workspace?' })
    expect(writes()).toEqual([])
    await user.click(within(dialog).getByRole('button', { name: 'Xoá khỏi workspace' }))
    await waitFor(() => expect(writes()).toContainEqual({ method: 'DELETE', path: `/workspaces/${WS_ID}/members/${U.vinh.node}` }))
    await waitFor(() => expect(search(router).member).toBeUndefined())
  })

  it.each([
    ['a plain member', U.vinh.node, 'Thành viên'],
    ['an owner', U.duc.node, 'Chủ sở hữu'],
  ])('on a phone the panel of %s shows the same locked pill as the table', async (_who, node, word) => {
    vi.stubGlobal('matchMedia', (q: string) => ({
      matches: q.includes('max-width: 767'), media: q, addEventListener: () => {}, removeEventListener: () => {},
    }))
    await renderAdmin(`/admin/users?member=${node}`)
    const panel = await screen.findByRole('dialog', { name: 'Thành viên' })
    const pills = within(panel).getByRole('list', { name: 'Vai trò của thành viên' })
    expect(within(pills).getByText(word)).toBeInTheDocument()
    expect(within(pills).getByLabelText('Không đổi được ở đây')).toBeInTheDocument()
    expect(within(pills).queryByRole('button')).toBeNull()
  })

  it('does not offer to remove yourself', async () => {
    await renderAdmin(`/admin/users?member=${U.hoa.node}`)
    const panel = await screen.findByRole('complementary', { name: 'Thành viên' })
    expect(within(panel).queryByRole('button', { name: 'Xoá khỏi workspace' })).toBeNull()
  })

  describe('states', () => {
    it('shows a skeleton while the people load', async () => {
      mode.hang = true
      await renderAdmin('/admin/users')
      expect(await screen.findByRole('table', { name: 'Thành viên' })).toHaveAttribute('aria-busy', 'true')
      expect(document.querySelectorAll('.skeleton').length).toBeGreaterThan(0)
    })

    it('says what failed and retries', async () => {
      mode.membersError = true
      const user = userEvent.setup()
      await renderAdmin('/admin/users')
      // A server error is tried once more before it is reported.
      expect(await screen.findByText(/Không tải được danh sách thành viên/, {}, { timeout: 4000 })).toBeInTheDocument()
      mode.membersError = false
      await user.click(screen.getByRole('button', { name: 'Thử lại' }))
      await within(await screen.findByRole('table')).findByText('Nguyễn Thu Lan')
    })

    it('has an empty state with the way forward', async () => {
      mode.empty = true
      await renderAdmin('/admin/users')
      expect(await screen.findByText(/Workspace chưa có thành viên nào/)).toBeInTheDocument()
      expect(screen.getAllByRole('button', { name: /Mời thành viên/ }).length).toBeGreaterThan(0)
    })

    it('says in words that this person may not manage', async () => {
      mode.forbidden = true
      await renderAdmin('/admin/users')
      expect(await screen.findByText(/Bạn chưa có quyền quản trị/)).toBeInTheDocument()
      expect(screen.queryByRole('table')).toBeNull()
    })
  })
})

// ---------------------------------------------------------------------------
// Mời thành viên
// ---------------------------------------------------------------------------

describe('Mời thành viên', () => {
  async function openInvite(user = userEvent.setup()) {
    await renderAdmin('/admin/users')
    await within(await screen.findByRole('table')).findByText('Nguyễn Thu Lan')
    await user.click(screen.getByRole('button', { name: /Mời thành viên/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Mời thành viên' })
    return { dialog, user }
  }
  const invitePosts = () => writes().filter((w) => w.method === 'POST' && w.path === `/workspaces/${WS_ID}/members`)

  it('takes emails as chips and asks for no id', async () => {
    const { dialog, user } = await openInvite()
    const field = within(dialog).getByRole('textbox', { name: 'Email' })
    await user.type(field, 'moi@novapay.vn, khac@novapay.vn ')
    const chips = within(dialog).getByRole('list', { name: 'Email sẽ mời' })
    expect(within(chips).getAllByRole('listitem').map((l) => l.textContent)).toEqual(['moi@novapay.vn', 'khac@novapay.vn'])
    expect(within(dialog).getByRole('button', { name: 'Mời 2 địa chỉ' })).toBeInTheDocument()
    for (const input of dialog.querySelectorAll('input')) {
      expect(`${input.getAttribute('aria-label') ?? ''}${input.getAttribute('placeholder') ?? ''}`).not.toMatch(/\bid\b|mã|ngac|node/i)
    }
    noIds()
  })

  it('splits a pasted list', async () => {
    const { dialog, user } = await openInvite()
    await user.click(within(dialog).getByRole('textbox', { name: 'Email' }))
    await user.paste('moi@novapay.vn;khac@novapay.vn, moi@novapay.vn ')
    expect(within(within(dialog).getByRole('list', { name: 'Email sẽ mời' })).getAllByRole('listitem')).toHaveLength(2)
  })

  it('refuses what is not an email, next to the field', async () => {
    const { dialog, user } = await openInvite()
    await user.type(within(dialog).getByRole('textbox', { name: 'Email' }), 'khong-phai-email, ')
    expect(await within(dialog).findByText(/không phải địa chỉ email/)).toBeInTheDocument()
    expect(within(dialog).queryByRole('list', { name: 'Email sẽ mời' })).toBeNull()
    await user.click(within(dialog).getByRole('button', { name: 'Mời' }))
    expect(writes()).toEqual([])
  })

  it('sends one invitation per address and says nothing about whether an account exists', async () => {
    const { dialog, user } = await openInvite()
    // One account holder, one stranger, one existing member: the same treatment for all three.
    await user.type(within(dialog).getByRole('textbox', { name: 'Email' }), 'moi@novapay.vn, nguoi.la@novapay.vn, lan@novapay.vn ')
    await user.click(within(dialog).getByRole('button', { name: 'Mời 3 địa chỉ' }))

    await waitFor(() => expect(invitePosts().map((w) => w.body)).toEqual([
      { email: 'moi@novapay.vn' }, { email: 'nguoi.la@novapay.vn' }, { email: 'lan@novapay.vn' },
    ]))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(toastTexts()).toContain('Đã gửi lời mời cho 3 địa chỉ')
    // No person was added by it: the table still has the same six, and no per-address result reached the screen.
    expect(document.body.textContent).not.toMatch(/Chưa có tài khoản|Đã là thành viên|không tìm thấy/i)
  })

  it('sends the chosen role and department with each invitation', async () => {
    const { dialog, user } = await openInvite()
    await user.type(within(dialog).getByRole('textbox', { name: 'Email' }), 'moi@novapay.vn ')
    await user.type(within(dialog).getByRole('combobox', { name: 'Chọn vai trò' }), 'kế')
    await user.click(await within(dialog).findByRole('option', { name: 'Kế toán' }))
    await user.click(within(dialog).getByRole('button', { name: 'Phòng ban' }))
    await user.type(within(dialog).getByRole('searchbox', { name: 'Tìm phòng ban' }), 'đối soát')
    await user.click(await within(dialog).findByRole('button', { name: /Đối soát/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Mời 1 địa chỉ' }))

    await waitFor(() => expect(writes()).toEqual([
      { method: 'POST', path: `/workspaces/${WS_ID}/members`, body: { email: 'moi@novapay.vn', role_id: RID.ketoan, department_id: D.doisoat } },
    ]))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  })

  it('names the address the server refuses and keeps it for another try; a rate limit stops the rest', async () => {
    mode.inviteStatus = 429
    const { dialog, user } = await openInvite()
    await user.type(within(dialog).getByRole('textbox', { name: 'Email' }), 'a@novapay.vn, b@novapay.vn ')
    await user.click(within(dialog).getByRole('button', { name: 'Mời 2 địa chỉ' }))
    const chips = await within(dialog).findByRole('list', { name: 'Email sẽ mời' })
    expect(within(chips).getAllByRole('listitem')).toHaveLength(2)
    expect(within(dialog).getAllByText(/quá nhiều địa chỉ trong một giờ/)).toHaveLength(2)
    expect(invitePosts()).toHaveLength(1)
    expect(screen.getByRole('dialog', { name: 'Mời thành viên' })).toBeInTheDocument()
  })

  it('says when this person may not invite', async () => {
    mode.inviteStatus = 403
    const { dialog, user } = await openInvite()
    await user.type(within(dialog).getByRole('textbox', { name: 'Email' }), 'a@novapay.vn ')
    await user.click(within(dialog).getByRole('button', { name: 'Mời 1 địa chỉ' }))
    expect(await within(dialog).findByText('Bạn không có quyền mời thành viên.')).toBeInTheDocument()
  })

  it('starts empty each time it opens, and Esc closes only a picker that is open', async () => {
    const { dialog, user } = await openInvite()
    await user.type(within(dialog).getByRole('textbox', { name: 'Email' }), 'moi@novapay.vn ')
    await user.click(within(dialog).getByRole('button', { name: 'Phòng ban' }))
    expect(within(dialog).getByRole('searchbox', { name: 'Tìm phòng ban' })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    expect(within(dialog).queryByRole('searchbox', { name: 'Tìm phòng ban' })).toBeNull()
    expect(screen.getByRole('dialog', { name: 'Mời thành viên' })).toBeInTheDocument()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())

    await user.click(screen.getByRole('button', { name: /Mời thành viên/ }))
    const again = await screen.findByRole('dialog', { name: 'Mời thành viên' })
    expect(within(again).queryByRole('list', { name: 'Email sẽ mời' })).toBeNull()
    expect(within(again).getByRole('textbox', { name: 'Email' })).toHaveValue('')
  })
})

describe('Lời mời đang chờ', () => {
  it('counts the open offers in the header and on Tổng quan, and lists them by address and name', async () => {
    await renderAdmin('/admin/users')
    expect(await screen.findByText('Khối Vận hành · 6 thành viên, 2 lời mời')).toBeInTheDocument()
    const list = await screen.findByRole('region', { name: 'Lời mời đang chờ' })
    expect(within(list).getByText('Lời mời đang chờ (2)')).toBeInTheDocument()
    const first = within(list).getByText('an.pham@novapay.vn').closest('li') as HTMLElement
    expect(first).toHaveTextContent('Mời bởi Lê Thị Hoa · vai trò Kế toán · phòng Đối soát')
    noIds()
  })

  it('shows the figure on Tổng quan with how many are about to lapse', async () => {
    await renderAdmin('/admin')
    const label = await screen.findByText('Lời mời chờ nhận')
    const card = label.parentElement as HTMLElement
    expect(card).toHaveTextContent('2')
    expect(card).toHaveTextContent('1 sắp hết hạn')
  })

  it('withdraws an offer', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/users')
    await user.click(await screen.findByRole('button', { name: 'Thu hồi lời mời của minh.ho@novapay.vn' }))
    await waitFor(() => expect(writes()).toContainEqual({ method: 'DELETE', path: `/workspaces/${WS_ID}/invitations/77777777-aaaa-4bbb-8ccc-000000000002` }))
    await waitFor(() => expect(screen.queryByText('minh.ho@novapay.vn')).toBeNull())
    expect(toastTexts()).toContain('Đã thu hồi lời mời của minh.ho@novapay.vn')
  })

  it('draws nothing when the person may manage but not invite', async () => {
    mode.invitationsForbidden = true
    await renderAdmin('/admin/users')
    await within(await screen.findByRole('table')).findByText('Nguyễn Thu Lan')
    expect(screen.queryByRole('region', { name: 'Lời mời đang chờ' })).toBeNull()
    expect(screen.getByText('Khối Vận hành · 6 thành viên')).toBeInTheDocument()
  })

  it('hides the list while filtering and when there are none', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/users')
    await screen.findByRole('region', { name: 'Lời mời đang chờ' })
    await user.type(screen.getByRole('searchbox', { name: 'Tìm theo tên hoặc email' }), 'lan')
    expect(screen.queryByRole('region', { name: 'Lời mời đang chờ' })).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Vai trò
// ---------------------------------------------------------------------------

describe('Vai trò', () => {
  it('names roles by what they were called, never by the node behind them', async () => {
    await renderAdmin('/admin/roles')
    await within(await screen.findByRole('table', { name: 'Vai trò' })).findByText('Kế toán')
    expect(rowNames()).toEqual(['Chủ sở hữu', 'Thành viên', 'Kế toán', 'Kiểm soát nội bộ', 'Quản lý tài sản'])
    const owners = within(table()).getByText('Chủ sở hữu').closest('[role="row"]') as HTMLElement
    expect(owners).toHaveTextContent('Hệ thống')
    const ketoan = within(table()).getByText('Kế toán').closest('[role="row"]') as HTMLElement
    expect(ketoan).toHaveTextContent('Tuỳ chỉnh')
    expect(screen.getByText('5 vai trò', { exact: false })).toBeInTheDocument()
    noIds()
  })

  it('shows what a role permits by area, in words', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin('/admin/roles')
    await user.click(await within(await screen.findByRole('table')).findByRole('button', { name: 'Kế toán' }))
    const panel = await screen.findByRole('complementary', { name: 'Vai trò' })
    expect(search(router).role).toBe(RID.ketoan)

    const docs = await within(panel).findByRole('list', { name: 'Quyền trên Tài liệu' })
    expect(within(docs).getAllByRole('listitem').map((l) => l.textContent)).toEqual(['Xem', 'Sửa', 'Chia sẻ'])
    expect(within(panel).getByRole('list', { name: 'Quyền trên Tài sản' })).toHaveTextContent('Xem')
    expect(within(panel).getByRole('button', { name: 'Sửa quyền' })).toBeInTheDocument()
    noIds()
  })

  it('shows members holding share on Documents and on channels, as migration 019 grants', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/roles')
    await user.click(await within(await screen.findByRole('table')).findByRole('button', { name: 'Thành viên' }))
    const panel = await screen.findByRole('complementary', { name: 'Vai trò' })
    const docs = await within(panel).findByRole('list', { name: 'Quyền trên Tài liệu' })
    expect(within(docs).getByText('Chia sẻ')).toBeInTheDocument()
    expect(within(panel).getByRole('list', { name: 'Quyền trên Tin nhắn' })).toHaveTextContent('Tạo nhóm chat')
    // A built-in role says it cannot be edited and has no way to try.
    expect(within(panel).getByText('Vai trò hệ thống không sửa được quyền.')).toBeInTheDocument()
    expect(within(panel).queryByRole('button', { name: 'Sửa quyền' })).toBeNull()
    expect(within(panel).queryByRole('button', { name: 'Xoá vai trò' })).toBeNull()
  })

  it('asks before deleting a role, in a dialog that really opens', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(`/admin/roles?role=${RID.taisan}`)
    const panel = await screen.findByRole('complementary', { name: 'Vai trò' })
    await user.click(await within(panel).findByRole('button', { name: 'Xoá vai trò' }))
    const dialog = await screen.findByRole('dialog', { name: 'Xoá vai trò Quản lý tài sản?' })
    expect(writes()).toEqual([])
    await user.click(within(dialog).getByRole('button', { name: 'Xoá vai trò' }))
    await waitFor(() => expect(writes()).toContainEqual({ method: 'DELETE', path: `/workspaces/${WS_ID}/roles/${RID.taisan}` }))
    await waitFor(() => expect(search(router).role).toBeUndefined())
  })

  it('creates a role by its name alone and opens it', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin('/admin/roles')
    await screen.findByRole('table')
    await user.click(screen.getByRole('button', { name: /Tạo vai trò/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Tạo vai trò' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Tên vai trò' }), 'Thủ quỹ')
    await user.click(within(dialog).getByRole('button', { name: 'Tạo vai trò' }))
    await waitFor(() => expect(writes()).toContainEqual({ method: 'POST', path: `/workspaces/${WS_ID}/roles`, body: { name: 'Thủ quỹ' } }))
    await waitFor(() => expect(search(router).role).toMatch(/^55555555/))
  })

  it('says next to the field when the server turns a name down', async () => {
    const user = userEvent.setup()
    await renderAdmin('/admin/roles')
    await screen.findByRole('table')
    await user.click(screen.getByRole('button', { name: /Tạo vai trò/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Tạo vai trò' })
    await user.type(within(dialog).getByRole('textbox', { name: 'Tên vai trò' }), 'Dept_Sales')
    await user.click(within(dialog).getByRole('button', { name: 'Tạo vai trò' }))
    expect(await within(dialog).findByText(/Tên này không dùng được/)).toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'Tạo vai trò' })).toBeInTheDocument()
  })

  describe('states', () => {
    it('shows a skeleton while roles load', async () => {
      api.mockImplementation((path, init) => (String(path).endsWith('/roles') ? new Promise(() => {}) : adminFixtureApi(path, init)))
      await renderAdmin('/admin/roles')
      expect(await screen.findByRole('table', { name: 'Vai trò' })).toHaveAttribute('aria-busy', 'true')
    })

    it('says what failed and retries', async () => {
      const { ApiError } = await import('../../api/client')
      let failing = true
      api.mockImplementation((path, init) =>
        failing && String(path).endsWith('/roles') ? Promise.reject(new ApiError('boom', 500)) : adminFixtureApi(path, init),
      )
      const user = userEvent.setup()
      await renderAdmin('/admin/roles')
      expect(await screen.findByText(/Không tải được danh sách vai trò/)).toBeInTheDocument()
      failing = false
      await user.click(screen.getByRole('button', { name: 'Thử lại' }))
      await within(await screen.findByRole('table')).findByText('Kế toán')
    })

    it('invites making the first custom role when there is none', async () => {
      mode.empty = true
      await renderAdmin('/admin/roles')
      expect(await screen.findByText(/Mới có hai vai trò hệ thống/)).toBeInTheDocument()
      expect(rowNames()).toEqual(['Chủ sở hữu', 'Thành viên'])
    })
  })
})

// ---------------------------------------------------------------------------
// Sửa quyền
// ---------------------------------------------------------------------------

describe('Sửa quyền', () => {
  const edit = (id = RID.ketoan) => `/admin/roles?role=${id}&edit=1`
  const cell = (op: string, area: string) => screen.getByRole('button', { name: `${op} trên ${area}` })

  it('draws the areas and operations the server answers with, not a table of its own', async () => {
    // Documents offers only Xem and Chia sẻ here; Assets only Duyệt. Nothing else exists.
    mode.areas = [
      { area: 'documents', operations: ['read', 'share'] },
      { area: 'assets', operations: ['approve'] },
    ]
    await renderAdmin(edit())
    const grid = await screen.findByRole('grid', { name: 'Quyền của Kế toán' })
    expect(within(grid).getAllByRole('columnheader').map((h) => h.textContent)).toEqual(['Vùng tài nguyên', 'Xem', 'Duyệt', 'Chia sẻ'])
    expect(within(grid).getAllByRole('rowheader').map((h) => h.textContent)).toEqual([
      'Tài liệuThư mục và tệp của workspace', 'Tài sảnKho tài sản và yêu cầu cấp tài sản',
    ])
    // Cells exist only where the server says the operation applies.
    expect(within(grid).getAllByRole('button').map((b) => b.getAttribute('aria-label'))).toEqual([
      'Xem trên Tài liệu', 'Chia sẻ trên Tài liệu', 'Duyệt trên Tài sản',
    ])
    expect(screen.queryByRole('button', { name: /Sửa trên|Quản lý trên|Mời trên|Tải lên trên/ })).toBeNull()
    expect(within(grid).getAllByText('Không áp dụng').length).toBe(3)
  })

  it('follows the real answer too: every area, the columns it needs, and what the role holds is pressed', async () => {
    await renderAdmin(edit())
    const grid = await screen.findByRole('grid', { name: 'Quyền của Kế toán' })
    expect(within(grid).getAllByRole('columnheader').map((h) => h.textContent)).toEqual([
      'Vùng tài nguyên', 'Xem', 'Sửa', 'Duyệt', 'Chia sẻ', 'Quản lý', 'Mời', 'Tạo nhóm chat',
    ])
    expect(cell('Xem', 'Tài liệu')).toHaveAttribute('aria-pressed', 'true')
    expect(cell('Chia sẻ', 'Tài liệu')).toHaveAttribute('aria-pressed', 'true')
    expect(cell('Xem', 'Tài sản')).toHaveAttribute('aria-pressed', 'true')
    expect(cell('Duyệt', 'Tài sản')).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByRole('button', { name: /Duyệt trên Tài liệu/ })).toBeNull()
    noIds()
  })

  it('saves only the areas that changed, each as the full set of its operations, and says who it affects', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(edit())
    await screen.findByRole('grid')
    await user.click(cell('Duyệt', 'Tài sản'))

    const bar = await screen.findByRole('region', { name: 'Thay đổi chưa lưu' })
    expect(bar).toHaveTextContent('1 thay đổi chưa lưu.')
    expect(bar).toHaveTextContent('14 người có vai trò Kế toán sẽ được Duyệt trên Tài sản.')
    await user.click(cell('Sửa', 'Tin nhắn'))
    expect(await screen.findByRole('region', { name: 'Thay đổi chưa lưu' })).toHaveTextContent('2 thay đổi chưa lưu.')

    await user.click(within(bar).getByRole('button', { name: 'Lưu' }))
    await waitFor(() => expect(writes()).toEqual([
      { method: 'PUT', path: `/workspaces/${WS_ID}/roles/${RID.ketoan}/permissions/channels`, body: { operations: ['write'] } },
      { method: 'PUT', path: `/workspaces/${WS_ID}/roles/${RID.ketoan}/permissions/assets`, body: { operations: ['read', 'approve'] } },
    ]))
    await waitFor(() => expect(search(router).edit).toBeUndefined())
    expect(search(router).role).toBe(RID.ketoan)
    const saved = useToastStore.getState().toasts.find((t) => t.message === 'Đã lưu quyền của Kế toán')
    expect(saved?.action?.label).toBe('Hoàn tác')
  })

  it('undoes a save by writing back what was there', async () => {
    const user = userEvent.setup()
    await renderAdmin(edit())
    await screen.findByRole('grid')
    await user.click(cell('Chia sẻ', 'Tài liệu'))
    await user.click(within(await screen.findByRole('region', { name: 'Thay đổi chưa lưu' })).getByRole('button', { name: 'Lưu' }))
    const toast = await waitFor(() => {
      const t = useToastStore.getState().toasts.find((x) => x.message === 'Đã lưu quyền của Kế toán')
      expect(t).toBeDefined()
      return t!
    })
    calls.length = 0
    act(() => toast.action!.onClick())
    await waitFor(() => expect(writes()).toEqual([
      { method: 'PUT', path: `/workspaces/${WS_ID}/roles/${RID.ketoan}/permissions/documents`, body: { operations: ['read', 'write', 'share'] } },
    ]))
  })

  it('cancels back to what is saved without writing anything', async () => {
    const user = userEvent.setup()
    await renderAdmin(edit())
    await screen.findByRole('grid')
    await user.click(cell('Duyệt', 'Tài sản'))
    await user.click(within(await screen.findByRole('region', { name: 'Thay đổi chưa lưu' })).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('region', { name: 'Thay đổi chưa lưu' })).toBeNull())
    expect(cell('Duyệt', 'Tài sản')).toHaveAttribute('aria-pressed', 'false')
    expect(writes()).toEqual([])
  })

  it('tells the person when saving fails, and leaves the editor showing what the server holds', async () => {
    mode.saveStatus = 409
    const user = userEvent.setup()
    await renderAdmin(edit())
    await screen.findByRole('grid')
    await user.click(cell('Duyệt', 'Tài sản'))
    await user.click(within(await screen.findByRole('region', { name: 'Thay đổi chưa lưu' })).getByRole('button', { name: 'Lưu' }))
    await waitFor(() => expect(useToastStore.getState().toasts.some((t) => /chưa lưu quyền của vai trò|Không lưu quyền của vai trò/.test(t.message))).toBe(true))
    expect(await screen.findByRole('grid')).toBeInTheDocument()
  })

  it('moves between cells with the arrow keys, skipping those that do not apply', async () => {
    const user = userEvent.setup()
    await renderAdmin(edit())
    await screen.findByRole('grid')
    cell('Sửa', 'Tài liệu').focus()
    await user.keyboard('{ArrowRight}')
    // "Duyệt" does not apply to Documents, so the next cell to the right is "Chia sẻ".
    expect(document.activeElement).toHaveAccessibleName('Chia sẻ trên Tài liệu')
    await user.keyboard('{ArrowDown}')
    // No area below offers "Chia sẻ": there is nowhere to go, and focus stays.
    expect(document.activeElement).toHaveAccessibleName('Chia sẻ trên Tài liệu')
    await user.keyboard('{ArrowLeft}{ArrowLeft}')
    expect(document.activeElement).toHaveAccessibleName('Xem trên Tài liệu')
    await user.keyboard('{ArrowDown}')
    expect(document.activeElement).toHaveAccessibleName('Xem trên Tin nhắn')
  })

  it('draws switches grouped by area on a phone, from the same answer', async () => {
    vi.stubGlobal('matchMedia', (q: string) => ({
      matches: q.includes('max-width: 767'), media: q, addEventListener: () => {}, removeEventListener: () => {},
    }))
    mode.areas = [{ area: 'documents', operations: ['read', 'share'] }]
    const user = userEvent.setup()
    await renderAdmin(edit())
    const group = await screen.findByRole('group', { name: 'Quyền của Kế toán' })
    expect(screen.queryByRole('grid')).toBeNull()
    expect(within(group).getAllByRole('switch').map((s) => s.getAttribute('aria-label'))).toEqual(['Xem trên Tài liệu', 'Chia sẻ trên Tài liệu'])
    expect(within(group).getByRole('switch', { name: 'Xem trên Tài liệu' })).toHaveAttribute('aria-checked', 'true')
    await user.click(within(group).getByRole('switch', { name: 'Chia sẻ trên Tài liệu' }))
    expect(await screen.findByRole('region', { name: 'Thay đổi chưa lưu' })).toBeInTheDocument()
  })

  it('does not edit a built-in role, and says why', async () => {
    await renderAdmin(edit(RID.owners))
    expect(await screen.findByText('Chủ sở hữu là vai trò hệ thống, không sửa được quyền.')).toBeInTheDocument()
    expect(screen.queryByRole('grid')).toBeNull()
  })

  it('says a role is gone when it is, and where to go', async () => {
    await renderAdmin(edit('55555555-aaaa-4bbb-8ccc-0000000009ff'))
    expect(await screen.findByText('Vai trò này không còn nữa.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Về danh sách vai trò' })).toBeInTheDocument()
  })

  it('has a skeleton while loading', async () => {
    api.mockImplementation((path, init) => (String(path).includes('/permission-areas') ? new Promise(() => {}) : adminFixtureApi(path, init)))
    await renderAdmin(edit())
    expect(await screen.findByLabelText('Đang tải quyền')).toHaveAttribute('aria-busy', 'true')
  })

  it('leaves for the role list from the breadcrumb', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(edit())
    await screen.findByRole('grid')
    await user.click(within(screen.getByRole('navigation', { name: 'Đường dẫn' })).getByRole('button', { name: 'Vai trò' }))
    await waitFor(() => expect(search(router).edit).toBeUndefined())
    expect(search(router).role).toBe(RID.ketoan)
  })
})

describe('the tabs', () => {
  it('move between the three screens and keep only the workspace', async () => {
    const user = userEvent.setup()
    const router = await renderAdmin(`/admin?ws=${WS_ID}&dept=${D.ketoan}`)
    await screen.findByRole('complementary', { name: 'Phòng ban' })
    await user.click(screen.getByRole('tab', { name: 'Người dùng' }))
    await within(await screen.findByRole('table')).findByText('Nguyễn Thu Lan')
    expect(router.state.location.pathname).toBe('/admin/users')
    expect(search(router)).toEqual({ ws: WS_ID })
    await user.click(screen.getByRole('tab', { name: 'Vai trò' }))
    await within(await screen.findByRole('table', { name: 'Vai trò' })).findByText('Kế toán')
    expect(screen.getByRole('tab', { name: 'Vai trò' })).toHaveAttribute('aria-selected', 'true')
  })
})
