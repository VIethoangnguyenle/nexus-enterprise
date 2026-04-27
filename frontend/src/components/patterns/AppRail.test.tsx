import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { AppRail } from './AppRail'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockLogout = vi.fn()
const mockSetActiveModule = vi.fn()
let mockActiveModule = 'messaging'

vi.mock('../../stores/auth.store', () => ({
  useAuthStore: (selector: (s: { logout: () => void; user: { username: string } }) => unknown) =>
    selector({ logout: mockLogout, user: { username: 'testuser' } }),
}))

vi.mock('../../stores/ui.store', () => ({
  useUiStore: (selector: (s: { activeModule: string; setActiveModule: (m: string) => void }) => unknown) =>
    selector({ activeModule: mockActiveModule, setActiveModule: mockSetActiveModule }),
}))

beforeEach(() => {
  vi.clearAllMocks()
  mockActiveModule = 'messaging'
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('AppRail', () => {
  it('renders all navigation items with aria-labels', () => {
    render(<AppRail />)

    expect(screen.getByLabelText('Messaging')).toBeInTheDocument()
    expect(screen.getByLabelText('Documents')).toBeInTheDocument()
    expect(screen.getByLabelText('Assets')).toBeInTheDocument()
    expect(screen.getByLabelText('Settings')).toBeInTheDocument()
  })

  it('renders NGAC logo', () => {
    render(<AppRail />)

    expect(screen.getByText('N')).toBeInTheDocument()
  })

  it('renders user avatar and logout button', () => {
    render(<AppRail />)

    expect(screen.getByLabelText('Logout')).toBeInTheDocument()
    expect(screen.getByTitle('testuser')).toBeInTheDocument()
  })

  it('calls setActiveModule when a rail item is clicked', async () => {
    const user = userEvent.setup()
    render(<AppRail />)

    await user.click(screen.getByLabelText('Documents'))
    expect(mockSetActiveModule).toHaveBeenCalledWith('documents')
  })

  it('calls logout when logout button is clicked', async () => {
    const user = userEvent.setup()
    render(<AppRail />)

    await user.click(screen.getByLabelText('Logout'))
    expect(mockLogout).toHaveBeenCalledOnce()
  })

  it('highlights active module with active styling', () => {
    mockActiveModule = 'assets'
    render(<AppRail />)

    const assetsBtn = screen.getByLabelText('Assets')
    expect(assetsBtn.className).toContain('bg-bg-active')
  })
})
