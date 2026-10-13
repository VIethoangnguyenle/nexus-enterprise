import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { useIncomingCallStore } from '../../stores/incoming-call.store'

// Mock the ringtone module
const mockStart = vi.fn()
const mockStop = vi.fn()
const mockPlaying = vi.fn(() => false)
vi.mock('../../lib/ringtone', () => ({
  createRingtone: () => ({ start: mockStart, stop: mockStop, playing: mockPlaying }),
}))

// Mock phone detection
let isPhone = false
vi.mock('../../hooks/usePhone', () => ({
  usePhone: () => isPhone,
}))

// Mock router
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => vi.fn(),
}))

// Mock calls API
vi.mock('../../api/calls', () => ({
  callsApi: { decline: vi.fn().mockResolvedValue({}) },
}))

// Import after mocks
import { IncomingCallLayer } from './IncomingCallLayer'

beforeEach(() => {
  vi.clearAllMocks()
  isPhone = false
  useIncomingCallStore.setState({ incoming: null })
})

afterEach(() => {
  useIncomingCallStore.setState({ incoming: null })
})

describe('IncomingCallLayer', () => {
  const ring = () => {
    useIncomingCallStore.getState().ring({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Phạm Hải Yến',
      callerAvatarUrl: '',
    })
  }

  it('renders nothing when there is no incoming call', () => {
    const { container } = render(<IncomingCallLayer />)
    expect(container.innerHTML).toBe('')
  })

  it('renders IncomingCallToast on desktop when ringing', () => {
    isPhone = false
    ring()
    render(<IncomingCallLayer />)
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
    // Toast width is set via inline style
    expect(screen.getByRole('alertdialog').style.width).toBe('360px')
  })

  it('renders IncomingCallScreen on phone when ringing', () => {
    isPhone = true
    ring()
    render(<IncomingCallLayer />)
    // Screen is full-screen, check for stage background text
    expect(screen.getByText('Phạm Hải Yến')).toBeInTheDocument()
  })

  it('starts the ringtone when ringing and stops on clear', () => {
    ring()
    const { unmount } = render(<IncomingCallLayer />)
    expect(mockStart).toHaveBeenCalled()
    unmount()
    expect(mockStop).toHaveBeenCalled()
  })

  it('does not start ringtone for invited mode', () => {
    useIncomingCallStore.getState().invite({
      callId: 'c1',
      channelId: 'ch1',
      kind: 'video',
      callerName: 'Yến',
      callerAvatarUrl: '',
    })
    render(<IncomingCallLayer />)
    expect(mockStart).not.toHaveBeenCalled()
  })
})
