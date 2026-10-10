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

const open = () => renderAuthScreen(CreateWorkspaceScreen, '/onboarding')
const nameField = () => screen.findByLabelText(/Tên workspace/)
const create = () => screen.getByRole('button', { name: 'Tạo workspace' })
const posts = (path: string) => calls.filter((c) => c.method === 'POST' && c.path === path)

beforeEach(() => {
  startAuthTest({ signedIn: true })
  useAuthStore.setState({ accessToken: jwtFor(ID.wsOps), tenantId: ID.wsOps })
  useToastStore.getState().clear()
})
afterEach(() => vi.restoreAllMocks())

describe('Create workspace', () => {
  it('asks for a name and nothing else: no URL, no slug, no logo, no invites', async () => {
    await open()
    expect(await screen.findByRole('heading', { level: 1, name: 'Tạo workspace' })).toBeInTheDocument()
    expect(await nameField()).toHaveFocus()
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
    expect(document.body.textContent).not.toMatch(/URL|enterpriseflow|logo/i)
    expect(screen.queryByLabelText(/Mời|email/i)).not.toBeInTheDocument()
    expect(document.querySelector('input[type="file"]')).toBeNull()
  })

  it('sends only the name, trimmed, then opens the new workspace', async () => {
    const router = await open()
    await userEvent.type(await nameField(), '  Tổ Đối soát  {Enter}')
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(posts('/me/workspaces')).toEqual([{ method: 'POST', path: '/me/workspaces', body: { name: 'Tổ Đối soát' } }])
    const created = world.workspaces[world.workspaces.length - 1]!
    expect(created.name).toBe('Tổ Đối soát')
    expect(where(router).search).toEqual({ ws: created.id })
    // The session now speaks for the workspace it just opened.
    expect(posts('/auth/switch-tenant')[0]?.body).toEqual({ tenant_id: created.id })
    expect(tenantIdFromToken(useAuthStore.getState().accessToken)).toBe(created.id)
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

  it('shows the step bar only to someone who has no workspace yet', async () => {
    world.summaries = []
    await open()
    expect(await screen.findByRole('img', { name: 'Bước 3 trên 3' })).toBeInTheDocument()
  })

  it('shows no step bar to someone who already has workspaces', async () => {
    await open()
    await nameField()
    await waitFor(() => expect(calls.some((c) => c.path === '/me/workspaces')).toBe(true))
    expect(screen.queryByRole('img', { name: /Bước/ })).not.toBeInTheDocument()
  })

  it('shows no identifier', async () => {
    await open()
    await userEvent.type(await nameField(), 'Tổ Đối soát')
    expectNoIds()
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
