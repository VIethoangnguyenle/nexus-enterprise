import { beforeEach, describe, expect, it, vi } from 'vitest'
import { moduleRootOf, rescopeSession } from './useSwitchWorkspace'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
import { useDriveStore } from '../stores/drive.store'
import { jwtFor } from '../test/auth-fixtures'
import { resolverFor } from '../lib/people-resolver'
import { keys } from './keys'

const switchTenant = vi.fn()
vi.mock('../api/auth', () => ({ authApi: { switchTenant: (id: string) => switchTenant(id) } }))

describe('moduleRootOf', () => {
  it('keeps the module and drops what belongs to the old workspace', () => {
    expect(moduleRootOf('/drive')).toBe('/drive')
    expect(moduleRootOf('/channels/9f2c')).toBe('/channels')
    expect(moduleRootOf('/documents/abc')).toBe('/documents')
    expect(moduleRootOf('/admin/roles')).toBe('/admin')
    expect(moduleRootOf('/approval')).toBe('/approval')
  })
  it('falls back to Tin nhắn for anything else', () => {
    expect(moduleRootOf('/')).toBe('/channels')
    expect(moduleRootOf('/driveway')).toBe('/channels')
  })
})

describe('rescopeSession', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    queryClient.clear()
    useDriveStore.setState({ selectedItemId: 'doc-1', expandedFolders: new Set(['f1']) })
  })

  it('keeps the cache and the Tài liệu state when the token already speaks for the workspace', async () => {
    useAuthStore.setState({ accessToken: jwtFor('ws-1') })
    queryClient.setQueryData(['workspaces'], { workspaces: [{ id: 'ws-1' }] })

    await rescopeSession('ws-1')

    expect(switchTenant).not.toHaveBeenCalled()
    expect(queryClient.getQueryData(['workspaces'])).toEqual({ workspaces: [{ id: 'ws-1' }] })
    expect(useDriveStore.getState().selectedItemId).toBe('doc-1')
  })

  it('does not refetch the workspace list when the tenant did not change', async () => {
    useAuthStore.setState({ accessToken: jwtFor('ws-1') })
    const list = vi.fn().mockResolvedValue({ workspaces: [{ id: 'ws-1' }] })
    const options = { queryKey: ['workspaces'], queryFn: list }
    await queryClient.fetchQuery(options)
    const observer = queryClient.getQueryCache().find({ queryKey: ['workspaces'] })!

    await rescopeSession('ws-1')
    await queryClient.ensureQueryData(options)

    expect(list).toHaveBeenCalledTimes(1)
    expect(observer.state.data).toEqual({ workspaces: [{ id: 'ws-1' }] })
  })

  it('swaps the token and drops the old workspace\'s data when the tenant changes', async () => {
    useAuthStore.setState({ accessToken: jwtFor('ws-1') })
    queryClient.setQueryData(['workspaces'], { workspaces: [{ id: 'ws-1' }] })
    switchTenant.mockResolvedValue({ access_token: jwtFor('ws-2') })

    await rescopeSession('ws-2')

    expect(switchTenant).toHaveBeenCalledWith('ws-2')
    expect(useAuthStore.getState().accessToken).toBe(jwtFor('ws-2'))
    expect(queryClient.getQueryData(['workspaces'])).toBeUndefined()
    expect(useDriveStore.getState().selectedItemId).toBeNull()
  })

  it('forgets the names the resolver found in the old workspace', async () => {
    useAuthStore.setState({ accessToken: jwtFor('ws-1') })
    switchTenant.mockResolvedValue({ access_token: jwtFor('ws-2') })
    const clear = vi.spyOn(resolverFor('ws-1'), 'clear')

    await rescopeSession('ws-2')

    expect(clear).toHaveBeenCalled()
    clear.mockRestore()
  })

  it('leaves the token and the cache alone when the switch is refused', async () => {
    useAuthStore.setState({ accessToken: jwtFor('ws-1') })
    queryClient.setQueryData(['workspaces'], { workspaces: [{ id: 'ws-1' }] })
    switchTenant.mockRejectedValue(new Error('forbidden'))

    await expect(rescopeSession('ws-2')).rejects.toThrow('forbidden')

    expect(useAuthStore.getState().accessToken).toBe(jwtFor('ws-1'))
    expect(queryClient.getQueryData(['workspaces'])).toBeDefined()
  })

  // A DM belongs to one workspace: after a switch no list built from the old
  // workspace's DMs (the DM list, unread counts, previews, followed threads,
  // mentions, the chat state) may still be served.
  it('drops every cached list that carries the old workspace\'s direct messages', async () => {
    useAuthStore.setState({ accessToken: jwtFor('ws-1') })
    switchTenant.mockResolvedValue({ access_token: jwtFor('ws-2') })
    const carriers = [
      keys.messaging.dms(),
      keys.messaging.chatState('ws-1'),
      keys.messaging.unreadCounts(),
      keys.messaging.conversationActivity(),
      keys.messaging.followedThreads(),
      keys.messaging.searchAll('needle'),
      keys.messaging.messages('dm-of-ws-1'),
      keys.messaging.channel('dm-of-ws-1'),
    ]
    carriers.forEach((k) => queryClient.setQueryData(k, { channels: [{ id: 'dm-of-ws-1', channel_type: 'dm' }] }))
    // control: they are there before the switch
    carriers.forEach((k) => expect(queryClient.getQueryData(k)).toBeDefined())

    await rescopeSession('ws-2')

    carriers.forEach((k) => expect(queryClient.getQueryData(k)).toBeUndefined())
  })
})
