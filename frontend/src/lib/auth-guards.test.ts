import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from '../stores/auth.store'
import { redirectIfSignedIn, requireSignedIn } from './auth-guards'

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
})
