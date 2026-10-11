import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { tenantIdFromToken, useAuthStore } from '../../stores/auth.store'
import { ID, apiError, calls, jwtFor, world } from '../../test/auth-fixtures'
import { renderAuthScreen, startAuthTest, where } from '../../test/auth-render'
import { expectNoIds } from '../../test/no-ids'
import { useToastStore } from '../primitives'
import { CreateWorkspaceScreen } from './CreateWorkspaceScreen'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))
vi.mock('../../api/auth', async (orig) => ({
  ...(await orig<typeof import('../../api/auth')>()),
  leaveFor: vi.fn(),
}))

const open = (url = '/onboarding') => renderAuthScreen(CreateWorkspaceScreen, '/onboarding', url)
const nameField = () => screen.findByLabelText(/Tên workspace/)
const create = () => screen.getByRole('button', { name: /^Tạo workspace/ })
const posts = (path: string) => calls.filter((c) => c.method === 'POST' && c.path === path)

beforeEach(() => {
  startAuthTest({ signedIn: true })
  useAuthStore.setState({ accessToken: jwtFor(ID.wsOps), tenantId: ID.wsOps })
  useToastStore.getState().clear()
})
afterEach(() => vi.restoreAllMocks())

describe('Create workspace', () => {
  it('asks for type and name: no URL, no slug, no logo, no invites', async () => {
    await open()
    expect(await screen.findByRole('heading', { level: 1, name: 'Tạo workspace' })).toBeInTheDocument()
    expect(await nameField()).toBeInTheDocument()
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
    expect(await screen.findByRole('radio', { name: /Cá nhân/ })).toBeChecked()
    expect(screen.getByRole('radio', { name: /Tổ chức/ })).not.toBeChecked()
    expect(document.body.textContent).not.toMatch(/URL|enterpriseflow|logo/i)
    expect(screen.queryByLabelText(/Mời|email/i)).not.toBeInTheDocument()
    expect(document.querySelector('input[type="file"]')).toBeNull()
  })

  it('renders type cards defaulting to personal, switching to organization updates placeholder and button label', async () => {
    await open()
    const personalRadio = await screen.findByRole('radio', { name: /Cá nhân/ })
    const orgRadio = screen.getByRole('radio', { name: /Tổ chức/ })
    expect(personalRadio).toBeChecked()
    expect(await nameField()).toHaveAttribute('placeholder', 'Ví dụ: Việc nhà, Nhóm chạy bộ')
    expect(screen.getByRole('button', { name: 'Tạo workspace cá nhân' })).toBeInTheDocument()
    expect(screen.getByText(/Workspace cá nhân chuyển thành workspace tổ chức được về sau/)).toBeInTheDocument()

    await userEvent.click(orgRadio)
    expect(orgRadio).toBeChecked()
    expect(await nameField()).toHaveAttribute('placeholder', 'Ví dụ: Tổ Đối soát')
    expect(screen.getByRole('button', { name: 'Tạo workspace tổ chức' })).toBeInTheDocument()
  })

  it('sends name and type personal by default, then opens the new workspace', async () => {
    const router = await open()
    await userEvent.type(await nameField(), '  Việc nhà  {Enter}')
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(posts('/me/workspaces')).toEqual([{ method: 'POST', path: '/me/workspaces', body: { name: 'Việc nhà', type: 'personal' } }])
    const created = world.workspaces[world.workspaces.length - 1]!
    expect(created.name).toBe('Việc nhà')
    expect(where(router).search).toEqual({ ws: created.id })
    // The session now speaks for the workspace it just opened.
    expect(posts('/auth/switch-tenant')[0]?.body).toEqual({ tenant_id: created.id })
    expect(tenantIdFromToken(useAuthStore.getState().accessToken)).toBe(created.id)
  })

  it('sends type organization when selected', async () => {
    const router = await open()
    await userEvent.click(await screen.findByRole('radio', { name: /Tổ chức/ }))
    await userEvent.type(await nameField(), 'Tổ Đối soát{Enter}')
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(posts('/me/workspaces')).toEqual([{ method: 'POST', path: '/me/workspaces', body: { name: 'Tổ Đối soát', type: 'organization' } }])
  })

  it('shows the button busy and sends one request on a double tap', async () => {
    await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát')
    await userEvent.dblClick(create())
    await waitFor(() => expect(posts('/me/workspaces').length).toBeGreaterThan(0))
    expect(posts('/me/workspaces')).toHaveLength(1)
  })

  it('does not accept an empty or blank name, and says so', async () => {
    await open()
    await nameField()
    await userEvent.click(create())
    expect(await screen.findByText(/Nhập tên workspace/)).toBeInTheDocument()
    await userEvent.type(await nameField(), '   {Enter}')
    expect(screen.getByText(/Nhập tên workspace/)).toBeInTheDocument()
    expect(posts('/me/workspaces')).toHaveLength(0)
    expect(await nameField()).toHaveAttribute('aria-invalid', 'true')
  })

  it('stops at 80 characters, like the server, and shows the count', async () => {
    await open()
    const input = await nameField()
    expect(input).toHaveAttribute('maxlength', '80')
    await userEvent.type(input, 'x'.repeat(90))
    expect(input).toHaveValue('x'.repeat(80))
    expect(screen.getByText('80/80')).toBeInTheDocument()
  })

  it('shows a refused name under the field', async () => {
    world.createError = apiError(400, { code: 'invalid_input' })
    await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát{Enter}')
    expect(await screen.findByText(/Tên workspace chưa hợp lệ/)).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/invalid_input/)
  })

  it('says how long to wait when too many have been made', async () => {
    world.createError = apiError(429, { code: 'rate_limited', retry_after_seconds: 2400 })
    const router = await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent(/quá nhiều workspace.*40 phút/)
    expect(where(router).path).toBe('/onboarding')
  })

  it.each([
    ['a server fault', apiError(500, { code: 'internal' }), /Máy chủ đang gặp sự cố/],
    ['no network', new TypeError('Failed to fetch'), /Không kết nối được/],
  ])('shows %s as an alert and lets the person try again', async (_n, error, text) => {
    world.createError = error
    const router = await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent(text)
    world.createError = undefined
    await userEvent.click(create())
    await waitFor(() => expect(where(router).path).toBe('/channels'))
  })

  it('when the workspace exists but cannot be opened, says so and returns to the list', async () => {
    world.switchError = apiError(500)
    const router = await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát{Enter}')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(useToastStore.getState().toasts.map((t) => t.message).join(' ')).toMatch(/Đã tạo.*chưa mở được/)
  })

  it('goes back to the list', async () => {
    const router = await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Quay lại' }))
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
  })

  it('shows no identifier', async () => {
    await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát')
    expectNoIds()
  })

  describe('Back', () => {
    const back = async () => userEvent.click(await screen.findByRole('button', { name: 'Quay lại' }))

    it('returns to where the person came from, kept with its workspace', async () => {
      const router = await open('/onboarding?from=%2Fchannels%3Fws%3Dp-1')
      await back()
      await waitFor(() => expect(where(router).path).toBe('/channels'))
      expect(where(router).search).toEqual({ ws: 'p-1' })
    })

    it('goes to Chọn workspace when it was opened from there', async () => {
      const router = await open()
      await back()
      await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    })

    it.each(['https://evil.example/x', '//evil.example/x', 'channels'])('ignores a return address that leaves the app: %s', async (from) => {
      const router = await open(`/onboarding?from=${encodeURIComponent(from)}`)
      await back()
      await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    })
  })

  it('puts the first focus on the selected type on desktop, not in the name field', async () => {
    await open()
    const radio = await screen.findByRole('radio', { name: /Cá nhân/ })
    await waitFor(() => expect(radio).toHaveFocus())
  })

  it('outlines the selected type card with a ring width Tailwind generates (no ring-1.5)', async () => {
    await open()
    const radio = await screen.findByRole('radio', { name: /Cá nhân/ })
    const card = radio.closest('label')!
    expect(card).toHaveClass('ring-2', 'ring-accent')
    expect(document.body.innerHTML).not.toMatch(/ring-1\.5/)
  })

  it('shows a focus ring on the type card that holds the keyboard focus', async () => {
    await open()
    const radio = await screen.findByRole('radio', { name: /Cá nhân/ })
    expect(radio.closest('label')).toHaveClass('has-[:focus-visible]:outline-2')
  })
})

describe('Create workspace: an address nobody has proved', () => {
  beforeEach(() => {
    world.me.email_verified = false
    world.providers = { google: true, otp: true, otp_fixed_code: true, otp_proves_email: false }
  })

  it('shows the way to prove it instead of a form that could only be turned down', async () => {
    await open()
    expect(await screen.findByText(/Cần xác minh hoa\.le@novapay\.vn trước khi tạo workspace/)).toBeInTheDocument()
    expect(screen.queryByLabelText(/Tên workspace/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tạo workspace' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Xác minh bằng Google/ })).toBeInTheDocument()
    expect(posts('/me/workspaces')).toHaveLength(0)
  })

  it('offers no code when none could prove anything, and says nothing is available if Google is off too', async () => {
    world.providers = { google: false, otp: true, otp_fixed_code: true, otp_proves_email: false }
    await open()
    expect(await screen.findByText(/Hiện chưa có cách xác minh nào được bật/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Gửi mã/ })).not.toBeInTheDocument()
  })

  it('shows the form once the address is proved', async () => {
    world.providers = { google: false, otp: true, otp_fixed_code: false, otp_proves_email: true }
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Gửi mã tới/ }))
    await userEvent.type(await screen.findByLabelText('Mã xác minh'), world.emailCode)
    expect(await nameField()).toBeInTheDocument()
    expect(screen.queryByText(/Cần xác minh/)).not.toBeInTheDocument()
  })

  it('a phone account is told why it cannot, and offered no form', async () => {
    world.me.email = ''
    await open()
    expect(await screen.findByText(/đăng ký bằng số điện thoại nên chưa tạo được workspace/)).toBeInTheDocument()
    expect(screen.queryByLabelText(/Tên workspace/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Xác minh|Gửi mã/ })).not.toBeInTheDocument()
  })

  it('if the server still refuses (the address changed under it), says to prove it', async () => {
    world.me.email_verified = true
    world.createError = apiError(403, { code: 'email_unverified' })
    await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát{Enter}')
    expect(await screen.findByRole('alert')).toHaveTextContent(/xác minh email/)
  })
})

