import { apiFetch } from '../api/client'
import { useAuthStore } from '../stores/auth.store'

export interface OneSignalSDK {
  init(options: {
    appId: string
    serviceWorkerPath?: string
    serviceWorkerParam?: { scope: string }
    allowLocalhostAsSecureOrigin?: boolean
    [key: string]: unknown
  }): Promise<void>
  login(externalId: string): Promise<void>
  logout(): Promise<void>
  Notifications: {
    permission: boolean | string
    permissionNative?: NotificationPermission
    requestPermission(): Promise<void>
    addEventListener(event: string, listener: (arg: unknown) => void): void
    removeEventListener(event: string, listener: (arg: unknown) => void): void
  }
  User: {
    PushSubscription: {
      id?: string | null
      token?: string | null
      optedIn?: boolean
      optIn(): Promise<void>
      optOut(): Promise<void>
      addEventListener(event: string, listener: (arg: unknown) => void): void
      removeEventListener(event: string, listener: (arg: unknown) => void): void
    }
  }
}

declare global {
  interface Window {
    OneSignalDeferred?: Array<(oneSignal: OneSignalSDK) => void | Promise<void>>
    OneSignal?: OneSignalSDK
  }
}

export function getOneSignalAppId(): string | undefined {
  const id = import.meta.env.VITE_ONESIGNAL_APP_ID
  return typeof id === 'string' && id.trim() !== '' ? id.trim() : undefined
}

export function isPushSupported(): boolean {
  if (typeof window === 'undefined') return false
  return 'Notification' in window && 'serviceWorker' in navigator && 'PushManager' in window
}

export function isIOS(): boolean {
  if (typeof window === 'undefined') return false
  return (
    /iPad|iPhone|iPod/.test(navigator.userAgent) ||
    (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
  )
}

export function isStandalone(): boolean {
  if (typeof window === 'undefined') return false
  return (
    window.matchMedia('(display-mode: standalone)').matches ||
    ('standalone' in navigator && (navigator as { standalone?: boolean }).standalone === true)
  )
}

export function isIosNotInstalled(): boolean {
  return isIOS() && !isStandalone()
}

export function getNotificationPermission(): NotificationPermission {
  if (typeof window === 'undefined' || !('Notification' in window)) return 'default'
  return Notification.permission
}

let initPromise: Promise<OneSignalSDK | null> | null = null
const INIT_TIMEOUT_MS = 10_000

/**
 * Returns true when running on localhost/127.0.0.1 without the explicit dev
 * override. OneSignal web push needs the registered origin, so loading the SDK
 * on a dev server wastes time and fails with a domain mismatch.
 */
export function isLocalDev(): boolean {
  if (typeof window === 'undefined') return false
  const host = window.location.hostname
  if (host === 'localhost' || host === '127.0.0.1') {
    return import.meta.env.VITE_ONESIGNAL_DEV !== '1'
  }
  return false
}

export async function initOneSignal(overrideAppId?: string): Promise<OneSignalSDK | null> {
  const appId = overrideAppId !== undefined ? (overrideAppId || undefined) : getOneSignalAppId()
  if (!appId) return null
  // Don't load the SDK on localhost (domain mismatch → init throws) unless explicitly opted in.
  if (isLocalDev()) return null
  if (initPromise) return initPromise

  initPromise = (async () => {
    if (window.OneSignal) {
      await window.OneSignal.init({
        appId,
        serviceWorkerPath: 'sw.js',
        serviceWorkerParam: { scope: '/' },
      })
      return window.OneSignal
    }

    if (typeof document === 'undefined') return null

    await new Promise<void>((resolve, reject) => {
      const existing = document.querySelector('script[src*="OneSignalSDK.page.js"]')
      if (existing) {
        resolve()
        return
      }
      const script = document.createElement('script')
      script.src = 'https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js'
      script.defer = true
      script.onload = () => resolve()
      script.onerror = (e) => reject(new Error('Failed to load OneSignal SDK script: ' + String(e)))
      document.head.appendChild(script)
    })

    return new Promise<OneSignalSDK>((resolve, reject) => {
      const timer = setTimeout(() => {
        reject(new Error('OneSignal init timed out'))
      }, INIT_TIMEOUT_MS)

      window.OneSignalDeferred = window.OneSignalDeferred || []
      window.OneSignalDeferred.push(async (OneSignal) => {
        try {
          await OneSignal.init({
            appId,
            serviceWorkerPath: 'sw.js',
            serviceWorkerParam: { scope: '/' },
          })
          clearTimeout(timer)
          resolve(OneSignal)
        } catch (err) {
          clearTimeout(timer)
          reject(err)
        }
      })
    })
  })().catch((err) => {
    // Clear so a later call can retry (e.g. after ad-blocker is disabled).
    initPromise = null
    throw err
  })

  return initPromise
}

let unsubAuthSync: (() => void) | null = null

export function startPushAuthSync(overrideAppId?: string): () => void {
  const appId = overrideAppId !== undefined ? (overrideAppId || undefined) : getOneSignalAppId()
  if (!appId || isLocalDev()) return () => {}

  if (unsubAuthSync) return unsubAuthSync

  let lastUserId: string | null = null

  const syncUser = async (userId: string | null) => {
    if (userId === lastUserId) return
    lastUserId = userId
    try {
      const sdk = await initOneSignal(appId)
      if (!sdk) return
      if (userId) {
        await sdk.login(userId)
      } else {
        await sdk.logout()
      }
    } catch (err) {
      console.warn('[push] auth sync failed', err)
    }
  }

  const currentUser = useAuthStore.getState().user
  if (currentUser?.id) {
    void syncUser(currentUser.id)
  }

  const unsubscribe = useAuthStore.subscribe((state) => {
    const nextId = state.user?.id ?? null
    void syncUser(nextId)
  })

  unsubAuthSync = () => {
    unsubscribe()
    unsubAuthSync = null
  }

  return unsubAuthSync
}

export async function isSubscriptionOptedIn(overrideAppId?: string): Promise<boolean> {
  const sdk = await initOneSignal(overrideAppId)
  if (!sdk) return false
  return !!sdk.User?.PushSubscription?.optedIn
}

export async function setSubscriptionOptIn(optIn: boolean, overrideAppId?: string): Promise<void> {
  const sdk = await initOneSignal(overrideAppId)
  if (!sdk) return
  if (optIn) {
    await sdk.User.PushSubscription.optIn()
  } else {
    await sdk.User.PushSubscription.optOut()
  }
}

export async function requestPushPermission(overrideAppId?: string): Promise<NotificationPermission> {
  const sdk = await initOneSignal(overrideAppId)
  if (sdk?.Notifications?.requestPermission) {
    await sdk.Notifications.requestPermission()
  }
  return getNotificationPermission()
}

const PUSH_PROMPT_DISMISS_KEY = 'nexus-push-prompt-dismissed'
const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000

export interface PromptDismissState {
  count: number
  dismissedAt: number
}

export function isPromptDismissed(): boolean {
  try {
    const raw = localStorage.getItem(PUSH_PROMPT_DISMISS_KEY)
    if (!raw) return false
    const parsed: PromptDismissState = JSON.parse(raw)
    if (parsed.count >= 3) return true
    if (Date.now() - parsed.dismissedAt < SEVEN_DAYS_MS) return true
    return false
  } catch {
    return false
  }
}

export function recordPromptDismiss(): void {
  try {
    const raw = localStorage.getItem(PUSH_PROMPT_DISMISS_KEY)
    const prev: PromptDismissState = raw ? JSON.parse(raw) : { count: 0, dismissedAt: 0 }
    const next: PromptDismissState = {
      count: (prev.count || 0) + 1,
      dismissedAt: Date.now(),
    }
    localStorage.setItem(PUSH_PROMPT_DISMISS_KEY, JSON.stringify(next))
  } catch {
    // localStorage safely ignored
  }
}

export function _resetOneSignalForTest(): void {
  initPromise = null
  if (unsubAuthSync) {
    unsubAuthSync()
    unsubAuthSync = null
  }
}

export interface PushPreferences {
  kinds: {
    direct: boolean
    mention: boolean
    thread_reply: boolean
  }
  quiet_hours: {
    enabled: boolean
    start: string // HH:MM
    end: string // HH:MM
    time_zone: string
  }
}

export const pushQueryKeys = {
  preferences: ['me', 'push-preferences'] as const,
}

export const pushApi = {
  getPreferences: () => apiFetch<PushPreferences>('/me/push-preferences'),
  updatePreferences: (prefs: PushPreferences) =>
    apiFetch<PushPreferences>('/me/push-preferences', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(prefs),
    }),
}
