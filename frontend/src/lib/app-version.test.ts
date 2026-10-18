import { afterEach, describe, expect, it, vi } from 'vitest'
import { appVersion, appVersionLabel } from './app-version'

afterEach(() => vi.unstubAllEnvs())

describe('appVersion', () => {
  it('is "dev" when the build carries no version', () => {
    vi.stubEnv('VITE_APP_VERSION', '')
    expect(appVersion()).toBe('dev')
    expect(appVersionLabel()).toBe('Phiên bản dev')
  })

  it('shows the stamped build, trimmed', () => {
    vi.stubEnv('VITE_APP_VERSION', ' 2026.10.18 · 6368e8e ')
    expect(appVersion()).toBe('2026.10.18 · 6368e8e')
    expect(appVersionLabel()).toBe('Phiên bản 2026.10.18 · 6368e8e')
  })
})
