import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { ComponentType } from 'react'

const mockProviders = vi.fn()
const mockStartGoogle = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  createFileRoute: () => (options: Record<string, unknown>) => ({ options }),
  useNavigate: () => vi.fn(),
}))

vi.mock('../../hooks/useAuth', () => {
  const mutation = () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null })
  return { useRequestOTP: mutation, useVerifyOTP: mutation }
})

vi.mock('../../api/auth', () => ({
  authApi: { providers: () => mockProviders() },
  startGoogleSignIn: () => mockStartGoogle(),
}))

const { Route } = await import('./login')
const LoginPage = (Route as unknown as { options: { component: ComponentType } }).options.component

function renderLogin(search = '') {
  window.history.replaceState(null, '', `/login${search}`)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <LoginPage />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  window.history.replaceState(null, '', '/')
})

describe('Login page — Google sign-in', () => {
  it('hides the Google button when the server reports Google sign-in is disabled', async () => {
    mockProviders.mockResolvedValue({ google: false })
    renderLogin()

    // Wait for the providers request to settle, then make sure nothing appeared.
    await screen.findByText('Single Sign-On (SSO)')
    await vi.waitFor(() => expect(mockProviders).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: /google/i })).not.toBeInTheDocument()
  })

  it('hides the Google button when the providers request fails', async () => {
    mockProviders.mockRejectedValue(new Error('network'))
    renderLogin()

    await vi.waitFor(() => expect(mockProviders).toHaveBeenCalled())
    expect(screen.queryByRole('button', { name: /google/i })).not.toBeInTheDocument()
  })

  it('shows the Google button when enabled and starts the server-side flow on click', async () => {
    mockProviders.mockResolvedValue({ google: true })
    renderLogin()

    const button = await screen.findByRole('button', { name: /google/i })
    await userEvent.click(button)

    expect(mockStartGoogle).toHaveBeenCalledTimes(1)
  })

  it('explains a Google error in a sentence and never shows the raw code', async () => {
    mockProviders.mockResolvedValue({ google: true })
    renderLogin('?error=google_unverified')

    expect(await screen.findByText(/email address isn.t verified/i)).toBeInTheDocument()
    expect(screen.queryByText(/google_unverified/)).not.toBeInTheDocument()
  })

  it('falls back to a generic sentence for an unknown error code', async () => {
    mockProviders.mockResolvedValue({ google: true })
    renderLogin('?error=something_new')

    expect(await screen.findByText(/couldn.t sign you in with google/i)).toBeInTheDocument()
    expect(screen.queryByText(/something_new/)).not.toBeInTheDocument()
  })

  it('shows no error box without an error parameter', async () => {
    mockProviders.mockResolvedValue({ google: true })
    renderLogin()

    await screen.findByRole('button', { name: /google/i })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
