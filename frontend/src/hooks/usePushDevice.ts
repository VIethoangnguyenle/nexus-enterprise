import { useCallback, useEffect, useState } from 'react'
import {
  getNotificationPermission,
  getOneSignalAppId,
  initOneSignal,
  isIosNotInstalled,
  isSubscriptionOptedIn,
  setSubscriptionOptIn,
} from '../lib/push'
import { toast } from '../components/primitives'

/**
 * Push on this device: whether the browser lets Nexus notify, whether this
 * device is subscribed, and the switch that changes it. `optedIn` is null while
 * unknown: the switch is never drawn on before the SDK confirms. A failed or
 * timed-out start of the SDK is a retryable state (`failed`, `retry`), never an
 * endless "checking".
 */
export function usePushDevice() {
  const appId = getOneSignalAppId()
  const [optedIn, setOptedIn] = useState<boolean | null>(null)
  const [failed, setFailed] = useState(false)
  const [attempt, setAttempt] = useState(0)
  const [busy, setBusy] = useState(false)
  const [permission, setPermission] = useState<NotificationPermission>(getNotificationPermission)
  const blocked = permission === 'denied'
  const ios = isIosNotInstalled()

  useEffect(() => {
    if (!appId) return

    let cancelled = false
    setFailed(false)
    setOptedIn(null)

    // Permission changes made in the browser's own settings flip the switch by themselves.
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

    let detach: (() => void) | null = null
    isSubscriptionOptedIn(appId)
      .then(async (on) => {
        if (cancelled) return
        setOptedIn(on)
        const sdk = await initOneSignal(appId)
        if (!sdk || cancelled) return
        const subListener = () => setOptedIn(!!sdk.User.PushSubscription.optedIn)
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
        setOptedIn(null)
        setFailed(true)
      })

    return () => {
      cancelled = true
      permStatus?.removeEventListener('change', onPermChange)
      detach?.()
    }
  }, [appId, attempt])

  const disabled = blocked || ios || busy || optedIn === null

  const toggle = useCallback(async () => {
    if (disabled) return
    const next = !optedIn
    setOptedIn(next)
    setBusy(true)
    try {
      await setSubscriptionOptIn(next)
      setPermission(getNotificationPermission())
    } catch (err) {
      console.warn('[push] could not change device subscription', err)
      setOptedIn(!next)
      toast.error('Không thay đổi được cài đặt thiết bị')
    } finally {
      setBusy(false)
    }
  }, [disabled, optedIn])

  const status = blocked
    ? 'Trình duyệt đang chặn'
    : ios
      ? 'Cần thêm Nexus vào màn hình chính'
      : failed
        ? 'Chưa kết nối được dịch vụ thông báo'
        : optedIn === null
          ? 'Đang kiểm tra…'
          : optedIn
            ? 'Đang bật trên thiết bị này'
            : 'Đang tắt trên thiết bị này'

  return {
    /** False where push is not configured at all (no OneSignal app). */
    available: !!appId,
    optedIn,
    failed,
    blocked,
    ios,
    disabled,
    /** One line saying where the device stands. */
    status,
    toggle,
    retry: () => setAttempt((n) => n + 1),
  }
}
