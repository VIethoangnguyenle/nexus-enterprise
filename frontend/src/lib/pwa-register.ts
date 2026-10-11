/// <reference types="vite-plugin-pwa/client" />
import { usePwaStore } from './pwa'
import { getOneSignalAppId } from './push'

/**
 * Register the service worker. Production builds only: in `make dev` Vite serves
 * source modules and the proxy table, and a worker would sit in front of both.
 * A failed registration leaves the app working as a plain web page.
 *
 * When OneSignal is configured it registers `/sw.js?appId=…&sdkVersion=…` itself,
 * so we must NOT call registerSW (which registers `/sw.js` — a different URL for the
 * same scope, causing an update loop on every load). Instead we attach to the
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
      usePwaStore.setState({
        applyUpdate: () => {
          if (reg.waiting) {
            reg.waiting.postMessage({ type: 'SKIP_WAITING' })
          }
          window.location.reload()
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
