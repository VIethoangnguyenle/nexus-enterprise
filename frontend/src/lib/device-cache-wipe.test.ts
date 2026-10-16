import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { keys as idbKeys, set } from 'idb-keyval'
import { apiFetch, refreshAccessToken } from '../api/client'
import { rescopeSession } from '../hooks/useSwitchWorkspace'
import { useAuthStore } from '../stores/auth.store'
import { jwtFor } from '../test/auth-fixtures'
import { deviceCacheStore, wipeDeviceCache } from './query-persist'

const switchTenant = vi.fn()
vi.mock('../api/auth', () => ({ authApi: { switchTenant: (id: string) => switchTenant(id) } }))

const held = async () => (await idbKeys(deviceCacheStore())).length
const fillDevice = () => set('user-a:ws-1', { owner: 'user-a' }, deviceCacheStore())
const reply = (status: number, body: unknown = {}) => new Response(JSON.stringify(body), { status })

beforeEach(async () => {
  await wipeDeviceCache()
  await fillDevice()
  useAuthStore.setState({ accessToken: jwtFor('ws-1'), user: { id: 'user-a', username: 'an' }, bootstrapping: false })
})
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

describe('the copy on the device goes with the session', () => {
  it('signing out empties it', async () => {
    useAuthStore.getState().logout()
    await vi.waitFor(async () => expect(await held()).toBe(0))
  })

  it('a 401 that the refresh cannot repair empties it', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply(401)))
    await expect(apiFetch('/me')).rejects.toMatchObject({ status: 401 })
    await vi.waitFor(async () => expect(await held()).toBe(0))
  })

  it('a refused refresh empties it', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply(403)))
    await refreshAccessToken()
    await vi.waitFor(async () => expect(await held()).toBe(0))
  })

  it('a failing server is not an answer about the session: the copy stays', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => reply(503)))
    await refreshAccessToken()
    await new Promise((r) => setTimeout(r, 30))
    expect(await held()).toBe(1)
  })

  it('switching workspace empties it', async () => {
    switchTenant.mockResolvedValue({ access_token: jwtFor('ws-2') })
    await rescopeSession('ws-2')
    await vi.waitFor(async () => expect(await held()).toBe(0))
  })

  it('asking for the workspace already open keeps it', async () => {
    await rescopeSession('ws-1')
    await new Promise((r) => setTimeout(r, 30))
    expect(await held()).toBe(1)
  })
})
