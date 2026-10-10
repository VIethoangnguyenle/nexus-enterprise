import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { leaveFor, startGoogleSignIn } from '../../api/auth'
import { apiFetch } from '../../api/client'
import { useToastStore } from '../primitives'
import { tenantIdFromToken, useAuthStore } from '../../stores/auth.store'
import { ID, apiError, calls, invitation, jwtFor, world } from '../../test/auth-fixtures'
import { renderAuthScreen, startAuthTest, where } from '../../test/auth-render'
import { expectNoIds } from '../../test/no-ids'
import { WorkspaceSelectScreen } from './WorkspaceSelectScreen'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))
vi.mock('../../api/auth', async (orig) => ({
  ...(await orig<typeof import('../../api/auth')>()),
  startGoogleSignIn: vi.fn(),
  leaveFor: vi.fn(),
}))

const open = (url = '/workspace-select') => renderAuthScreen(WorkspaceSelectScreen, '/workspace-select', url)
const row = (name: string | RegExp) => screen.findByRole('button', { name })
const posts = (path: string | RegExp) =>
  calls.filter((c) => c.method === 'POST' && (typeof path === 'string' ? c.path === path : path.test(c.path)))
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)

beforeEach(() => {
  startAuthTest({ signedIn: true })
  // The session starts scoped to the first workspace.
  useAuthStore.setState({ accessToken: jwtFor(ID.wsOps), tenantId: ID.wsOps })
  useToastStore.getState().clear()
  vi.mocked(leaveFor).mockClear()
  vi.mocked(startGoogleSignIn).mockClear()
})
afterEach(() => vi.restoreAllMocks())

describe('Workspace selection: the list', () => {
  it('names each workspace with its headcount and the person’s role', async () => {
    await open()
    const ops = await row(/Khối Vận hành/)
    expect(within(ops).getByText('64 thành viên · Thành viên')).toBeInTheDocument()
    const project = await row(/Dự án Ví điện tử 2026/)
    expect(within(project).getByText('18 thành viên · Chủ sở hữu')).toBeInTheDocument()
    expect(screen.getByText(/Đăng nhập bằng hoa\.le@novapay\.vn/)).toBeInTheDocument()
  })

  it('names the company when the workspace claimed a domain, and only then', async () => {
    world.summaries = [
      { id: ID.wsOps, name: 'Khối Vận hành', role: 'member', member_count: 64, domain: 'novapay.vn' },
      { id: ID.wsProject, name: 'Dự án Ví điện tử 2026', role: 'owner', member_count: 18, domain: '' },
    ]
    await open()
    expect(within(await row(/Khối Vận hành/)).getByText('novapay.vn · 64 thành viên · Thành viên')).toBeInTheDocument()
    expect(within(await row(/Dự án Ví điện tử/)).getByText('18 thành viên · Chủ sở hữu')).toBeInTheDocument()
  })

  it('lists the person’s own memberships and nothing the graph merely reaches', async () => {
    // The workspace service would also list a third one; the server will not
    // re-scope the session to it, so it must not be offered.
    world.workspaces = [...world.workspaces, { id: '7c1d0000-1111-4a00-8000-00000000b999', name: 'Chỉ nằm trong đồ thị' }]
    await open()
    await row(/Khối Vận hành/)
    expect(screen.queryByText(/Chỉ nằm trong đồ thị/)).not.toBeInTheDocument()
    expect(calls.some((c) => c.path === '/me/workspaces')).toBe(true)
    expect(calls.some((c) => c.path === '/workspaces')).toBe(false)
  })

  it('shows the shape of the rows while they load', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    vi.mocked(apiFetch).mockImplementation(async (path, opts) => {
      if (path === '/me/workspaces') await gate
      return original(path, opts)
    })
    await open()
    await waitFor(() => expect(document.querySelectorAll('[data-skeleton]').length).toBeGreaterThan(0))
    act(() => release())
    await row(/Khối Vận hành/)
    expect(document.querySelectorAll('[data-skeleton]')).toHaveLength(0)
  })

  it('says it could not load, and retries', async () => {
    const healthy = world.summaries
    world.summaries = apiError(500)
    await open()
    expect(await screen.findByText(/Không tải được danh sách workspace/)).toBeInTheDocument()
    world.summaries = healthy
    await userEvent.click(screen.getByRole('button', { name: /Thử lại/ }))
    await row(/Khối Vận hành/)
  })

  it('says there is nothing yet, and offers to make one', async () => {
    world.summaries = []
    const router = await open()
    expect(await screen.findByText(/Bạn chưa thuộc workspace nào/)).toBeInTheDocument()
    expect(screen.getByText(/Nhờ quản trị viên mời hoa\.le@novapay\.vn/)).toBeInTheDocument()
    await userEvent.click(screen.getAllByRole('button', { name: /Tạo workspace/ })[0]!)
    await waitFor(() => expect(where(router).path).toBe('/onboarding'))
  })

  it('offers “Tạo workspace mới” below the list', async () => {
    const router = await open()
    await row(/Khối Vận hành/)
    await userEvent.click(screen.getByRole('button', { name: /Tạo workspace mới/ }))
    await waitFor(() => expect(where(router).path).toBe('/onboarding'))
  })

  it('has no “Join an Organization”, no plan label, no URL', async () => {
    await open()
    await row(/Khối Vận hành/)
    expect(document.body.textContent).not.toMatch(/Join an Organization|Free Tier|enterpriseflow|https?:\/\//i)
  })

  it('moves between rows with the arrow keys', async () => {
    await open()
    const first = await row(/Khối Vận hành/)
    first.focus()
    await userEvent.keyboard('{ArrowDown}')
    expect(await row(/Dự án Ví điện tử 2026/)).toHaveFocus()
    await userEvent.keyboard('{ArrowUp}')
    expect(first).toHaveFocus()
  })

  it('shows no identifier', async () => {
    world.invitations = [invitation()]
    await open()
    await row(/Khối Vận hành/)
    expectNoIds()
  })
})

describe('Workspace selection: entering a workspace', () => {
  it('re-scopes the session to a workspace it is not scoped to, then opens it', async () => {
    const router = await open()
    await userEvent.click(await row(/Dự án Ví điện tử 2026/))
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(posts('/auth/switch-tenant')[0]?.body).toEqual({ tenant_id: ID.wsProject })
    expect(where(router).search).toEqual({ ws: ID.wsProject })
    expect(tenantIdFromToken(useAuthStore.getState().accessToken)).toBe(ID.wsProject)
  })

  it('does not ask the server again for the workspace the session already speaks for', async () => {
    const router = await open()
    await userEvent.click(await row(/Khối Vận hành/))
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(posts('/auth/switch-tenant')).toHaveLength(0)
  })

  it('shows the row busy while it switches, and cannot be double-tapped', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    vi.mocked(apiFetch).mockImplementation(async (path, opts) => {
      if (path === '/auth/switch-tenant') await gate
      return original(path, opts)
    })
    await open()
    const project = await row(/Dự án Ví điện tử 2026/)
    await userEvent.click(project)
    await waitFor(() => expect(project).toBeDisabled())
    expect(await row(/Khối Vận hành/)).toBeDisabled()
    await userEvent.click(project)
    act(() => release())
    await waitFor(() => expect(posts('/auth/switch-tenant')).toHaveLength(1))
  })

  it('stays on the list and says why when the workspace cannot be opened', async () => {
    world.switchError = apiError(403, { code: 'access_denied' })
    const router = await open()
    await userEvent.click(await row(/Dự án Ví điện tử 2026/))
    expect(await screen.findByRole('alert')).toHaveTextContent(/không còn là thành viên/)
    expect(where(router).path).toBe('/workspace-select')
    expect((await row(/Dự án Ví điện tử 2026/))).toBeEnabled()
  })

  it('opens the workspace named in the link, even among several', async () => {
    const router = await open(`/workspace-select?ws=${ID.wsProject}`)
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(where(router).search).toEqual({ ws: ID.wsProject })
  })

  it('ignores a workspace in the link that the person does not belong to', async () => {
    const router = await open('/workspace-select?ws=00000000-0000-4000-8000-000000000000')
    await row(/Khối Vận hành/)
    expect(where(router).path).toBe('/workspace-select')
  })
})

describe('Workspace selection: going straight in', () => {
  beforeEach(() => {
    world.summaries = [{ id: ID.wsOps, name: 'Khối Vận hành', role: 'owner', member_count: 1, domain: '' }]
    // The workspace service lists more than the person's memberships: it must not matter.
    world.workspaces = [{ id: ID.wsOps, name: 'Khối Vận hành' }, { id: ID.wsProject, name: 'Dự án Ví điện tử 2026' }]
  })

  it('opens the only workspace when there is no invitation to answer', async () => {
    const router = await open()
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(where(router).search).toEqual({ ws: ID.wsOps })
  })

  it('stops at the list when there is an invitation to answer', async () => {
    world.invitations = [invitation()]
    const router = await open()
    await screen.findByText(/Trần Minh Đức/)
    expect(where(router).path).toBe('/workspace-select')
  })

  it('stops at the list when it cannot tell whether an invitation is waiting', async () => {
    world.invitationsError = apiError(500)
    const router = await open()
    await screen.findByText(/Không tải được lời mời/)
    expect(where(router).path).toBe('/workspace-select')
  })

  it('stops at the list for an unverified address, which is told why it sees no invitations', async () => {
    world.me.email_verified = false
    const router = await open()
    await screen.findByText(/sẽ hiện ở đây sau khi bạn xác minh/)
    expect(where(router).path).toBe('/workspace-select')
  })

  it('goes straight in for a phone account, which has no address to verify', async () => {
    world.me.email = ''
    world.me.email_verified = false
    const router = await open()
    await waitFor(() => expect(where(router).path).toBe('/channels'))
  })
})

describe('Workspace selection: invitations', () => {
  beforeEach(() => {
    world.invitations = [invitation(), invitation({ id: ID.invOther, workspace_name: 'Dự án Alpha', inviter_name: 'Lê Quang Vinh', role_name: '', expires_at: new Date(Date.now() + 3_600_000 * 2.5).toISOString() })]
  })

  it('lists each offer with who invited, to what, in which role and when it lapses', async () => {
    await open()
    const first = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    expect(first).toHaveTextContent('Trần Minh Đức mời bạn vào Kiểm soát nội bộ')
    expect(first).toHaveTextContent('Kế toán')
    expect(first).toHaveTextContent('Hết hạn sau 5 ngày')
    const second = screen.getByText(/Dự án Alpha/).closest('li')!
    expect(second).toHaveTextContent('Lê Quang Vinh mời bạn vào Dự án Alpha')
    expect(second).toHaveTextContent('Hết hạn sau 2 giờ')
  })

  it('puts the offers above the person’s own workspaces', async () => {
    await open()
    const offer = await screen.findByText(/Kiểm soát nội bộ/)
    const own = await row(/Khối Vận hành/)
    expect(offer.compareDocumentPosition(own) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('accepting joins, opens that workspace, and sends no body and no address', async () => {
    const router = await open()
    const offer = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    await userEvent.click(within(offer).getByRole('button', { name: 'Tham gia' }))
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(posts(`/invitations/${ID.invControl}/accept`)).toEqual([
      { method: 'POST', path: `/invitations/${ID.invControl}/accept`, body: undefined },
    ])
    expect(posts('/auth/switch-tenant')).toHaveLength(1)
    expect(String(where(router).search.ws)).toMatch(/^7c1d0000-1111-4a00-8000-0000000eee/)
  })

  it('shows the row busy while it joins', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    vi.mocked(apiFetch).mockImplementation(async (path, opts) => {
      if (/accept$/.test(path)) await gate
      return original(path, opts)
    })
    await open()
    const offer = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    await userEvent.click(within(offer).getByRole('button', { name: 'Tham gia' }))
    await waitFor(() => expect(within(offer).getByRole('button', { name: /Tham gia/ })).toBeDisabled())
    expect(within(offer).getByRole('button', { name: 'Từ chối' })).toBeDisabled()
    act(() => release())
    await waitFor(() => expect(posts(/accept$/)).toHaveLength(1))
  })

  it('tells the person when the role or department that came with the offer was not given', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    vi.mocked(apiFetch).mockImplementation(async (path, opts) => {
      const out = await original(path, opts)
      return /accept$/.test(path) ? { ...(out as object), role_applied: false } : out
    })
    await open()
    const offer = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    await userEvent.click(within(offer).getByRole('button', { name: 'Tham gia' }))
    await waitFor(() => expect(toasts().join(' ')).toMatch(/vai trò.*chưa được áp dụng/i))
  })

  it('declining removes the offer and says so', async () => {
    const router = await open()
    const offer = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    await userEvent.click(within(offer).getByRole('button', { name: 'Từ chối' }))
    await waitFor(() => expect(screen.queryByText(/Kiểm soát nội bộ/)).not.toBeInTheDocument())
    expect(posts(`/invitations/${ID.invControl}/decline`)).toHaveLength(1)
    expect(toasts()).toContain('Đã từ chối lời mời.')
    expect(screen.getByText(/Dự án Alpha/)).toBeInTheDocument()
    expect(where(router).path).toBe('/workspace-select')
  })

  it.each([
    [404, /không còn nữa/],
    [409, /đã được xử lý/],
    [400, /đã hết hạn/],
    [403, /không còn quyền mời/],
    [500, /Máy chủ đang gặp sự cố/],
  ])('says what happened when joining fails with %i, and stays', async (status, text) => {
    world.acceptError = apiError(status)
    const router = await open()
    const offer = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    await userEvent.click(within(offer).getByRole('button', { name: 'Tham gia' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(text)
    expect(where(router).path).toBe('/workspace-select')
    expect(document.body.textContent).not.toMatch(/server words/)
  })

  it('says what happened when declining fails', async () => {
    world.declineError = apiError(409)
    await open()
    const offer = (await screen.findByText(/Kiểm soát nội bộ/)).closest('li')!
    await userEvent.click(within(offer).getByRole('button', { name: 'Từ chối' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/đã được xử lý/)
  })

  it('says it could not load the offers, keeps the workspaces, and retries', async () => {
    world.invitationsError = apiError(500)
    await open()
    expect(await screen.findByText(/Không tải được lời mời/)).toBeInTheDocument()
    await row(/Khối Vận hành/)
    world.invitationsError = undefined
    await userEvent.click(screen.getByRole('button', { name: /Thử lại/ }))
    expect(await screen.findByText(/Trần Minh Đức/)).toBeInTheDocument()
  })

  it('shows no offers section when there are none', async () => {
    world.invitations = []
    await open()
    await row(/Khối Vận hành/)
    expect(screen.queryByText(/Lời mời/)).not.toBeInTheDocument()
  })
})

describe('Workspace selection: an address nobody has proved', () => {
  beforeEach(() => {
    world.me.email_verified = false
    // Offers exist, but the server shows none to an unverified account.
    world.invitations = [invitation()]
  })

  it('shows no offers, and says why in one calm line with the address', async () => {
    await open()
    expect(await screen.findByText(/Lời mời gửi tới hoa\.le@novapay\.vn sẽ hiện ở đây sau khi bạn xác minh/)).toBeInTheDocument()
    expect(screen.queryByText(/Trần Minh Đức/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tham gia' })).not.toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/999999|test mode/i)
  })

  it('shows none even if the server were to send some: the screen does not decide who may see an offer', async () => {
    const original = vi.mocked(apiFetch).getMockImplementation()!
    vi.mocked(apiFetch).mockImplementation(async (path, opts) =>
      path === '/invitations' ? { invitations: [invitation()] } : original(path, opts))
    await open()
    await screen.findByText(/sẽ hiện ở đây sau khi bạn xác minh/)
    expect(screen.queryByText(/Trần Minh Đức/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Tham gia' })).not.toBeInTheDocument()
  })

  it('offers Google as a way to prove the address of this account, which signs nobody in', async () => {
    world.providers = { google: true, otp: true, otp_fixed_code: true, otp_proves_email: false }
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Xác minh bằng Google/ }))
    await waitFor(() => expect(leaveFor).toHaveBeenCalledWith('https://accounts.example/auth?state=s'))
    expect(posts('/me/email/verify/google')).toEqual([{ method: 'POST', path: '/me/email/verify/google', body: undefined }])
    // "Sign in with Google" would end this session and start another person's.
    expect(startGoogleSignIn).not.toHaveBeenCalled()
  })

  it('says so when the server cannot start the Google check', async () => {
    world.providers = { google: true, otp: true, otp_fixed_code: true, otp_proves_email: false }
    world.googleVerifyError = apiError(503, { code: 'google_unavailable' })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Xác minh bằng Google/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/tạm ngưng/)
    expect(leaveFor).not.toHaveBeenCalled()
  })

  it('offers a code to the address only when a code would prove it', async () => {
    world.providers = { google: false, otp: true, otp_fixed_code: true, otp_proves_email: false }
    await open()
    await screen.findByText(/sẽ hiện ở đây sau khi bạn xác minh/)
    expect(screen.queryByRole('button', { name: /Gửi mã/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Google/ })).not.toBeInTheDocument()
    // It says honestly that nothing is available instead of pretending.
    expect(screen.getByText(/Hiện chưa có cách xác minh nào được bật/)).toBeInTheDocument()
  })

  it('proves the address through a delivered code, signs nobody in, and then shows the offers', async () => {
    world.providers = { google: false, otp: true, otp_fixed_code: false, otp_proves_email: true }
    const token = useAuthStore.getState().accessToken
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Gửi mã tới hoa\.le@novapay\.vn/ }))
    await waitFor(() => expect(posts('/me/email/verify')[0]?.body).toEqual({}))
    await userEvent.type(await screen.findByLabelText('Mã xác minh'), world.emailCode)

    expect(await screen.findByText(/Trần Minh Đức/)).toBeInTheDocument()
    expect(screen.queryByText(/sẽ hiện ở đây sau khi bạn xác minh/)).not.toBeInTheDocument()
    expect(toasts()).toContain('Đã xác minh email.')
    expect(posts('/me/email/verify')[1]?.body).toEqual({ code: world.emailCode })
    // Not a sign-in: no session was started, the token is the one it was.
    expect(posts('/auth/otp/request')).toHaveLength(0)
    expect(posts('/auth/otp/verify')).toHaveLength(0)
    expect(useAuthStore.getState().accessToken).toBe(token)
  })

  it('a wrong code is a wrong code, and the address stays unproved', async () => {
    world.providers = { google: false, otp: true, otp_fixed_code: false, otp_proves_email: true }
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Gửi mã tới hoa\.le@novapay\.vn/ }))
    await userEvent.type(await screen.findByLabelText('Mã xác minh'), '000000')
    expect(await screen.findByText(/Còn 4 lần thử/)).toBeInTheDocument()
    expect(screen.getByText(/sẽ hiện ở đây sau khi bạn xác minh/)).toBeInTheDocument()
  })

  it('says why no code was sent when the server cannot deliver one that proves anything', async () => {
    world.providers = { google: true, otp: true, otp_fixed_code: false, otp_proves_email: true }
    world.emailRequestError = apiError(503, { code: 'verification_unavailable' })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: /Gửi mã tới/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent(/Dùng Google/)
  })

  it('shows the way back from Google: proved, and a refusal that names the right account', async () => {
    world.me.email_verified = true
    world.invitations = [invitation()]
    const router = await open('/workspace-select?verified=1')
    expect(await screen.findByText(/Trần Minh Đức/)).toBeInTheDocument()
    await waitFor(() => expect(toasts()).toContain('Đã xác minh email.'))
    await waitFor(() => expect(where(router).search).toEqual({}))
  })

  it('explains a Google check that was refused, once, and clears it from the address', async () => {
    world.providers = { google: true, otp: true, otp_fixed_code: true, otp_proves_email: false }
    const router = await open('/workspace-select?verify_error=google_mismatch')
    expect(await screen.findByRole('alert')).toHaveTextContent('Chọn đúng tài khoản Google của hoa.le@novapay.vn')
    expect(document.body.textContent).not.toContain('google_mismatch')
    await waitFor(() => expect(where(router).search).toEqual({}))
    // Still unproved, and still offered the way to try again.
    expect(screen.getByRole('button', { name: /Xác minh bằng Google/ })).toBeInTheDocument()
  })

  it('does not suggest verifying a phone account, which has no address', async () => {
    world.me.email = ''
    world.invitations = []
    world.workspaces = [{ id: ID.wsOps, name: 'Khối Vận hành' }, { id: ID.wsProject, name: 'Dự án Ví điện tử 2026' }]
    await open()
    await row(/Khối Vận hành/)
    expect(screen.queryByRole('button', { name: /Xác minh|Gửi mã/ })).not.toBeInTheDocument()
    expect(screen.getByText(/đăng ký bằng số điện thoại/)).toBeInTheDocument()
  })
})

describe('Workspace selection: changing account', () => {
  it('ends the session through the one logout path and returns to sign-in', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(null, { status: 204 }))
    const router = await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Đổi tài khoản' }))
    await waitFor(() => expect(where(router).path).toBe('/login'))
    expect(fetchSpy).toHaveBeenCalledWith('/api/auth/logout', expect.objectContaining({ method: 'POST', credentials: 'include' }))
    expect(useAuthStore.getState().user).toBeNull()
  })
})
