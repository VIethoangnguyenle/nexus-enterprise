import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from '../stores/auth.store'
import { redirectIfSignedIn, requireSignedIn, requireSignedInToWorkspace } from './auth-guards'

const signedIn = () => useAuthStore.setState({ user: { id: 'u1', username: 'hoa.le' }, accessToken: 't', bootstrapping: false })
const signedOut = () => useAuthStore.setState({ user: null, accessToken: null, bootstrapping: false })

function destination(fn: () => void): string | null {
  try {
    fn()
    return null
  } catch (e) {
    return (e as { options?: { to?: string } }).options?.to ?? 'threw something else'
  }
}

describe('sign-in route guards', () => {
  beforeEach(signedOut)

  it('the login page sends a signed-in person on to workspace selection', () => {
    signedIn()
    expect(destination(redirectIfSignedIn)).toBe('/workspace-select')
  })

  it('the login page lets a signed-out person in', () => {
    expect(destination(redirectIfSignedIn)).toBeNull()
  })

  it('the pages after sign-in send a signed-out person to the login page', () => {
    expect(destination(requireSignedIn)).toBe('/login')
  })

  it('the pages after sign-in let a signed-in person through', () => {
    signedIn()
    expect(destination(requireSignedIn)).toBeNull()
  })

  it('the workspace sends a signed-out person to the login page before anything of it loads, keeping a workspace link', () => {
    const redirectOf = (search: Record<string, unknown>) => {
      try {
        requireSignedInToWorkspace({ search })
        return null
      } catch (e) {
        return (e as { options?: { to?: string; search?: unknown } }).options ?? null
      }
    }
    expect(redirectOf({ ws: 'w1' })).toMatchObject({ to: '/login', search: { ws: 'w1' } })
    expect(redirectOf({})).toMatchObject({ to: '/login', search: {} })
    signedIn()
    expect(redirectOf({ ws: 'w1' })).toBeNull()
  })
})
