import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Bell, BellOff, MessageSquare, AtSign, MessagesSquare, Moon, AlertTriangle } from 'lucide-react'
import {
  getOneSignalAppId,
  getNotificationPermission,
  isIosNotInstalled,
  isSubscriptionOptedIn,
  setSubscriptionOptIn,
  initOneSignal,
  pushApi,
  pushQueryKeys,
  type PushPreferences,
} from '../../lib/push'
import { Button, Pressable, Select, toast } from '../primitives'
import { EmptyState } from '../spaces/EmptyState'

const HOURS = Array.from({ length: 24 }, (_, i) => `${String(i).padStart(2, '0')}:00`)

const DEFAULT_PREFERENCES: PushPreferences = {
  kinds: {
    direct: true,
    mention: true,
    thread_reply: true,
  },
  quiet_hours: {
    enabled: false,
    start: '22:00',
    end: '07:00',
    time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Ho_Chi_Minh',
  },
}

export function NotificationSettings() {
  const appId = getOneSignalAppId()
  const queryClient = useQueryClient()

  // M3: null = unknown/loading. Never show the switch as 'on' before the SDK confirms.
  const [deviceOptedIn, setDeviceOptedIn] = useState<boolean | null>(null)
  const [deviceFailed, setDeviceFailed] = useState(false)
  const [attempt, setAttempt] = useState(0)
  const [deviceBusy, setDeviceBusy] = useState(false)
  const [permission, setPermission] = useState<NotificationPermission>(getNotificationPermission)
  const [quietError, setQuietError] = useState<string | null>(null)
  const isBlocked = permission === 'denied'
  const isIos = isIosNotInstalled()

  useEffect(() => {
    if (!appId) return

    let cancelled = false
    setDeviceFailed(false)
    setDeviceOptedIn(null)

    // Subscribe to permission changes via the Permissions API (fires when the user
    // toggles "Allow notifications" in browser settings — fulfils the "công tắc sẽ tự bật" promise).
    let permStatus: PermissionStatus | null = null
    const onPermChange = () => setPermission(getNotificationPermission())
    if (navigator.permissions?.query) {
      navigator.permissions
        .query({ name: 'notifications' })
        .then((status) => {
          if (cancelled) return
          permStatus = status
          permStatus.addEventListener('change', onPermChange)
        })
        .catch(() => {
          // Permissions API unsupported for notifications (older Safari): the SDK events cover it.
        })
    }

    // Initial state and reactive updates from the SDK. A failed or timed-out init must
    // surface as a retryable state, never as an endless "checking".
    let detach: (() => void) | null = null
    isSubscriptionOptedIn(appId)
      .then(async (on) => {
        if (cancelled) return
        setDeviceOptedIn(on)
        const sdk = await initOneSignal(appId)
        if (!sdk || cancelled) return
        const subListener = () => setDeviceOptedIn(!!sdk.User.PushSubscription.optedIn)
        sdk.User.PushSubscription.addEventListener('change', subListener)
        sdk.Notifications.addEventListener('permissionChange', onPermChange)
        detach = () => {
          sdk.User.PushSubscription.removeEventListener('change', subListener)
          sdk.Notifications.removeEventListener('permissionChange', onPermChange)
        }
      })
      .catch((err) => {
        console.warn('[push] could not read device subscription', err)
        if (cancelled) return
        setDeviceOptedIn(null)
        setDeviceFailed(true)
      })

    return () => {
      cancelled = true
      permStatus?.removeEventListener('change', onPermChange)
      detach?.()
    }
  }, [appId, attempt])

  const { data: preferences, isLoading, isError, refetch } = useQuery({
    queryKey: pushQueryKeys.preferences,
    queryFn: pushApi.getPreferences,
    enabled: !!appId,
  })

  const mutation = useMutation({
    mutationFn: (newPrefs: PushPreferences) => pushApi.updatePreferences(newPrefs),
    onMutate: async (newPrefs) => {
      await queryClient.cancelQueries({ queryKey: pushQueryKeys.preferences })
      const previous = queryClient.getQueryData<PushPreferences>(pushQueryKeys.preferences)
      queryClient.setQueryData<PushPreferences>(pushQueryKeys.preferences, newPrefs)
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(pushQueryKeys.preferences, context.previous)
      }
      toast.error('Không lưu được cài đặt thông báo')
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: pushQueryKeys.preferences })
    },
  })

  if (!appId) {
    return (
      <div className="flex-1 grid place-items-center bg-base py-12">
        <EmptyState
          icon={<BellOff size={24} strokeWidth={1.75} />}
          text="Thông báo đẩy không khả dụng trên môi trường này."
        />
      </div>
    )
  }

  if (isError) {
    return (
      <div className="flex-1 grid place-items-center bg-base py-12">
        <div className="flex flex-col items-center gap-3 text-center">
          <AlertTriangle size={24} strokeWidth={1.75} className="text-danger" />
          <p className="text-sm text-ink-muted">Không tải được cài đặt thông báo.</p>
          <Button variant="ghost" size="sm" onClick={() => void refetch()}>
            Thử lại
          </Button>
        </div>
      </div>
    )
  }

  // Use fetched preferences, or defaults only while still loading (never on error).
  const prefs = preferences ?? DEFAULT_PREFERENCES

  const handleDeviceToggle = async () => {
    if (isBlocked || isIos || deviceBusy || deviceOptedIn === null) return
    const next = !deviceOptedIn
    setDeviceOptedIn(next)
    setDeviceBusy(true)
    try {
      await setSubscriptionOptIn(next)
      setPermission(getNotificationPermission())
    } catch (err) {
      console.warn('[push] could not change device subscription', err)
      setDeviceOptedIn(!next)
      toast.error('Không thay đổi được cài đặt thiết bị')
    } finally {
      setDeviceBusy(false)
    }
  }

  const updateKinds = (key: keyof PushPreferences['kinds']) => {
    const nextPrefs: PushPreferences = {
      ...prefs,
      kinds: {
        ...prefs.kinds,
        [key]: !prefs.kinds[key],
      },
    }
    mutation.mutate(nextPrefs)
  }

  const updateQuietHours = (changes: Partial<PushPreferences['quiet_hours']>) => {
    const merged = { ...prefs.quiet_hours, ...changes }
    // M5: validate start ≠ end (the server returns 400, but we catch it client-side with a clear message).
    if (merged.start === merged.end && merged.enabled) {
      setQuietError('Giờ bắt đầu và kết thúc không được trùng.')
      return
    }
    setQuietError(null)
    const nextPrefs: PushPreferences = {
      ...prefs,
      quiet_hours: {
        ...merged,
        // M4: always send the browser's timezone so the server applies the correct window.
        time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone || prefs.quiet_hours.time_zone,
      },
    }
    mutation.mutate(nextPrefs)
  }

  const deviceSubtext = isBlocked
    ? 'Trình duyệt đang chặn'
    : isIos
      ? 'Cần thêm Nexus vào màn hình chính'
      : deviceFailed
        ? 'Chưa kết nối được dịch vụ thông báo'
        : deviceOptedIn === null
          ? 'Đang kiểm tra…'
          : deviceOptedIn
            ? 'Đang bật trên thiết bị này'
            : 'Đang tắt trên thiết bị này'

  const deviceDisabled = isBlocked || isIos || deviceBusy || deviceOptedIn === null
  const dimmed = deviceOptedIn === false || deviceOptedIn === null || isBlocked

  return (
    <div className="flex-1 flex flex-col gap-6 max-w-2xl animate-fade-in">
      {/* Banners */}
      {isBlocked && (
        <div
          role="status"
          className="p-3.5 rounded-xl bg-danger-wash text-ink border border-danger/20 text-xs flex flex-col gap-2"
        >
          <strong className="font-semibold text-danger">Trình duyệt đang chặn thông báo của Nexus</strong>
          <ol className="list-decimal pl-4 space-y-1 text-ink-muted">
            <li>Bấm biểu tượng ổ khoá cạnh địa chỉ trang.</li>
            <li>Chọn Quyền → Thông báo → Cho phép.</li>
            <li>Quay lại đây, công tắc sẽ tự bật.</li>
          </ol>
        </div>
      )}

      {isIos && (
        <div
          role="status"
          className="p-3.5 rounded-xl bg-info-wash text-ink border border-info/20 text-xs flex flex-col gap-1.5"
        >
          <strong className="font-semibold text-info">
            Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone.
          </strong>
        </div>
      )}

      {/* Group 1: Thiết bị này */}
      <section aria-labelledby="heading-device" className="flex flex-col gap-3">
        <h2 id="heading-device" className="text-xs font-semibold tracking-wider uppercase text-ink-muted">
          Thiết bị này
        </h2>
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <Bell size={16} strokeWidth={1.75} />
            </span>
            <div className="grid gap-0.5">
              <span id="label-dev" className="text-sm font-semibold text-ink">
                Thông báo đẩy
              </span>
              <span className="text-xs text-ink-muted">{deviceSubtext}</span>
              {deviceFailed && !isBlocked && !isIos ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="justify-self-start"
                  onClick={() => setAttempt((n) => n + 1)}
                >
                  Thử lại
                </Button>
              ) : null}
            </div>
          </div>
          <Pressable
            role="switch"
            aria-checked={deviceOptedIn ?? false}
            aria-label="Thông báo đẩy trên thiết bị này"
            disabled={deviceDisabled}
            onClick={handleDeviceToggle}
            className={`relative shrink-0 w-11 h-6 rounded-full transition-opacity ${
              deviceDisabled ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'
            }`}
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                deviceOptedIn ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${deviceOptedIn ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>
      </section>

      {/* Group 2: Báo cho tôi khi */}
      <section
        aria-labelledby="heading-kinds"
        className={`flex flex-col gap-3 transition-opacity ${dimmed ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <h2 id="heading-kinds" className="text-xs font-semibold tracking-wider uppercase text-ink-muted">
          Báo cho tôi khi
        </h2>

        {/* Kind 1: Tin nhắn trực tiếp */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <MessageSquare size={16} strokeWidth={1.75} />
            </span>
            <span id="label-direct" className="text-sm font-semibold text-ink">
              Tin nhắn trực tiếp
            </span>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.direct}
            aria-label="Tin nhắn trực tiếp"
            disabled={isLoading || mutation.isPending}
            onClick={() => updateKinds('direct')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.direct ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.direct ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>

        {/* Kind 2: Được nhắc tên */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <AtSign size={16} strokeWidth={1.75} />
            </span>
            <div className="grid gap-0.5">
              <span id="label-mention" className="text-sm font-semibold text-ink">
                Được nhắc tên
              </span>
              <span className="text-xs text-ink-muted">Trong nhóm và thread</span>
            </div>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.mention}
            aria-label="Được nhắc tên"
            disabled={isLoading || mutation.isPending}
            onClick={() => updateKinds('mention')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.mention ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.mention ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>

        {/* Kind 3: Thread bạn theo dõi */}
        <div className="flex items-center justify-between gap-4 py-2 border-b border-line">
          <div className="flex items-center gap-3 min-w-0">
            <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
              <MessagesSquare size={16} strokeWidth={1.75} />
            </span>
            <div className="grid gap-0.5">
              <span id="label-threads" className="text-sm font-semibold text-ink">
                Thread bạn theo dõi
              </span>
              <span className="text-xs text-ink-muted">Trả lời mới trong thread đang theo dõi</span>
            </div>
          </div>
          <Pressable
            role="switch"
            aria-checked={prefs.kinds.thread_reply}
            aria-label="Thread bạn theo dõi"
            disabled={isLoading || mutation.isPending}
            onClick={() => updateKinds('thread_reply')}
            className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
          >
            <span
              aria-hidden="true"
              className={`absolute inset-0 rounded-full transition-colors duration-press ${
                prefs.kinds.thread_reply ? 'bg-accent' : 'bg-sunk field-line'
              }`}
            />
            <span
              aria-hidden="true"
              className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                transition-transform duration-press ${prefs.kinds.thread_reply ? 'translate-x-5' : ''}`}
            />
          </Pressable>
        </div>
      </section>

      {/* Group 3: Giờ yên lặng */}
      <section
        aria-labelledby="heading-quiet"
        className={`flex flex-col gap-3 transition-opacity ${dimmed ? 'opacity-50 pointer-events-none' : ''}`}
      >
        <h2 id="heading-quiet" className="text-xs font-semibold tracking-wider uppercase text-ink-muted">
          Giờ yên lặng
        </h2>

        <div className="flex flex-col gap-3 py-2">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3 min-w-0">
              <span aria-hidden="true" className="w-8 h-8 rounded-lg bg-sunk text-ink-muted flex items-center justify-center shrink-0">
                <Moon size={16} strokeWidth={1.75} />
              </span>
              <div className="grid gap-0.5">
                <span id="label-quiet" className="text-sm font-semibold text-ink">
                  Giờ yên lặng
                </span>
                <span className="text-xs text-ink-muted">Không báo trong khoảng giờ này</span>
              </div>
            </div>
            <Pressable
              role="switch"
              aria-checked={prefs.quiet_hours.enabled}
              aria-label="Giờ yên lặng"
              disabled={isLoading || mutation.isPending}
              onClick={() => updateQuietHours({ enabled: !prefs.quiet_hours.enabled })}
              className="relative shrink-0 w-11 h-6 rounded-full cursor-pointer"
            >
              <span
                aria-hidden="true"
                className={`absolute inset-0 rounded-full transition-colors duration-press ${
                  prefs.quiet_hours.enabled ? 'bg-accent' : 'bg-sunk field-line'
                }`}
              />
              <span
                aria-hidden="true"
                className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-raised shadow-overlay
                  transition-transform duration-press ${prefs.quiet_hours.enabled ? 'translate-x-5' : ''}`}
              />
            </Pressable>
          </div>

          {prefs.quiet_hours.enabled && (
            <div className="flex items-center gap-3 pl-11 pt-1 animate-fade-in">
              <label htmlFor="q-from" className="text-xs font-medium text-ink-muted">
                Từ
              </label>
              <Select
                id="q-from"
                value={prefs.quiet_hours.start}
                onChange={(e) => updateQuietHours({ start: e.target.value })}
                className="h-9 px-2 rounded-lg bg-raised border border-line text-sm font-medium text-ink focus-ring"
              >
                {HOURS.map((h) => (
                  <option key={h} value={h}>
                    {h}
                  </option>
                ))}
              </Select>

              <span className="text-xs text-ink-muted">đến</span>

              <label htmlFor="q-to" className="text-xs font-medium text-ink-muted sr-only">
                Đến
              </label>
              <Select
                id="q-to"
                value={prefs.quiet_hours.end}
                onChange={(e) => updateQuietHours({ end: e.target.value })}
                className="h-9 px-2 rounded-lg bg-raised border border-line text-sm font-medium text-ink focus-ring"
              >
                {HOURS.map((h) => (
                  <option key={h} value={h}>
                    {h}
                  </option>
                ))}
              </Select>
            </div>
          )}
          {quietError && (
            <p className="text-xs text-danger pl-11 animate-fade-in" role="alert">{quietError}</p>
          )}
        </div>
      </section>
    </div>
  )
}
