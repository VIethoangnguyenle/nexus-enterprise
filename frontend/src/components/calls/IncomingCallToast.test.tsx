import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ApiError } from '../../api/client'
import { expectCallRoute } from '../../test/call-nav'
import { useToastStore } from '../primitives'
import { IncomingCallToast } from './IncomingCallToast'
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

const invited: IncomingCallInfo = {
  ...ringing,
  mode: 'invited',
}

const answeredElsewhere: IncomingCallInfo = {
  ...ringing,
  mode: 'answered_elsewhere',
}

const onClear = vi.fn()

beforeEach(() => {
  vi.clearAllMocks()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('IncomingCallToast', () => {
  it('renders caller name and video call text when ringing', () => {
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    expect(screen.getByText('Phạm Hải Yến')).toBeInTheDocument()
    expect(screen.getByText('đang gọi video cho bạn…')).toBeInTheDocument()
  })

  it('renders audio call text for an audio call', () => {
    render(<IncomingCallToast incoming={{ ...ringing, kind: 'audio' }} onClear={onClear} />)
    expect(screen.getByText('đang gọi thoại cho bạn…')).toBeInTheDocument()
  })

  it('shows three buttons: Từ chối, Thoại, Video', () => {
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    expect(screen.getByRole('button', { name: /Từ chối/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Thoại/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Video/i })).toBeInTheDocument()
  })

  it('Từ chối calls callsApi.decline and clears', async () => {
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Từ chối/i }))
    expect(decline).toHaveBeenCalledWith('c1')
    expect(onClear).toHaveBeenCalledWith('c1')
  })

  it('Thoại navigates to the call route with answer=audio', async () => {
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /^Thoại$/i }))
    expectCallRoute(navigate, 'c1', 'audio')
    expect(onClear).toHaveBeenCalledWith('c1')
  })

  it('Video navigates to the call route with answer=video', async () => {
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /^Video$/i }))
    expectCallRoute(navigate, 'c1', 'video')
    expect(onClear).toHaveBeenCalledWith('c1')
  })

  it('invited mode shows invite text and Tham gia / Bỏ qua', () => {
    render(<IncomingCallToast incoming={invited} onClear={onClear} />)
    expect(screen.getByText(/mời bạn vào cuộc gọi/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Tham gia/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Bỏ qua/i })).toBeInTheDocument()
  })

  it('Bỏ qua clears without declining', async () => {
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={invited} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Bỏ qua/i }))
    expect(decline).not.toHaveBeenCalled()
    expect(onClear).toHaveBeenCalledWith('c1')
  })

  it('answered_elsewhere shows the notification text', () => {
    render(<IncomingCallToast incoming={answeredElsewhere} onClear={onClear} />)
    expect(screen.getByText('Đã trả lời trên thiết bị khác')).toBeInTheDocument()
  })

  it('has role=alertdialog with an accessible label', () => {
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  })

  it('puts focus on the answer button when it opens, and gives it back when it goes', () => {
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()
    const { unmount } = render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    expect(screen.getByRole('button', { name: /^Video$/i })).toHaveFocus()
    unmount()
    expect(opener).toHaveFocus()
    opener.remove()
  })

  it('an invitation focuses "Tham gia"', () => {
    render(<IncomingCallToast incoming={invited} onClear={onClear} />)
    expect(screen.getByRole('button', { name: /Tham gia/i })).toHaveFocus()
  })

  it('a refused decline for a call that no longer rings is not worth a word', async () => {
    useToastStore.getState().clear()
    decline.mockRejectedValueOnce(new ApiError('gone', 409, { code: 'call_ended' }))
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Từ chối/i }))
    await Promise.resolve()
    expect(onClear).toHaveBeenCalledWith('c1')
    expect(useToastStore.getState().toasts).toHaveLength(0)
  })

  it('any other failed decline is told', async () => {
    useToastStore.getState().clear()
    decline.mockRejectedValueOnce(new ApiError('boom', 500))
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /Từ chối/i }))
    await vi.waitFor(() => expect(useToastStore.getState().toasts).toHaveLength(1))
  })
})
