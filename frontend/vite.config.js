import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { TanStackRouterVite } from '@tanstack/router-plugin/vite'

// Dev mode: proxy directly to native Go services (each on unique port)
// Docker mode: proxy to Traefik on :80 (single entrypoint)
const isDev = process.env.VITE_DEV_MODE === 'true'

const devProxy = {
  // Auth service — :8180
  '/api/auth': { target: 'http://localhost:8180', changeOrigin: true },
  '/api/users': { target: 'http://localhost:8180', changeOrigin: true },

  // Messaging service — :8183 (REST) + :8081 (WebSocket)
  // IMPORTANT: /api/messages MUST come before /api/me (prefix collision)
  '/api/messages': { target: 'http://localhost:8183', changeOrigin: true },
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
        // /api/workspaces/:id/channels/* → messaging service
        if (/^\/api\/workspaces\/[^/]+\/channels/.test(url)) {
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
        return originalWeb(req, res, opts)
      }
    },
  },

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

  // Asset service — :8184
  '/api/assets': { target: 'http://localhost:8184', changeOrigin: true },
  '/api/asset-types': { target: 'http://localhost:8184', changeOrigin: true },
  '/api/asset-requests': { target: 'http://localhost:8184', changeOrigin: true },

  // Drive service — :8185
  '/api/drive': { target: 'http://localhost:8185', changeOrigin: true },

  // Approval service — :8186
  '/api/approval': { target: 'http://localhost:8186', changeOrigin: true },
}

const dockerProxy = {
  '/api': { target: 'http://localhost:80', changeOrigin: true },
  '/ws': { target: 'ws://localhost:8081', ws: true },
}

export default defineConfig({
  plugins: [
    TanStackRouterVite({ routesDirectory: './src/routes', generatedRouteTree: './src/routeTree.gen.ts' }),
    tailwindcss(),
    react(),
  ],
  server: {
    proxy: isDev ? devProxy : dockerProxy,
  },
})
