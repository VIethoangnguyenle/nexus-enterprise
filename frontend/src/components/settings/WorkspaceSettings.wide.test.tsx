import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch } from '../../api/client'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
import { GROUP_CREATION_CHOICES } from './GroupCreationRow'
import { WS, renderSettings, resetSettingsServer, server, settingsAnswer, viewport } from './settings-test-server'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

beforeEach(() => {
  viewport(false)
  resetSettingsServer()
  api.mockReset()
  api.mockImplementation(settingsAnswer as typeof apiFetch)
  useToastStore.getState().clear()
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
})

const page = () => screen.findByRole('region', { name: 'Cài đặt workspace' })
const writesTo = (suffix: string) => server.writes.filter((w) => w.path.split('?')[0]!.endsWith(suffix))
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)

describe('Cài đặt workspace on a wide screen: an organization administrator', () => {
  it('the header, Thông tin as a form, every admin row with its count, the rule, modules, the type card and Rời', async () => {
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const ws = await page()
    expect(within(ws).getByRole('heading', { level: 1, name: 'Cài đặt workspace' })).toBeInTheDocument()
    expect(await within(ws).findByText('Bạn là Chủ sở hữu')).toBeInTheDocument()

    expect(await within(ws).findByRole('textbox', { name: 'Tên workspace' })).toHaveValue('NovaPay')
    expect(within(ws).getByRole('textbox', { name: /Mô tả/ })).toHaveValue('Thanh toán số')
    expect(within(ws).getByRole('button', { name: 'Lưu thay đổi' })).toBeDisabled()

    for (const [label, value] of [['Thành viên', '7'], ['Phòng ban', '1'], ['Vai trò và quyền', '3'], ['Mẫu phê duyệt', '2'], ['Loại tài sản', '1']] as const) {
      expect(await within(ws).findByRole('button', { name: new RegExp(`^${label}\\s*${value}$`) }), label).toBeInTheDocument()
    }
    expect(within(ws).getByRole('button', { name: /^Phạm vi danh bạ/ })).toBeInTheDocument()
    expect(await within(ws).findByText('2 lời mời đang chờ')).toBeInTheDocument()
    expect(await within(ws).findByText('2,4 GB / 10 GB · 312 tệp')).toBeInTheDocument()
    expect(await within(ws).findByRole('combobox', { name: 'Ai được tạo nhóm' })).toHaveValue('everyone')

    expect(within(ws).getByRole('heading', { name: 'Loại workspace' })).toBeInTheDocument()
    expect(within(ws).getByRole('heading', { name: 'Workspace tổ chức' })).toBeInTheDocument()
    expect(within(ws).getByText('Rời NovaPay')).toBeInTheDocument()
    expect(within(ws).getByRole('button', { name: 'Rời workspace' })).toBeInTheDocument()
    // Subpages have no way back on a wide screen: none is offered.
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    // No icon picker: there is no server behind one.
    expect(screen.queryByRole('button', { name: /Đổi biểu tượng/ })).toBeNull()
    expectNoIds()
  })

  it('saves the name from the form, sending only what changed; the switcher and sidebar read the new name', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const name = await screen.findByRole('textbox', { name: 'Tên workspace' })
    await user.clear(name)
    await user.type(name, 'NovaPay Group')
    const save = screen.getByRole('button', { name: 'Lưu thay đổi' })
    expect(save).toBeEnabled()
    const lists = () => api.mock.calls.filter(([p]) => p === '/workspaces').length
    const before = lists()
    await user.click(save)
    await waitFor(() => expect(writesTo('/details')).toEqual([
      { method: 'PATCH', path: `/workspaces/${WS}/details`, body: { name: 'NovaPay Group' } },
    ]))
    await waitFor(() => expect(toasts()).toContain('Đã lưu workspace'))
    // The list of workspaces the switcher and sidebar read was asked for again.
    await waitFor(() => expect(lists()).toBeGreaterThan(before))
    expect(queryClient.getQueryData(keys.workspaces.details(WS))).toMatchObject({ name: 'NovaPay Group' })
  })

  it('changes the description alone', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const desc = await screen.findByRole('textbox', { name: /Mô tả/ })
    await user.clear(desc)
    await user.type(desc, 'Mô tả mới')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(writesTo('/details')).toEqual([
      { method: 'PATCH', path: `/workspaces/${WS}/details`, body: { description: 'Mô tả mới' } },
    ]))
  })

  it('explains a refusal from the server and keeps the typing', async () => {
    const user = userEvent.setup()
    server.detailsFails = new ApiError('denied', 403)
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const name = await screen.findByRole('textbox', { name: 'Tên workspace' })
    await user.type(name, ' 2')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(toasts().some((m) => /Bạn chưa có quyền lưu thông tin workspace/.test(m))).toBe(true))
    expect(name).toHaveValue('NovaPay 2')
  })

  it('says it could not load, and retries', async () => {
    const user = userEvent.setup()
    server.detailsDown = true
    await renderSettings(`/settings/workspace?ws=${WS}`)
    expect(await screen.findByText(/Không tải được thông tin workspace/)).toBeInTheDocument()
    server.detailsDown = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByRole('textbox', { name: 'Tên workspace' })).toBeInTheDocument()
  })

  it('refuses an empty name and sends nothing', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.clear(await screen.findByRole('textbox', { name: 'Tên workspace' }))
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    expect(await screen.findByText('Nhập tên cho workspace.')).toBeInTheDocument()
    expect(writesTo('/details')).toEqual([])
  })

  it('Mời opens the invite dialog in place', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    await screen.findByText('2 lời mời đang chờ')
    await user.click(screen.getByRole('button', { name: 'Mời' }))
    expect(await screen.findByRole('dialog', { name: 'Mời thành viên' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/settings/workspace')
  })

  it('locking who creates groups asks first, in the words of the choice, and saves only on Khoá', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const box = await screen.findByRole('combobox', { name: 'Ai được tạo nhóm' })
    await waitFor(() => expect(box).toHaveValue('everyone'))
    await user.selectOptions(box, 'admins')
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(GROUP_CREATION_CHOICES[1]!.hint!)).toBeInTheDocument()
    expect(writesTo('/locks')).toEqual([])
    expect(box).toHaveValue('everyone')

    await user.click(within(dialog).getByRole('button', { name: 'Khoá' }))
    await waitFor(() => expect(writesTo('/locks')).toEqual([
      { method: 'PUT', path: `/workspaces/${encodeURIComponent(WS)}/locks`, body: { group_creation: 'admins' } },
    ]))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await waitFor(() => expect(box).toHaveValue('admins'))
  })

  it('a change of the closed drop-down (an arrow key) to admins sends nothing; Huỷ keeps the old value', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const box = await screen.findByRole('combobox', { name: 'Ai được tạo nhóm' })
    await waitFor(() => expect(box).toHaveValue('everyone'))
    fireEvent.change(box, { target: { value: 'admins' } })
    const dialog = await screen.findByRole('dialog')
    expect(writesTo('/locks')).toEqual([])
    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(box).toHaveValue('everyone')
    expect(writesTo('/locks')).toEqual([])
  })

  it('opening it again to every member saves at once, without asking', async () => {
    const user = userEvent.setup()
    server.groupCreation = 'admins'
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const box = await screen.findByRole('combobox', { name: 'Ai được tạo nhóm' })
    await waitFor(() => expect(box).toHaveValue('admins'))
    await user.selectOptions(box, 'everyone')
    expect(screen.queryByRole('dialog')).toBeNull()
    await waitFor(() => expect(writesTo('/locks')).toEqual([
      { method: 'PUT', path: `/workspaces/${encodeURIComponent(WS)}/locks`, body: { group_creation: 'everyone' } },
    ]))
  })

  it('the drop-down is off while the rule is being saved, so a second pick cannot fire', async () => {
    const user = userEvent.setup()
    server.groupCreation = 'admins'
    let release!: () => void
    api.mockImplementation(((path: string, init?: RequestInit) => {
      if (path.split('?')[0]!.endsWith('/locks') && init?.method === 'PUT') {
        return new Promise<unknown>((resolve) => { release = () => resolve(settingsAnswer(path, init)) })
      }
      return settingsAnswer(path, init)
    }) as typeof apiFetch)
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const box = await screen.findByRole('combobox', { name: 'Ai được tạo nhóm' })
    await waitFor(() => expect(box).toHaveValue('admins'))
    await user.selectOptions(box, 'everyone')
    await waitFor(() => expect(box).toBeDisabled())
    release()
    await waitFor(() => expect(box).toBeEnabled())
    expect(writesTo('/locks')).toHaveLength(1)
  })

  it('the rows lead to the existing desktop screens, keeping the workspace', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Vai trò và quyền/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/roles'))
    expect(router.state.location.search).toEqual({ ws: WS })
  })

  it('Mẫu phê duyệt opens the templates tab of Phê duyệt', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Mẫu phê duyệt/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/approval'))
    expect(router.state.location.search).toEqual({ ws: WS, tab: 'templates' })
  })
})

describe('Cài đặt workspace on a wide screen: an organization member', () => {
  beforeEach(() => {
    server.canManage = false
    server.role = 'member'
    server.groupCreation = 'admins'
  })

  it('read-only information, the rule locked, Hồ sơ trong tổ chức and Rời; no admin rows, nothing to save', async () => {
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const ws = await page()
    expect(await within(ws).findByText('Bạn là Thành viên')).toBeInTheDocument()
    expect(await within(ws).findByText('NovaPay', { selector: 'span' })).toBeInTheDocument()
    expect(within(ws).getByText('Thanh toán số')).toBeInTheDocument()
    expect(within(ws).getAllByText('Do quản trị viên đặt')).toHaveLength(2)
    expect(within(ws).queryByRole('textbox')).toBeNull()
    expect(within(ws).queryByRole('button', { name: 'Lưu thay đổi' })).toBeNull()

    const rule = (await within(ws).findByText('Quản trị viên NovaPay đã đặt')).closest('[aria-disabled]') as HTMLElement
    expect(rule).toHaveAttribute('aria-disabled', 'true')
    expect(within(rule).getByText('Chỉ quản trị viên')).toBeInTheDocument()
    expect(within(ws).queryByRole('combobox')).toBeNull()

    for (const label of ['Thành viên và tổ chức', 'Mời thành viên', 'Phòng ban', 'Vai trò và quyền', 'Mẫu phê duyệt', 'Loại tài sản', 'Dung lượng Tài liệu']) {
      expect(within(ws).queryByText(label), label).toBeNull()
    }
    expect(within(ws).getByRole('button', { name: /^Hồ sơ trong tổ chức/ })).toBeInTheDocument()
    expect(within(ws).getByRole('heading', { name: 'Workspace tổ chức' })).toBeInTheDocument()
    expect(within(ws).getByRole('button', { name: 'Rời workspace' })).toBeInTheDocument()
    expect(api.mock.calls.some(([p]) => String(p).includes('/admin/members') || String(p).includes('/invitations'))).toBe(false)
    expectNoIds()
  })

  it('Quản trị viên opens the list in place, not on a page of its own', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    const row = await screen.findByRole('button', { name: /^Quản trị viên/ })
    for (const name of server.ownerNames) expect(screen.queryByText(name), name).toBeNull()
    await user.click(row)
    for (const name of server.ownerNames) expect(await screen.findByText(name), name).toBeInTheDocument()
    expect(screen.getByText(/Nhờ một quản trị viên/)).toBeInTheDocument()
    // Still the same page, with the rest of it.
    expect(screen.getByRole('heading', { level: 1, name: 'Cài đặt workspace' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rời workspace' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/settings/workspace')
    await user.click(row)
    for (const name of server.ownerNames) expect(screen.queryByText(name), name).toBeNull()
  })

  it('Quản trị viên says whether its list is open, and which list it opens', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const row = await screen.findByRole('button', { name: /^Quản trị viên/ })
    expect(row).toHaveAttribute('aria-expanded', 'false')
    const controls = row.getAttribute('aria-controls')
    expect(controls).toBeTruthy()
    await user.click(row)
    expect(row).toHaveAttribute('aria-expanded', 'true')
    const list = document.getElementById(controls!)
    expect(list).not.toBeNull()
    expect(within(list!).getByText(server.ownerNames[0]!)).toBeInTheDocument()
    await user.click(row)
    expect(row).toHaveAttribute('aria-expanded', 'false')
  })

  it('Hồ sơ trong tổ chức opens its section', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Hồ sơ trong tổ chức/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/org-profile'))
  })
})

describe('Cài đặt workspace on a wide screen: a personal workspace', () => {
  it('the name, storage and the type card; no description, rule, members or Rời', async () => {
    server.personal = true
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const ws = await page()
    expect(await within(ws).findByText('Workspace cá nhân · chỉ bạn')).toBeInTheDocument()
    expect(await within(ws).findByRole('textbox', { name: 'Tên workspace' })).toHaveValue('Lê Thị Hoa')
    expect(within(ws).queryByRole('textbox', { name: /Mô tả/ })).toBeNull()
    expect(await within(ws).findByText('0 B / 5 GB · 0 tệp')).toBeInTheDocument()
    expect(within(ws).getByRole('heading', { name: 'Workspace cá nhân' })).toBeInTheDocument()
    for (const label of ['Ai được tạo nhóm', 'Thành viên và tổ chức', 'Quản trị viên', 'Hồ sơ trong tổ chức']) {
      expect(within(ws).queryByText(label), label).toBeNull()
    }
    expect(within(ws).queryByText(/^Rời/)).toBeNull()
    expect(within(ws).queryByRole('button', { name: 'Rời workspace' })).toBeNull()
    expect(api.mock.calls.some(([p]) => String(p).includes('/locks'))).toBe(false)
    expectNoIds()
  })

  it('a legacy name made from an internal handle never reaches the screen, not even in the form', async () => {
    const user = userEvent.setup()
    server.personal = true
    server.workspaceName = "user_42's workspace"
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const ws = await page()
    const name = await within(ws).findByRole('textbox', { name: 'Tên workspace' })
    expect(name).toHaveValue('Không gian của bạn')
    expect(within(ws).getByText('Không gian của bạn', { selector: 'b' })).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/user_42/)
    expect(screen.queryByDisplayValue(/user_42/)).toBeNull()
    expectNoIds()
    // Nothing changed, nothing to save; a real new name is sent as typed.
    expect(within(ws).getByRole('button', { name: 'Lưu thay đổi' })).toBeDisabled()
    await user.clear(name)
    await user.type(name, 'Việc riêng')
    await user.click(within(ws).getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(writesTo('/details').map((w) => w.body)).toEqual([{ name: 'Việc riêng' }]))
  })

  it('an organization member reads the display name, not a legacy one', async () => {
    server.canManage = false
    server.role = 'member'
    server.workspaceName = "user_42's workspace"
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const ws = await page()
    expect(await within(ws).findByText('Bạn là Thành viên')).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/user_42/)
    expect(screen.queryByDisplayValue(/user_42/)).toBeNull()
    expectNoIds()
  })
})

describe('Rời workspace on a wide screen', () => {
  const leaves = () => server.writes.filter((w) => w.method === 'POST' && w.path === `/workspaces/${WS}/leave`)
  const openDialog = async (user: ReturnType<typeof userEvent.setup>) => {
    await user.click(await screen.findByRole('button', { name: 'Rời workspace' }))
    return screen.findByRole('dialog', { name: /^Rời / })
  }

  it('asks first, says what is lost, and sends nothing until confirmed', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const dialog = await openDialog(user)
    expect(within(dialog).getByText(/mất quyền truy cập mọi thứ trong/)).toBeInTheDocument()
    expect(leaves()).toHaveLength(0)
    await user.click(within(dialog).getByRole('button', { name: 'Huỷ' }))
    expect(leaves()).toHaveLength(0)
  })

  it('leaves, says so, and goes to the workspace picker', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
    expect(leaves()).toHaveLength(1)
    expect(leaves()[0]!.body).toBeUndefined() // nobody is named: it is always the caller
    expect(toasts().some((m) => /Bạn đã rời/.test(m))).toBe(true)
  })

  it('is there for a member without Quản lý too', async () => {
    const user = userEvent.setup()
    server.canManage = false
    server.role = 'member'
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/workspace-select'))
  })

  it('tells the last Owner what to do instead, and stays; opening it again starts clean', async () => {
    const user = userEvent.setup()
    server.leaveFails = new ApiError('last', 409, { reason: 'last_owner' })
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))
    expect(await screen.findByText(/chủ sở hữu cuối cùng của workspace này/)).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/settings/workspace')

    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    await openDialog(user)
    expect(screen.queryByText(/chủ sở hữu cuối cùng của workspace này/)).toBeNull()
  })

  it('explains any other refusal and stays', async () => {
    const user = userEvent.setup()
    server.leaveFails = new ApiError('boom', 500)
    const router = await renderSettings(`/settings/workspace?ws=${WS}`)
    const dialog = await openDialog(user)
    await user.click(within(dialog).getByRole('button', { name: 'Rời workspace' }))
    await waitFor(() => expect(toasts().some((m) => /Máy chủ đang gặp sự cố/.test(m))).toBe(true))
    expect(router.state.location.pathname).toBe('/settings/workspace')
  })
})

describe('Loại workspace on a wide screen: information, no conversion', () => {
  const card = () => screen.findByRole('region', { name: 'Loại workspace' })

  it('a personal workspace says what it has, for its owner, with nothing to convert it', async () => {
    server.personal = true
    server.detailsExtra = { is_owner: true, owner_name: 'Lê Thị Hoa' }
    await renderSettings(`/settings/workspace?ws=${WS}`)
    expect(await screen.findByRole('heading', { level: 3, name: 'Workspace cá nhân' })).toBeInTheDocument()
    expect(screen.getByText('Có Tin nhắn, Tài liệu và Danh bạ. Không có Tài sản, Phê duyệt và duyệt tài liệu.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Chuyển thành workspace tổ chức' })).toBeNull()
    expect(screen.queryByRole('alertdialog')).toBeNull()
  })

  it('a personal workspace seen by someone else has nothing to convert it either', async () => {
    server.personal = true
    server.detailsExtra = { is_owner: false, owner_name: 'Lê Thị Hoa' }
    await renderSettings(`/settings/workspace?ws=${WS}`)
    expect(await screen.findByRole('heading', { level: 3, name: 'Workspace cá nhân' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Chuyển thành workspace tổ chức' })).toBeNull()
  })

  it.each([
    ['no type', { type: undefined }],
    ['a type it does not know', { type: 'mystery' }],
  ])('shows no type card when the server sends %s', async (_case, extra) => {
    server.detailsExtra = extra
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await screen.findByRole('textbox', { name: 'Tên workspace' })
    expect(screen.queryByRole('heading', { level: 3, name: /Workspace (cá nhân|tổ chức)/ })).toBeNull()
  })

  it('an organization that was converted says who did it, and where to set Phê duyệt and Tài sản', async () => {
    server.detailsExtra = { converted_by_name: 'Lê Thị Hoa', converted_at: '2026-10-11T10:42:00Z' }
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const type = await card()
    expect(await within(type).findByRole('heading', { level: 3, name: 'Workspace tổ chức' })).toBeInTheDocument()
    expect(within(type).getByText(/đã chuyển sang workspace tổ chức/)).toBeInTheDocument()
    expect(within(type).getByText('Lê Thị Hoa')).toBeInTheDocument()
    expect(within(type).getByRole('link', { name: /Đặt quyền Phê duyệt và Tài sản/ })).toBeInTheDocument()
  })
})
