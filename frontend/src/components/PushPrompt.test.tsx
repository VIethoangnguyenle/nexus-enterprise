import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { PushPrompt } from './PushPrompt'
import * as pushLib from '../lib/push'
import { toast } from './primitives'

vi.mock('./primitives', async () => {
  const actual = await vi.importActual('./primitives')
  const mockToast = Object.assign(vi.fn(), {
    error: vi.fn(),
    info: vi.fn(),
  })
  return {
    ...actual,
    toast: mockToast,
  }
})

describe('PushPrompt card', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()

    // Default: App ID present, push supported, permission 'default', not iOS, not dismissed
    vi.spyOn(pushLib, 'getOneSignalAppId').mockReturnValue('test-app-id')
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(true)
    vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('default')
    vi.spyOn(pushLib, 'isPromptDismissed').mockReturnValue(false)
    vi.spyOn(pushLib, 'isIosNotInstalled').mockReturnValue(false)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders nothing when App ID is absent', () => {
    vi.spyOn(pushLib, 'getOneSignalAppId').mockReturnValue(undefined)
    const { container } = render(<PushPrompt />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when push is not supported', () => {
    vi.spyOn(pushLib, 'isPushSupported').mockReturnValue(false)
    const { container } = render(<PushPrompt />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when permission is already granted', () => {
    vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('granted')
    const { container } = render(<PushPrompt />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when permission is denied', () => {
    vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('denied')
    const { container } = render(<PushPrompt />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when user already dismissed it', () => {
    vi.spyOn(pushLib, 'isPromptDismissed').mockReturnValue(true)
    const { container } = render(<PushPrompt />)
    expect(container.firstChild).toBeNull()
  })

  it('renders prompt card when permission is default and not dismissed', () => {
    render(<PushPrompt />)
    expect(screen.getByText('Nhận thông báo khi có tin mới?')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Bật thông báo' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Để sau' })).toBeInTheDocument()
  })

  it('dismisses card and records dismissal when "Để sau" is clicked', async () => {
    const user = userEvent.setup()
    const recordSpy = vi.spyOn(pushLib, 'recordPromptDismiss')

    render(<PushPrompt />)
    await user.click(screen.getByRole('button', { name: 'Để sau' }))

    expect(recordSpy).toHaveBeenCalled()
    expect(screen.queryByText('Nhận thông báo khi có tin mới?')).not.toBeInTheDocument()
  })

  it('requests permission and shows success toast when "Bật thông báo" is allowed', async () => {
    const user = userEvent.setup()
    vi.spyOn(pushLib, 'requestPushPermission').mockResolvedValue('granted')

    render(<PushPrompt />)
    await user.click(screen.getByRole('button', { name: 'Bật thông báo' }))

    expect(pushLib.requestPushPermission).toHaveBeenCalled()
    expect(toast).toHaveBeenCalledWith('Đã bật thông báo trên thiết bị này')
    expect(screen.queryByText('Nhận thông báo khi có tin mới?')).not.toBeInTheDocument()
  })

  it('shows error toast when "Bật thông báo" is blocked by browser', async () => {
    const user = userEvent.setup()
    vi.spyOn(pushLib, 'requestPushPermission').mockResolvedValue('denied')

    render(<PushPrompt />)
    await user.click(screen.getByRole('button', { name: 'Bật thông báo' }))

    expect(toast.error).toHaveBeenCalledWith('Trình duyệt đang chặn thông báo. Mở lại trong Cài đặt → Thông báo')
    expect(screen.queryByText('Nhận thông báo khi có tin mới?')).not.toBeInTheDocument()
  })

  it('shows a retry-later toast and logs when the notification service cannot start', async () => {
    const user = userEvent.setup()
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.spyOn(pushLib, 'requestPushPermission').mockRejectedValue(new Error('OneSignal init timed out'))

    render(<PushPrompt />)
    await user.click(screen.getByRole('button', { name: 'Bật thông báo' }))

    expect(toast.error).toHaveBeenCalledWith('Không bật được thông báo. Thử lại sau.')
    expect(warn).toHaveBeenCalledWith('[push] could not enable notifications', expect.any(Error))
  })

  it('shows iOS home screen hint instead of action button when iOS Safari not installed', () => {
    vi.spyOn(pushLib, 'isIosNotInstalled').mockReturnValue(true)

    render(<PushPrompt />)
    expect(screen.getByText(/Thêm Nexus vào màn hình chính để nhận thông báo/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Bật thông báo' })).not.toBeInTheDocument()
  })
})
