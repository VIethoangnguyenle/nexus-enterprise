import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { startGoogleSignIn } from '../../api/auth'
import { publicFetch } from '../../api/client'
import { ID, calls, apiError, world } from '../../test/auth-fixtures'
import { renderAuthScreen, startAuthTest, where } from '../../test/auth-render'
import { expectNoIds } from '../../test/no-ids'
import { useAuthStore } from '../../stores/auth.store'
import { LoginScreen } from './LoginScreen'

vi.mock('../../api/client', async (orig) => ({
  ...(await orig<typeof import('../../api/client')>()),
  apiFetch: vi.fn(),
  publicFetch: vi.fn(),
}))
vi.mock('../../api/auth', async (orig) => ({
  ...(await orig<typeof import('../../api/auth')>()),
  startGoogleSignIn: vi.fn(),
}))

const login = (url = '/login') => renderAuthScreen(LoginScreen, '/login', url)
const field = () => screen.findByLabelText('Email hoặc số điện thoại')
const sendCode = () => screen.getByRole('button', { name: 'Nhận mã đăng nhập' })
const codeInput = () => screen.findByLabelText('Mã xác minh')
const boxes = () => Array.from(document.querySelectorAll<HTMLElement>('[data-otp-box]'))
const requestsTo = (path: string) => calls.filter((c) => c.path === path)

beforeEach(() => {
  startAuthTest({ signedIn: false })
  vi.mocked(startGoogleSignIn).mockClear()
})
afterEach(() => vi.restoreAllMocks())

describe('Login: which ways in are offered', () => {
  it('offers Google and a code, Google first', async () => {
    await login()
    const google = await screen.findByRole('button', { name: /Tiếp tục với Google/ })
    expect(google).toBeInTheDocument()
    expect(await field()).toBeInTheDocument()
    expect(google.compareDocumentPosition(await field()) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('starts the server-side Google flow on click', async () => {
    await login()
    await userEvent.click(await screen.findByRole('button', { name: /Tiếp tục với Google/ }))
    expect(startGoogleSignIn).toHaveBeenCalledTimes(1)
  })

  it('hides Google when the server has it switched off', async () => {
    world.providers = { google: false, otp: true }
    await login()
    await field()
    await waitFor(() => expect(requestsTo('/auth/providers')).toHaveLength(1))
    expect(screen.queryByRole('button', { name: /Google/ })).not.toBeInTheDocument()
  })

  it('hides Google, but keeps the code form, when the providers request fails', async () => {
    world.providers = apiError(502)
    await login()
    await field()
    await waitFor(() => expect(requestsTo('/auth/providers')).toHaveLength(1))
    expect(screen.queryByRole('button', { name: /Google/ })).not.toBeInTheDocument()
  })

  it('keeps the code form while the providers are still unknown', async () => {
    world.providers = 'pending'
    await login()
    expect(await field()).toBeInTheDocument()
  })

  it('keeps Google and drops the code form when codes are switched off', async () => {
    world.providers = { google: true, otp: false }
    await login()
    await screen.findByRole('button', { name: /Google/ })
    expect(screen.queryByLabelText('Email hoặc số điện thoại')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Nhận mã đăng nhập' })).not.toBeInTheDocument()
  })

  it('says sign-in is paused, and offers a retry, when no way in exists', async () => {
    world.providers = { google: false, otp: false }
    await login()
    expect(await screen.findByText(/Đăng nhập đang tạm ngưng/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Thử lại/ })).toBeInTheDocument()
    expect(screen.queryByLabelText('Email hoặc số điện thoại')).not.toBeInTheDocument()
  })

  it('has no SSO button', async () => {
    await login()
    await field()
    expect(screen.queryByText(/SSO|Single Sign/i)).not.toBeInTheDocument()
  })
})

describe('Login: Google errors', () => {
  it('explains the failure in a sentence and never prints the code', async () => {
    await login('/login?error=google_unverified')
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/Google chưa xác minh email/)
    expect(document.body.textContent).not.toContain('google_unverified')
  })

  it('shows a generic sentence for a code it does not know', async () => {
    await login('/login?error=something_new')
    expect(await screen.findByRole('alert')).toHaveTextContent(/Chưa đăng nhập được bằng Google/)
    expect(document.body.textContent).not.toContain('something_new')
  })

  it('shows no alert without an error', async () => {
    await login()
    await field()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('Login: the email or phone field', () => {
  it('takes focus on arrival', async () => {
    await login()
    await waitFor(() => expect(screen.getByLabelText('Email hoặc số điện thoại')).toHaveFocus())
  })

  it('stays quiet while typing and explains a bad value on blur, with an example', async () => {
    await login()
    const input = await field()
    await userEvent.type(input, 'hoa.le@novapay')
    expect(screen.queryByText(/Email chưa đầy đủ/)).not.toBeInTheDocument()
    await userEvent.tab()
    expect(await screen.findByText(/Email chưa đầy đủ, ví dụ hoa\.le@novapay\.vn/)).toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'true')
  })

  it('refuses an empty submit with a message instead of a dead button', async () => {
    await login()
    await field()
    await userEvent.click(sendCode())
    expect(await screen.findByText(/Nhập email hoặc số điện thoại/)).toBeInTheDocument()
    expect(requestsTo('/auth/otp/request')).toHaveLength(0)
  })

  it('does not ask for a code for something that is not an address', async () => {
    await login()
    await userEvent.type(await field(), 'hoa.le@novapay{Enter}')
    expect(await screen.findByText(/Email chưa đầy đủ/)).toBeInTheDocument()
    expect(requestsTo('/auth/otp/request')).toHaveLength(0)
  })

  it('clears the message as soon as the value is fixed', async () => {
    await login()
    const input = await field()
    await userEvent.type(input, 'hoa.le@novapay')
    await userEvent.tab()
    await screen.findByText(/Email chưa đầy đủ/)
    await userEvent.type(input, '.vn')
    expect(screen.queryByText(/Email chưa đầy đủ/)).not.toBeInTheDocument()
  })

  it('says how long a code lasts', async () => {
    await login()
    await field()
    expect(screen.getByText(/mã 6 số, có hiệu lực 5 phút/)).toBeInTheDocument()
  })
})

describe('Login: asking for a code', () => {
  it('sends exactly the identifier and its type, for an email', async () => {
    await login()
    await userEvent.type(await field(), '  Hoa.Le@NovaPay.vn  {Enter}')
    await screen.findByText('Nhập mã 6 số')
    expect(requestsTo('/auth/otp/request')[0]).toEqual({
      method: 'POST', path: '/auth/otp/request', body: { identifier: 'Hoa.Le@NovaPay.vn', type: 'email' },
    })
  })

  it('sends a phone number without its spaces, typed as a phone', async () => {
    await login()
    await userEvent.type(await field(), '0912 345 678')
    await userEvent.click(sendCode())
    await screen.findByText('Nhập mã 6 số')
    expect(requestsTo('/auth/otp/request')[0]?.body).toEqual({ identifier: '0912345678', type: 'phone' })
    expect(screen.getByText(/0912 ••• 678/)).toBeInTheDocument()
  })

  it('shows the button as busy while the request is out, and only one request is sent', async () => {
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const original = vi.mocked(publicFetch).getMockImplementation()!
    vi.mocked(publicFetch).mockImplementation(async (path, opts) => {
      if (path === '/auth/otp/request') await gate
      return original(path, opts)
    })
    await login()
    await userEvent.type(await field(), 'hoa.le@novapay.vn')
    await userEvent.click(sendCode())
    const button = screen.getByRole('button', { name: /Nhận mã đăng nhập/ })
    expect(button).toBeDisabled()
    await userEvent.click(button)
    release()
    await screen.findByText('Nhập mã 6 số')
    expect(requestsTo('/auth/otp/request')).toHaveLength(1)
  })

  it.each([
    ['rate limited', apiError(429, { code: 'otp_rate_limited', retry_after_seconds: 700 }), /quá nhiều lần.*12 phút/],
    ['paused', apiError(503, { code: 'otp_unavailable' }), /Đăng nhập đang tạm ngưng/],
    ['server fault', apiError(500, { code: 'internal' }), /Máy chủ đang gặp sự cố/],
    ['offline', new TypeError('Failed to fetch'), /Không kết nối được/],
  ])('shows the failure inline when it is %s, and lets the person try again', async (_n, error, text) => {
    world.requestError = error
    await login()
    await userEvent.type(await field(), 'hoa.le@novapay.vn')
    await userEvent.click(sendCode())
    expect(await screen.findByRole('alert')).toHaveTextContent(text)
    expect(screen.queryByText('Nhập mã 6 số')).not.toBeInTheDocument()
    expect(sendCode()).toBeEnabled()
    expect(document.body.textContent).not.toMatch(/server words|internal|otp_/)
    // The failure goes away once it works.
    world.requestError = undefined
    await userEvent.click(sendCode())
    expect(await screen.findByText('Nhập mã 6 số')).toBeInTheDocument()
  })
})

describe('Login: entering the code', () => {
  async function toCodeStep(id = 'hoa.le@novapay.vn') {
    const router = await login()
    await userEvent.type(await field(), `${id}{Enter}`)
    await screen.findByText('Nhập mã 6 số')
    return router
  }

  it('moves focus to the code field', async () => {
    await toCodeStep()
    await waitFor(() => expect(screen.getByLabelText('Mã xác minh')).toHaveFocus())
  })

  it('says where the code went, how long it lasts, and offers to change the address', async () => {
    await toCodeStep()
    expect(screen.getByText('hoa.le@novapay.vn')).toBeInTheDocument()
    expect(screen.getByText(/Mã có hiệu lực tới \d{2}:\d{2}/)).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Đổi email' }))
    expect(await field()).toHaveValue('hoa.le@novapay.vn')
  })

  it('sends session_id and code, then lands on workspace selection', async () => {
    const router = await toCodeStep()
    await userEvent.type(await codeInput(), '481209')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(requestsTo('/auth/otp/verify')[0]?.body).toEqual({ session_id: 'session-1', code: '481209' })
    expect(useAuthStore.getState().user).toEqual({ id: ID.me, username: 'hoa.le', ngac_node_id: ID.node })
    expect(JSON.stringify(useAuthStore.getState().user)).not.toContain('novapay')
    expect(useAuthStore.getState().accessToken).toBeTruthy()
  })

  it('takes a pasted code', async () => {
    const router = await toCodeStep()
    const input = await codeInput()
    input.focus()
    await userEvent.paste('481 209')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
  })

  it('sends a person who owes a profile to the profile step', async () => {
    world.verifyResult = { needs_profile: true, is_new_user: true }
    const router = await toCodeStep()
    await userEvent.type(await codeInput(), '481209')
    await waitFor(() => expect(where(router).path).toBe('/register'))
  })

  it('keeps the chosen workspace from the link through sign-in', async () => {
    const router = await login('/login?ws=7c1d0000-1111-4a00-8000-00000000b002')
    await userEvent.type(await field(), 'hoa.le@novapay.vn{Enter}')
    await userEvent.type(await codeInput(), '481209')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(where(router).search).toEqual({ ws: '7c1d0000-1111-4a00-8000-00000000b002' })
  })

  it('does not carry a Google error into the next step', async () => {
    const router = await login('/login?error=google_cancelled')
    await userEvent.type(await field(), 'hoa.le@novapay.vn{Enter}')
    await userEvent.type(await codeInput(), '481209')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
    expect(where(router).search).toEqual({})
  })

  it('says a wrong code is wrong, how many tries remain, and empties the boxes', async () => {
    await toCodeStep()
    const input = await codeInput()
    await userEvent.type(input, '000000')
    expect(await screen.findByText(/Mã chưa đúng\. Còn 4 lần thử\./)).toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(boxes().map((b) => b.textContent)).toEqual(['', '', '', '', '', ''])
    expect(input).toHaveFocus()
  })

  it('uses the error colour with a fade, and no shake', async () => {
    await toCodeStep()
    await userEvent.type(await codeInput(), '000000')
    await screen.findByText(/Mã chưa đúng/)
    expect(boxes().every((b) => b.dataset.status === 'error')).toBe(true)
    expect(document.body.innerHTML).not.toMatch(/shake/i)
  })

  it('lets the person type again after a wrong code, and counts down the tries', async () => {
    const router = await toCodeStep()
    const input = await codeInput()
    await userEvent.type(input, '000000')
    await screen.findByText(/Còn 4 lần thử/)
    await userEvent.type(input, '111111')
    await screen.findByText(/Còn 3 lần thử/)
    await userEvent.type(input, '481209')
    await waitFor(() => expect(where(router).path).toBe('/workspace-select'))
  })

  it('reads the message to assistive technology', async () => {
    await toCodeStep()
    const input = await codeInput()
    await userEvent.type(input, '000000')
    const message = await screen.findByText(/Còn 4 lần thử/)
    expect(message.closest('[aria-live]')).toBeTruthy()
    expect(input.getAttribute('aria-describedby')).toBe(message.closest('[id]')?.id)
  })

  it('after the last try, asks for a new code and stops taking digits', async () => {
    await toCodeStep()
    const input = await codeInput()
    for (let i = 0; i < 5; i++) await userEvent.type(input, '000000')
    expect(await screen.findByText(/hết lượt thử/)).toBeInTheDocument()
    expect(input).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Gửi lại' })).toBeEnabled()
  })

  it('an expired code says so, and Gửi lại gives a fresh one', async () => {
    await toCodeStep()
    world.verifyError = apiError(401, { code: 'otp_expired' })
    const input = await codeInput()
    await userEvent.type(input, '481209')
    expect(await screen.findByText(/Mã đã hết hạn/)).toBeInTheDocument()
    world.verifyError = undefined
    await userEvent.click(screen.getByRole('button', { name: 'Gửi lại' }))
    await waitFor(() => expect(requestsTo('/auth/otp/request')).toHaveLength(2))
    await waitFor(() => expect(screen.getByLabelText('Mã xác minh')).toBeEnabled())
    expect(screen.queryByText(/Mã đã hết hạn/)).not.toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Mã xác minh'), '481209')
    await waitFor(() => expect(requestsTo('/auth/otp/verify').slice(-1)[0]?.body).toEqual({ session_id: 'session-2', code: '481209' }))
  })

  it.each([
    ['paused', apiError(503, { code: 'otp_unavailable' }), /Đăng nhập đang tạm ngưng/],
    ['offline', new TypeError('Failed to fetch'), /Không kết nối được/],
    ['a server fault', apiError(500, { code: 'internal' }), /Máy chủ đang gặp sự cố/],
  ])('shows %s inline and lets the person retype the code', async (_n, error, text) => {
    await toCodeStep()
    world.verifyError = error
    const input = await codeInput()
    await userEvent.type(input, '481209')
    expect(await screen.findByText(text)).toBeInTheDocument()
    expect(input).toBeEnabled()
    expect(boxes().map((b) => b.textContent)).toEqual(['', '', '', '', '', ''])
  })

  it('the way back keeps the address and drops the code', async () => {
    await toCodeStep()
    await userEvent.type(await codeInput(), '000000')
    await screen.findByText(/Còn 4 lần thử/)
    await userEvent.click(screen.getByRole('button', { name: 'Quay lại' }))
    expect(await field()).toHaveValue('hoa.le@novapay.vn')
    expect(screen.queryByText(/Còn 4 lần thử/)).not.toBeInTheDocument()
  })
})

describe('Login: no test-mode hint, ever', () => {
  it.each([
    ['fixed code in force', { google: true, otp: true, otp_fixed_code: true }],
    ['random codes', { google: true, otp: true, otp_fixed_code: false }],
  ])('prints no code and no "test mode" line (%s)', async (_n, providers) => {
    world.providers = providers
    await login()
    await userEvent.type(await field(), 'hoa.le@novapay.vn{Enter}')
    await screen.findByText('Nhập mã 6 số')
    expect(document.body.textContent).not.toMatch(/999999|test mode|chế độ thử|mã thử/i)
    expect(document.body.innerHTML).not.toContain('999999')
  })

  it('prints no code on the first step either', async () => {
    world.providers = { google: true, otp: true, otp_fixed_code: true }
    await login()
    await field()
    await waitFor(() => expect(requestsTo('/auth/providers')).toHaveLength(1))
    expect(document.body.textContent).not.toMatch(/999999|test mode|OTP code is/i)
  })
})

describe('Login: nothing internal on screen', () => {
  it('shows no identifier on either step', async () => {
    await login()
    await userEvent.type(await field(), 'hoa.le@novapay.vn{Enter}')
    await screen.findByText('Nhập mã 6 số')
    expectNoIds()
    expect(within(document.body).queryByText(/session-\d/)).not.toBeInTheDocument()
  })
})
