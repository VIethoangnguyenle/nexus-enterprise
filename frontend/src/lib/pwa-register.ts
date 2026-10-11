/// <reference types="vite-plugin-pwa/client" />
import { usePwaStore } from './pwa'

/**
 * Register the service worker. Production builds only: in `make dev` Vite serves
 * source modules and the proxy table, and a worker would sit in front of both.
 * A failed registration leaves the app working as a plain web page.
 */
export async function registerPwa(): Promise<void> {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return
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
