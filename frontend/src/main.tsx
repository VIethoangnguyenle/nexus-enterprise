import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtools } from '@tanstack/react-query-devtools'
import { queryClient } from './lib/query-client'
import { appBuster, createDeviceCache } from './lib/query-persist'
import { tenantIdFromToken, useAuthStore } from './stores/auth.store'
import { getSyncSnapshot, restoreSyncSnapshot } from './stores/websocket.store'
import { routeTree } from './routeTree.gen'
// Self-hosted type (DESIGN.md §3): latin + vietnamese subsets only, and only the
// weights the scale uses. Body 400/500/600, display 600/700.
import '@fontsource/be-vietnam-pro/latin-400.css'
import '@fontsource/be-vietnam-pro/vietnamese-400.css'
import '@fontsource/be-vietnam-pro/latin-500.css'
import '@fontsource/be-vietnam-pro/vietnamese-500.css'
import '@fontsource/be-vietnam-pro/latin-600.css'
import '@fontsource/be-vietnam-pro/vietnamese-600.css'
import '@fontsource/bricolage-grotesque/latin-600.css'
import '@fontsource/bricolage-grotesque/vietnamese-600.css'
import '@fontsource/bricolage-grotesque/latin-700.css'
import '@fontsource/bricolage-grotesque/vietnamese-700.css'
import './index.css'
import { trackAppHeight } from './lib/app-height'
import { preventPageZoom } from './lib/no-page-zoom'
import { registerPwa } from './lib/pwa-register'
import { startPushAuthSync } from './lib/push'
import { installTenantMoved } from './lib/tenant-moved'

const router = createRouter({ routeTree, context: { queryClient } })

declare module '@tanstack/react-router' {
  interface Register { router: typeof router }
}

// Size the shell to the visible viewport so the on-screen keyboard never covers the composer.
trackAppHeight(window.visualViewport)
preventPageZoom()

void registerPwa()
startPushAuthSync()

// A refresh can move the session to another workspace (the person was removed from
// the one they had open): the app follows by opening the one they still have.
installTenantMoved((destination) => {
  void router.navigate(destination as never)
})

// The copy of the last session kept on the device: the first screen is built from
// it while the fresh data loads. Cleared at sign-out, 401 and workspace switch
// (wipeDeviceCache), and never read for another person or build.
const deviceCache = createDeviceCache({
  queryClient,
  getScope: () => {
    const { user, accessToken } = useAuthStore.getState()
    const workspaceId = tenantIdFromToken(accessToken)
    return user?.id && workspaceId ? { userId: user.id, workspaceId } : null
  },
  getUserId: () => useAuthStore.getState().user?.id ?? null,
  sync: { get: getSyncSnapshot, restore: restoreSyncSnapshot },
  buster: appBuster(),
})

function mount() {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        <ReactQueryDevtools buttonPosition="bottom-left" />
      </QueryClientProvider>
    </StrictMode>,
  )
}

// Saving starts only after the restore, so an empty cache never overwrites the copy being read.
void deviceCache.restore().then(() => {
  deviceCache.start()
  // The token arrives with the boot refresh; a copy for another workspace than it speaks for is dropped then.
  useAuthStore.subscribe(() => { void deviceCache.checkScope() })
  mount()
})
