import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import viteConfig from '../../vite.config.js?raw'

// Outside the Vite root, so read from disk rather than imported.
const nginx = readFileSync(resolve(__dirname, '../../../deploy/nginx/default.conf'), 'utf8')

// The worker URL OneSignal registers is set on its dashboard ("Typical site"), which wins over
// `serviceWorkerPath` in code, so the file must exist at the SDK's default name.
const workers = import.meta.glob('../../public/OneSignalSDKWorker.js', {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

describe('OneSignal worker', () => {
  it('exists at the default name and runs the app worker', () => {
    const src = Object.values(workers)[0]
    expect(src, 'public/OneSignalSDKWorker.js is missing').toBeDefined()
    expect(src).toMatch(/importScripts\(\s*['"]\/sw\.js['"]\s*\)/)
  })

  it('is always revalidated, like /sw.js', () => {
    expect(nginx).toMatch(/location = \/OneSignalSDKWorker\.js \{[^}]*Cache-Control "no-cache"/)
  })

  it('is not precached by the app worker', () => {
    expect(viteConfig).toMatch(/globIgnores:[^\]]*OneSignalSDKWorker\.js/)
  })
})
