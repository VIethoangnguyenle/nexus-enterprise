/// <reference types="vite-plugin-pwa/client" />
import { usePwaStore } from './pwa'
import { getOneSignalAppId } from './push'

const MIN_CHECK_GAP_MS = 60_000
const CHECK_INTERVAL_MS = 30 * 60_000

/**
 * Browsers only look for a new worker on a full navigation, and an installed PWA resumed from
 * the app switcher never navigates. So ask explicitly: when the page comes back to the
 * foreground (throttled) and on a slow interval while it is visible. A worker that finished
 * installing while the app was in the background is surfaced on resume as well.
 */
function watchForUpdates(reg: ServiceWorkerRegistration): void {
  let lastCheck = 0
  const check = () => {
    if (document.visibilityState !== 'visible') return
    if (reg.waiting) usePwaStore.setState({ needRefresh: true })
    const now = Date.now()
    if (now - lastCheck < MIN_CHECK_GAP_MS) return
    lastCheck = now
    // Offline or a transient network error: the next resume or tick tries again.
    reg.update().catch(() => {})
  }
  document.addEventListener('visibilitychange', check)
  window.addEventListener('pageshow', (e) => { if (e.persisted) check() })
  window.addEventListener('focus', check)
  window.setInterval(check, CHECK_INTERVAL_MS)
}

/**
 * Register the service worker. Production builds only: in `make dev` Vite serves
 * source modules and the proxy table, and a worker would sit in front of both.
 * A failed registration leaves the app working as a plain web page.
 *
 * When OneSignal is configured it registers the worker itself, at the URL its dashboard
 * names (`/OneSignalSDKWorker.js?appId=…`, which imports `/sw.js`), so we must NOT call
 * registerSW (which registers `/sw.js` — a different URL for the same scope, causing an
 * update loop on every load). Instead we attach to the
 * existing registration and listen for updatefound / waiting worker so that the
 * "Có bản mới" update bar still works.
 */
export async function registerPwa(): Promise<void> {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return

  if (getOneSignalAppId()) {
    // OneSignal owns registration. Attach to whatever SW is active/waiting.
    try {
      const reg = await navigator.serviceWorker.ready
      const wireUpdate = (newWorker: ServiceWorker) => {
        if (newWorker.state === 'installed') {
          usePwaStore.setState({ needRefresh: true })
        } else {
          newWorker.addEventListener('statechange', () => {
            if (newWorker.state === 'installed') {
              usePwaStore.setState({ needRefresh: true })
            }
          })
        }
      }
      if (reg.waiting) {
        usePwaStore.setState({ needRefresh: true })
      }
      reg.addEventListener('updatefound', () => {
        if (reg.installing) wireUpdate(reg.installing)
      })
      watchForUpdates(reg)
      usePwaStore.setState({
        applyUpdate: () => {
          const waiting = reg.waiting
          if (!waiting) {
            window.location.reload()
            return
          }
          // Reload only once the new worker controls the page: until then every navigation is
          // answered with the old worker's app shell, so an early reload lands on the old version.
          navigator.serviceWorker.addEventListener('controllerchange', () => window.location.reload(), { once: true })
          waiting.postMessage({ type: 'SKIP_WAITING' })
        },
      })
    } catch (err) {
      console.warn('[pwa] could not attach to OneSignal service worker', err)
    }
    return
  }

  // No OneSignal — own the registration via workbox-window.
  try {
    const { registerSW } = await import('virtual:pwa-register')
    const update = registerSW({
      onNeedRefresh: () => usePwaStore.setState({ needRefresh: true }),
      onRegisteredSW: (_url, reg) => { if (reg) watchForUpdates(reg) },
      onRegisterError: (err) => console.warn('[pwa] service worker registration failed', err),
    })
    // Another tab may already have activated the new worker: nothing is waiting any more, and
    // asking again would do nothing, so just load the page that worker now serves.
    usePwaStore.setState({
      applyUpdate: () => {
        void navigator.serviceWorker.getRegistration().then((reg) => {
          if (reg?.waiting) void update(true)
          else window.location.reload()
        })
      },
    })
  } catch (err) {
    console.warn('[pwa] service worker unavailable', err)
  }
}
