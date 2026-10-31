import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, publicFetch } from '../../api/client'
import { QR_MAX_RENEWALS } from '../../hooks/useQrLogin'
import { authFixtureApi } from '../../test/auth-fixtures'
import { renderAuthScreen, startAuthTest, where } from '../../test/auth-render'
import { expectNoIds } from '../../test/no-ids'
import { useAuthStore } from '../../stores/auth.store'
import { LoginScreen } from './LoginScreen'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))

// A fake auth service for the QR routes; everything else is the shared fixture.
let server: {
  creates: number
  expiresIn: number
  status: Record<string, unknown>
  gone: boolean
  failing: number
  polls: string[]
}

const tokenOf = (n: number) => `token-${n}`.padEnd(43, 'x')

function qrFetch(path: string, options: RequestInit = {}): Promise<unknown> {
  if (path === '/auth/qr') {
    server.creates += 1
    return Promise.resolve({ token: tokenOf(server.creates), expires_in: server.expiresIn })
  }
  if (path === '/auth/qr/status') {
    server.polls.push(JSON.parse(String(options.body)).token)
    if (server.failing > 0) {
      server.failing -= 1
      return Promise.reject(new ApiError('Bad Gateway', 502))
    }
    if (server.gone) return Promise.reject(new ApiError('gone', 410, { code: 'qr_unavailable' }))
    return Promise.resolve(structuredClone(server.status))
  }
  return authFixtureApi(path, options)
}

const tick = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms))
const switchToQr = () => userEvent.click(screen.getByRole('button', { name: 'Mã QR' }))

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  startAuthTest({ signedIn: false })
  localStorage.clear()
  server = { creates: 0, expiresIn: 120, status: { status: 'pending' }, gone: false, failing: 0, polls: [] }
  vi.mocked(publicFetch).mockImplementation(qrFetch as typeof publicFetch)
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('Login by QR code on a computer', () => {
  it('goes to the profile step first when one is owed', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    const router = await renderAuthScreen(LoginScreen, '/login')
    await screen.findByRole('img', { name: 'Mã QR đăng nhập' })
    server.status = {
      status: 'approved', access_token: 'h.eyJ0ZW5hbnRfaWQiOiJ0LTEifQ.s', needs_profile: true,
      user: { id: 'u-1', username: 'hoa.le', ngac_node_id: 'n-1', email: 'hoa@novapay.vn', union_id: 'x', email_verified: true },
    }
    await tick(2100)
    await waitFor(() => expect(where(router).path).toBe('/register'))
  })

  it('switches to a QR code from the corner and remembers the choice', async () => {
    await renderAuthScreen(LoginScreen, '/login')
    await switchToQr()
    expect(await screen.findByRole('img', { name: 'Mã QR đăng nhập' })).toBeInTheDocument()
    expect(screen.getByText(/Mã tự làm mới sau/)).toBeInTheDocument()
    expect(localStorage.getItem('nexus.login.method')).toBe('qr')
    expect(server.creates).toBe(1)

    await userEvent.click(screen.getByRole('button', { name: 'Email hoặc Google' }))
    expect(await screen.findByLabelText('Email hoặc số điện thoại')).toBeInTheDocument()
    expect(localStorage.getItem('nexus.login.method')).toBe('form')
  })

  it('opens on the QR code when that was the last choice', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    await renderAuthScreen(LoginScreen, '/login')
    expect(await screen.findByRole('img', { name: 'Mã QR đăng nhập' })).toBeInTheDocument()
  })

  it('shows who scanned, by name only', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    server.status = { status: 'scanned', scanner: { display_name: 'Lê Thị Hoa', avatar_url: '' } }
    await renderAuthScreen(LoginScreen, '/login')
    await tick(2100)
    expect(await screen.findByText('Lê Thị Hoa đã quét')).toBeInTheDocument()
    expect(screen.getByText('Xác nhận trên điện thoại')).toBeInTheDocument()
    expectNoIds()
  })

  it('signs in straight into the phone\'s workspace once the phone approves', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    const router = await renderAuthScreen(LoginScreen, '/login')
    await screen.findByRole('img', { name: 'Mã QR đăng nhập' })
    server.status = {
      // Payload {"tenant_id":"t-1"}: the session is for the phone's workspace.
      status: 'approved', access_token: 'h.eyJ0ZW5hbnRfaWQiOiJ0LTEifQ.s', needs_profile: false,
      user: { id: 'u-1', username: 'hoa.le', ngac_node_id: 'n-1', email: 'hoa@novapay.vn', union_id: 'x', email_verified: true },
    }
    await tick(2100)
    await waitFor(() => expect(where(router).path).toBe('/channels'))
    expect(where(router).search).toMatchObject({ ws: 't-1' })
    expect(useAuthStore.getState().tenantId).toBe('t-1')
    expect(useAuthStore.getState().user?.id).toBe('u-1')
  })

  it('says the phone declined, and stops polling until asked for a new code', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    server.status = { status: 'declined' }
    await renderAuthScreen(LoginScreen, '/login')
    expect(await screen.findByText('Đã huỷ trên điện thoại')).toBeInTheDocument()
    const polled = server.polls.length
    await tick(6000)
    expect(server.polls.length).toBe(polled)

    server.status = { status: 'pending' }
    await userEvent.click(screen.getByRole('button', { name: /Lấy mã mới/ }))
    await waitFor(() => expect(server.creates).toBe(2))
    expect(await screen.findByText(/Mã tự làm mới sau/)).toBeInTheDocument()
  })

  it('says the code expired when the server no longer knows it', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    server.gone = true
    await renderAuthScreen(LoginScreen, '/login')
    expect(await screen.findByText('Mã đã hết hạn')).toBeInTheDocument()
  })

  it(`swaps an unscanned code before it runs out, ${QR_MAX_RENEWALS} times, then waits for a click`, async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    server.expiresIn = 10 // each code is swapped 5 s after it arrives
    await renderAuthScreen(LoginScreen, '/login')
    await screen.findByRole('img', { name: 'Mã QR đăng nhập' })
    for (let i = 0; i < QR_MAX_RENEWALS + 2; i++) await tick(5100)
    expect(server.creates).toBe(1 + QR_MAX_RENEWALS)
    expect(await screen.findByText('Mã đã hết hạn')).toBeInTheDocument()
  })

  it('keeps the code and says the connection is gone after three failed polls', async () => {
    localStorage.setItem('nexus.login.method', 'qr')
    server.failing = 3
    await renderAuthScreen(LoginScreen, '/login')
    await screen.findByRole('img', { name: 'Mã QR đăng nhập' })
    // The first poll runs on arrival; two more fail on the next two beats.
    for (let i = 0; i < 2; i++) await tick(2100)
    expect(await screen.findByText('Mất kết nối, đang thử lại')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Mã QR đăng nhập' })).toBeInTheDocument()
    await tick(2100)
    await waitFor(() => expect(screen.queryByText('Mất kết nối, đang thử lại')).not.toBeInTheDocument())
  })
})
