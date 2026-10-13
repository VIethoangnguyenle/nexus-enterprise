import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { NotificationSettings } from './NotificationSettings'
import * as pushLib from '../../lib/push'
import { toast } from '../primitives'

vi.mock('../primitives', async () => {
  const actual = await vi.importActual('../primitives')
  const mockToast = Object.assign(vi.fn(), {
    error: vi.fn(),
    info: vi.fn(),
  })
  return {
    ...actual,
    toast: mockToast,
  }
})

describe('NotificationSettings component', () => {
  let queryClient: QueryClient
  const defaultPrefs: pushLib.PushPreferences = {
    kinds: {
      direct: true,
      mention: true,
      thread_reply: true,
    },
    quiet_hours: {
      enabled: false,
      start: '22:00',
      end: '07:00',
      time_zone: 'Asia/Ho_Chi_Minh',
    },
  }

  function renderWithQuery(ui: React.ReactElement) {
    return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
  }

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    vi.restoreAllMocks()

    vi.spyOn(pushLib, 'getOneSignalAppId').mockReturnValue('test-app-id')
    vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('granted')
    vi.spyOn(pushLib, 'isIosNotInstalled').mockReturnValue(false)
    vi.spyOn(pushLib, 'isSubscriptionOptedIn').mockResolvedValue(true)
    vi.spyOn(pushLib, 'setSubscriptionOptIn').mockResolvedValue(undefined)
    vi.spyOn(pushLib, 'initOneSignal').mockResolvedValue(null)
    vi.spyOn(pushLib.pushApi, 'getPreferences').mockResolvedValue(defaultPrefs)
    vi.spyOn(pushLib.pushApi, 'updatePreferences').mockResolvedValue(defaultPrefs)
  })

  afterEach(() => {
    vi.restoreAllMocks()
    queryClient.clear()
  })

  it('shows not available state when App ID is not configured', async () => {
    vi.spyOn(pushLib, 'getOneSignalAppId').mockReturnValue(undefined)

    renderWithQuery(<NotificationSettings />)

    expect(screen.getByText(/không khả dụng/i)).toBeInTheDocument()
    expect(screen.queryByText('Thiết bị này')).not.toBeInTheDocument()
  })

  it('renders device push switch and preference toggles when App ID is present', async () => {
    renderWithQuery(<NotificationSettings />)

    expect(await screen.findByText('Thiết bị này')).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên thiết bị này' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Tin nhắn trực tiếp' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Được nhắc tên' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Thread bạn theo dõi' })).toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Giờ yên lặng' })).toBeInTheDocument()
  })

  it('shows blocked banner and disables device switch when permission is blocked', async () => {
    vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('denied')

    renderWithQuery(<NotificationSettings />)

    expect(await screen.findByText(/Trình duyệt đang chặn thông báo/i)).toBeInTheDocument()
    const devSwitch = screen.getByRole('switch', { name: 'Thông báo đẩy trên thiết bị này' })
    expect(devSwitch).toBeDisabled()
  })

  it('shows iOS banner when on iOS Safari not added to home screen', async () => {
    vi.spyOn(pushLib, 'isIosNotInstalled').mockReturnValue(true)

    renderWithQuery(<NotificationSettings />)

    expect(await screen.findByText(/Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone/i)).toBeInTheDocument()
  })

  it('toggles device push opt-in / opt-out', async () => {
    const user = userEvent.setup()
    renderWithQuery(<NotificationSettings />)

    const devSwitch = await screen.findByRole('switch', { name: 'Thông báo đẩy trên thiết bị này' })
    // Wait for the initial SDK check to resolve (M3: starts as null/loading)
    await waitFor(() => {
      expect(devSwitch).toHaveAttribute('aria-checked', 'true')
    })

    await user.click(devSwitch)
    expect(pushLib.setSubscriptionOptIn).toHaveBeenCalledWith(false)
    expect(devSwitch).toHaveAttribute('aria-checked', 'false')
  })

  it('toggles kind and sends PUT with updated preferences', async () => {
    const user = userEvent.setup()
    const updateSpy = vi.spyOn(pushLib.pushApi, 'updatePreferences').mockResolvedValue({
      ...defaultPrefs,
      kinds: { ...defaultPrefs.kinds, direct: false },
    })

    renderWithQuery(<NotificationSettings />)

    const dmSwitch = await screen.findByRole('switch', { name: 'Tin nhắn trực tiếp' })
    expect(dmSwitch).toHaveAttribute('aria-checked', 'true')

    await user.click(dmSwitch)

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledWith({
        ...defaultPrefs,
        kinds: {
          ...defaultPrefs.kinds,
          direct: false,
        },
      })
    })
  })

  it('toggles quiet hours and time inputs', async () => {
    const user = userEvent.setup()
    const updateSpy = vi.spyOn(pushLib.pushApi, 'updatePreferences').mockResolvedValue({
      ...defaultPrefs,
      quiet_hours: { ...defaultPrefs.quiet_hours, enabled: true },
    })

    renderWithQuery(<NotificationSettings />)

    const quietSwitch = await screen.findByRole('switch', { name: 'Giờ yên lặng' })
    expect(quietSwitch).toHaveAttribute('aria-checked', 'false')

    await user.click(quietSwitch)

    await waitFor(() => {
      expect(updateSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          quiet_hours: expect.objectContaining({ enabled: true }),
        }),
      )
    })
  })

  it('rolls back and shows toast on mutation error', async () => {
    const user = userEvent.setup()
    vi.spyOn(pushLib.pushApi, 'updatePreferences').mockRejectedValue(new Error('Network error'))

    renderWithQuery(<NotificationSettings />)

    const mentionSwitch = await screen.findByRole('switch', { name: 'Được nhắc tên' })
    expect(mentionSwitch).toHaveAttribute('aria-checked', 'true')

    await user.click(mentionSwitch)

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
      expect(mentionSwitch).toHaveAttribute('aria-checked', 'true')
    })
  })

  it('shows error state with retry when GET preferences fails (M2)', async () => {
    vi.spyOn(pushLib.pushApi, 'getPreferences').mockRejectedValue(new Error('500'))

    renderWithQuery(<NotificationSettings />)

    await waitFor(() => {
      expect(screen.getByText(/Không tải được cài đặt thông báo/i)).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Thử lại' })).toBeInTheDocument()
    // Controls should NOT be visible
    expect(screen.queryByRole('switch', { name: 'Tin nhắn trực tiếp' })).not.toBeInTheDocument()
  })

  it('shows device switch as disabled initially before SDK responds (M3)', async () => {
    // isSubscriptionOptedIn never resolves
    vi.spyOn(pushLib, 'isSubscriptionOptedIn').mockReturnValue(new Promise(() => {}))

    renderWithQuery(<NotificationSettings />)

    await waitFor(() => {
      expect(screen.getByText(/Đang kiểm tra/)).toBeInTheDocument()
    })
    const devSwitch = screen.getByRole('switch', { name: 'Thông báo đẩy trên thiết bị này' })
    expect(devSwitch).toBeDisabled()
  })

  it('shows a retryable failure instead of staying on "checking" when the SDK cannot start', async () => {
    const user = userEvent.setup()
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const optedIn = vi
      .spyOn(pushLib, 'isSubscriptionOptedIn')
      .mockRejectedValueOnce(new Error('OneSignal init timed out'))
      .mockResolvedValue(true)

    renderWithQuery(<NotificationSettings />)

    expect(await screen.findByText('Chưa kết nối được dịch vụ thông báo')).toBeInTheDocument()
    expect(screen.queryByText(/Đang kiểm tra/)).not.toBeInTheDocument()
    expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên thiết bị này' })).toBeDisabled()
    expect(warn).toHaveBeenCalledWith('[push] could not read device subscription', expect.any(Error))

    await user.click(screen.getByRole('button', { name: 'Thử lại' }))

    await waitFor(() => {
      expect(screen.getByRole('switch', { name: 'Thông báo đẩy trên thiết bị này' })).toHaveAttribute(
        'aria-checked',
        'true',
      )
    })
    expect(optedIn).toHaveBeenCalledTimes(2)
    expect(screen.queryByText('Chưa kết nối được dịch vụ thông báo')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Thử lại' })).not.toBeInTheDocument()
  })

  it('shows the failure when wiring SDK listeners fails after the first read', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.spyOn(pushLib, 'initOneSignal').mockRejectedValue(new Error('boom'))

    renderWithQuery(<NotificationSettings />)

    expect(await screen.findByText('Chưa kết nối được dịch vụ thông báo')).toBeInTheDocument()
  })

  it('keeps the blocked state without a retry action', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.spyOn(pushLib, 'getNotificationPermission').mockReturnValue('denied')
    vi.spyOn(pushLib, 'isSubscriptionOptedIn').mockRejectedValue(new Error('x'))

    renderWithQuery(<NotificationSettings />)

    expect(await screen.findByText('Trình duyệt đang chặn')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Thử lại' })).not.toBeInTheDocument()
  })
})
