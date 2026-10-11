/// Nexus service worker (injectManifest). Hand-written so the push worker can be
/// merged in later. It caches the app shell and nothing else: no /api response and
/// no WebSocket ever passes through a cache here.
import { cleanupOutdatedCaches, createHandlerBoundToURL, precacheAndRoute } from 'workbox-precaching'
import { NavigationRoute, registerRoute } from 'workbox-routing'

// Only the members used here; the full worker lib clashes with the DOM lib the app is typed with.
interface WorkerScope {
  __WB_MANIFEST: Array<string | { url: string; revision: string | null }>
  addEventListener(type: 'message', listener: (event: { data?: { type?: string } }) => void): void
  skipWaiting(): Promise<void>
}
declare const self: WorkerScope

// Precache the build's HTML/JS/CSS/fonts/icons (the list is injected at build time).
precacheAndRoute(self.__WB_MANIFEST)
cleanupOutdatedCaches()

// Any page navigation, online or not, is answered with the app shell — except the
// API, which must always reach the network (and fail loudly when it cannot).
registerRoute(new NavigationRoute(createHandlerBoundToURL('/index.html'), { denylist: [/^\/api\//] }))

// A new worker waits until the page asks for it (UpdateBar), so a reload never lands under a draft.
self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') void self.skipWaiting()
})
