import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, logoutSession } from '../../api/client'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { SESSION_OTHER, WS, renderSettings, resetSettingsServer, server, settingsAnswer, viewport } from './settings-test-server'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

beforeEach(() => {
  viewport(true)
  resetSettingsServer()
  api.mockReset()
  api.mockImplementation(settingsAnswer as typeof apiFetch)
  vi.mocked(logoutSession).mockClear()
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
})

const rowNames = () => screen.getAllByRole('button').map((b) => b.textContent ?? '')
const hasRow = (label: string) => rowNames().some((t) => t.startsWith(label))

describe('Tôi', () => {
  it('in an organization: account rows, the workspace rows with Phê duyệt, Tài sản and Hồ sơ trong tổ chức, one Đăng xuất', async () => {
    await renderSettings(`/settings?ws=${WS}`)
    await screen.findByRole('heading', { name: 'Tôi' })
    await screen.findByText('Lê Thị Hoa')
    expect(await screen.findByText('NovaPay · Chủ sở hữu')).toBeInTheDocument()
    expect(screen.getByText('Kế toán trưởng · Phòng Tài chính')).toBeInTheDocument()
    for (const label of ['Tài khoản và hồ sơ', 'Thông báo', 'Giao diện, ngôn ngữ', 'Thiết bị và phiên', 'Hồ sơ trong tổ chức',
      'Chat', 'Lịch', 'Tài liệu', 'Phê duyệt', 'Tài sản', 'Cài đặt workspace']) {
      expect(hasRow(label), label).toBe(true)
    }
    expect(screen.getByRole('heading', { name: 'Trong NovaPay' })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Đăng xuất' })).toHaveLength(1)
    expect(screen.queryByText(/Thu gọn sidebar/)).toBeNull()
    expectNoIds()
  })

  it('in a personal workspace: no Hồ sơ trong tổ chức, Phê duyệt or Tài sản; Lịch says it holds only the personal calendar', async () => {
    server.personal = true
    await renderSettings(`/settings?ws=${WS}`)
    await screen.findByText('Workspace cá nhân')
    expect(await screen.findByText('hoa@novapay.vn')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Trong workspace này' })).toBeInTheDocument()
    for (const label of ['Hồ sơ trong tổ chức', 'Phê duyệt', 'Tài sản']) expect(hasRow(label), label).toBe(false)
    expect(screen.getByText('Chỉ lịch cá nhân')).toBeInTheDocument()
    expect(screen.getByText('Tên, dung lượng')).toBeInTheDocument()
  })

  it('signs out through the one logout path', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: 'Đăng xuất' }))
    expect(logoutSession).toHaveBeenCalledTimes(1)
  })

  it('pauses notifications until a moment, then shows until when', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Tạm dừng thông báo/ }))
    await user.click(await screen.findByRole('radio', { name: '1 giờ' }))
    await waitFor(() => expect(server.writes.some((w) => w.path === '/me/push-preferences')).toBe(true))
    const put = server.writes.find((w) => w.path === '/me/push-preferences')!.body as { pause_until: string; kinds: object }
    expect(Date.parse(put.pause_until) - Date.now()).toBeGreaterThan(59 * 60_000)
    expect(put.kinds).toEqual(server.push.kinds)
    expect(await screen.findByRole('button', { name: /^Tạm dừng thông báo\s*Đến \d\d:\d\d/ })).toBeInTheDocument()
  })

  it('on a wide screen /settings is the section list, not Tôi', async () => {
    viewport(false)
    await renderSettings(`/settings?ws=${WS}`)
    expect(await screen.findByRole('navigation', { name: 'Mục cài đặt' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Tôi' })).toBeNull()
  })
})

describe('Cài đặt workspace', () => {
  it('an organization administrator: information, members and organization, the rule, modules, storage and Rời', async () => {
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await screen.findByText('Bạn là Chủ sở hữu')
    for (const label of ['Tên workspace', 'Mô tả', 'Thành viên', 'Mời thành viên', 'Phòng ban', 'Vai trò và quyền', 'Phạm vi danh bạ',
      'Ai được tạo nhóm', 'Mẫu phê duyệt', 'Loại tài sản']) {
      expect(await screen.findByRole('button', { name: new RegExp(`^${label}`) }), label).toBeInTheDocument()
    }
    expect(await screen.findByText('2 lời mời đang chờ')).toBeInTheDocument()
    expect(await screen.findByText('2,4 GB / 10 GB · 312 tệp')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Rời NovaPay/ })).toBeInTheDocument()
    expect(screen.queryByText(/Chuyển thành workspace tổ chức/)).toBeNull()
    expectNoIds()
  })

  it('an administrator changes who creates groups; the choice is saved to the locks', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Ai được tạo nhóm/ }))
    await user.click(await screen.findByRole('radio', { name: /^Chỉ quản trị viên/ }))
    await waitFor(() => expect(server.writes).toContainEqual({ method: 'PUT', path: `/workspaces/${encodeURIComponent(WS)}/locks`, body: { group_creation: 'admins' } }))
    expect(await screen.findByRole('button', { name: /^Ai được tạo nhóm\s*Chỉ quản trị viên/ })).toBeInTheDocument()
  })

  it('a member: read-only information, the administrators, the rule locked without a name, Hồ sơ trong tổ chức and Rời; no admin rows', async () => {
    server.canManage = false
    server.role = 'member'
    server.groupCreation = 'admins'
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await screen.findByText('Bạn là Thành viên')
    const rule = (await screen.findByText('Ai được tạo nhóm')).closest('[aria-disabled]') as HTMLElement
    expect(rule).toHaveAttribute('aria-disabled', 'true')
    expect(within(rule).getByText('Chỉ quản trị viên')).toBeInTheDocument()
    expect(within(rule).getByText('Quản trị viên NovaPay đã đặt')).toBeInTheDocument()
    for (const name of server.ownerNames) expect(within(rule).queryByText(new RegExp(name))).toBeNull()
    expect(screen.queryByRole('button', { name: /^Ai được tạo nhóm/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /^Tên workspace/ })).toBeNull()
    for (const label of ['Thành viên và tổ chức', 'Mời thành viên', 'Vai trò và quyền', 'Mẫu phê duyệt', 'Dung lượng Tài liệu']) {
      expect(screen.queryByText(label), label).toBeNull()
    }
    expect(screen.getByRole('button', { name: /^Hồ sơ trong tổ chức/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Quản trị viên/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Rời NovaPay/ })).toBeInTheDocument()
    // Nothing a member does here reaches the admin endpoints.
    expect(api.mock.calls.some(([p]) => String(p).includes('/admin/members') || String(p).includes('/invitations'))).toBe(false)
  })

  it('a personal workspace: its name and storage only; no rule, members or Rời', async () => {
    server.personal = true
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await screen.findByText('Workspace cá nhân · chỉ bạn')
    expect(await screen.findByRole('button', { name: /^Tên workspace/ })).toBeInTheDocument()
    expect(await screen.findByText('Dung lượng Tài liệu')).toBeInTheDocument()
    expect(await screen.findByText('0 B / 5 GB · 0 tệp')).toBeInTheDocument()
    for (const label of ['Ai được tạo nhóm', 'Thành viên và tổ chức', 'Mô tả', 'Rời']) expect(screen.queryByText(new RegExp(`^${label}`)), label).toBeNull()
    expect(api.mock.calls.some(([p]) => String(p).includes('/locks'))).toBe(false)
  })

  it('renames the workspace on a page of its own: Lưu only once changed, back cancels', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/workspace?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Tên workspace/ }))
    const save = await screen.findByRole('button', { name: 'Lưu' })
    expect(save).toBeDisabled()
    const field = screen.getByRole('textbox', { name: 'Tên workspace' })
    await user.clear(field)
    await user.type(field, 'NovaPay Group')
    expect(save).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(server.writes.some((w) => w.path.endsWith('/details'))).toBe(false)
  })
})

describe('module pages', () => {
  it('Tài liệu: a switch saves only that key at once', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/drive?ws=${WS}`)
    const sw = await screen.findByRole('switch', { name: /^Tải ảnh xem trước/ })
    expect(sw).toHaveAttribute('aria-checked', 'true')
    await user.click(sw)
    await waitFor(() => expect(server.writes).toHaveLength(1))
    expect(server.writes[0]!.body).toEqual({ cellular_previews: false })
    expect(sw).toHaveAttribute('aria-checked', 'false')
  })

  it('offers no row that would change nothing: no sender for the share and asset notices, no link previews, no asset order, no approval tab', async () => {
    await renderSettings(`/settings/drive?ws=${WS}`)
    await screen.findByRole('switch', { name: 'Thư mục trước tệp' })
    expect(screen.queryByRole('switch', { name: 'Được chia sẻ tài liệu' })).toBeNull()
    expect(screen.queryByText('Thông báo')).toBeNull()
    cleanup()
    resetClient()

    await renderSettings(`/settings/assets?ws=${WS}`)
    await screen.findByRole('button', { name: /^Tab mở đầu/ })
    for (const label of ['Yêu cầu tài sản', 'Bảo trì']) expect(screen.queryByRole('switch', { name: new RegExp(`^${label}`) }), label).toBeNull()
    expect(screen.queryByRole('button', { name: /^Sắp xếp/ })).toBeNull()
    cleanup()
    resetClient()

    await renderSettings(`/settings/chat?ws=${WS}`)
    await screen.findByRole('switch', { name: /^Enter để gửi/ })
    expect(screen.queryByRole('switch', { name: 'Xem trước liên kết' })).toBeNull()
    cleanup()
    resetClient()

    await renderSettings(`/settings/approval?ws=${WS}`)
    expect(await screen.findByRole('button', { name: /^Sắp xếp\s*Áp dụng cho Chờ tôi duyệt/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /^Tab mở đầu/ })).toBeNull()
  })

  it('on a wide screen the options that only shape the phone are not offered', async () => {
    viewport(false)
    await renderSettings(`/settings/chat?ws=${WS}`)
    await screen.findByRole('heading', { level: 1, name: 'Chat' })
    expect(screen.queryByRole('switch', { name: /^Enter để gửi/ })).toBeNull()
    for (const label of ['Mở Chat ở', 'Sắp xếp', 'Cảm xúc khi chạm đúp']) expect(screen.queryByRole('button', { name: new RegExp(`^${label}`) }), label).toBeNull()
    cleanup()
    resetClient()

    await renderSettings(`/settings/calendar?ws=${WS}`)
    await screen.findByRole('radiogroup', { name: 'Tuần bắt đầu vào' })
    for (const label of ['Chế độ xem mặc định', 'Thời lượng mặc định']) expect(screen.queryByRole('button', { name: new RegExp(`^${label}`) }), label).toBeNull()
    cleanup()
    resetClient()

    await renderSettings(`/settings/approval?ws=${WS}`)
    await screen.findByRole('switch', { name: 'Đề nghị chờ bạn duyệt' })
    expect(screen.queryByRole('button', { name: /^Sắp xếp/ })).toBeNull()
  })

  it('Lịch: Giờ làm việc shows the hours and its page saves only the end picked', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/calendar?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Giờ làm việc\s*08:00–17:30/ }))
    expect(await screen.findByRole('heading', { name: 'Giờ làm việc' })).toBeInTheDocument()
    const start = screen.getByRole('radiogroup', { name: 'Bắt đầu' })
    expect(within(start).getByRole('radio', { name: '08:00' })).toHaveAttribute('aria-checked', 'true')
    const end = screen.getByRole('radiogroup', { name: 'Kết thúc' })
    await user.click(within(end).getByRole('radio', { name: '18:00' }))
    await waitFor(() => expect(server.writes).toEqual([
      { method: 'PUT', path: `/workspaces/${WS}/calendar/settings`, body: { workEnd: '18:00' } },
    ]))
    expect(within(end).getByRole('radio', { name: '18:00' })).toHaveAttribute('aria-checked', 'true')
    await user.click(screen.getByRole('button', { name: 'Quay lại' }))
    expect(await screen.findByRole('button', { name: /^Giờ làm việc\s*08:00–18:00/ })).toBeInTheDocument()
  })

  it('Lịch: an end not after the start is refused on the page and nothing is sent', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/calendar?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Giờ làm việc/ }))
    const end = await screen.findByRole('radiogroup', { name: 'Kết thúc' })
    await user.click(within(end).getByRole('radio', { name: '08:00' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Giờ kết thúc phải sau giờ bắt đầu.')
    expect(server.writes).toEqual([])
  })

  it('Tài liệu: a refused save puts the old value back', async () => {
    const user = userEvent.setup()
    server.settingsDown = true
    await renderSettings(`/settings/drive?ws=${WS}`)
    const sw = await screen.findByRole('switch', { name: 'Thư mục trước tệp' })
    await user.click(sw)
    await waitFor(() => expect(server.writes).toHaveLength(1))
    await waitFor(() => expect(sw).toHaveAttribute('aria-checked', 'true'))
  })

  it('Chat: a choice page picks, saves only that key and goes back', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/chat?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Sắp xếp/ }))
    await user.click(await screen.findByRole('radio', { name: 'Tên' }))
    await waitFor(() => expect(server.writes).toContainEqual({ method: 'PUT', path: `/workspaces/${encodeURIComponent(WS)}/chat-settings`, body: { sort: 'name' } }))
    expect(await screen.findByRole('heading', { name: 'Cài đặt Chat' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Sắp xếp\s*Tên/ })).toBeInTheDocument()
  })

  it('Chat: the push kinds are the account switches; turning one off sends the whole preferences', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/chat?ws=${WS}`)
    const dm = await screen.findByRole('switch', { name: 'Tin nhắn trực tiếp' })
    await waitFor(() => expect(dm).toBeEnabled())
    await user.click(dm)
    await waitFor(() => expect(server.writes.some((w) => w.path === '/me/push-preferences')).toBe(true))
    expect(server.push.kinds.direct).toBe(false)
    expect(server.push.quiet_hours.enabled).toBe(true)
  })

  it('Phê duyệt and Tài sản have no page in a personal workspace', async () => {
    server.personal = true
    const router = await renderSettings(`/settings/approval?ws=${WS}`)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  })

  it('an address naming no page lands on Cài đặt', async () => {
    const router = await renderSettings(`/settings/convert?ws=${WS}`)
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  })
})

describe('Thiết bị và phiên', () => {
  it('lists this device first and another by name, never by id; signs the other out after confirming', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/sessions?ws=${WS}`)
    expect(await screen.findByText('Máy này')).toBeInTheDocument()
    expect(screen.getByText('Safari · iPhone')).toBeInTheDocument()
    expect(screen.getByText('Chrome · Windows')).toBeInTheDocument()
    expectNoIds()
    await user.click(screen.getByRole('button', { name: 'Đăng xuất Chrome · Windows' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Đăng xuất' }))
    await waitFor(() => expect(server.writes).toContainEqual({ method: 'DELETE', path: `/auth/sessions/${SESSION_OTHER}`, body: undefined }))
    await waitFor(() => expect(screen.queryByText('Chrome · Windows')).toBeNull())
    expect(screen.getByText('Bạn không đăng nhập ở thiết bị nào khác.')).toBeInTheDocument()
  })

  it('offers no way to end the session in use', async () => {
    await renderSettings(`/settings/sessions?ws=${WS}`)
    await screen.findByText('Máy này')
    expect(screen.queryByRole('button', { name: 'Đăng xuất Safari · iPhone' })).toBeNull()
  })
})

describe('Thông báo', () => {
  it('quiet hours: a change carries the browser zone; a start equal to the end is refused here', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/notifications?ws=${WS}`)
    await user.click(await screen.findByRole('button', { name: /^Giờ yên lặng/ }))
    await user.selectOptions(await screen.findByRole('combobox', { name: 'Bắt đầu' }), '07:00')
    expect(await screen.findByRole('alert')).toHaveTextContent('không được trùng')
    expect(server.writes).toHaveLength(0)
    await user.selectOptions(screen.getByRole('combobox', { name: 'Bắt đầu' }), '21:00')
    await waitFor(() => expect(server.writes).toHaveLength(1))
    const sent = server.writes[0]!.body as { quiet_hours: { start: string; time_zone: string } }
    expect(sent.quiet_hours.start).toBe('21:00')
    expect(sent.quiet_hours.time_zone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Ho_Chi_Minh')
  })
})
