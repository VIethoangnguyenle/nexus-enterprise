import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth.store'
import { queryClient } from '../lib/query-client'

describe('auth store logout', () => {
  beforeEach(() => {
    queryClient.clear()
    useAuthStore.setState({ accessToken: 'token', user: { id: 'u1', username: 'alice' } })
  })

  it('drops every cached query so the next user in this tab sees none of it', () => {
    queryClient.setQueryData(['workspaces'], [{ id: 'ws-1', name: 'Khối Vận hành' }])
    queryClient.setQueryData(['drive', 'ws-1', 'folder', 'root'], { items: [] })

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
