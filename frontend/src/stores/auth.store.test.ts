import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore, tenantIdFromToken } from './auth.store'
import { queryClient } from '../lib/query-client'
import { keys } from '../hooks/keys'

describe('auth store logout', () => {
  beforeEach(() => {
    queryClient.clear()
    useAuthStore.setState({ accessToken: 'token', user: { id: 'u1', username: 'alice' } })
  })

  it('drops every cached query so the next user in this tab sees none of it', () => {
    queryClient.setQueryData(keys.workspaces.all(), [{ id: 'ws-1', name: 'Khối Vận hành' }])
    queryClient.setQueryData(keys.drive.folder('ws-1'), { items: [] })
    queryClient.setQueryData(keys.permissions.object('tenant-1', 'node-1'), { read: true })

    useAuthStore.getState().logout()

    expect(queryClient.getQueryCache().getAll()).toHaveLength(0)
  })

  it('clears the session fields', () => {
    useAuthStore.getState().logout()

    const s = useAuthStore.getState()
    expect(s.accessToken).toBeNull()
    expect(s.user).toBeNull()
  })
})

const jwt = (payload: object) => `h.${btoa(JSON.stringify(payload)).replace(/\+/g, '-').replace(/\//g, '_')}.s`

describe('tenantIdFromToken', () => {
  it('reads the tenant out of the payload, including url-safe base64', () => {
    expect(tenantIdFromToken(jwt({ tenant_id: 'tenant-?>~' }))).toBe('tenant-?>~')
  })

  it('is null for no token, a malformed token, or a token without a tenant', () => {
    expect(tenantIdFromToken(null)).toBeNull()
    expect(tenantIdFromToken('no-dots')).toBeNull()
    expect(tenantIdFromToken('h.!!!.s')).toBeNull()
    expect(tenantIdFromToken(jwt({ sub: 'u1' }))).toBeNull()
  })
})

describe('auth store session', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState({ accessToken: null, tenantId: null, user: null, bootstrapping: true })
  })

  it('login stores the token, derives the tenant and ends bootstrapping', () => {
    useAuthStore.getState().login(jwt({ tenant_id: 't1' }), { id: 'u1', username: 'alice' })
    const s = useAuthStore.getState()
    expect(s.tenantId).toBe('t1')
    expect(s.user?.username).toBe('alice')
    expect(s.bootstrapping).toBe(false)
  })

  it('setAccessToken re-derives the tenant, so a tenant switch cannot leave a stale one', () => {
    useAuthStore.getState().setAccessToken(jwt({ tenant_id: 't1' }))
    useAuthStore.getState().setAccessToken(jwt({ tenant_id: 't2' }))
    expect(useAuthStore.getState().tenantId).toBe('t2')
    useAuthStore.getState().setAccessToken(null)
    expect(useAuthStore.getState().tenantId).toBeNull()
  })

  it('counts as authenticated from the persisted user alone, before the refresh returns a token', () => {
    useAuthStore.setState({ user: { id: 'u1', username: 'alice' }, accessToken: null })
    expect(useAuthStore.getState().isAuthenticated()).toBe(true)
    useAuthStore.setState({ user: null })
    expect(useAuthStore.getState().isAuthenticated()).toBe(false)
  })

  it('never writes the access token to localStorage', () => {
    useAuthStore.getState().login(jwt({ tenant_id: 't1' }), { id: 'u1', username: 'alice' })
    const stored = Object.values(localStorage).join('')
    expect(stored).toContain('alice')
    expect(stored).not.toContain('"accessToken"')
  })

  it('setBootstrapped ends bootstrapping without touching the session', () => {
    useAuthStore.getState().setBootstrapped()
    expect(useAuthStore.getState().bootstrapping).toBe(false)
  })
})
