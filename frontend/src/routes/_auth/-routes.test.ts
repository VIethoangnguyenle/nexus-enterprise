import { describe, expect, it, vi } from 'vitest'

vi.mock('@tanstack/react-router', async (orig) => ({
  ...(await orig<typeof import('@tanstack/react-router')>()),
  createFileRoute: () => (options: Record<string, unknown>) => ({ options }),
}))

type Options = {
  beforeLoad?: () => void
  component?: unknown
  validateSearch?: (s: Record<string, unknown>) => Record<string, unknown>
}
const optionsOf = (route: unknown) => (route as { options: Options }).options

const login = optionsOf((await import('./login')).Route)
const register = optionsOf((await import('./register')).Route)
const select = optionsOf((await import('./workspace-select')).Route)
const onboarding = optionsOf((await import('./onboarding')).Route)
const { validateSelectSearch } = await import('./workspace-select')
const { redirectIfSignedIn, requireSignedIn } = await import('../../lib/auth-guards')

describe('the sign-in routes', () => {
  it('the login page turns away a signed-in person; the pages after it turn away a signed-out one', () => {
    expect(login.beforeLoad).toBe(redirectIfSignedIn)
    for (const o of [register, select, onboarding]) expect(o.beforeLoad).toBe(requireSignedIn)
  })

  it('each route renders its own screen', () => {
    for (const o of [login, register, select, onboarding]) expect(typeof o.component).toBe('function')
  })

  it('the login page keeps a Google error code and a workspace link, and nothing else', () => {
    const v = login.validateSearch!
    expect(v({ error: 'google_state', ws: 'w1', junk: 'x' })).toEqual({ error: 'google_state', ws: 'w1' })
    expect(v({ error: '' })).toEqual({})
    expect(v({ error: 5 })).toEqual({})
    expect(v({})).toEqual({})
  })

  it('the workspace list keeps a workspace link and the outcome of proving an address, and nothing else', () => {
    expect(validateSelectSearch({ ws: 'w1', verified: '1', verify_error: 'google_mismatch', junk: 'x' })).toEqual({
      ws: 'w1', verified: '1', verify_error: 'google_mismatch',
    })
    expect(validateSelectSearch({ verified: '0' })).toEqual({})
    expect(validateSelectSearch({ verify_error: '' })).toEqual({})
    expect(validateSelectSearch({})).toEqual({})
  })

  it('there is no welcome page with a timed redirect', () => {
    const routeFiles = Object.keys(import.meta.glob('./*.tsx'))
    expect(routeFiles.length).toBeGreaterThanOrEqual(4)
    expect(routeFiles.some((f) => /welcome/.test(f))).toBe(false)
  })
})
