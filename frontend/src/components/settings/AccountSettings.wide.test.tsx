import '@testing-library/jest-dom/vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { apiFetch, logoutSession } from '../../api/client'
import * as pushLib from '../../lib/push'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
import { SESSION_OTHER, WS, renderSettings, resetSettingsServer, server, settingsAnswer, viewport } from './settings-test-server'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

/** Every PATCH of the person's own profile, as sent. */
let profilePatches: unknown[] = []

beforeEach(() => {
  viewport(false)
  resetSettingsServer()
  profilePatches = []
  api.mockReset()
  api.mockImplementation(((path: string, init?: RequestInit) => {
    if (path === '/me/profile' && init?.method === 'PATCH') {
      profilePatches.push(JSON.parse(String(init.body)))
      return Promise.resolve({ status: 'updated' })
    }
    return settingsAnswer(path, init)
  }) as typeof apiFetch)
  vi.mocked(logoutSession).mockClear()
  useToastStore.getState().clear()
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const page = (name: string) => screen.findByRole('region', { name })
const pushPuts = () => server.writes.filter((w) => w.method === 'PUT' && w.path === '/me/push-preferences')

describe('Tài khoản và hồ sơ on a wide screen', () => {
  it('the form, what an administrator set read only with the role, and Đăng xuất last', async () => {
    await renderSettings(`/settings/account?ws=${WS}`)
    const account = await page('Tài khoản và hồ sơ')
    expect(within(account).getByText('Dùng chung mọi workspace')).toBeInTheDocument()
    const form = await within(account).findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(within(form).getByLabelText(/Chức danh/)).toHaveValue('Kế toán trưởng')
    expect(within(form).getByLabelText(/Nơi làm việc/)).toHaveValue('')
    expect(within(form).getByText('hoa@novapay.vn')).toBeInTheDocument()
    expect(within(form).queryByRole('textbox', { name: /Email/ })).toBeNull()
    expect(within(form).getByText('Phòng Tài chính')).toBeInTheDocument()
    expect(within(form).getByText('Vai trò')).toBeInTheDocument()
    expect(within(form).getByText('Chủ sở hữu')).toBeInTheDocument()
    const buttons = within(account).getAllByRole('button')
    expect(buttons[buttons.length - 1]).toHaveTextContent('Đăng xuất')
    // Nothing opens a page of its own: every field is edited in place.
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expectNoIds()
  })

  it('a personal workspace shows no department and no role', async () => {
    server.personal = true
    await renderSettings(`/settings/account?ws=${WS}`)
    const form = await screen.findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(within(form).queryByText('Phòng ban')).toBeNull()
    expect(within(form).queryByText('Vai trò')).toBeNull()
  })

  it('Lưu thay đổi sends only the fields that changed', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    const place = await screen.findByLabelText(/Nơi làm việc/)
    await waitFor(() => expect(screen.getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    await user.type(place, 'Hà Nội')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches).toEqual([{ location: 'Hà Nội' }]))
  })

  it('Đăng xuất signs out of this browser', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await screen.findByRole('form', { name: 'Hồ sơ' })
    const buttons = screen.getAllByRole('button', { name: 'Đăng xuất' })
    await user.click(buttons[buttons.length - 1]!)
    expect(logoutSession).toHaveBeenCalledOnce()
  })
})

describe('Hồ sơ trong tổ chức on a wide screen', () => {
  it('Chức danh in a form, the department and role read only, under the workspace', async () => {
    await renderSettings(`/settings/org-profile?ws=${WS}`)
    const org = await page('Hồ sơ trong tổ chức')
    expect(await within(org).findByText('Trong NovaPay')).toBeInTheDocument()
    await waitFor(() => expect(within(org).getByLabelText(/Chức danh/)).toHaveValue('Kế toán trưởng'))
    expect(within(org).queryByLabelText('Tên hiển thị')).toBeNull()
    expect(within(org).getByText('Phòng Tài chính')).toBeInTheDocument()
    expect(within(org).getByText('Chủ sở hữu')).toBeInTheDocument()
    expect(within(org).getByRole('button', { name: 'Lưu thay đổi' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expectNoIds()
  })

  it('saving sends only the title', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/org-profile?ws=${WS}`)
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.clear(title)
    await user.type(title, 'Giám đốc tài chính')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches).toEqual([{ title: 'Giám đốc tài chính' }]))
    await waitFor(() => expect(useToastStore.getState().toasts.map((t) => t.message)).toContain('Đã lưu hồ sơ'))
  })

  it('an empty title is fine, a too long one is refused and nothing is sent', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/org-profile?ws=${WS}`)
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.clear(title)
    await user.type(title, 'x'.repeat(121))
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    expect(await screen.findByText('Tối đa 120 ký tự.')).toBeInTheDocument()
    expect(profilePatches).toEqual([])
    expect(title).toHaveFocus()
  })
})

describe('Thông báo on a wide screen', () => {
  it('pause, quiet hours on one row, Theo phân hệ; no kind switches', async () => {
    await renderSettings(`/settings/notifications?ws=${WS}`)
    const notif = await page('Thông báo')
    expect(within(notif).getByText('Dùng chung mọi workspace')).toBeInTheDocument()
    expect(within(notif).getByRole('combobox', { name: 'Tạm dừng thông báo' })).toHaveValue('off')
    await waitFor(() => expect(within(notif).getByRole('combobox', { name: 'Bắt đầu' })).toHaveValue('22:00'))
    expect(within(notif).getByRole('combobox', { name: 'Kết thúc' })).toHaveValue('07:00')
    expect(within(notif).getByRole('switch', { name: 'Giờ yên lặng' })).toHaveAttribute('aria-checked', 'true')
    for (const label of ['Tin nhắn trực tiếp', 'Được nhắc tên', 'Thread bạn theo dõi', 'Cuộc gọi đến', 'Báo cho tôi khi']) {
      expect(within(notif).queryByText(label), label).toBeNull()
    }
    const modules = within(notif).getByRole('region', { name: 'Theo phân hệ' })
    for (const label of ['Chat', 'Lịch', 'Phê duyệt', 'Tài liệu', 'Tài sản']) {
      expect(within(modules).getByRole('button', { name: new RegExp(`^${label}`) }), label).toBeInTheDocument()
    }
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expectNoIds()
  })

  it('picking a pause changes only pause_until, then the box says until when', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/notifications?ws=${WS}`)
    const before = structuredClone(server.push)
    const box = await screen.findByRole('combobox', { name: 'Tạm dừng thông báo' })
    await user.selectOptions(box, '1h')
    await waitFor(() => expect(pushPuts()).toHaveLength(1))
    const sent = pushPuts()[0]!.body as typeof server.push
    expect(Date.parse(sent.pause_until!) - Date.now()).toBeGreaterThan(59 * 60_000)
    expect(sent.kinds).toEqual(before.kinds)
    expect(sent.quiet_hours).toEqual(before.quiet_hours)
    await waitFor(() => expect((box as HTMLSelectElement).selectedOptions[0]!.textContent).toMatch(/^Đến \d{2}:\d{2}$/))

    await user.selectOptions(box, 'off')
    await waitFor(() => expect(pushPuts()).toHaveLength(2))
    expect((pushPuts()[1]!.body as typeof server.push).pause_until).toBeNull()
    await waitFor(() => expect(box).toHaveValue('off'))
  })

  it('quiet hours: a time saves with the browser zone; off dims the times; equal times are refused here', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/notifications?ws=${WS}`)
    const start = await screen.findByRole('combobox', { name: 'Bắt đầu' })
    await waitFor(() => expect(start).toHaveValue('22:00'))
    await user.selectOptions(start, '07:00')
    expect(await screen.findByRole('alert')).toHaveTextContent('không được trùng')
    expect(pushPuts()).toHaveLength(0)
    await user.selectOptions(start, '21:00')
    await waitFor(() => expect(pushPuts()).toHaveLength(1))
    const sent = pushPuts()[0]!.body as typeof server.push
    expect(sent.quiet_hours.start).toBe('21:00')
    expect(sent.quiet_hours.time_zone).toBe(Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Ho_Chi_Minh')

    expect(start.closest('.opacity-50')).toBeNull()
    await user.click(screen.getByRole('switch', { name: 'Giờ yên lặng' }))
    await waitFor(() => expect(pushPuts()).toHaveLength(2))
    expect((pushPuts()[1]!.body as typeof server.push).quiet_hours.enabled).toBe(false)
    await waitFor(() => expect(screen.getByRole('switch', { name: 'Giờ yên lặng' })).toHaveAttribute('aria-checked', 'false'))
    expect(start.closest('.opacity-50')).not.toBeNull()
  })

  it('a Theo phân hệ row opens that module’s section', async () => {
    const user = userEvent.setup()
    const router = await renderSettings(`/settings/notifications?ws=${WS}`)
    const modules = await screen.findByRole('region', { name: 'Theo phân hệ' })
    await user.click(within(modules).getByRole('button', { name: /^Chat/ }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/settings/chat'))
    expect(router.state.location.search).toEqual({ ws: WS })
  })

  it('a personal workspace has no Phê duyệt or Tài sản row', async () => {
    server.personal = true
    await renderSettings(`/settings/notifications?ws=${WS}`)
    const modules = await screen.findByRole('region', { name: 'Theo phân hệ' })
    expect(within(modules).queryByRole('button', { name: /^Phê duyệt/ })).toBeNull()
    expect(within(modules).queryByRole('button', { name: /^Tài sản/ })).toBeNull()
    expect(within(modules).getByRole('button', { name: /^Tài liệu/ })).toBeInTheDocument()
  })

  describe('push on this browser', () => {
    beforeEach(() => {
      vi.spyOn(pushLib, 'getOneSignalAppId').mockReturnValue('app')
      vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('granted')
      vi.spyOn(pushLib, 'isIosNotInstalled').mockReturnValue(false)
      vi.spyOn(pushLib, 'isSubscriptionOptedIn').mockResolvedValue(true)
      vi.spyOn(pushLib, 'setSubscriptionOptIn').mockResolvedValue(undefined)
      vi.spyOn(pushLib, 'initOneSignal').mockResolvedValue(null)
    })

    it('a switch for this device', async () => {
      await renderSettings(`/settings/notifications?ws=${WS}`)
      const sw = await screen.findByRole('switch', { name: 'Thông báo đẩy trên máy này' })
      await waitFor(() => expect(sw).toHaveAttribute('aria-checked', 'true'))
      expect(screen.queryByText(/đang chặn thông báo/)).toBeNull()
    })

    it('a browser that blocks notifications gets the three steps to allow them', async () => {
      vi.mocked(pushLib.getNotificationPermission).mockReturnValue('denied')
      await renderSettings(`/settings/notifications?ws=${WS}`)
      const banner = (await screen.findByText(/Trình duyệt đang chặn thông báo của Nexus/)).closest('[role="status"]') as HTMLElement
      expect(within(banner).getAllByRole('listitem')).toHaveLength(3)
      expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên máy này' })).toBeDisabled()
    })

    it('an iPhone without Nexus on the home screen is told to add it', async () => {
      vi.mocked(pushLib.isIosNotInstalled).mockReturnValue(true)
      await renderSettings(`/settings/notifications?ws=${WS}`)
      expect(await screen.findByText('Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone.')).toBeInTheDocument()
    })
  })
})

describe('Giao diện, ngôn ngữ on a wide screen', () => {
  it('theme cards, reduced motion, language and the sidebar switch', async () => {
    await renderSettings(`/settings/appearance?ws=${WS}`)
    const look = await page('Giao diện, ngôn ngữ')
    expect(within(look).getByText('Dùng chung mọi workspace')).toBeInTheDocument()
    const themes = within(look).getByRole('radiogroup', { name: 'Chủ đề' })
    expect(within(themes).getAllByRole('radio').map((r) => r.textContent)).toEqual(['Theo hệ thống', 'Sáng', 'Tối'])
    expect(within(look).getByRole('radiogroup', { name: 'Giảm chuyển động' })).toBeInTheDocument()
    expect(within(look).getByRole('radiogroup', { name: 'Ngôn ngữ' })).toBeInTheDocument()
    expect(within(look).getByRole('switch', { name: 'Thu gọn sidebar' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
  })
})

describe('Thiết bị và phiên on a wide screen', () => {
  it('lists the devices; signing another out asks first', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/sessions?ws=${WS}`)
    const sessions = await page('Thiết bị và phiên')
    expect(within(sessions).getByText('Dùng chung mọi workspace')).toBeInTheDocument()
    expect(await within(sessions).findByText('Safari · iPhone')).toBeInTheDocument()
    await user.click(within(sessions).getByRole('button', { name: 'Đăng xuất Chrome · Windows' }))
    const dialog = await screen.findByRole('dialog')
    expect(server.writes).toEqual([])
    await user.click(within(dialog).getByRole('button', { name: 'Đăng xuất' }))
    await waitFor(() => expect(server.writes).toContainEqual({ method: 'DELETE', path: `/auth/sessions/${SESSION_OTHER}`, body: undefined }))
    await waitFor(() => expect(screen.queryByText('Chrome · Windows')).toBeNull())
    expectNoIds()
  })
})
