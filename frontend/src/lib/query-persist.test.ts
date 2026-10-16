import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { QueryClient } from '@tanstack/react-query'
import { get, keys as idbKeys, set } from 'idb-keyval'
import { keys } from '../hooks/keys'
import {
  PERSIST_MAX_AGE, RECENT_CONVERSATIONS, MESSAGE_PAGE_KEEP, createDeviceCache, wipeDeviceCache, deviceCacheStore,
  type PersistScope,
} from './query-persist'

// A store whose reads can be held back, to stand in for a slow IndexedDB.
const hold = vi.hoisted(() => ({ gate: null as Promise<void> | null }))
vi.mock('idb-keyval', async (importOriginal) => {
  const real = await importOriginal<typeof import('idb-keyval')>()
  return {
    ...real,
    get: async (...args: Parameters<typeof real.get>) => {
      if (hold.gate) await hold.gate
      return real.get(...args)
    },
  }
})

const A: PersistScope = { userId: 'user-a', workspaceId: 'ws-1' }
const B: PersistScope = { userId: 'user-b', workspaceId: 'ws-1' }

let scope: PersistScope | null
let cursors: unknown
let now: number

function newClient() {
  return new QueryClient({ defaultOptions: { queries: { gcTime: Infinity, retry: false } } })
}

function setup(over: { buster?: string; client?: QueryClient; throttleMs?: number } = {}) {
  const queryClient = over.client ?? newClient()
  const restoreSync = vi.fn((raw: unknown) => { cursors = raw })
  const cache = createDeviceCache({
    queryClient,
    getScope: () => scope,
    getUserId: () => scope?.userId ?? null,
    sync: { get: () => cursors, restore: restoreSync },
    buster: over.buster ?? 'build-1',
    throttleMs: over.throttleMs ?? 0,
  })
  return { queryClient, cache, restoreSync }
}

const messagePage = (channelId: string, newest: number, count = 3) => ({
  messages: Array.from({ length: count }, (_, i) => ({
    id: `${channelId}-m${i}`, channel_id: channelId, sender_id: 'u', content: `hi ${i}`,
    created_at: new Date(newest - i * 1000).toISOString(),
  })),
  has_more: false,
})

const stored = async () => (await idbKeys(deviceCacheStore())).length

beforeEach(async () => {
  scope = A
  cursors = { owner: 'user-a|ws-1', user: 4, workspaces: { 'ws-1': 9 } }
  now = Date.UTC(2026, 9, 16)
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(now)
  await wipeDeviceCache()
})
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

describe('persist and restore', () => {
  it('ignores a slow read that finishes after the restore gave up', async () => {
    const first = setup()
    first.queryClient.setQueryData(keys.messaging.chatState('ws-1'), { channels: ['general'] })
    await first.cache.flush()

    let release!: () => void
    hold.gate = new Promise<void>((resolve) => { release = resolve })
    const second = setup()
    vi.useRealTimers()
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    const restoring = second.cache.restore()
    await vi.advanceTimersByTimeAsync(2000)
    expect(await restoring).toBe(false)

    vi.useRealTimers()
    release()
    hold.gate = null
    await new Promise((resolve) => setTimeout(resolve, 100))
    expect(second.queryClient.getQueryData(keys.messaging.chatState('ws-1'))).toBeUndefined()
    expect(second.restoreSync).not.toHaveBeenCalled()
    expect(second.cache.restoredWorkspaceId()).toBeNull()
  })

  it('shows saved data on the next start before any fetch resolves', async () => {
    const first = setup()
    first.queryClient.setQueryData(keys.messaging.chatState('ws-1'), { channels: ['general'] })
    first.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    first.queryClient.setQueryData(keys.contacts.list('ws-1'), { contacts: [{ id: 'c1' }] })
    await first.cache.flush()

    const second = setup()
    expect(await second.cache.restore()).toBe(true)
    // restore() resolved with no request made: this is what the first paint reads.
    expect(second.queryClient.getQueryData(keys.messaging.chatState('ws-1'))).toEqual({ channels: ['general'] })
    expect(second.queryClient.getQueryData(keys.auth.me())).toEqual({ name: 'An' })
    expect(second.queryClient.getQueryData(keys.contacts.list('ws-1'))).toEqual({ contacts: [{ id: 'c1' }] })
    // Restored data is the old copy: its age is kept, so it counts as stale and a refetch follows.
    expect(second.queryClient.getQueryState(keys.auth.me())?.dataUpdatedAt).toBe(now)
    expect(second.cache.restoredWorkspaceId()).toBe('ws-1')
  })

  it('stores the sync cursors with the cache and restores them together', async () => {
    const first = setup()
    first.queryClient.setQueryData(keys.messaging.chatState('ws-1'), { channels: [] })
    await first.cache.flush()

    cursors = undefined
    const second = setup()
    await second.cache.restore()
    expect(second.restoreSync).toHaveBeenCalledWith({ owner: 'user-a|ws-1', user: 4, workspaces: { 'ws-1': 9 } })
  })

  it('does not hand cursors back when nothing is restored', async () => {
    const { cache, restoreSync } = setup()
    expect(await cache.restore()).toBe(false)
    expect(restoreSync).not.toHaveBeenCalled()
  })

  it('only dehydrates the allowed queries', async () => {
    const { queryClient, cache } = setup()
    queryClient.setQueryData(keys.messaging.chatState('ws-1'), { ok: 1 })
    queryClient.setQueryData(keys.messaging.channels('ws-1'), { ok: 1 })
    queryClient.setQueryData(['drive', 'ws-1'], { files: [{ url: 'https://minio/presigned?sig=1' }] })
    queryClient.setQueryData(['download-url', 'f1'], { url: 'https://minio/presigned?sig=2' })
    queryClient.setQueryData(['thumbnail', 'f1'], 'blob:abc')
    queryClient.setQueryData(['auth', 'otp'], { code: '123456' })
    queryClient.setQueryData(['admin', 'users'], { users: [] })
    await cache.flush()

    const second = setup()
    await second.cache.restore()
    const restored = second.queryClient.getQueryCache().getAll().map((q) => JSON.stringify(q.queryKey))
    expect(restored.sort()).toEqual([
      JSON.stringify(keys.messaging.channels('ws-1')),
      JSON.stringify(keys.messaging.chatState('ws-1')),
    ].sort())
  })

  it('never writes a token, an OTP or a presigned URL to the device', async () => {
    const { queryClient, cache } = setup()
    queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    queryClient.setQueryData(['download-url', 'f1'], { url: 'https://minio/presigned?X-Amz-Signature=zz' })
    queryClient.setQueryData(['auth', 'otp'], { code: '123456' })
    await cache.flush()
    const raw = JSON.stringify(await get('user-a:ws-1', deviceCacheStore()))
    expect(raw).toContain('An')
    expect(raw).not.toMatch(/Signature|123456|presigned/)
  })

  it('keeps messages for only the 20 most recent conversations, newest page only', async () => {
    const { queryClient, cache } = setup()
    const base = Date.UTC(2026, 9, 15)
    for (let i = 0; i < RECENT_CONVERSATIONS + 5; i++) {
      queryClient.setQueryData(keys.messaging.messages(`c${i}`), messagePage(`c${i}`, base + i * 60_000))
    }
    const long = messagePage('c24', base + 24 * 60_000, MESSAGE_PAGE_KEEP + 30)
    queryClient.setQueryData(keys.messaging.messages('c24'), long)
    await cache.flush()

    const second = setup()
    await second.cache.restore()
    const restored = second.queryClient.getQueryCache().findAll({ queryKey: keys.messaging.messagesAll() })
    expect(restored).toHaveLength(RECENT_CONVERSATIONS)
    const ids = restored.map((q) => q.queryKey[1])
    expect(ids).toContain('c24')
    expect(ids).not.toContain('c0')
    const trimmed = second.queryClient.getQueryData<{ messages: { id: string }[]; has_more: boolean }>(keys.messaging.messages('c24'))!
    expect(trimmed.messages).toHaveLength(MESSAGE_PAGE_KEEP)
    expect(trimmed.messages[0]!.id).toBe('c24-m0') // newest first
    expect(trimmed.has_more).toBe(true)
  })

  it('leaves out messages not yet sent and the local preview of an attachment', async () => {
    const { queryClient, cache } = setup()
    const page = messagePage('c1', now, 2)
    page.messages.unshift({ id: 'temp-1', channel_id: 'c1', sender_id: 'u', content: 'sending', created_at: new Date(now).toISOString() })
    ;(page.messages[1] as Record<string, unknown>).attachments = [{ drive_file_id: 'f', file_name: 'a.png', localUrl: 'blob:local' }]
    queryClient.setQueryData(keys.messaging.messages('c1'), page)
    await cache.flush()

    const second = setup()
    await second.cache.restore()
    const got = second.queryClient.getQueryData<{ messages: { id: string; attachments?: { localUrl?: string; file_name: string }[] }[] }>(keys.messaging.messages('c1'))!
    expect(got.messages.map((m) => m.id)).toEqual(['c1-m0', 'c1-m1'])
    expect(got.messages[0]!.attachments![0]!.file_name).toBe('a.png')
    expect(got.messages[0]!.attachments![0]!.localUrl).toBeUndefined()
  })

  it('does not save while no session scope is known', async () => {
    const { queryClient, cache } = setup()
    queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    scope = null
    await cache.flush()
    expect(await stored()).toBe(0)
  })
})

describe('who may read the store', () => {
  it('gives user B nothing of user A on the same device', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()

    scope = B
    const b = setup()
    expect(await b.cache.restore()).toBe(false)
    expect(b.queryClient.getQueryData(keys.auth.me())).toBeUndefined()
    expect(b.restoreSync).not.toHaveBeenCalled()
  })

  it('refuses an entry whose recorded owner is not the reader', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    // A's entry ends up under B's key.
    await set('user-b:ws-1', await get('user-a:ws-1', deviceCacheStore()), deviceCacheStore())
    await set('last:user-b', 'ws-1', deviceCacheStore())

    scope = B
    const b = setup()
    expect(await b.cache.restore()).toBe(false)
    expect(b.queryClient.getQueryData(keys.auth.me())).toBeUndefined()
    expect(await stored()).toBe(0)
  })

  it('drops the restored data when the session turns out to speak for another workspace', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()

    const b = setup()
    await b.cache.restore()
    scope = { userId: 'user-a', workspaceId: 'ws-2' }
    await b.cache.checkScope()
    expect(b.queryClient.getQueryData(keys.auth.me())).toBeUndefined()
    expect(b.restoreSync).toHaveBeenLastCalledWith(null)
    expect(await stored()).toBe(0)
  })

  it('keeps the restored data when the session speaks for the same workspace', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    const b = setup()
    await b.cache.restore()
    await b.cache.checkScope()
    expect(b.queryClient.getQueryData(keys.auth.me())).toEqual({ name: 'An' })
  })
})

describe('when everything is wiped', () => {
  it('wipeDeviceCache empties the store (logout, 401, refresh failure, workspace switch)', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    expect(await stored()).toBeGreaterThan(0)
    await wipeDeviceCache()
    expect(await stored()).toBe(0)
    const again = setup()
    expect(await again.cache.restore()).toBe(false)
  })

  it('a save that was waiting when the wipe came does not bring the data back', async () => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout', 'Date'] })
    try {
      const { queryClient, cache } = setup({ throttleMs: 1000 })
      const stop = cache.start()
      queryClient.setQueryData(keys.auth.me(), { name: 'An' })
      await wipeDeviceCache() // logout while the throttled save is still waiting
      await vi.advanceTimersByTimeAsync(2000)
      stop()
      expect(await stored()).toBe(0)
    } finally {
      vi.useRealTimers()
    }
  })

  it('a different build discards the old store', async () => {
    const a = setup({ buster: 'build-1' })
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    const b = setup({ buster: 'build-2' })
    expect(await b.cache.restore()).toBe(false)
    expect(b.queryClient.getQueryData(keys.auth.me())).toBeUndefined()
    expect(await stored()).toBe(0)
  })

  it('data older than 7 days is dropped', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    vi.setSystemTime(now + PERSIST_MAX_AGE + 1000)
    const b = setup()
    expect(await b.cache.restore()).toBe(false)
    expect(b.queryClient.getQueryData(keys.auth.me())).toBeUndefined()
    expect(await stored()).toBe(0)
  })

  it('data just under 7 days old is still shown', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    vi.setSystemTime(now + PERSIST_MAX_AGE - 1000)
    const b = setup()
    expect(await b.cache.restore()).toBe(true)
  })

  it('a signed-out device holds nothing', async () => {
    const a = setup()
    a.queryClient.setQueryData(keys.auth.me(), { name: 'An' })
    await a.cache.flush()
    scope = null
    const b = setup()
    expect(await b.cache.restore()).toBe(false)
    expect(await stored()).toBe(0)
  })

  it('an unreadable store starts the app empty instead of failing it', async () => {
    await set('last:user-a', 'ws-1', deviceCacheStore())
    await set('user-a:ws-1', 'not an object', deviceCacheStore())
    const b = setup()
    expect(await b.cache.restore()).toBe(false)
    expect(await stored()).toBe(0)
  })
})
