import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, U, WS_ID } from '../../test/chat-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { ApiError, apiFetch } from '../../api/client'
import { validateSettingsSearch } from '../../lib/settings-search'
import { loadPreferences, switchPreferencesTo } from '../../lib/preferences'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { useToastStore } from '../primitives'
import { SettingsScreen } from './SettingsScreen'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn() }))
const api = vi.mocked(apiFetch)

interface Call { method: string; path: string; body?: Record<string, unknown> }
const calls: Call[] = []
let canManage = true
let details = { name: 'Khối Vận hành', description: 'Đối soát, thanh toán và hỗ trợ đối tác.' }
let profile = CONTACTS.map((c) => ({ ...c }))
let detailsFails: ApiError | null = null
let profileFails: ApiError | null = null
let detailsLoadFails = false
let leaveFails: ApiError | null = null
let assignedDepartment = 'Đối soát'

function fixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (path === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: details.name }] })
  if (path === `/workspaces/${WS_ID}/contacts`) return ok({ contacts: profile, total: profile.length })
  // The person's own record, with the department an administrator assigned here.
  if (path === `/me?workspace=${WS_ID}`) {
    const me = profile.find((c) => c.user_id === U.hoa.id)!
    return ok({
      user: {
        id: me.user_id, username: me.username, ngac_node_id: me.ngac_node_id, email: me.email, union_id: 'u',
        display_name: me.display_name, title: me.title, location: me.location, avatar_url: me.avatar_url,
        email_verified: true, needs_profile: false,
      },
      current_tenant: { id: WS_ID, name: details.name, role: 'member', open_id: 'o', department: assignedDepartment },
    })
  }
  if (path === `/workspaces/${WS_ID}/details`) {
    if (method === 'PATCH') {
      if (detailsFails) return Promise.reject(detailsFails)
      details = { ...details, ...body }
    } else if (detailsLoadFails) return Promise.reject(new ApiError('boom', 500))
    return ok({ ...details, can_manage: canManage })
  }
  if (path === '/me/profile' && method === 'PATCH') {
    if (profileFails) return Promise.reject(profileFails)
    profile = profile.map((c) => c.user_id === U.hoa.id ? {
      ...c,
      ...(body?.display_name ? { display_name: body.display_name as string } : {}),
      ...(body && 'title' in body ? { title: body.title as string } : {}),
      ...(body && 'location' in body ? { location: body.location as string } : {}),
    } : c)
    return ok({ status: 'updated' })
  }
  if (path === `/workspaces/${WS_ID}/leave` && method === 'POST') return leaveFails ? Promise.reject(leaveFails) : ok({ status: 'left' })
  if (path === `/workspaces/${WS_ID}/admin/members`) return ok({ members: Array.from({ length: 6 }, (_, i) => ({ ngac_node_id: `n${i}` })) })
  if (path === `/workspaces/${WS_ID}/roles`) return ok({ roles: [{ id: 'r1', kind: 'custom', name: 'Kế toán', ngac_node_id: 'x', member_count: 2 }], system_roles: [{ id: 'r2', kind: 'owners', ngac_node_id: 'y', member_count: 1 }] })
  return Promise.reject(new Error(`unexpected ${method} ${path}`))
}

async function renderSettings(url = '/settings') {
  const root = createRootRoute()
  const settings = createRoute({ getParentRoute: () => root, path: '/settings', validateSearch: validateSettingsSearch, component: SettingsScreen })
  const admin = createRoute({ getParentRoute: () => root, path: '/admin', component: () => <p>Quản trị</p> })
  const picker = createRoute({ getParentRoute: () => root, path: '/workspace-select', component: () => <p>Chọn workspace</p> })
  const router = createRouter({
    routeTree: root.addChildren([settings, admin, picker]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  renderWithClient(<RouterProvider router={router} />)
  return router
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  calls.length = 0
  canManage = true
  details = { name: 'Khối Vận hành', description: 'Đối soát, thanh toán và hỗ trợ đối tác.' }
  profile = CONTACTS.map((c) => ({ ...c, email: c.email, location: '' }))
  profile[0]!.email = 'hoa.le@novapay.vn'
  detailsFails = null
  profileFails = null
  detailsLoadFails = false
  leaveFails = null
  assignedDepartment = 'Đối soát'
  localStorage.clear()
  switchPreferencesTo(U.hoa.id)
  api.mockImplementation(fixtureApi)
})
afterEach(() => {
  cleanup() // unmount first, so nothing mounted refetches into the cleared cache
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
  switchPreferencesTo(undefined)
})

const patch = (path: string) => calls.filter((c) => c.method === 'PATCH' && c.path === path)
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)

describe('SettingsScreen: tabs', () => {
  it('opens Hồ sơ, with the three tabs in the mockup\'s order', async () => {
    await renderSettings()
    expect(await screen.findByRole('heading', { level: 1, name: 'Cài đặt' })).toBeInTheDocument()
    expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Hồ sơ', 'Workspace', 'Giao diện'])
    expect(screen.getByRole('tab', { name: 'Hồ sơ' })).toHaveAttribute('aria-selected', 'true')
    expect(await screen.findByRole('form', { name: 'Hồ sơ' })).toBeInTheDocument()
  })

  it('keeps the tab in the address, so a link or reload lands on it', async () => {
    const user = userEvent.setup()
    const router = await renderSettings()
    await user.click(await screen.findByRole('tab', { name: 'Giao diện' }))
    expect(router.state.location.search).toEqual({ tab: 'giao-dien' })
    expect(await screen.findByRole('radiogroup', { name: 'Chủ đề' })).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: 'Hồ sơ' }))
    expect(router.state.location.search).toEqual({})
  })

  it('opens the tab a link names', async () => {
    await renderSettings('/settings?tab=giao-dien')
    expect(await screen.findByRole('tab', { name: 'Giao diện', selected: true })).toBeInTheDocument()
  })

  it('opens Hồ sơ for a tab it does not know', async () => {
    await renderSettings('/settings?tab=nope')
    expect(await screen.findByRole('tab', { name: 'Hồ sơ', selected: true })).toBeInTheDocument()
  })
})

describe('SettingsScreen: Hồ sơ', () => {
  it('shows what can be changed, and what can only be seen with the reason, and no id', async () => {
    await renderSettings()
    const form = await screen.findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(within(form).getByLabelText(/Chức danh/)).toHaveValue('Kế toán trưởng')
    expect(within(form).getByText('hoa.le@novapay.vn')).toBeInTheDocument()
    expect(within(form).getByText(/Đổi email cần quản trị viên\./)).toBeInTheDocument()
    expect(within(form).getByText('Do quản trị viên đặt.')).toBeInTheDocument()
    // The department is the one an administrator assigned in this workspace.
    expect(within(form).getByText('Đối soát')).toBeInTheDocument()
    expect(within(form).queryByRole('textbox', { name: /Email/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /Đổi ảnh/ })).toBeNull() // there is no upload to offer
    expectNoIds()
  })

  it('reads the person from their own record, never by searching the directory', async () => {
    await renderSettings()
    const form = await screen.findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(calls.map((c) => c.path)).toContain(`/me?workspace=${WS_ID}`)
    expect(calls.filter((c) => c.path === `/workspaces/${WS_ID}/contacts`)).toHaveLength(0)
  })

  it('shows no department of its own when the administrator has not assigned one', async () => {
    assignedDepartment = ''
    await renderSettings()
    const form = await screen.findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(within(form).getByText('Chưa có')).toBeInTheDocument()
    expect(within(form).queryByRole('textbox', { name: /Phòng ban/ })).toBeNull()
  })

  it('never sends a department or an avatar, whatever was changed', async () => {
    const user = userEvent.setup()
    await renderSettings()
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.type(title, '!')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(patch('/me/profile')).toHaveLength(1))
    expect(Object.keys(patch('/me/profile')[0]!.body!)).toEqual(['title'])
  })

  it('cannot save until something changes', async () => {
    await renderSettings()
    const save = await screen.findByRole('button', { name: 'Lưu thay đổi' })
    expect(save).toBeDisabled()
    await userEvent.type(screen.getByLabelText(/Chức danh/), '!')
    expect(save).toBeEnabled()
  })

  it('sends only the field that changed, says so, and the new name shows everywhere', async () => {
    const user = userEvent.setup()
    await renderSettings()
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.clear(title)
    await user.type(title, 'Giám đốc tài chính')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))

    await waitFor(() => expect(patch('/me/profile')).toHaveLength(1))
    expect(patch('/me/profile')[0]!.body).toEqual({ title: 'Giám đốc tài chính' })
    await waitFor(() => expect(toasts()).toContain('Đã lưu hồ sơ'))
    // The person's own record was read again, and the directory, which is where
    // every other screen takes this person's name from, was refreshed too.
    await waitFor(() => expect(calls.filter((c) => c.path === `/me?workspace=${WS_ID}`).length).toBeGreaterThan(1))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Lưu thay đổi' })).toBeDisabled())
  })

  it('refuses an empty name when saving or leaving the field, and sends nothing', async () => {
    const user = userEvent.setup()
    await renderSettings()
    const name = await screen.findByLabelText('Tên hiển thị')
    await waitFor(() => expect(name).toHaveValue('Lê Thị Hoa'))
    await user.clear(name)
    await user.tab()
    expect(await screen.findByText('Nhập tên để mọi người nhận ra bạn.')).toBeInTheDocument()
    expect(name).toHaveAttribute('aria-invalid', 'true')

    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    expect(patch('/me/profile')).toHaveLength(0)
    expect(name).toHaveFocus()
  })

  it('keeps what was typed and explains when the server refuses', async () => {
    const user = userEvent.setup()
    profileFails = new ApiError('bad', 400)
    await renderSettings()
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.type(title, ' cấp cao')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))

    await waitFor(() => expect(toasts().some((m) => /Không lưu hồ sơ được/.test(m))).toBe(true))
    expect(title).toHaveValue('Kế toán trưởng cấp cao')
    expect(toasts()).not.toContain('Đã lưu hồ sơ')
  })

  it('Huỷ puts the stored values back', async () => {
    const user = userEvent.setup()
    await renderSettings()
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.type(title, 'xyz')
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(title).toHaveValue('Kế toán trưởng')
  })

  it('says it could not load, and retries', async () => {
    const user = userEvent.setup()
    let fail = true
    api.mockImplementation((path, init) =>
      fail && path.startsWith('/me?workspace=') ? Promise.reject(new ApiError('boom', 500)) : fixtureApi(path, init))
    await renderSettings()
    expect(await screen.findByText(/Không tải được hồ sơ của bạn/)).toBeInTheDocument()
    fail = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByLabelText('Tên hiển thị')).toBeInTheDocument()
  })
})

describe('SettingsScreen: Giao diện', () => {
  it('lists the three themes as radios, the device\'s choice first and selected', async () => {
    await renderSettings('/settings?tab=giao-dien')
    const group = await screen.findByRole('radiogroup', { name: 'Chủ đề' })
    expect(within(group).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Theo hệ thống', 'Sáng', 'Tối'])
    expect(within(group).getByRole('radio', { name: 'Theo hệ thống' })).toBeChecked()
  })

  it('applies a theme at once and keeps it for this person on this device', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=giao-dien')
    await user.click(await screen.findByRole('radio', { name: 'Tối' }))

    expect(document.documentElement.dataset.theme).toBe('dark')
    expect(screen.getByRole('radio', { name: 'Tối' })).toBeChecked()
    expect(loadPreferences(U.hoa.id).theme).toBe('dark')
    expect(loadPreferences('someone-else').theme).toBe('system')

    await user.click(screen.getByRole('radio', { name: 'Sáng' }))
    expect(document.documentElement.dataset.theme).toBeUndefined()
  })

  it('moves between the themes with the arrow keys', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=giao-dien')
    const themes = await screen.findByRole('radiogroup', { name: 'Chủ đề' })
    within(themes).getByRole('radio', { name: 'Theo hệ thống' }).focus()
    await user.keyboard('{ArrowRight}')
    expect(within(themes).getByRole('radio', { name: 'Sáng' })).toBeChecked()
    await user.keyboard('{ArrowRight}')
    expect(within(themes).getByRole('radio', { name: 'Tối' })).toBeChecked()
    await user.keyboard('{ArrowRight}')
    expect(within(themes).getByRole('radio', { name: 'Theo hệ thống' })).toBeChecked()
  })

  it('"Luôn bật" for reduced motion marks the page, and going back to the system clears it', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=giao-dien')
    await user.click(await screen.findByRole('radio', { name: 'Luôn bật' }))
    expect(document.documentElement.dataset.motion).toBe('reduce')
    await user.click(within(screen.getByRole('radiogroup', { name: 'Giảm chuyển động' })).getByRole('radio', { name: 'Theo hệ thống' }))
    expect(document.documentElement.dataset.motion).toBeUndefined()
  })

  it('collapses the sidebar with a switch that has a name', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=giao-dien')
    const sw = await screen.findByRole('switch', { name: 'Thu gọn sidebar' })
    expect(sw).not.toBeChecked()
    await user.click(sw)
    expect(sw).toBeChecked()
    expect(loadPreferences(U.hoa.id).sidebarCollapsed).toBe(true)
  })

  it('has no switch for realtime colour: it is information, not a preference', async () => {
    await renderSettings('/settings?tab=giao-dien')
    await screen.findByRole('radiogroup', { name: 'Chủ đề' })
    expect(screen.queryByText(/Màu theo người/)).toBeNull()
  })
})

describe('SettingsScreen: Workspace', () => {
  it('lets a manager change the name and description, sending only what changed', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=workspace')
    const name = await screen.findByLabelText('Tên workspace')
    await waitFor(() => expect(name).toHaveValue('Khối Vận hành'))
    await user.clear(name)
    await user.type(name, 'Khối Thanh toán')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))

    await waitFor(() => expect(patch(`/workspaces/${WS_ID}/details`)).toHaveLength(1))
    expect(patch(`/workspaces/${WS_ID}/details`)[0]!.body).toEqual({ name: 'Khối Thanh toán' })
    await waitFor(() => expect(toasts()).toContain('Đã lưu workspace'))
    // The switcher and sidebar read the list of workspaces: it was refreshed.
    await waitFor(() => expect(calls.filter((c) => c.path === '/workspaces').length).toBeGreaterThan(1))
    expect(queryClient.getQueryData(keys.workspaces.details(WS_ID))).toMatchObject({ name: 'Khối Thanh toán' })
  })

  it('changes the description alone', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=workspace')
    const desc = await screen.findByLabelText(/Mô tả/)
    await waitFor(() => expect(desc).toHaveValue('Đối soát, thanh toán và hỗ trợ đối tác.'))
    await user.clear(desc)
    await user.type(desc, 'Mô tả mới')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(patch(`/workspaces/${WS_ID}/details`)).toHaveLength(1))
    expect(patch(`/workspaces/${WS_ID}/details`)[0]!.body).toEqual({ description: 'Mô tả mới' })
  })

  it('refuses an empty name', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=workspace')
    const name = await screen.findByLabelText('Tên workspace')
    await waitFor(() => expect(name).toHaveValue('Khối Vận hành'))
    await user.clear(name)
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    expect(await screen.findByText('Nhập tên cho workspace.')).toBeInTheDocument()
    expect(patch(`/workspaces/${WS_ID}/details`)).toHaveLength(0)
  })

  it('explains a refusal from the server and keeps the typing', async () => {
    const user = userEvent.setup()
    detailsFails = new ApiError('denied', 403)
    await renderSettings('/settings?tab=workspace')
    const name = await screen.findByLabelText('Tên workspace')
    await waitFor(() => expect(name).toHaveValue('Khối Vận hành'))
    await user.type(name, ' 2')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(toasts().some((m) => /Bạn chưa có quyền lưu thông tin workspace/.test(m))).toBe(true))
    expect(name).toHaveValue('Khối Vận hành 2')
  })

  it('shows managers how big the workspace is, and where to manage it', async () => {
    await renderSettings('/settings?tab=workspace')
    expect(await screen.findByText('6 thành viên, 2 vai trò')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Mở Quản trị' })).toBeInTheDocument()
  })

  it('shows everyone else the same facts as text, with the reason, and nothing to save', async () => {
    canManage = false
    await renderSettings('/settings?tab=workspace')
    expect(await screen.findByText('Khối Vận hành')).toBeInTheDocument()
    expect(screen.getByText('Đối soát, thanh toán và hỗ trợ đối tác.')).toBeInTheDocument()
    expect(screen.getByText(/Chỉ người có quyền Quản lý mới đổi được/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Lưu thay đổi' })).toBeNull()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(calls.some((c) => c.path.endsWith('/admin/members'))).toBe(false) // the manager-only list is not even asked for
    expectNoIds()
  })

  it('offers no icon picker: it has no server behind it', async () => {
    await renderSettings('/settings?tab=workspace')
    await screen.findByLabelText('Tên workspace')
    expect(screen.queryByRole('button', { name: /Đổi biểu tượng/ })).toBeNull()
  })

  it('says it could not load, and retries', async () => {
    const user = userEvent.setup()
    detailsLoadFails = true
    await renderSettings('/settings?tab=workspace')
    expect(await screen.findByText(/Không tải được thông tin workspace/)).toBeInTheDocument()
    detailsLoadFails = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByLabelText('Tên workspace')).toBeInTheDocument()
  })
})

describe('SettingsScreen: Rời workspace', () => {
  const leaves = () => calls.filter((c) => c.method === 'POST' && c.path === `/workspaces/${WS_ID}/leave`)
  const openDialog = async (user: ReturnType<typeof userEvent.setup>) => {
    await user.click(await screen.findByRole('button', { name: 'Rời workspace' }))
    return screen.findByRole('dialog', { name: /^Rời / })
  }

  it('asks first, says what is lost, and sends nothing until confirmed', async () => {
    const user = userEvent.setup()
    await renderSettings('/settings?tab=workspace')
    const dialog = await openDialog(user)
    expect(within(dialog).getByText(/mất quyền truy cập mọi thứ trong/)).toBeInTheDocument()
    expect(leaves()).toHaveLength(0)

    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    expect(leaves()).toHaveLength(0)
  })

  it('leaves, says so, and goes to the workspace picker', async () => {
    const user = userEvent.setup()
    const router = await renderSettings('/settings?tab=workspace')
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
    expect(leaves()).toHaveLength(1)
    expect(leaves()[0]!.body).toBeUndefined() // nobody is named: it is always the caller
    expect(toasts().some((m) => /Bạn đã rời/.test(m))).toBe(true)
  })

  it('is there for a member without Quản lý too', async () => {
    const user = userEvent.setup()
    canManage = false
    const router = await renderSettings('/settings?tab=workspace')
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
  })

  it('tells the last Owner what to do instead, and stays', async () => {
    const user = userEvent.setup()
    leaveFails = new ApiError('last', 409, { reason: 'last_owner' })
    const router = await renderSettings('/settings?tab=workspace')
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))

    expect(await screen.findByText(/chủ sở hữu cuối cùng của workspace này/)).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/settings')

    // Opening it again starts clean.
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await openDialog(user)
    expect(screen.queryByText(/chủ sở hữu cuối cùng của workspace này/)).toBeNull()
  })

  it('explains any other refusal and stays', async () => {
    const user = userEvent.setup()
    leaveFails = new ApiError('boom', 500)
    const router = await renderSettings('/settings?tab=workspace')
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))
    await waitFor(() => expect(toasts().some((m) => /Máy chủ đang gặp sự cố/.test(m))).toBe(true))
    expect(router.state.location.pathname).toBe('/settings')
  })
})
