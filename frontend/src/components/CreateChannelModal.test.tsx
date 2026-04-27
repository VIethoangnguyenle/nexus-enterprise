import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { CreateChannelModal } from './CreateChannelModal'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockMutate = vi.fn()
const mockNavigate = vi.fn()
let mockWsId = 'ws-1'

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('../hooks/useMessaging', () => ({
  useCreateChannel: () => ({
    mutate: mockMutate,
    isPending: false,
    error: null,
  }),
}))

vi.mock('../hooks/useWorkspaces', () => ({
  useWorkspaces: () => ({
    data: { workspaces: mockWsId ? [{ id: mockWsId, name: 'Test Workspace' }] : [] },
  }),
}))

beforeEach(() => {
  vi.clearAllMocks()
  mockWsId = 'ws-1'
})

// ---------------------------------------------------------------------------
// 11.1: Renders form fields
// ---------------------------------------------------------------------------

describe('CreateChannelModal', () => {
  it('renders heading, input, and action buttons', () => {
    render(<CreateChannelModal onClose={vi.fn()} />)

    expect(screen.getByText('Create Channel')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('e.g. general, engineering')).toBeInTheDocument()
    expect(screen.getByText('Cancel')).toBeInTheDocument()
    expect(screen.getByText('Create')).toBeInTheDocument()
  })

  // ---------------------------------------------------------------------------
  // 11.2: Disable create when name empty
  // ---------------------------------------------------------------------------

  it('disables create button when name is empty', () => {
    render(<CreateChannelModal onClose={vi.fn()} />)

    const createBtn = screen.getByText('Create')
    expect(createBtn).toBeDisabled()
  })

  // ---------------------------------------------------------------------------
  // 11.3: Disable create when wsId empty
  // ---------------------------------------------------------------------------

  it('shows error when no workspace available', () => {
    mockWsId = ''
    render(<CreateChannelModal onClose={vi.fn()} />)

    expect(screen.getByText(/No workspace available/)).toBeInTheDocument()
  })

  // ---------------------------------------------------------------------------
  // 11.4: Submits with channel_type 'workspace'
  // ---------------------------------------------------------------------------

  it('submits with channel_type workspace when name is provided', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<CreateChannelModal onClose={onClose} />)

    const input = screen.getByPlaceholderText('e.g. general, engineering')
    await user.type(input, 'engineering')

    const createBtn = screen.getByText('Create')
    expect(createBtn).not.toBeDisabled()

    await user.click(createBtn)

    expect(mockMutate).toHaveBeenCalledWith(
      { name: 'engineering', channel_type: 'workspace' },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    )
  })

  it('calls onClose when Cancel is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<CreateChannelModal onClose={onClose} />)

    await user.click(screen.getByText('Cancel'))
    expect(onClose).toHaveBeenCalledOnce()
  })
})
