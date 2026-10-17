import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import { keys as idbKeys, set } from 'idb-keyval'
import { refreshAccessToken } from '../api/client'
import { useAuthStore } from '../stores/auth.store'
import { useToastStore } from '../components/primitives/Toast'
import { jwtFor, UUID_RE } from '../test/auth-fixtures'
import { queryClient } from './query-client'
import { deviceCacheStore, wipeDeviceCache } from './query-persist'
import { clearSelfLeaves, noteSelfLeave } from './self-leave'
import { installTenantMoved, type MovedDestination } from './tenant-moved'

// A person removed from the workspace they have open keeps their session. The
// server moves it on refresh; the app has to follow: drop what it holds of the
// workspace it left, open the one it is in now, and say so in words.

const reply = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status })
const held = async () => (await idbKeys(deviceCacheStore())).length
const toasts = () => useToastStore.getState().toasts.map((t) => t.message)

let navigate: Mock<(to: MovedDestination) => void>
let stop: () => void

beforeEach(async () => {
  clearSelfLeaves()
  await wipeDeviceCache()
  await set('user-a:ws-1', { owner: 'user-a' }, deviceCacheStore())
  queryClient.setQueryData(['channels', 'ws-1'], [{ id: 'c1' }])
  useToastStore.getState().clear()
  useAuthStore.setState({ accessToken: jwtFor('ws-1'), tenantId: 'ws-1', user: { id: 'user-a', username: 'an' }, bootstrapping: false })
  navigate = vi.fn<(to: MovedDestination) => void>()
  stop = installTenantMoved(navigate)
})
afterEach(() => { stop(); vi.unstubAllGlobals(); queryClient.clear() })

describe('a refresh that moves the session', () => {
  it('opens the workspace the person still has, drops the old one, and says so', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply({ access_token: jwtFor('ws-2'), tenant_id: 'ws-2', moved: true })))

    await refreshAccessToken()

    expect(useAuthStore.getState().tenantId).toBe('ws-2')
    expect(navigate).toHaveBeenCalledExactlyOnceWith({ to: '/channels', search: { ws: 'ws-2' } })
    expect(queryClient.getQueryData(['channels', 'ws-1'])).toBeUndefined()
    await vi.waitFor(async () => expect(await held()).toBe(0))
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]).toMatch(/không còn/i)
    expect(toasts()[0]).not.toMatch(UUID_RE)
    expect(toasts()[0]).not.toContain('ws-')
  })

  it('says there is no workspace left, and opens none', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply({ access_token: jwtFor(''), tenant_id: '', moved: true })))

    await refreshAccessToken()

    // Not a spinner with nothing behind it: the picker, with no workspace in the address.
    expect(navigate).toHaveBeenCalledExactlyOnceWith({ to: '/workspace-select', search: {} })
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]).toMatch(/không còn thuộc workspace nào/i)
  })

  // The paired allow: an ordinary refresh changes nothing the person can see.
  it('does nothing when the session stays where it was', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply({ access_token: jwtFor('ws-1'), tenant_id: 'ws-1' })))

    await refreshAccessToken()

    expect(navigate).not.toHaveBeenCalled()
    expect(toasts()).toEqual([])
    expect(queryClient.getQueryData(['channels', 'ws-1'])).toEqual([{ id: 'c1' }])
    expect(await held()).toBe(1)
  })

  // A person who left on purpose is told by the leave flow itself ("Bạn đã rời ..."); the
  // server then moves their session and the socket closes, which must not read as a removal.
  it('says nothing about a workspace the person has just left themselves', async () => {
    noteSelfLeave('ws-1')
    vi.stubGlobal('fetch', vi.fn(async () => reply({ access_token: jwtFor('ws-2'), tenant_id: 'ws-2', moved: true })))

    await refreshAccessToken()

    expect(toasts()).toEqual([])
    expect(navigate).not.toHaveBeenCalled()
  })

  it('but a later removal from another workspace is announced', async () => {
    noteSelfLeave('ws-9')
    vi.stubGlobal('fetch', vi.fn(async () => reply({ access_token: jwtFor('ws-2'), tenant_id: 'ws-2', moved: true })))

    await refreshAccessToken()

    expect(toasts()).toHaveLength(1)
  })

  it('does nothing once uninstalled', async () => {
    stop()
    vi.stubGlobal('fetch', vi.fn(async () => reply({ access_token: jwtFor('ws-2'), tenant_id: 'ws-2', moved: true })))

    await refreshAccessToken()

    expect(navigate).not.toHaveBeenCalled()
  })
})
