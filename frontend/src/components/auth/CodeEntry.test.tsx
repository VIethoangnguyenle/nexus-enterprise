import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { publicFetch } from '../../api/client'
import type { OTPVerifyResponse } from '../../api/auth'
import { queryClient } from '../../lib/query-client'
import { useAuthStore } from '../../stores/auth.store'
import { calls, apiError, world } from '../../test/auth-fixtures'
import { startAuthTest } from '../../test/auth-render'
import { CodeEntry } from './CodeEntry'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))

const EMAIL = { kind: 'email' as const, value: 'hoa.le@novapay.vn' }
const requestsTo = (path: string) => calls.filter((c) => c.path === path)

type SignInProps = Extract<Parameters<typeof CodeEntry>[0], { mode?: 'sign-in' }>

function mount(over: Partial<SignInProps> = {}) {
  const onVerified = vi.fn<(r: OTPVerifyResponse) => void>()
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <CodeEntry
        identifier={EMAIL}
        session={{ sessionId: 'session-1', expiresAt: new Date(Date.now() + 300_000) }}
        onVerified={onVerified}
        {...over}
      />
    </QueryClientProvider>,
  )
  return { ...utils, onVerified }
}

const resend = () => screen.getByRole('button', { name: 'Gửi lại' })

beforeEach(() => {
  startAuthTest({ signedIn: false })
  vi.useFakeTimers({ shouldAdvanceTime: true })
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

const tick = (seconds: number) => act(() => { vi.advanceTimersByTime(seconds * 1000) })
const user = () => userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) })

describe('CodeEntry: the resend countdown', () => {
  it('starts at one minute with Gửi lại locked, and counts down in whole seconds', () => {
    mount()
    expect(screen.getByText('Gửi lại mã sau 1:00')).toBeInTheDocument()
    expect(resend()).toBeDisabled()
    tick(18)
    expect(screen.getByText('Gửi lại mã sau 0:42')).toBeInTheDocument()
  })

  it('keeps the digits from jumping by using tabular figures', () => {
    mount()
    expect(screen.getByText('Gửi lại mã sau 1:00').className).toMatch(/tnum/)
  })

  it('unlocks Gửi lại at zero and the countdown line goes away', () => {
    mount()
    tick(60)
    expect(resend()).toBeEnabled()
    expect(screen.queryByText(/Gửi lại mã sau/)).not.toBeInTheDocument()
  })

  it('resending asks for a new code for the same address, resets the boxes and restarts the countdown', async () => {
    mount()
    const u = user()
    tick(60)
    await u.type(screen.getByLabelText('Mã xác minh'), '000000')
    await screen.findByText(/Còn 4 lần thử/)
    await u.click(resend())
    await waitFor(() => expect(requestsTo('/auth/otp/request')).toHaveLength(1))
    expect(requestsTo('/auth/otp/request')[0]?.body).toEqual({ identifier: 'hoa.le@novapay.vn', type: 'email' })
    expect(await screen.findByText('Gửi lại mã sau 1:00')).toBeInTheDocument()
    expect(screen.queryByText(/Còn 4 lần thử/)).not.toBeInTheDocument()
    expect(screen.getByLabelText('Mã xác minh')).not.toHaveAttribute('aria-invalid')
  })

  it('the new code is checked against the new session', async () => {
    mount()
    const u = user()
    tick(60)
    await u.click(resend())
    await waitFor(() => expect(requestsTo('/auth/otp/request')).toHaveLength(1))
    await screen.findByText('Gửi lại mã sau 1:00')
    await u.type(screen.getByLabelText('Mã xác minh'), '481209')
    await waitFor(() => expect(requestsTo('/auth/otp/verify')).toHaveLength(1))
    expect(requestsTo('/auth/otp/verify')[0]?.body).toEqual({ session_id: 'session-1', code: '481209' })
  })

  it('a refused resend says when to come back and keeps the button locked for that long', async () => {
    mount()
    const u = user()
    tick(60)
    world.requestError = apiError(429, { code: 'otp_rate_limited', retry_after_seconds: 720 })
    await u.click(resend())
    expect(await screen.findByRole('alert')).toHaveTextContent(/quá nhiều lần.*12 phút/)
    expect(resend()).toBeDisabled()
    expect(screen.getByText('Gửi lại mã sau 12:00')).toBeInTheDocument()
  })

  it('a resend that fails for another reason can be tried again at once', async () => {
    mount()
    const u = user()
    tick(60)
    world.requestError = new TypeError('Failed to fetch')
    await u.click(resend())
    expect(await screen.findByRole('alert')).toHaveTextContent(/Không kết nối được/)
    expect(resend()).toBeEnabled()
  })

  it('stops the timer when it leaves the screen', () => {
    const { unmount } = mount()
    const running = vi.getTimerCount()
    unmount()
    expect(vi.getTimerCount()).toBeLessThan(running)
  })
})

describe('CodeEntry: a code that cannot work any more does not make the person wait', () => {
  it('unlocks Gửi lại straight away when the code expired', async () => {
    mount()
    const u = user()
    world.verifyError = apiError(401, { code: 'otp_expired' })
    await u.type(screen.getByLabelText('Mã xác minh'), '481209')
    await screen.findByText(/Mã đã hết hạn/)
    expect(resend()).toBeEnabled()
  })
})

describe('CodeEntry: the code field', () => {
  it('has an accessible name, numeric input and autofill', () => {
    mount()
    const input = screen.getByLabelText('Mã xác minh')
    expect(input).toHaveAttribute('inputmode', 'numeric')
    expect(input).toHaveAttribute('autocomplete', 'one-time-code')
  })

  it('reports success to its parent with the whole response', async () => {
    const { onVerified } = mount()
    await user().type(screen.getByLabelText('Mã xác minh'), '481209')
    await waitFor(() => expect(onVerified).toHaveBeenCalledTimes(1))
    expect(onVerified.mock.calls[0]?.[0]).toMatchObject({ is_new_user: false, needs_profile: false })
  })

  it('shows the checking state, then stays busy until the parent moves on', async () => {
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const original = vi.mocked(publicFetch).getMockImplementation()!
    vi.mocked(publicFetch).mockImplementation(async (path, opts) => {
      if (path === '/auth/otp/verify') await gate
      return original(path, opts)
    })
    mount()
    const input = screen.getByLabelText('Mã xác minh')
    await user().type(input, '481209')
    await waitFor(() => expect(input).toHaveAttribute('aria-busy', 'true'))
    expect(input).toHaveAttribute('readonly')
    act(() => release())
    // The parent moves on; until it does the field stays locked on the right code.
    await waitFor(() => expect(input).toHaveAttribute('readonly'))
    expect(input).toHaveValue('481209')
  })
})

// Proving the address of the account you are in uses the same field and the
// same life, against other endpoints, and signs nobody in.
describe('CodeEntry: proving the address of the signed-in account', () => {
  const mountVerify = () => {
    const onVerified = vi.fn()
    const utils = render(
      <QueryClientProvider client={queryClient}>
        <CodeEntry
          mode="verify-email"
          identifier={EMAIL}
          session={{ sessionId: 'for-this-account', expiresAt: new Date(Date.now() + 300_000) }}
          onVerified={onVerified}
        />
      </QueryClientProvider>,
    )
    return { ...utils, onVerified }
  }

  it('sends the code to the account-proving route, not the sign-in route, and signs nobody in', async () => {
    const before = useAuthStore.getState().accessToken
    const { onVerified } = mountVerify()
    await user().type(screen.getByLabelText('Mã xác minh'), world.emailCode)
    await waitFor(() => expect(onVerified).toHaveBeenCalledTimes(1))

    expect(calls.filter((c) => c.path === '/me/email/verify')).toEqual([
      { method: 'POST', path: '/me/email/verify', body: { code: world.emailCode } },
    ])
    expect(requestsTo('/auth/otp/verify')).toHaveLength(0)
    expect(useAuthStore.getState().accessToken).toBe(before)
  })

  it('says how many tries remain after a wrong code', async () => {
    mountVerify()
    await user().type(screen.getByLabelText('Mã xác minh'), '000000')
    expect(await screen.findByText(/Mã chưa đúng\. Còn 4 lần thử\./)).toBeInTheDocument()
  })

  it('asks for another code through the same route, with no body but an empty one', async () => {
    mountVerify()
    const u = user()
    tick(60)
    await u.click(resend())
    await waitFor(() => expect(calls.filter((c) => c.path === '/me/email/verify')).toHaveLength(1))
    expect(calls.find((c) => c.path === '/me/email/verify')?.body).toEqual({})
    expect(requestsTo('/auth/otp/request')).toHaveLength(0)
    expect(await screen.findByText('Gửi lại mã sau 1:00')).toBeInTheDocument()
  })

  it('a refused request says when to come back', async () => {
    mountVerify()
    const u = user()
    tick(60)
    world.emailRequestError = apiError(429, { code: 'otp_rate_limited', retry_after_seconds: 600 })
    await u.click(resend())
    expect(await screen.findByRole('alert')).toHaveTextContent(/10 phút/)
    expect(resend()).toBeDisabled()
  })
})
