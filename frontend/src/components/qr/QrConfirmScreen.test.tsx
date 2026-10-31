import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClientProvider } from '@tanstack/react-query'
import {
  Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, apiFetch } from '../../api/client'
import { queryClient } from '../../lib/query-client'
import { takeQrReturn } from '../../lib/qr-login'
import { useAuthStore } from '../../stores/auth.store'
import { expectNoIds } from '../../test/no-ids'
import { QrConfirmScreen } from './QrConfirmScreen'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))

const TOKEN = 'Abc_def-0123456789abcdefghijklmnopqrstuvwxy'
const ME_ID = '7f3c2a10-0000-4000-8000-000000000001'

let calls: { path: string; body?: unknown }[]
let scanError: Error | null
let approveError: Error | null

function fakeApi(path: string, options: RequestInit = {}): Promise<unknown> {
  const body = options.body ? JSON.parse(String(options.body)) : undefined
  calls.push({ path, body })
  if (path === '/me') {
    return Promise.resolve({ user: {
      id: ME_ID, username: 'hoa.le', ngac_node_id: 'n', email: 'hoa@novapay.vn', union_id: 'u',
      display_name: 'Lê Thị Hoa', title: 'Kế toán', location: '', avatar_url: '', email_verified: true, needs_profile: false,
    } })
  }
  if (path === '/auth/qr/scan') {
    if (scanError) return Promise.reject(scanError)
    return Promise.resolve({ browser: 'Chrome', os: 'Windows', created_at: new Date().toISOString(), tenant_name: 'NovaPay' })
  }
  if (path === '/auth/qr/approve') return approveError ? Promise.reject(approveError) : Promise.resolve({})
  if (path === '/auth/qr/decline') return Promise.resolve({})
  return Promise.reject(new Error(`unexpected ${path}`))
}

async function open(token = TOKEN) {
  const root = createRootRoute({ component: Outlet })
  const marker = (name: string) => () => <div data-testid="arrived">{name}</div>
  const qr = createRoute({
    getParentRoute: () => root, path: '/qr/$token',
    component: function Qr() { return <QrConfirmScreen token={qr.useParams().token} /> },
  })
  const others = ['/', '/login', '/scan', '/workspace-select'].map((p) =>
    createRoute({ getParentRoute: () => root, path: p, component: marker(p) }))
  const router = createRouter({
    routeTree: root.addChildren([qr, ...others]),
    history: createMemoryHistory({ initialEntries: [`/qr/${token}`] }),
  })
  await router.load()
  render(<QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider>)
  return router
}

const signIn = (signedIn: boolean) => useAuthStore.setState(signedIn
  ? { user: { id: ME_ID, username: 'hoa.le', ngac_node_id: 'n' }, accessToken: 'h.e30.s', tenantId: null, bootstrapping: false }
  : { user: null, accessToken: null, tenantId: null, bootstrapping: false })

beforeEach(() => {
  calls = []
  scanError = null
  approveError = null
  queryClient.clear()
  sessionStorage.clear()
  vi.mocked(apiFetch).mockImplementation(fakeApi as typeof apiFetch)
  signIn(true)
})
afterEach(() => vi.restoreAllMocks())

describe('Confirming a computer from the phone', () => {
  it('asks to sign in first, and comes back to this code afterwards', async () => {
    signIn(false)
    const router = await open()
    expect(await screen.findByRole('heading', { name: 'Đăng nhập trên điện thoại trước' })).toBeInTheDocument()
    expect(calls).toHaveLength(0)
    await userEvent.click(screen.getByRole('button', { name: 'Đăng nhập' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(takeQrReturn()).toBe(TOKEN)
  })

  it('names the computer, the account and the workspace, and warns about forwarded codes', async () => {
    await open()
    expect(await screen.findByText('Chrome trên Windows')).toBeInTheDocument()
    expect(calls.find((c) => c.path === '/auth/qr/scan')?.body).toEqual({ token: TOKEN })
    expect(await screen.findByText('Lê Thị Hoa')).toBeInTheDocument()
    expect(screen.getByText('NovaPay · Kế toán')).toBeInTheDocument()
    expect(screen.getByRole('note')).toHaveTextContent('Nếu ai đó gửi mã cho bạn quét, hãy huỷ.')
    expect(document.body.textContent).not.toContain(TOKEN)
    expectNoIds()
  })

  it('approves and says so', async () => {
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Đăng nhập' }))
    expect(await screen.findByRole('heading', { name: 'Đã đăng nhập trên máy tính' })).toBeInTheDocument()
    expect(calls.map((c) => c.path)).toContain('/auth/qr/approve')
    expect(screen.getByText('Chrome trên Windows đã vào NovaPay với tài khoản của bạn.')).toBeInTheDocument()
  })

  it('declines when Huỷ is tapped, and leaves', async () => {
    const router = await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Huỷ' }))
    await waitFor(() => expect(calls.map((c) => c.path)).toContain('/auth/qr/decline'))
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(calls.map((c) => c.path)).not.toContain('/auth/qr/approve')
  })

  it('says the code is no longer usable when the server does', async () => {
    scanError = new ApiError('gone', 410, { code: 'qr_unavailable' })
    await open()
    expect(await screen.findByRole('heading', { name: 'Mã này không còn dùng được' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Đăng nhập' })).not.toBeInTheDocument()
  })

  it('refuses a member who has left the workspace, at approval too', async () => {
    approveError = new ApiError('forbidden', 403, { code: 'access_denied' })
    await open()
    await userEvent.click(await screen.findByRole('button', { name: 'Đăng nhập' }))
    expect(await screen.findByRole('heading', { name: 'Không đăng nhập được vào NovaPay' })).toBeInTheDocument()
    expect(screen.queryByText('Đã đăng nhập trên máy tính')).not.toBeInTheDocument()
  })

  it('never sends a malformed token to the server', async () => {
    await open('not-a-token')
    expect(await screen.findByRole('heading', { name: 'Mã này không còn dùng được' })).toBeInTheDocument()
    expect(calls.filter((c) => c.path.startsWith('/auth/qr'))).toHaveLength(0)
  })
})
