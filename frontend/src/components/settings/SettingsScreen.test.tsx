import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter, useSearch,
} from '@tanstack/react-router'
import { renderWithClient, resetClient } from '../../test/render'
import { CONTACTS, U, WS_ID } from '../../test/chat-fixtures'
import { expectNoIds } from '../../test/no-ids'
import { ApiError, apiFetch, apiUpload } from '../../api/client'
import type { WorkspaceDetails } from '../../api/workspaces'
import { tabOf, validateSettingsSearch, type SettingsSearch } from '../../lib/settings-search'
import { loadPreferences, switchPreferencesTo } from '../../lib/preferences'
import { queryClient } from '../../lib/query-client'
import { keys } from '../../hooks/keys'
import { useToastStore } from '../primitives'
import { AppSidebar } from '../patterns/AppSidebar'
import { PhoneSearchBar } from '../patterns/PhoneSearchBar'
import { AppearanceTab } from './AppearanceTab'
import { NotificationSettings } from './NotificationSettings'
import { ProfileTab } from './ProfileTab'
import { SettingsScreen } from './SettingsScreen'
import { SettingsSectionScreen } from './SettingsSectionScreen'
import { WorkspaceTab } from './WorkspaceTab'

vi.mock('../../api/client', async (orig) => ({ ...(await orig<typeof import('../../api/client')>()), apiFetch: vi.fn(), apiUpload: vi.fn() }))
const api = vi.mocked(apiFetch)
const upload = vi.mocked(apiUpload)

interface Call { method: string; path: string; body?: Record<string, unknown> }
const calls: Call[] = []
let canManage = true
let details: Omit<WorkspaceDetails, 'can_manage'> = { name: 'Khối Vận hành', description: 'Đối soát, thanh toán và hỗ trợ đối tác.' }
let profile = CONTACTS.map((c) => ({ ...c }))
let detailsFails: ApiError | null = null
let profileFails: ApiError | null = null
let detailsLoadFails = false
let leaveFails: ApiError | null = null
let assignedDepartment = 'Đối soát'
let avatarFails: ApiError | null = null
let removeFails: ApiError | null = null
const PICTURE = '/api/auth/avatars/0123456789abcdef0123456789abcdef'

const setHoaAvatar = (url: string) => {
  profile = profile.map((c) => c.user_id === U.hoa.id ? { ...c, avatar_url: url } : c)
}

/** PUT /me/avatar goes through apiUpload, as a form. */
function fixtureUpload(path: string, body: FormData, method = 'POST'): Promise<unknown> {
  calls.push({ method, path })
  if (path !== '/me/avatar' || method !== 'PUT' || !(body.get('file') instanceof File)) {
    return Promise.reject(new Error(`unexpected upload ${method} ${path}`))
  }
  if (avatarFails) return Promise.reject(avatarFails)
  setHoaAvatar(PICTURE)
  return Promise.resolve({ avatar_url: PICTURE })
}

function fixtureApi(path: string, init?: RequestInit): Promise<unknown> {
  const method = (init?.method || 'GET').toUpperCase()
  const body = typeof init?.body === 'string' ? JSON.parse(init.body) : undefined
  calls.push({ method, path, ...(body ? { body } : {}) })
  const ok = (v: unknown) => Promise.resolve(structuredClone(v))

  if (path === '/workspaces') return ok({ workspaces: [{ id: WS_ID, name: details.name, ...(details.type ? { type: details.type } : {}) }] })
  if (path === '/me/workspaces') {
    return ok({ workspaces: [{ id: WS_ID, name: details.name, role: 'owner', member_count: 1, domain: '', ...(details.type ? { type: details.type } : {}) }] })
  }
  if (path === '/approval/pending') return ok({ items: [], total: 0 })
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
  if (path === '/me/avatar' && method === 'DELETE') {
    if (removeFails) return Promise.reject(removeFails)
    setHoaAvatar('')
    return ok({ avatar_url: '' })
  }
  if (path === `/workspaces/${WS_ID}/leave` && method === 'POST') return leaveFails ? Promise.reject(leaveFails) : ok({ status: 'left' })
  if (path === `/workspaces/${WS_ID}/admin/members`) return ok({ members: Array.from({ length: 6 }, (_, i) => ({ ngac_node_id: `n${i}` })) })
  if (path === `/workspaces/${WS_ID}/roles`) return ok({ roles: [{ id: 'r1', kind: 'custom', name: 'Kế toán', ngac_node_id: 'x', member_count: 2 }], system_roles: [{ id: 'r2', kind: 'owners', ngac_node_id: 'y', member_count: 1 }] })
  return Promise.reject(new Error(`unexpected ${method} ${path}`))
}

/**
 * The panes of the former desktop tabs (Hồ sơ, Workspace, Giao diện, Thông báo), at the address that used to open
 * each. Desktop Cài đặt is now a section list; these panes stay in the tree, and are tested here on their own, until
 * their options are shown on the sections that replace them.
 */
function TabPane() {
  const tab = tabOf(useSearch({ strict: false }) as SettingsSearch)
  return (
    <>
      {tab === 'ho-so' && <ProfileTab />}
      {tab === 'workspace' && <WorkspaceTab />}
      {tab === 'giao-dien' && <AppearanceTab />}
      {tab === 'thong-bao' && <NotificationSettings />}
    </>
  )
}

async function renderSettings(url = '/settings', { withNav = false } = {}) {
  const root = createRootRoute(withNav
    ? { component: () => (<><AppSidebar workspaceName={details.name} /><PhoneSearchBar scope={null} /><Outlet /></>) }
    : undefined)
  const settings = createRoute({ getParentRoute: () => root, path: '/settings', validateSearch: validateSettingsSearch, component: TabPane })
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

/** Desktop Cài đặt as the app mounts it: `/settings` and `/settings/<section>`. */
async function renderShell(url = '/settings') {
  const root = createRootRoute()
  const settings = createRoute({ getParentRoute: () => root, path: '/settings', validateSearch: validateSettingsSearch, component: () => <Outlet /> })
  const index = createRoute({ getParentRoute: () => settings, path: '/', component: SettingsScreen })
  const section = createRoute({ getParentRoute: () => settings, path: '$section', component: SettingsSectionScreen })
  const router = createRouter({
    routeTree: root.addChildren([settings.addChildren([index, section])]),
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
  avatarFails = null
  removeFails = null
  upload.mockImplementation(fixtureUpload as typeof apiUpload)
  localStorage.clear()
  switchPreferencesTo(U.hoa.id)
  api.mockImplementation(fixtureApi)
})
afterEach(() => {
  vi.unstubAllEnvs()
  cleanup() // unmount first, so nothing mounted refetches into the cleared cache
  vi.restoreAllMocks()
  resetClient()
  useToastStore.getState().clear()
  switchPreferencesTo(undefined)
})

const patch = (path: string) => calls.filter((c) => c.method === 'PATCH' && c.path === path)
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)

describe('SettingsScreen: section list', () => {
  const sections = () => screen.findByRole('navigation', { name: 'Mục cài đặt' })

  it('opens Tài khoản và hồ sơ beside the section list, in place of the old tabs', async () => {
    await renderShell()
    expect(await screen.findByRole('heading', { level: 1, name: 'Cài đặt' })).toBeInTheDocument()
    const list = await sections()
    expect(within(list).getByRole('link', { name: 'Tài khoản và hồ sơ' })).toHaveAttribute('aria-current', 'page')
    for (const name of ['Thông báo', 'Giao diện, ngôn ngữ', 'Cài đặt workspace']) {
      expect(within(list).getByRole('link', { name })).toBeInTheDocument()
    }
    expect(screen.queryByRole('tab')).toBeNull()
    expect(await screen.findByRole('heading', { level: 1, name: 'Tài khoản và hồ sơ' })).toBeInTheDocument()
  })

  it('ends with the build the person is running, whatever section is open', async () => {
    vi.stubEnv('VITE_APP_VERSION', '2026.10.18 · 6368e8e')
    const user = userEvent.setup()
    await renderShell()
    expect(within(await sections()).getByText('Giới thiệu · 2026.10.18 · 6368e8e')).toBeInTheDocument()
    await user.click(within(await sections()).getByRole('link', { name: 'Giao diện, ngôn ngữ' }))
    expect(await screen.findByRole('heading', { level: 1, name: 'Giao diện, ngôn ngữ' })).toBeInTheDocument()
    expect(within(await sections()).getByText('Giới thiệu · 2026.10.18 · 6368e8e')).toBeInTheDocument()
  })

  it('keeps the section in the address, so a link or reload lands on it', async () => {
    const user = userEvent.setup()
    const router = await renderShell()
    await user.click(within(await sections()).getByRole('link', { name: 'Giao diện, ngôn ngữ' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/appearance'))
    expect(await screen.findByRole('switch', { name: 'Giảm chuyển động' })).toBeInTheDocument()

    await user.click(within(await sections()).getByRole('link', { name: 'Tài khoản và hồ sơ' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/account'))
    expect(router.state.location.search).toEqual({})
  })

  it('opens the section that holds what an old tab link names', async () => {
    const router = await renderShell('/settings?tab=giao-dien')
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/appearance'))
    expect(within(await sections()).getByRole('link', { name: 'Giao diện, ngôn ngữ' })).toHaveAttribute('aria-current', 'page')
  })

  it('opens Tài khoản và hồ sơ for a tab it does not know', async () => {
    const router = await renderShell('/settings?tab=nope')
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/account'))
    expect(within(await sections()).getByRole('link', { name: 'Tài khoản và hồ sơ' })).toHaveAttribute('aria-current', 'page')
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
    expectNoIds()
  })

  it('says nothing about departments in a personal workspace', async () => {
    details = { ...details, type: 'personal' }
    await renderSettings()
    const form = await screen.findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(within(form).queryByText('Phòng ban')).toBeNull()
    expect(screen.queryByText('Đối soát')).toBeNull()
    expect(screen.getByText('Kế toán trưởng')).toBeInTheDocument()
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

describe('SettingsScreen: ảnh đại diện', () => {
  const png = () => new File([new Uint8Array(2048)], 'anh.png', { type: 'image/png' })
  const picker = () => screen.getByLabelText('Tệp ảnh từ máy') as HTMLInputElement
  const sent = (method: string) => calls.filter((c) => c.method === method && c.path === '/me/avatar')
  const picture = () => screen.findByRole('button', { name: 'Đổi ảnh đại diện' })
  /** Waits for the picture to show, then opens the menu: it offers Bỏ ảnh. */
  const openWithPicture = async (user: ReturnType<typeof userEvent.setup>) => {
    await waitFor(() => expect(document.querySelector('img[src^="/api/auth/avatars/"]')).not.toBeNull())
    await user.click(await picture())
    expect(await screen.findByRole('menuitem', { name: 'Bỏ ảnh' })).toBeInTheDocument()
  }

  it('changes the picture by touching it: no Đổi ảnh or Bỏ ảnh buttons beside it, no hint line', async () => {
    await renderSettings()
    expect(await picture()).toHaveAttribute('aria-haspopup', 'menu')
    expect(screen.queryByRole('button', { name: 'Đổi ảnh' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Bỏ ảnh' })).toBeNull()
    expect(screen.queryByText(/JPG hoặc PNG, tối đa 5 MB/)).toBeNull()
    expect(picker()).toHaveAttribute('accept', 'image/jpeg,image/png')
  })

  it('shows the name beside the picture', async () => {
    await renderSettings()
    const button = await picture()
    expect(within(button.parentElement!.parentElement!).getByText('Lê Thị Hoa')).toBeInTheDocument()
  })

  it('offers only Tải ảnh lên while there is no picture', async () => {
    const user = userEvent.setup()
    await renderSettings()
    await user.click(await picture())
    const items = await screen.findAllByRole('menuitem')
    expect(items.map((i) => i.textContent)).toEqual(['Tải ảnh lên'])
  })

  it('uploads the chosen picture at once, says so, and the picture shows', async () => {
    const user = userEvent.setup()
    await renderSettings()
    await picture()
    await user.upload(picker(), png())

    await waitFor(() => expect(sent('PUT')).toHaveLength(1))
    await waitFor(() => expect(toasts()).toContain('Đã đổi ảnh'))
    // The person's own record was read again and now carries the picture: the menu offers Bỏ ảnh.
    await openWithPicture(user)
    expect(patch('/me/profile')).toHaveLength(0)
    expectNoIds(document.body)
  })

  it('refuses a file that is not a JPG or PNG without sending anything', async () => {
    const user = userEvent.setup({ applyAccept: false })
    await renderSettings()
    await picture()
    await user.upload(picker(), new File(['GIF89a'], 'anh.gif', { type: 'image/gif' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/JPG hoặc PNG/)
    expect(sent('PUT')).toHaveLength(0)
  })

  it('refuses a file over 5 MB without sending anything', async () => {
    const user = userEvent.setup()
    await renderSettings()
    await picture()
    await user.upload(picker(), new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'lon.png', { type: 'image/png' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/5 MB/)
    expect(sent('PUT')).toHaveLength(0)
  })

  it('explains in words when the server refuses the picture', async () => {
    avatarFails = new ApiError('the file is not a JPEG or PNG picture', 400, { code: 'avatar_not_image' })
    const user = userEvent.setup()
    await renderSettings()
    await picture()
    await user.upload(picker(), png())

    expect(await screen.findByRole('alert')).toHaveTextContent(/JPG hoặc PNG/)
    expect(screen.queryByText(/not a JPEG/)).toBeNull()
    expect(toasts()).not.toContain('Đã đổi ảnh')
  })

  it('asks to come back when the server is busy with another picture', async () => {
    avatarFails = new ApiError('busy', 503, { code: 'avatar_busy' })
    const user = userEvent.setup()
    await renderSettings()
    await picture()
    await user.upload(picker(), png())
    expect(await screen.findByRole('alert')).toHaveTextContent(/bận/)
  })

  it('Bỏ ảnh removes the picture and says so', async () => {
    setHoaAvatar(PICTURE)
    const user = userEvent.setup()
    await renderSettings()
    await openWithPicture(user)
    await user.click(screen.getByRole('menuitem', { name: 'Bỏ ảnh' }))

    await waitFor(() => expect(sent('DELETE')).toHaveLength(1))
    await waitFor(() => expect(toasts()).toContain('Đã bỏ ảnh'))
    // The picture is gone, so the menu is back to Tải ảnh lên.
    await waitFor(() => expect(document.querySelector('img[src^="/api/auth/avatars/"]')).toBeNull())
    await user.click(await picture())
    expect((await screen.findAllByRole('menuitem')).map((i) => i.textContent)).toEqual(['Tải ảnh lên'])
  })

  it('keeps the picture and says why when removing it fails', async () => {
    setHoaAvatar(PICTURE)
    removeFails = new ApiError('boom', 500)
    const user = userEvent.setup()
    await renderSettings()
    await openWithPicture(user)
    await user.click(screen.getByRole('menuitem', { name: 'Bỏ ảnh' }))

    await waitFor(() => expect(toasts().some((m) => /bỏ ảnh đại diện/.test(m))).toBe(true))
    expect(toasts()).not.toContain('Đã bỏ ảnh')
    await openWithPicture(user)
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
    expect(sw).toBeChecked() // the rail is the default
    await user.click(sw)
    expect(sw).not.toBeChecked()
    expect(loadPreferences(U.hoa.id).sidebarCollapsed).toBe(false)
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

describe('SettingsScreen: Loại workspace (informational, no conversion)', () => {
  it('shows personal workspace card for owner without a convert button', async () => {
    details = {
      name: 'Không gian của bạn',
      description: '',
      type: 'personal',
      is_owner: true,
      owner_name: 'Lê Thị Hoa',
    }
    await renderSettings('/settings?tab=workspace')

    expect(await screen.findByRole('heading', { level: 3, name: 'Workspace cá nhân' })).toBeInTheDocument()
    expect(screen.getByText('Có Tin nhắn, Tài liệu và Danh bạ. Không có Tài sản, Phê duyệt và duyệt tài liệu.')).toBeInTheDocument()
    // Conversion has been removed — no convert button should exist.
    expect(screen.queryByRole('button', { name: 'Chuyển thành workspace tổ chức' })).toBeNull()
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })

  it('shows personal workspace card for non-owner without owner explanation or convert button', async () => {
    details = {
      name: 'Không gian của bạn',
      description: '',
      type: 'personal',
      is_owner: false,
      owner_name: 'Lê Thị Hoa',
    }
    await renderSettings('/settings?tab=workspace')

    expect(await screen.findByRole('heading', { level: 3, name: 'Workspace cá nhân' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Chuyển thành workspace tổ chức' })).toBeNull()
  })

  it('shows no type card when the server sends no type or one it does not know', async () => {
    details = { name: 'Khối Vận hành', description: '' }
    await renderSettings('/settings?tab=workspace')
    await screen.findByRole('form', { name: 'Workspace' })
    expect(screen.queryByRole('heading', { level: 3, name: /Workspace (cá nhân|tổ chức)/ })).toBeNull()
  })

  it('shows organization card with audit line and admin link', async () => {
    details = {
      name: 'Khối Vận hành',
      description: '',
      type: 'organization',
      converted_by_name: 'Lê Thị Hoa',
      converted_at: '2026-10-11T10:42:00Z',
    }
    await renderSettings('/settings?tab=workspace')

    expect(await screen.findByRole('heading', { level: 3, name: 'Workspace tổ chức' })).toBeInTheDocument()
    expect(screen.getByText(/đã chuyển sang workspace tổ chức/)).toBeInTheDocument()
    expect(screen.getByText('Lê Thị Hoa')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Đặt quyền Phê duyệt và Tài sản/ })).toBeInTheDocument()
  })
})
