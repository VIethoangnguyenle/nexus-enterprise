import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { ComponentType } from 'react'

const mockNavigate = vi.fn()
const mockComplete = vi.fn()

vi.mock('@tanstack/react-router', () => ({
  createFileRoute: () => (options: Record<string, unknown>) => ({ options }),
  useNavigate: () => mockNavigate,
}))

vi.mock('../api/auth', () => ({
  completeGoogleSignIn: () => mockComplete(),
}))

const { Route } = await import('./auth.google.done')
const GoogleDonePage = (Route as unknown as { options: { component: ComponentType } }).options.component

beforeEach(() => {
  vi.clearAllMocks()
})

describe('/auth/google/done', () => {
  it('finishes the session and continues to workspace selection, like the code flow', async () => {
    mockComplete.mockResolvedValue({ needsProfile: false })
    render(<GoogleDonePage />)

    expect(screen.getByText(/Đang đăng nhập bằng Google/)).toBeInTheDocument()
    await vi.waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/workspace-select', replace: true })),
    )
    expect(mockComplete).toHaveBeenCalledTimes(1)
  })

  it('asks a new person for their profile first', async () => {
    mockComplete.mockResolvedValue({ needsProfile: true })
    render(<GoogleDonePage />)

    await vi.waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith(expect.objectContaining({ to: '/register', replace: true })),
    )
  })

  it('sends the user back to the login page with an error when the session cannot be established', async () => {
    mockComplete.mockResolvedValue(null)
    render(<GoogleDonePage />)

    await vi.waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith(
        expect.objectContaining({ to: '/login', search: { error: 'google_session' }, replace: true }),
      ),
    )
  })

  it('runs the bootstrap only once even if the effect fires twice', async () => {
    mockComplete.mockResolvedValue({ needsProfile: false })
    const { rerender } = render(<GoogleDonePage />)
    rerender(<GoogleDonePage />)

    await vi.waitFor(() => expect(mockNavigate).toHaveBeenCalled())
    expect(mockComplete).toHaveBeenCalledTimes(1)
  })
})
