import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch, apiUpload, logoutSession } from '../../api/client'
import { loadPreferences, switchPreferencesTo } from '../../lib/preferences'
import * as pushLib from '../../lib/push'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { resetClient } from '../../test/render'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
import {
  AVATAR_URL, SESSION_OTHER, WS, renderSettings, resetSettingsServer, server, settingsAnswer, settingsUpload, viewport,
} from './settings-test-server'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  apiUpload: vi.fn(),
  logoutSession: vi.fn(),
}))
const api = vi.mocked(apiFetch)

beforeEach(() => {
  viewport(false)
  resetSettingsServer()
  api.mockReset()
  api.mockImplementation(settingsAnswer as typeof apiFetch)
  vi.mocked(apiUpload).mockImplementation(settingsUpload as typeof apiUpload)
  vi.mocked(logoutSession).mockClear()
  useToastStore.getState().clear()
})
afterEach(() => {
  cleanup()
  resetClient()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

const page = (name: string) => screen.findByRole('region', { name })
const pushPuts = () => server.writes.filter((w) => w.method === 'PUT' && w.path === '/me/push-preferences')
/** Every PATCH of the person's own profile, as sent. */
const profilePatches = () => server.writes.filter((w) => w.method === 'PATCH' && w.path === '/me/profile').map((w) => w.body)
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)
/** The form of Tài khoản và hồ sơ, once the stored profile is in it. */
async function accountForm() {
  await renderSettings(`/settings/account?ws=${WS}`)
  const form = await screen.findByRole('form', { name: 'Hồ sơ' })
  await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
  return form
}

describe('Tài khoản và hồ sơ on a wide screen', () => {
  it('the form with the email read only and Đăng xuất last; in an organization no title, department or role', async () => {
    await renderSettings(`/settings/account?ws=${WS}`)
    const account = await page('Tài khoản và hồ sơ')
    expect(within(account).getByText('Dùng chung mọi workspace')).toBeInTheDocument()
    const form = await within(account).findByRole('form', { name: 'Hồ sơ' })
    await waitFor(() => expect(within(form).getByLabelText('Tên hiển thị')).toHaveValue('Lê Thị Hoa'))
    expect(within(form).getByLabelText(/Nơi làm việc/)).toHaveValue('')
    expect(within(form).getByText('hoa@novapay.vn')).toBeInTheDocument()
    expect(within(form).getByText(/Đổi email cần quản trị viên\./)).toBeInTheDocument()
    expect(within(form).queryByRole('textbox', { name: /Email/ })).toBeNull()
    // What belongs to the organization lives only on Hồ sơ trong tổ chức.
    expect(within(form).queryByLabelText(/Chức danh/)).toBeNull()
    expect(within(account).queryByText('Kế toán trưởng')).toBeNull()
    expect(within(account).queryByText('Phòng ban')).toBeNull()
    expect(within(account).queryByText('Phòng Tài chính')).toBeNull()
    expect(within(account).queryByText('Vai trò')).toBeNull()
    expect(within(account).queryByText('Chủ sở hữu')).toBeNull()
    const buttons = within(account).getAllByRole('button')
    expect(buttons[buttons.length - 1]).toHaveTextContent('Đăng xuất')
    // Nothing opens a page of its own: every field is edited in place.
    expect(screen.queryByRole('button', { name: 'Quay lại' })).toBeNull()
    expectNoIds()
  })

  it('a personal workspace keeps the title here, with no department and no role', async () => {
    server.personal = true
    const form = await accountForm()
    expect(within(form).getByLabelText(/Chức danh/)).toHaveValue('Kế toán trưởng')
    expect(within(form).queryByText('Phòng ban')).toBeNull()
    expect(within(form).queryByText('Vai trò')).toBeNull()
    expect(screen.queryByText('Phòng Tài chính')).toBeNull()
    // The title still shows under the name.
    expect(within(form).getByText('Kế toán trưởng')).toBeInTheDocument()
  })

  it('a personal workspace saves only the title when only the title changed', async () => {
    const user = userEvent.setup()
    server.personal = true
    await accountForm()
    const title = screen.getByLabelText(/Chức danh/)
    await user.clear(title)
    await user.type(title, 'Giám đốc tài chính')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches()).toEqual([{ title: 'Giám đốc tài chính' }]))
  })

  it('in an organization a save never sends the title, even when the stored one changed meanwhile', async () => {
    const user = userEvent.setup()
    const form = await accountForm()
    const name = within(form).getByLabelText('Tên hiển thị')
    await user.clear(name)
    await user.type(name, 'Hoa Lê')
    // Someone sets another title while this form is being edited; the form is not where the title lives.
    server.profile.title = 'Giám đốc'
    const reads = () => api.mock.calls.filter(([p]) => String(p).startsWith('/me?')).length
    const before = reads()
    await queryClient.invalidateQueries({ queryKey: keys.auth.profile(WS) })
    await waitFor(() => expect(reads()).toBeGreaterThan(before))
    await user.click(within(form).getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches()).toEqual([{ display_name: 'Hoa Lê' }]))
  })

  it('reads the person from their own record, never by searching the directory', async () => {
    await accountForm()
    const paths = api.mock.calls.map(([p]) => String(p))
    expect(paths).toContain(`/me?workspace=${WS}`)
    expect(paths.some((p) => p.includes('/contacts'))).toBe(false)
  })

  it('Lưu thay đổi and Huỷ are off until something changes', async () => {
    const user = userEvent.setup()
    await accountForm()
    const save = screen.getByRole('button', { name: 'Lưu thay đổi' })
    expect(save).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Huỷ' })).toBeDisabled()
    await user.type(screen.getByLabelText(/Nơi làm việc/), '!')
    expect(save).toBeEnabled()
  })

  it('Lưu thay đổi sends only the fields that changed', async () => {
    const user = userEvent.setup()
    await accountForm()
    await user.type(screen.getByLabelText(/Nơi làm việc/), 'Hà Nội')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches()).toEqual([{ location: 'Hà Nội' }]))
  })

  it('a save says so, reads the record again and leaves nothing to save', async () => {
    const user = userEvent.setup()
    await accountForm()
    const place = screen.getByLabelText(/Nơi làm việc/)
    await user.type(place, 'Hà Nội')
    const reads = () => api.mock.calls.filter(([p]) => String(p).startsWith('/me?')).length
    const before = reads()
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches()).toEqual([{ location: 'Hà Nội' }]))
    await waitFor(() => expect(toasts()).toContain('Đã lưu hồ sơ'))
    await waitFor(() => expect(reads()).toBeGreaterThan(before))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Lưu thay đổi' })).toBeDisabled())
    expect(place).toHaveValue('Hà Nội')
  })

  it('refuses an empty name when leaving the field or saving, and sends nothing', async () => {
    const user = userEvent.setup()
    await accountForm()
    const name = screen.getByLabelText('Tên hiển thị')
    await user.clear(name)
    await user.tab()
    expect(await screen.findByText('Nhập tên để mọi người nhận ra bạn.')).toBeInTheDocument()
    expect(name).toHaveAttribute('aria-invalid', 'true')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    expect(profilePatches()).toEqual([])
    expect(name).toHaveFocus()
  })

  it('keeps what was typed and explains when the server refuses', async () => {
    const user = userEvent.setup()
    server.profileFails = new ApiError('bad', 400)
    await accountForm()
    const place = screen.getByLabelText(/Nơi làm việc/)
    await user.type(place, 'Hà Nội')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(toasts().some((m) => /Không lưu hồ sơ được/.test(m))).toBe(true))
    expect(place).toHaveValue('Hà Nội')
    expect(toasts()).not.toContain('Đã lưu hồ sơ')
  })

  it('Huỷ puts the stored values back', async () => {
    const user = userEvent.setup()
    await accountForm()
    const name = screen.getByLabelText('Tên hiển thị')
    await user.type(name, 'xyz')
    await user.click(screen.getByRole('button', { name: 'Huỷ' }))
    expect(name).toHaveValue('Lê Thị Hoa')
  })

  it('says it could not load the profile, and retries', async () => {
    const user = userEvent.setup()
    server.meDown = true
    await renderSettings(`/settings/account?ws=${WS}`)
    expect(await screen.findByText(/Không tải được hồ sơ của bạn/)).toBeInTheDocument()
    server.meDown = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    expect(await screen.findByLabelText('Tên hiển thị')).toBeInTheDocument()
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

describe('Ảnh đại diện on a wide screen', () => {
  const png = () => new File([new Uint8Array(2048)], 'anh.png', { type: 'image/png' })
  const picker = () => screen.getByLabelText('Tệp ảnh từ máy') as HTMLInputElement
  const sent = (method: string) => server.writes.filter((w) => w.method === method && w.path === '/me/avatar')
  const picture = () => screen.findByRole('button', { name: 'Đổi ảnh đại diện' })
  /** Waits for the picture to show, then opens the menu: it offers Bỏ ảnh. */
  const openWithPicture = async (user: ReturnType<typeof userEvent.setup>) => {
    await waitFor(() => expect(document.querySelector('img[src^="/api/auth/avatars/"]')).not.toBeNull())
    await user.click(await picture())
    expect(await screen.findByRole('menuitem', { name: 'Bỏ ảnh' })).toBeInTheDocument()
  }

  it('changes the picture by touching it: no Đổi ảnh or Bỏ ảnh buttons beside it, no hint line', async () => {
    await renderSettings(`/settings/account?ws=${WS}`)
    expect(await picture()).toHaveAttribute('aria-haspopup', 'menu')
    expect(screen.queryByRole('button', { name: 'Đổi ảnh' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Bỏ ảnh' })).toBeNull()
    expect(screen.queryByText(/JPG hoặc PNG, tối đa 5 MB/)).toBeNull()
    expect(picker()).toHaveAttribute('accept', 'image/jpeg,image/png')
  })

  it('shows the name beside the picture', async () => {
    await renderSettings(`/settings/account?ws=${WS}`)
    const button = await picture()
    expect(within(button.parentElement!.parentElement!).getByText('Lê Thị Hoa')).toBeInTheDocument()
  })

  it('offers only Tải ảnh lên while there is no picture', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await user.click(await picture())
    expect((await screen.findAllByRole('menuitem')).map((i) => i.textContent)).toEqual(['Tải ảnh lên'])
  })

  it('uploads the chosen picture at once, says so, and the picture shows', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await picture()
    await user.upload(picker(), png())
    await waitFor(() => expect(sent('PUT')).toHaveLength(1))
    await waitFor(() => expect(toasts()).toContain('Đã đổi ảnh'))
    // The person's own record was read again and now carries the picture: the menu offers Bỏ ảnh.
    await openWithPicture(user)
    expect(profilePatches()).toEqual([])
    expectNoIds(document.body)
  })

  it('refuses a file that is not a JPG or PNG without sending anything', async () => {
    const user = userEvent.setup({ applyAccept: false })
    await renderSettings(`/settings/account?ws=${WS}`)
    await picture()
    await user.upload(picker(), new File(['GIF89a'], 'anh.gif', { type: 'image/gif' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/JPG hoặc PNG/)
    expect(sent('PUT')).toHaveLength(0)
  })

  it('refuses a file over 5 MB without sending anything', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await picture()
    await user.upload(picker(), new File([new Uint8Array(5 * 1024 * 1024 + 1)], 'lon.png', { type: 'image/png' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/5 MB/)
    expect(sent('PUT')).toHaveLength(0)
  })

  it('explains in words when the server refuses the picture', async () => {
    server.avatarFails = new ApiError('the file is not a JPEG or PNG picture', 400, { code: 'avatar_not_image' })
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await picture()
    await user.upload(picker(), png())
    expect(await screen.findByRole('alert')).toHaveTextContent(/JPG hoặc PNG/)
    expect(screen.queryByText(/not a JPEG/)).toBeNull()
    expect(toasts()).not.toContain('Đã đổi ảnh')
  })

  it('asks to come back when the server is busy with another picture', async () => {
    server.avatarFails = new ApiError('busy', 503, { code: 'avatar_busy' })
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await picture()
    await user.upload(picker(), png())
    expect(await screen.findByRole('alert')).toHaveTextContent(/bận/)
  })

  it('Bỏ ảnh removes the picture and says so', async () => {
    server.profile.avatar_url = AVATAR_URL
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
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
    server.profile.avatar_url = AVATAR_URL
    server.removeFails = new ApiError('boom', 500)
    const user = userEvent.setup()
    await renderSettings(`/settings/account?ws=${WS}`)
    await openWithPicture(user)
    await user.click(screen.getByRole('menuitem', { name: 'Bỏ ảnh' }))
    await waitFor(() => expect(toasts().some((m) => /bỏ ảnh đại diện/.test(m))).toBe(true))
    expect(toasts()).not.toContain('Đã bỏ ảnh')
    await openWithPicture(user)
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

  it('shows Chưa có for a department the administrator has not assigned, with nothing to type in', async () => {
    server.department = ''
    await renderSettings(`/settings/org-profile?ws=${WS}`)
    const form = await screen.findByRole('form', { name: 'Hồ sơ trong tổ chức' })
    await waitFor(() => expect(within(form).getByLabelText(/Chức danh/)).toHaveValue('Kế toán trưởng'))
    expect(within(form).getByText('Chưa có')).toBeInTheDocument()
    expect(within(form).getByText('Do quản trị viên đặt.')).toBeInTheDocument()
    expect(within(form).queryByRole('textbox', { name: /Phòng ban/ })).toBeNull()
  })

  it('saving sends only the title', async () => {
    const user = userEvent.setup()
    await renderSettings(`/settings/org-profile?ws=${WS}`)
    const title = await screen.findByLabelText(/Chức danh/)
    await waitFor(() => expect(title).toHaveValue('Kế toán trưởng'))
    await user.clear(title)
    await user.type(title, 'Giám đốc tài chính')
    await user.click(screen.getByRole('button', { name: 'Lưu thay đổi' }))
    await waitFor(() => expect(profilePatches()).toEqual([{ title: 'Giám đốc tài chính' }]))
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
    expect(profilePatches()).toEqual([])
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
    // A fixed morning: an hour later is still today, so the label has no date whatever the clock says.
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 20, 10, 0))
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
    await waitFor(() => expect((box as HTMLSelectElement).selectedOptions[0]!.textContent).toBe('Đến 11:00'))

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

  it('the pause and the quiet hours are off while the stored preferences are still being read', async () => {
    let answer!: () => void
    api.mockImplementation(((path: string, init?: RequestInit) => {
      if (path === '/me/push-preferences' && (init?.method ?? 'GET') === 'GET') {
        return new Promise<unknown>((resolve) => { answer = () => resolve(settingsAnswer(path, init)) })
      }
      return settingsAnswer(path, init)
    }) as typeof apiFetch)
    await renderSettings(`/settings/notifications?ws=${WS}`)
    const pause = await screen.findByRole('combobox', { name: 'Tạm dừng thông báo' })
    const start = screen.getByRole('combobox', { name: 'Bắt đầu' })
    const end = screen.getByRole('combobox', { name: 'Kết thúc' })
    const quiet = screen.getByRole('switch', { name: 'Giờ yên lặng' })
    for (const control of [pause, start, end, quiet]) expect(control).toBeDisabled()
    fireEvent.change(pause, { target: { value: '1h' } })
    fireEvent.change(start, { target: { value: '21:00' } })
    expect(pushPuts()).toEqual([])

    answer()
    await waitFor(() => expect(start).toHaveValue('22:00'))
    for (const control of [pause, start, end, quiet]) expect(control).toBeEnabled()
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

  it('says push is not on this environment, with no switch, when Nexus has no push service', async () => {
    await renderSettings(`/settings/notifications?ws=${WS}`)
    expect(await screen.findByText('Chưa có trên môi trường này')).toBeInTheDocument()
    expect(screen.queryByRole('switch', { name: 'Thông báo đẩy trên máy này' })).toBeNull()
  })

  it('says the preferences could not load, and retries', async () => {
    const user = userEvent.setup()
    let down = true
    api.mockImplementation(((path: string, init?: RequestInit) =>
      down && path === '/me/push-preferences' ? Promise.reject(new ApiError('boom', 500)) : settingsAnswer(path, init)) as typeof apiFetch)
    await renderSettings(`/settings/notifications?ws=${WS}`)
    expect(await screen.findByText(/Không tải được cài đặt thông báo/)).toBeInTheDocument()
    down = false
    await user.click(screen.getByRole('button', { name: 'Thử lại' }))
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'Bắt đầu' })).toHaveValue('22:00'))
    expect(screen.queryByText(/Không tải được cài đặt thông báo/)).toBeNull()
  })

  it('a refused save puts the switch back and says so', async () => {
    const user = userEvent.setup()
    server.settingsDown = true
    await renderSettings(`/settings/notifications?ws=${WS}`)
    const quiet = await screen.findByRole('switch', { name: 'Giờ yên lặng' })
    await waitFor(() => expect(quiet).toHaveAttribute('aria-checked', 'true'))
    await user.click(quiet)
    await waitFor(() => expect(toasts()).toContain('Không lưu được cài đặt thông báo'))
    await waitFor(() => expect(quiet).toHaveAttribute('aria-checked', 'true'))
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

    it('the switch opts this device out, and says so at once', async () => {
      const user = userEvent.setup()
      await renderSettings(`/settings/notifications?ws=${WS}`)
      const sw = await screen.findByRole('switch', { name: 'Thông báo đẩy trên máy này' })
      await waitFor(() => expect(sw).toHaveAttribute('aria-checked', 'true'))
      await user.click(sw)
      expect(pushLib.setSubscriptionOptIn).toHaveBeenCalledWith(false)
      expect(sw).toHaveAttribute('aria-checked', 'false')
    })

    it('the switch is off and says it is checking until the push service answers', async () => {
      vi.mocked(pushLib.isSubscriptionOptedIn).mockReturnValue(new Promise(() => {}))
      await renderSettings(`/settings/notifications?ws=${WS}`)
      expect(await screen.findByText(/Đang kiểm tra/)).toBeInTheDocument()
      expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên máy này' })).toBeDisabled()
    })

    it('a push service that cannot start says so and offers to try again, instead of checking forever', async () => {
      const user = userEvent.setup()
      const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
      const optedIn = vi.mocked(pushLib.isSubscriptionOptedIn)
      optedIn.mockReset().mockRejectedValueOnce(new Error('OneSignal init timed out')).mockResolvedValue(true)
      await renderSettings(`/settings/notifications?ws=${WS}`)
      expect(await screen.findByText('Chưa kết nối được dịch vụ thông báo')).toBeInTheDocument()
      expect(screen.queryByText(/Đang kiểm tra/)).toBeNull()
      expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên máy này' })).toBeDisabled()
      expect(warn).toHaveBeenCalledWith('[push] could not read device subscription', expect.any(Error))

      await user.click(screen.getByRole('button', { name: 'Thử kết nối lại' }))
      await waitFor(() => expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên máy này' })).toHaveAttribute('aria-checked', 'true'))
      expect(optedIn).toHaveBeenCalledTimes(2)
      expect(screen.queryByText('Chưa kết nối được dịch vụ thông báo')).toBeNull()
      expect(screen.queryByRole('button', { name: 'Thử kết nối lại' })).toBeNull()
    })

    it('says so when the push service fails after the first read', async () => {
      vi.spyOn(console, 'warn').mockImplementation(() => {})
      vi.mocked(pushLib.initOneSignal).mockRejectedValue(new Error('boom'))
      await renderSettings(`/settings/notifications?ws=${WS}`)
      expect(await screen.findByText('Chưa kết nối được dịch vụ thông báo')).toBeInTheDocument()
    })

    it('a blocked browser stays blocked, with no offer to try again', async () => {
      vi.spyOn(console, 'warn').mockImplementation(() => {})
      vi.mocked(pushLib.getNotificationPermission).mockReturnValue('denied')
      vi.mocked(pushLib.isSubscriptionOptedIn).mockRejectedValue(new Error('x'))
      await renderSettings(`/settings/notifications?ws=${WS}`)
      expect(await screen.findByText('Trình duyệt đang chặn')).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'Thử kết nối lại' })).toBeNull()
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

  describe('the choices, kept for the person on this device', () => {
    const open = () => renderSettings(`/settings/appearance?ws=${WS}`)
    beforeEach(() => {
      localStorage.clear()
      switchPreferencesTo('u-hoa')
    })
    afterEach(() => {
      localStorage.clear()
      switchPreferencesTo(undefined)
    })

    it('the device\'s choice of theme comes first and is selected', async () => {
      await open()
      const group = await screen.findByRole('radiogroup', { name: 'Chủ đề' })
      expect(within(group).getByRole('radio', { name: 'Theo hệ thống' })).toBeChecked()
    })

    it('a theme applies at once and is kept for this person only', async () => {
      const user = userEvent.setup()
      await open()
      await user.click(await screen.findByRole('radio', { name: 'Tối' }))
      expect(document.documentElement.dataset.theme).toBe('dark')
      expect(screen.getByRole('radio', { name: 'Tối' })).toBeChecked()
      expect(loadPreferences('u-hoa').theme).toBe('dark')
      expect(loadPreferences('someone-else').theme).toBe('system')
      await user.click(screen.getByRole('radio', { name: 'Sáng' }))
      expect(document.documentElement.dataset.theme).toBeUndefined()
    })

    it('the arrow keys move between the themes', async () => {
      const user = userEvent.setup()
      await open()
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
      await open()
      await user.click(await screen.findByRole('radio', { name: 'Luôn bật' }))
      expect(document.documentElement.dataset.motion).toBe('reduce')
      await user.click(within(screen.getByRole('radiogroup', { name: 'Giảm chuyển động' })).getByRole('radio', { name: 'Theo hệ thống' }))
      expect(document.documentElement.dataset.motion).toBeUndefined()
    })

    it('Thu gọn sidebar is a switch, on by default, and kept', async () => {
      const user = userEvent.setup()
      await open()
      const sw = await screen.findByRole('switch', { name: 'Thu gọn sidebar' })
      expect(sw).toBeChecked() // the rail is the default
      await user.click(sw)
      expect(sw).not.toBeChecked()
      expect(loadPreferences('u-hoa').sidebarCollapsed).toBe(false)
    })

    it('has no switch for realtime colour: it is information, not a preference', async () => {
      await open()
      await screen.findByRole('radiogroup', { name: 'Chủ đề' })
      expect(screen.queryByText(/Màu theo người/)).toBeNull()
    })
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
