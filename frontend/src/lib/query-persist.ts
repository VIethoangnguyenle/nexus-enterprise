import { dehydrate, type DehydratedState, type Query, type QueryClient, type QueryKey } from '@tanstack/react-query'
import {
  persistQueryClientRestore, type PersistedClient, type Persister,
} from '@tanstack/react-query-persist-client'
import { clear, createStore, del, get, set, type UseStore } from 'idb-keyval'
import { keys } from '../hooks/keys'
import type { Message } from '../api/messaging'
import { toMillis } from './format'

/** How long a copy on the device may be shown. Older is thrown away, not shown. */
export const PERSIST_MAX_AGE = 7 * 24 * 60 * 60 * 1000
/** Conversations whose newest page of messages is kept. */
export const RECENT_CONVERSATIONS = 20
/** Messages kept per conversation: the newest page. */
export const MESSAGE_PAGE_KEEP = 50
/** At most one write per this long while the cache keeps changing. */
const SAVE_THROTTLE_MS = 1500
/** Starting the app never waits longer than this for the device. */
const RESTORE_BUDGET_MS = 1500

declare const __APP_VERSION__: string | undefined

/**
 * The build's id: a copy written by another build is not trusted, since the
 * shape of cached data can change between versions.
 */
export const appBuster = (): string => (typeof __APP_VERSION__ === 'string' ? __APP_VERSION__ : 'dev')

/** Whose data a session speaks for: the person and the workspace its token is scoped to. */
export interface PersistScope {
  userId: string
  workspaceId: string
}

/** What is written for one person in one workspace. */
interface Entry {
  owner: string
  workspaceId: string
  client: PersistedClient
  /** The delta-sync cursors, saved with the cache they describe so a replay starts from what the cache shows. */
  sync: unknown
}

// --- the store ---------------------------------------------------------------

let store: UseStore | null = null

/** One IndexedDB store holds every entry; wiping clears all of it. */
export function deviceCacheStore(): UseStore {
  store ??= createStore('nexus-device-cache', 'entries')
  return store
}

const hasIndexedDb = () => typeof indexedDB !== 'undefined'
const entryKey = (userId: string, workspaceId: string) => `${userId}:${workspaceId}`
const pointerKey = (userId: string) => `last:${userId}`

/**
 * Bumped by every wipe. A save that was waiting when the wipe came compares
 * against it and gives up, so signing out cannot be followed by a write of the
 * data that was just removed.
 */
let generation = 0

/**
 * Removes everything the device holds: every person's, every workspace's. Called
 * when the session ends (sign-out, 401, refresh refused), when the workspace
 * changes, and when a copy is too old or from another build. Never throws: an
 * unavailable store is already empty for our purposes.
 */
export async function wipeDeviceCache(): Promise<void> {
  generation++
  if (!hasIndexedDb()) return
  try {
    await clear(deviceCacheStore())
  } catch (err) {
    console.warn('[device-cache] wipe failed', err)
  }
}

// --- what may be written -------------------------------------------------------

/** Heads of the keys the first screen is built from. Taken from the key factories, so a rename cannot orphan them. */
const PERSISTED_HEADS: ReadonlySet<unknown> = new Set([
  keys.messaging.chatState('')[0],
  keys.messaging.channelsAll()[0],
  keys.messaging.dms()[0],
  keys.messaging.unreadCounts()[0],
  keys.messaging.conversationActivity()[0],
  keys.messaging.sections('')[0],
  keys.shortcuts.mentionsUnreadAll()[0],
  keys.notificationLevels.unreadThreads()[0],
  keys.contacts.every()[0],
])
const MESSAGES_HEAD = keys.messaging.messagesAll()[0]
const sameKey = (a: QueryKey, b: QueryKey) => a.length === b.length && a.every((part, i) => part === b[i])

function isPersistedKey(key: QueryKey): boolean {
  return PERSISTED_HEADS.has(key[0]) || sameKey(key, keys.auth.me()) || sameKey(key, keys.workspaces.all())
}

interface MessagePage {
  messages?: Message[]
  has_more?: boolean
}

const newestOf = (q: Query): number => {
  const first = (q.state.data as MessagePage | undefined)?.messages?.find((m) => !m.id.startsWith('temp-'))
  return first ? toMillis(first.created_at) : q.state.dataUpdatedAt
}

/** The conversations whose newest message is the most recent, as channel ids. */
function recentConversationIds(queryClient: QueryClient): Set<string> {
  return new Set(
    queryClient
      .getQueryCache()
      .findAll({ queryKey: keys.messaging.messagesAll() })
      .filter((q) => q.state.status === 'success')
      .sort((a, b) => newestOf(b) - newestOf(a))
      .slice(0, RECENT_CONVERSATIONS)
      .map((q) => String(q.queryKey[1])),
  )
}

/**
 * The newest page only, as the server sent it: what is not sent yet (`temp-`)
 * is left out, and so is the local preview of an attachment (an object URL that
 * means nothing after a reload). Attachments are metadata; file contents and
 * download links are never in this cache.
 */
function messagePageForDevice(data: unknown): MessagePage {
  const page = (data ?? {}) as MessagePage
  const sent = (page.messages ?? []).filter((m) => !m.id.startsWith('temp-'))
  return {
    messages: sent.slice(0, MESSAGE_PAGE_KEEP).map((m) => {
      if (!m.attachments) return m
      return { ...m, attachments: m.attachments.map(({ localUrl: _local, ...rest }) => rest) }
    }),
    has_more: !!page.has_more || sent.length > MESSAGE_PAGE_KEEP,
  }
}

/** The cache reduced to what the device may keep. */
function dehydrateForDevice(queryClient: QueryClient): DehydratedState {
  const recent = recentConversationIds(queryClient)
  const state = dehydrate(queryClient, {
    shouldDehydrateQuery: (q) => {
      if (q.state.status !== 'success') return false
      if (q.queryKey[0] === MESSAGES_HEAD) return recent.has(String(q.queryKey[1]))
      return isPersistedKey(q.queryKey)
    },
    shouldDehydrateMutation: () => false,
  })
  return {
    ...state,
    queries: state.queries.map((q) =>
      q.queryKey[0] === MESSAGES_HEAD ? { ...q, state: { ...q.state, data: messagePageForDevice(q.state.data) } } : q,
    ),
  }
}

// --- what the screen was started from ------------------------------------------

let restoredCopy: { at: number; workspaceId: string } | null = null

/** When the app started from a copy on the device: when it was loaded, and for which workspace. Null otherwise. */
export const getRestoredCopy = () => restoredCopy

// --- the cache ---------------------------------------------------------------

export interface DeviceCacheDeps {
  queryClient: QueryClient
  /** The person and workspace the live session speaks for; null until the token is known. */
  getScope: () => PersistScope | null
  /** The person the app was left open for. Known before the token: it is persisted with the auth store. */
  getUserId: () => string | null
  /** The delta-sync cursors, saved and restored with the cache. */
  sync: { get: () => unknown; restore: (raw: unknown) => void }
  buster: string
  throttleMs?: number
}

export interface DeviceCache {
  /** Hydrates the query cache from the device. True when a copy was shown. Never throws. */
  restore: () => Promise<boolean>
  /** Starts saving as the cache changes; returns the stop function. */
  start: () => () => void
  /** Saves now (and when the page is going away). */
  flush: () => Promise<void>
  /** Drops the restored copy if the live session turns out to belong to another person or workspace. */
  checkScope: () => Promise<void>
  /** The workspace the restored copy was for; null when none was restored. */
  restoredWorkspaceId: () => string | null
}

const isEntry = (raw: unknown): raw is Entry => {
  const e = raw as Partial<Entry> | null
  return !!e && typeof e === 'object' && typeof e.owner === 'string' && typeof e.workspaceId === 'string' &&
    !!e.client && typeof e.client === 'object' && typeof e.client.timestamp === 'number' && !!e.client.clientState
}

export function createDeviceCache(deps: DeviceCacheDeps): DeviceCache {
  const { queryClient, getScope, getUserId, sync, buster } = deps
  const throttleMs = deps.throttleMs ?? SAVE_THROTTLE_MS

  let restoredWorkspace: string | null = null
  let restoredUser: string | null = null
  let verified = false
  let pendingSync: unknown
  let delivered = false
  let removed = false
  // Bumped each time a restore starts or gives up; a read that belongs to an
  // older attempt must not hydrate anything or leave state behind.
  let attempt = 0

  const persister: Persister = {
    persistClient: async () => {},
    removeClient: async () => {
      removed = true
      await wipeDeviceCache()
    },
    restoreClient: async () => {
      const mine = attempt
      const userId = getUserId()
      if (!userId) {
        await wipeDeviceCache()
        return undefined
      }
      const workspaceId = await get<string>(pointerKey(userId), deviceCacheStore())
      if (mine !== attempt || !workspaceId) return undefined
      const entry = await get<unknown>(entryKey(userId, workspaceId), deviceCacheStore())
      if (mine !== attempt) return undefined
      // What was stored must name this person; anything else is not theirs to read.
      if (!isEntry(entry) || entry.owner !== userId || entry.workspaceId !== workspaceId) {
        await wipeDeviceCache()
        return undefined
      }
      restoredWorkspace = workspaceId
      restoredUser = userId
      pendingSync = entry.sync
      delivered = true
      return entry.client
    },
  }

  async function save(expectedGeneration: number): Promise<void> {
    if (!hasIndexedDb()) return
    const scope = getScope()
    if (!scope || expectedGeneration !== generation) return
    const entry: Entry = {
      owner: scope.userId,
      workspaceId: scope.workspaceId,
      client: { timestamp: Date.now(), buster, clientState: dehydrateForDevice(queryClient) },
      sync: sync.get(),
    }
    try {
      await set(entryKey(scope.userId, scope.workspaceId), entry, deviceCacheStore())
      await set(pointerKey(scope.userId), scope.workspaceId, deviceCacheStore())
      // A wipe that came while writing must not be undone by it.
      if (expectedGeneration !== generation) await clear(deviceCacheStore())
    } catch (err) {
      console.warn('[device-cache] save failed', err)
      try { await del(entryKey(scope.userId, scope.workspaceId), deviceCacheStore()) } catch { /* nothing more to do */ }
    }
  }

  const flush = () => save(generation)

  return {
    async restore() {
      if (!hasIndexedDb()) return false
      delivered = false
      removed = false
      restoredCopy = null
      attempt++
      let budget: ReturnType<typeof setTimeout> | undefined
      try {
        await Promise.race([
          persistQueryClientRestore({
            queryClient,
            persister,
            maxAge: PERSIST_MAX_AGE,
            buster,
            // Without observers a restored query would be collected after the
            // default gcTime and drop out of the next save.
            hydrateOptions: { defaultOptions: { queries: { gcTime: PERSIST_MAX_AGE } } },
          }),
          new Promise<void>((resolve) => { budget = setTimeout(resolve, RESTORE_BUDGET_MS) }),
        ])
      } catch (err) {
        console.warn('[device-cache] restore failed', err)
        await wipeDeviceCache()
        restoredWorkspace = null
        return false
      } finally {
        clearTimeout(budget)
        // Gave up before anything arrived: a read still in flight is abandoned.
        if (!delivered) attempt++
      }
      if (!delivered || removed) {
        restoredWorkspace = null
        return false
      }
      verified = false
      sync.restore(pendingSync)
      restoredCopy = { at: Date.now(), workspaceId: restoredWorkspace! }
      return true
    },

    start() {
      let timer: ReturnType<typeof setTimeout> | null = null
      let waitingFor = generation
      const unsubscribe = queryClient.getQueryCache().subscribe((event) => {
        if (event.type !== 'added' && event.type !== 'updated' && event.type !== 'removed') return
        if (timer) return
        waitingFor = generation
        timer = setTimeout(() => {
          timer = null
          void save(waitingFor)
        }, throttleMs)
      })
      const onHide = () => { if (document.visibilityState === 'hidden') void flush() }
      document.addEventListener('visibilitychange', onHide)
      window.addEventListener('pagehide', onHide)
      return () => {
        unsubscribe()
        if (timer) clearTimeout(timer)
        document.removeEventListener('visibilitychange', onHide)
        window.removeEventListener('pagehide', onHide)
      }
    },

    flush,

    async checkScope() {
      const scope = getScope()
      if (!scope || restoredWorkspace === null || verified) return
      verified = true
      if (scope.workspaceId === restoredWorkspace && scope.userId === restoredUser) return
      queryClient.clear()
      sync.restore(null)
      restoredWorkspace = null
      restoredCopy = null
      await wipeDeviceCache()
    },

    restoredWorkspaceId: () => restoredWorkspace,
  }
}
