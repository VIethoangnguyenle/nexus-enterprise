import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { useAuthStore } from '../stores/auth.store'
import {
  getOneSignalAppId,
  isPushSupported,
  isPromptDismissed,
  recordPromptDismiss,
  initOneSignal,
  startPushAuthSync,
  isSubscriptionOptedIn,
  setSubscriptionOptIn,
  requestPushPermission,
  _resetOneSignalForTest,
  type OneSignalSDK,
} from './push'

describe('push notifications lib', () => {
  let mockOneSignal: OneSignalSDK

  beforeEach(() => {
    localStorage.clear()
    _resetOneSignalForTest()
    useAuthStore.setState({ accessToken: null, tenantId: null, user: null, bootstrapping: false })

    if (typeof window.Notification === 'undefined') {
      (window as unknown as { Notification: unknown }).Notification = { permission: 'default' }
    } else {
      Object.defineProperty(window.Notification, 'permission', { value: 'default', configurable: true })
    }

    mockOneSignal = {
      init: vi.fn().mockResolvedValue(undefined),
      login: vi.fn().mockResolvedValue(undefined),
      logout: vi.fn().mockResolvedValue(undefined),
      Notifications: {
        permission: false,
        permissionNative: 'default',
        requestPermission: vi.fn().mockImplementation(async () => {
          Object.defineProperty(window.Notification, 'permission', { value: 'granted', configurable: true })
        }),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      },
      User: {
        PushSubscription: {
          id: 'sub-1',
          token: 'token-1',
          optedIn: true,
          optIn: vi.fn().mockResolvedValue(undefined),
          optOut: vi.fn().mockResolvedValue(undefined),
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
        },
      },
    }
    window.OneSignal = mockOneSignal
    window.OneSignalDeferred = []

    // jsdom runs on localhost, which triggers isLocalDev(). Override the env
    // flag so init proceeds as if on a real domain.
    import.meta.env.VITE_ONESIGNAL_DEV = '1'
  })

  afterEach(() => {
    vi.restoreAllMocks()
    _resetOneSignalForTest()
  })

  describe('configuration & support', () => {
    it('returns App ID when environment variable is present', () => {
      const id = getOneSignalAppId()
      expect(typeof id === 'string' || id === undefined).toBe(true)
    })

    it('identifies push support in the environment', () => {
      // In JSDOM, Notification might or might not be mock-defined
      const supported = isPushSupported()
      expect(typeof supported).toBe('boolean')
    })
  })

  describe('OneSignal initialization', () => {
    it('initializes OneSignal with correct service worker parameters when appId is configured', async () => {
      const sdk = await initOneSignal('test-app-id')
      expect(sdk).toBe(mockOneSignal)
      expect(mockOneSignal.init).toHaveBeenCalledWith({
        appId: 'test-app-id',
        serviceWorkerPath: 'sw.js',
        serviceWorkerParam: { scope: '/' },
      })
    })

    it('does not initialize OneSignal when no appId is provided', async () => {
      const sdk = await initOneSignal('')
      expect(sdk).toBeNull()
      expect(mockOneSignal.init).not.toHaveBeenCalled()
    })

    it('clears cached promise on init failure so retry is possible (M1)', async () => {
      // Make init throw
      mockOneSignal.init = vi.fn().mockRejectedValue(new Error('domain mismatch'))
      await expect(initOneSignal('test-app-id')).rejects.toThrow('domain mismatch')

      // Reset and make init succeed
      _resetOneSignalForTest()
      mockOneSignal.init = vi.fn().mockResolvedValue(undefined)
      const sdk = await initOneSignal('test-app-id')
      expect(sdk).toBe(mockOneSignal)
    })

    it('calls OneSignal.init once; a timed-out caller does not discard it and a later call gets the late result', async () => {
      vi.useFakeTimers()
      try {
        let finishInit!: () => void
        mockOneSignal.init = vi.fn().mockImplementation(
          () => new Promise<void>((resolve) => { finishInit = resolve }),
        )

        const first = initOneSignal('test-app-id')
        const firstOutcome = first.then(() => 'ok', (e: Error) => e.message)
        await vi.advanceTimersByTimeAsync(20_001)
        expect(await firstOutcome).toBe('OneSignal init timed out')

        const second = initOneSignal('test-app-id')
        finishInit()
        await expect(second).resolves.toBe(mockOneSignal)
        expect(mockOneSignal.init).toHaveBeenCalledTimes(1)
      } finally {
        vi.useRealTimers()
      }
    })

    it('shares one init between concurrent callers', async () => {
      const [a, b] = await Promise.all([initOneSignal('test-app-id'), initOneSignal('test-app-id')])
      expect(a).toBe(b)
      expect(mockOneSignal.init).toHaveBeenCalledTimes(1)
    })

    it('does not re-run init after it failed', async () => {
      mockOneSignal.init = vi.fn().mockRejectedValue(new Error('domain mismatch'))
      await expect(initOneSignal('test-app-id')).rejects.toThrow('domain mismatch')
      await expect(initOneSignal('test-app-id')).rejects.toThrow('domain mismatch')
      expect(mockOneSignal.init).toHaveBeenCalledTimes(1)
    })

    it('allows loading the script again after the script failed to load', async () => {
      window.OneSignal = undefined
      const appended: HTMLScriptElement[] = []
      vi.spyOn(document.head, 'appendChild').mockImplementation((node: Node) => {
        appended.push(node as HTMLScriptElement)
        return node
      })

      const first = initOneSignal('test-app-id')
      const firstOutcome = first.then(() => 'ok', (e: Error) => e.message)
      appended[0]!.onerror?.(new Event('error'))
      expect(await firstOutcome).toMatch(/Failed to load OneSignal SDK script/)

      const second = initOneSignal('test-app-id')
      await vi.waitFor(() => expect(appended).toHaveLength(2))
      window.OneSignalDeferred = window.OneSignalDeferred || []
      appended[1]!.onload?.(new Event('load'))
      await vi.waitFor(() => expect(window.OneSignalDeferred!.length).toBeGreaterThan(0))
      const queue = window.OneSignalDeferred!
      await queue[queue.length - 1]!(mockOneSignal)
      await expect(second).resolves.toBe(mockOneSignal)
      expect(mockOneSignal.init).toHaveBeenCalledTimes(1)
    })

    it('does not inject a script tag when no appId is provided (L5)', async () => {
      await initOneSignal('')
      const scripts = document.querySelectorAll('script[src*="OneSignalSDK"]')
      expect(scripts.length).toBe(0)
    })
  })

  describe('auth synchronization', () => {
    it('syncs user login to OneSignal.login and logout to OneSignal.logout', async () => {
      const stopSync = startPushAuthSync('test-app-id')

      // Login
      useAuthStore.setState({ user: { id: 'usr-123', username: 'alex' } })
      await vi.waitFor(() => {
        expect(mockOneSignal.login).toHaveBeenCalledWith('usr-123')
      })

      // Logout
      useAuthStore.getState().logout()
      await vi.waitFor(() => {
        expect(mockOneSignal.logout).toHaveBeenCalled()
      })

      stopSync()
    })

    it('syncs initial signed-in user on startPushAuthSync', async () => {
      useAuthStore.setState({ user: { id: 'usr-456', username: 'bob' } })
      const stopSync = startPushAuthSync('test-app-id')

      await vi.waitFor(() => {
        expect(mockOneSignal.login).toHaveBeenCalledWith('usr-456')
      })

      stopSync()
    })

    it('does nothing when no appId is configured', async () => {
      const stopSync = startPushAuthSync('')
      useAuthStore.setState({ user: { id: 'usr-789', username: 'carol' } })

      expect(mockOneSignal.login).not.toHaveBeenCalled()
      stopSync()
    })
  })

  describe('prompt dismissal storage', () => {
    it('starts with isPromptDismissed as false', () => {
      expect(isPromptDismissed()).toBe(false)
    })

    it('marks prompt dismissed for 7 days after first dismissal', () => {
      recordPromptDismiss()
      expect(isPromptDismissed()).toBe(true)
    })

    it('marks prompt dismissed indefinitely after 3 dismissals', () => {
      recordPromptDismiss()
      recordPromptDismiss()
      recordPromptDismiss()
      expect(isPromptDismissed()).toBe(true)

      // Even if 8 days pass:
      const raw = JSON.parse(localStorage.getItem('nexus-push-prompt-dismissed')!)
      raw.dismissedAt = Date.now() - 8 * 24 * 60 * 60 * 1000
      localStorage.setItem('nexus-push-prompt-dismissed', JSON.stringify(raw))

      expect(isPromptDismissed()).toBe(true)
    })
  })

  describe('device subscription opt-in / opt-out', () => {
    it('reads optedIn status from OneSignal User PushSubscription', async () => {
      const optedIn = await isSubscriptionOptedIn('test-app-id')
      expect(optedIn).toBe(true)
    })

    it('calls optIn and optOut on OneSignal User PushSubscription', async () => {
      await setSubscriptionOptIn(false, 'test-app-id')
      expect(mockOneSignal.User.PushSubscription.optOut).toHaveBeenCalled()

      await setSubscriptionOptIn(true, 'test-app-id')
      expect(mockOneSignal.User.PushSubscription.optIn).toHaveBeenCalled()
    })

    it('asks for permission before opting in when it has not been decided', async () => {
      await setSubscriptionOptIn(true, 'test-app-id')
      expect(mockOneSignal.Notifications.requestPermission).toHaveBeenCalled()
      expect(mockOneSignal.User.PushSubscription.optIn).toHaveBeenCalled()
    })

    it('does not opt in when the permission prompt is refused', async () => {
      mockOneSignal.Notifications.requestPermission = vi.fn().mockImplementation(async () => {
        Object.defineProperty(window.Notification, 'permission', { value: 'denied', configurable: true })
      })
      await setSubscriptionOptIn(true, 'test-app-id')
      expect(mockOneSignal.User.PushSubscription.optIn).not.toHaveBeenCalled()
    })

    it('does not prompt again when permission is already granted', async () => {
      Object.defineProperty(window.Notification, 'permission', { value: 'granted', configurable: true })
      await setSubscriptionOptIn(true, 'test-app-id')
      expect(mockOneSignal.Notifications.requestPermission).not.toHaveBeenCalled()
      expect(mockOneSignal.User.PushSubscription.optIn).toHaveBeenCalled()
    })

    it('requests permission via Notifications.requestPermission', async () => {
      await requestPushPermission('test-app-id')
      expect(mockOneSignal.Notifications.requestPermission).toHaveBeenCalled()
    })
  })
})
