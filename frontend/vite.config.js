import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'
import { VitePWA } from 'vite-plugin-pwa'
import { readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { versionWorkerImport } from './src/lib/worker-version'

// This build's id: names the worker import below and busts the device cache (src/lib/query-persist.ts).
const version = process.env.GITHUB_SHA || Date.now().toString(36)

// Writes this build's id into the copied OneSignalSDKWorker.js (see src/lib/worker-version.ts):
// without it an installed iPhone app keeps the first version it ever loaded.
function versionOneSignalWorker() {
  let outDir = 'dist'
  return {
    name: 'version-onesignal-worker',
    apply: 'build',
    configResolved(config) {
      outDir = config.build.outDir
    },
    writeBundle() {
      const file = join(outDir, 'OneSignalSDKWorker.js')
      writeFileSync(file, versionWorkerImport(readFileSync(file, 'utf8'), version))
    },
  }
}

// Dev mode: proxy directly to native Go services (each on unique port)
// Docker mode: proxy to Traefik on :80 (single entrypoint)
const isDev = process.env.VITE_DEV_MODE === 'true'

const devProxy = {
  // LiveKit signalling (WebSocket) — :7880, prefix stripped. Matches nginx /livekit/ in production.
  '/livekit': {
    target: 'http://localhost:7880',
    ws: true,
    changeOrigin: true,
    rewrite: (path) => path.replace(/^\/livekit/, ''),
  },

  // Auth service — :8180
  '/api/auth': { target: 'http://localhost:8180', changeOrigin: true },

  // Messaging service — :8183 (REST) + :8081 (WebSocket)
  // IMPORTANT: /api/messages MUST come before /api/me (prefix collision)
  '/api/messages': { target: 'http://localhost:8183', changeOrigin: true },
  // Push preferences are messaging's; declared before /api/me (auth) for the same reason.
  '/api/me/push-preferences': { target: 'http://localhost:8183', changeOrigin: true },
  // /api/me, /api/me/profile and /api/me/workspaces (list, create) are all the auth service.
  '/api/me': { target: 'http://localhost:8180', changeOrigin: true },

  // Workspace service — :8181 (with sub-routing for nested service paths)
  '/api/workspaces': {
    target: 'http://localhost:8181',
    changeOrigin: true,
    configure: (proxy) => {
      const originalWeb = proxy.web.bind(proxy)
      proxy.web = (req, res, opts = {}) => {
        const url = req.url || ''
        // /api/workspaces/:id/drive/* → drive service
        if (/^\/api\/workspaces\/[^/]+\/drive/.test(url)) {
          return originalWeb(req, res, { ...opts, target: 'http://localhost:8185' })
        }
        // /api/workspaces/:id/documents/* → document service
        if (/^\/api\/workspaces\/[^/]+\/documents/.test(url)) {
          return originalWeb(req, res, { ...opts, target: 'http://localhost:8182' })
        }
        // /api/workspaces/:id/channels/*, /chat-sections/* and /chat-state → messaging service
        if (/^\/api\/workspaces\/[^/]+\/(channels|chat-sections|chat-state)/.test(url)) {
          return originalWeb(req, res, { ...opts, target: 'http://localhost:8183' })
        }
        // /api/workspaces/:id/contacts → auth service
        if (/^\/api\/workspaces\/[^/]+\/contacts/.test(url)) {
          return originalWeb(req, res, { ...opts, target: 'http://localhost:8180' })
        }
        // /api/workspaces/:id/asset* → asset service (assets, asset-types, asset-requests)
        if (/^\/api\/workspaces\/[^/]+\/asset/.test(url)) {
          return originalWeb(req, res, { ...opts, target: 'http://localhost:8184' })
        }
        // /api/workspaces/:id/calendar* → calendar service
        if (/^\/api\/workspaces\/[^/]+\/calendar/.test(url)) {
          return originalWeb(req, res, { ...opts, target: 'http://localhost:8187' })
        }
        return originalWeb(req, res, opts)
      }
    },
  },

  // Invitations, answered by the person invited (not under a workspace) — workspace service :8181
  '/api/invitations': { target: 'http://localhost:8181', changeOrigin: true },

  // Document service — :8182
  '/api/documents': { target: 'http://localhost:8182', changeOrigin: true },

  // Messaging service — :8183 (WebSocket)
  '/api/ws': {
    target: 'ws://localhost:8081',
    ws: true,
    configure: (proxy) => {
      proxy.on('error', (err) => {
        if (err.code !== 'EPIPE' && err.code !== 'ECONNRESET') {
          console.error('[ws proxy error]', err.message)
        }
      })
    },
  },
  '/api/channels': { target: 'http://localhost:8183', changeOrigin: true },
  '/api/dms': { target: 'http://localhost:8183', changeOrigin: true },
  '/api/threads': { target: 'http://localhost:8183', changeOrigin: true },
  '/api/notifications': { target: 'http://localhost:8183', changeOrigin: true },
  '/api/polls': { target: 'http://localhost:8183', changeOrigin: true },
  '/api/tasks': { target: 'http://localhost:8183', changeOrigin: true },
  '/api/calls': { target: 'http://localhost:8183', changeOrigin: true },

  // Asset service — :8184
  '/api/assets': { target: 'http://localhost:8184', changeOrigin: true },
  '/api/asset-types': { target: 'http://localhost:8184', changeOrigin: true },
  '/api/asset-requests': { target: 'http://localhost:8184', changeOrigin: true },

  // Drive service — :8185
  '/api/drive': { target: 'http://localhost:8185', changeOrigin: true },

  // Calendar service — :8187 (the per-workspace routes are re-dispatched under /api/workspaces above)
  '/api/calendar': { target: 'http://localhost:8187', changeOrigin: true },

  // Approval service — :8186
  '/api/approval': { target: 'http://localhost:8186', changeOrigin: true },
  // Tenant schema provisioning also lives in the approval service, because the
  // approval tables are the ones held per-tenant. Without this entry the path
  // falls through to the Docker catch-all on :80 and 404s in native dev, so
  // approvals stay unusable for every newly created tenant.
  '/api/admin': { target: 'http://localhost:8186', changeOrigin: true },
}

const dockerProxy = {
  // ws: true so /api/ws upgrades through Traefik, which forwards it to the hub unchanged.
  '/api': { target: 'http://localhost:80', changeOrigin: true, ws: true },
}

export default defineConfig({
  // Build-time constant for the service worker: import.meta.env is not forwarded
  // to the injectManifest SW build, but `define` constants are.
  define: {
    __ONESIGNAL__: JSON.stringify(!!process.env.VITE_ONESIGNAL_APP_ID),
    __APP_VERSION__: JSON.stringify(version),
  },
  plugins: [
    TanStackRouterVite({ routesDirectory: './src/routes', generatedRouteTree: './src/routeTree.gen.ts', autoCodeSplitting: true }),
    tailwindcss(),
    react(),
    versionOneSignalWorker(),
    // App shell only: the worker is src/sw.ts (injectManifest) and registers in production builds only.
    VitePWA({
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'sw.ts',
      registerType: 'prompt',
      injectRegister: false,
      manifestFilename: 'manifest.webmanifest',
      injectManifest: {
        rollupFormat: 'iife',
        // The largest chunk (the conversation stream with its editor) stays well under this.
        maximumFileSizeToCacheInBytes: 4 * 1024 * 1024,
        globPatterns: ['**/*.{html,js,css,woff2,svg,png,webmanifest}'],
        globIgnores: ['**/favicon-*.png', 'OneSignalSDKWorker.js'],
      },
      manifest: {
        name: 'Nexus',
        short_name: 'Nexus',
        description: 'Không gian làm việc: tin nhắn, tài liệu, phê duyệt và tài sản.',
        lang: 'vi',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        // DESIGN.md --color-base (light) and the --color-accent icon ground, as hex for the manifest.
        theme_color: '#f1f8f7',
        background_color: '#f1f8f7',
        icons: [
          { src: '/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: '/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: '/icon-maskable-192.png', sizes: '192x192', type: 'image/png', purpose: 'maskable' },
          { src: '/icon-maskable-512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
    }),
  ],
  build: {
    rolldownOptions: {
      output: {
        // Stable vendors get their own files, so a deploy that only touches app code
        // leaves them in the browser cache (their hashes do not change).
        codeSplitting: {
          groups: [
            { name: 'vendor-react', test: /node_modules[\\/](react|react-dom|scheduler)[\\/]/, priority: 30 },
            { name: 'vendor-tanstack', test: /node_modules[\\/]@tanstack[\\/]/, priority: 20 },
            { name: 'vendor-motion', test: /node_modules[\\/](motion|motion-dom|motion-utils|framer-motion)[\\/]/, priority: 10 },
          ],
        },
      },
    },
  },
  server: {
    proxy: isDev ? devProxy : dockerProxy,
  },
})
