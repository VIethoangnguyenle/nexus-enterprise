import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch } from '../../api/client'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
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
    expectNoIds()
  })

  it('saves the name and description from the form, sending only what changed', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    const name = await screen.findByRole('textbox', { name: 'Tên workspace' })
    await user.clear(name)
    await user.type(name, 'NovaPay Group')
    const save = screen.getByRole('button', { name: 'Lưu thay đổi' })
    expect(save).toBeEnabled()
    await user.click(save)
    await waitFor(() => expect(writesTo('/details')).toEqual([
      { method: 'PATCH', path: `/workspaces/${WS}/details`, body: { name: 'NovaPay Group' } },
    ]))
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

  it('who creates groups is picked from the drop-down and saved to the locks at once', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Ai được tạo nhóm' }), 'admins')
    await waitFor(() => expect(writesTo('/locks')).toEqual([
      { method: 'PUT', path: `/workspaces/${encodeURIComponent(WS)}/locks`, body: { group_creation: 'admins' } },
    ]))
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Ai được tạo nhóm' })).toHaveValue('admins'))
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
})
