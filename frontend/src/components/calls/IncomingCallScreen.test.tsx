import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError } from '../../api/client'
import { expectCallRoute } from '../../test/call-nav'
import { useToastStore } from '../primitives'
import { IncomingCallScreen } from './IncomingCallScreen'
import type { IncomingCallInfo } from '../../stores/incoming-call.store'

const decline = vi.fn().mockResolvedValue({})

const navigate = vi.fn()
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigate,
}))

vi.mock('../../api/calls', () => ({
  callsApi: { decline: (...args: unknown[]) => decline(...args) },
}))

const ringing: IncomingCallInfo = {
  callId: 'c1',
  channelId: 'ch1',
  kind: 'video',
  callerName: 'Phạm Hải Yến',
  callerAvatarUrl: '',
  mode: 'ringing',
}

const onClear = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
})

describe('IncomingCallScreen', () => {
  it('renders caller name and subtitle on stage background', () => {
    render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    expect(screen.getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(screen.getByText(/Cuộc gọi video đến/)).toBeInTheDocument()
  })

  it('shows three round action buttons', () => {
    render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    expect(screen.getByRole('button', { name: /Từ chối/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Thoại/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Video/i })).toBeInTheDocument()
  })

  it('Từ chối calls decline and clears', async () => {
    const user = userEvent.setup()
    render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Từ chối/i }))
    expect(decline).toHaveBeenCalledWith('c1')
    expect(onClear).toHaveBeenCalledWith('c1')
  })

  it('Video navigates to call with answer=video', async () => {
    const user = userEvent.setup()
    render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /^Video$/i }))
    expectCallRoute(navigate, 'c1', 'video')
  })

  it('answered_elsewhere shows the notification text on stage', () => {
    render(<IncomingCallScreen incoming={{ ...ringing, mode: 'answered_elsewhere' }} onClear={onClear} />)
    expect(screen.getByText('Đã trả lời trên thiết bị khác')).toBeInTheDocument()
  })

  it('invited mode shows invite text and Tham gia / Bỏ qua', () => {
    render(<IncomingCallScreen incoming={{ ...ringing, mode: 'invited' }} onClear={onClear} />)
    expect(screen.getByText(/mời bạn vào cuộc gọi/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Tham gia/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Bỏ qua/i })).toBeInTheDocument()
  })

  it('is a modal dialog named by the caller, and takes focus in and gives it back', () => {
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()
    const { unmount } = render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAccessibleName(/Phạm Hải Yến/)
    expect(dialog.contains(document.activeElement)).toBe(true)
    unmount()
    expect(opener).toHaveFocus()
    opener.remove()
  })

  it('the other modes are dialogs too', () => {
    const { rerender } = render(<IncomingCallScreen incoming={{ ...ringing, mode: 'invited' }} onClear={onClear} />)
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-modal', 'true')
    rerender(<IncomingCallScreen incoming={{ ...ringing, mode: 'answered_elsewhere' }} onClear={onClear} />)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('a decline that fails because the call is over is quiet; another failure is told', async () => {
    useToastStore.getState().clear()
    const user = userEvent.setup()
    decline.mockRejectedValueOnce(new ApiError('gone', 409, { code: 'call_ended' }))
    const { unmount } = render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Từ chối/i }))
    await Promise.resolve()
    expect(useToastStore.getState().toasts).toHaveLength(0)
    unmount()
    decline.mockRejectedValueOnce(new ApiError('boom', 500))
    render(<IncomingCallScreen incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Từ chối/i }))
    await vi.waitFor(() => expect(useToastStore.getState().toasts).toHaveLength(1))
  })
})
