import { describe, it, expect, beforeEach } from 'vitest'
import { useAuthStore } from './auth.store'
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
