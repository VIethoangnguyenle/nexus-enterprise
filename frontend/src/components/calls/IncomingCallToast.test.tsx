import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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
    expect(navigate).toHaveBeenCalledWith(
      expect.objectContaining({ to: '/calls/$callId', params: { callId: 'c1' }, search: { answer: 'audio' } }),
    )
    expect(onClear).toHaveBeenCalledWith('c1')
  })

  it('Video navigates to the call route with answer=video', async () => {
    const user = userEvent.setup()
    render(<IncomingCallToast incoming={ringing} onClear={onClear} />)
    await user.click(screen.getByRole('button', { name: /^Video$/i }))
    expect(navigate).toHaveBeenCalledWith(
      expect.objectContaining({ to: '/calls/$callId', params: { callId: 'c1' }, search: { answer: 'video' } }),
    )
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
})
