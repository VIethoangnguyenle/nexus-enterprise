import { contactsApi, type ResolvedPersonWire } from '../api/contacts'
import { onSessionEnd } from './session-end'

/**
 * Names and pictures for people the workspace directory does not list: a hidden department's
 * members, someone who has left. They still wrote the messages, own the files, sit on the
 * approval, so their name has to be found somewhere other than Danh bạ.
 *
 * One store per workspace. `request` only queues; the answer comes from a batched POST, and only
 * then are subscribers told, so asking from inside a render never changes what that render sees.
 */

/** A person the resolver found. `name` is empty when they have not saved a profile yet. */
export interface ResolvedEntry {
  userId: string
  nodeId: string
  name: string
  avatarUrl: string
}

/** What a render reads. A new object whenever something changed, the same one until then. */
export interface ResolverSnapshot {
  byUserId: ReadonlyMap<string, ResolvedEntry>
  byNodeId: ReadonlyMap<string, ResolvedEntry>
}

export interface PeopleResolver {
  /** Asks for a user id's name. Idempotent; returns at once. */
  request(userId: string): void
  /** Asks for an NGAC node id's name. */
  requestNode(nodeId: string): void
  /**
   * Asks again for people whose profile changed; old names stay until new ones land. With `ids`
   * (user or node ids) only those held people are asked about; without, everyone held is.
   */
  refresh(ids?: string[]): void
  /** Forgets everyone and ignores answers still on their way. */
  clear(): void
  subscribe(listener: () => void): () => void
  getSnapshot(): ResolverSnapshot
}

interface Options {
  /** How long misses are collected before one request goes out. */
  batchMs?: number
  now?: () => number
}

/** The most ids the server takes in one call. */
const MAX_PER_CALL = 200
const BATCH_MS = 50
const BACKOFF_START_MS = 2_000
const BACKOFF_MAX_MS = 60_000
/** After this many failures in a row the resolver stops retrying alone; the next render that needs the name asks again. */
const AUTO_RETRIES = 5

const EMPTY: ResolverSnapshot = { byUserId: new Map(), byNodeId: new Map() }

/**
 * Ids are UUIDs. Anything else (a bot's name, an empty string) can never be a person of the
 * workspace, and the server rejects the whole call that carries it, so it is never sent.
 */
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

type Kind = 'user' | 'node'
const idKey = (kind: Kind, id: string) => `${kind}:${id}`

export function createPeopleResolver(workspaceId: string, { batchMs = BATCH_MS, now = Date.now }: Options = {}): PeopleResolver {
  let snapshot: ResolverSnapshot = EMPTY
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | undefined
  const listeners = new Set<() => void>()
  /** Asked for, not yet sent. */
  const pending = new Map<string, { kind: Kind; id: string }>()
  const inFlight = new Set<string>()
  /** Asked and answered, found or not: not asked again. */
  const answered = new Set<string>()
  const failures = new Map<string, { retryAt: number; attempts: number }>()
  /** In flight when a refresh came: the answer predates the change, so it is asked for again when it lands. */
  const stale = new Set<string>()
  let retryTimer: ReturnType<typeof setTimeout> | undefined

  const notify = () => listeners.forEach((l) => l())

  function enqueue(kind: Kind, id: string) {
    if (!UUID.test(id)) return
    const key = idKey(kind, id)
    if (pending.has(key) || inFlight.has(key) || answered.has(key)) return
    const failed = failures.get(key)
    if (failed && now() < failed.retryAt) return
    pending.set(key, { kind, id })
    timer ??= setTimeout(flush, batchMs)
  }

  function flush() {
    timer = undefined
    const batch = [...pending.values()]
    pending.clear()
    for (let i = 0; i < batch.length; i += MAX_PER_CALL) void send(batch.slice(i, i + MAX_PER_CALL))
  }

  /** One timer for the earliest backoff that ends: then every failed id that is due is asked for again. */
  function scheduleRetry() {
    if (retryTimer) clearTimeout(retryTimer)
    retryTimer = undefined
    let next = Infinity
    for (const f of failures.values()) if (f.attempts < AUTO_RETRIES) next = Math.min(next, f.retryAt)
    if (next === Infinity) return
    retryTimer = setTimeout(() => {
      retryTimer = undefined
      for (const [key, f] of failures) {
        if (f.attempts >= AUTO_RETRIES || now() < f.retryAt) continue
        const sep = key.indexOf(':')
        enqueue(key.slice(0, sep) as Kind, key.slice(sep + 1))
      }
      scheduleRetry()
    }, Math.max(0, next - now()))
  }

  async function send(chunk: { kind: Kind; id: string }[]) {
    const mine = generation
    const keys = chunk.map((c) => idKey(c.kind, c.id))
    keys.forEach((k) => inFlight.add(k))
    const userIds = chunk.filter((c) => c.kind === 'user').map((c) => c.id)
    const nodeIds = chunk.filter((c) => c.kind === 'node').map((c) => c.id)
    const body = { ...(userIds.length ? { user_ids: userIds } : {}), ...(nodeIds.length ? { node_ids: nodeIds } : {}) }
    try {
      const res = await contactsApi.resolve(workspaceId, body)
      if (mine !== generation) return // cleared meanwhile: this answer belongs to the previous session
      keys.forEach((k) => { answered.add(k); failures.delete(k) })
      land(res.people ?? [])
    } catch {
      if (mine !== generation) return
      for (const k of keys) {
        const attempts = (failures.get(k)?.attempts ?? 0) + 1
        failures.set(k, { attempts, retryAt: now() + Math.min(BACKOFF_START_MS * 2 ** (attempts - 1), BACKOFF_MAX_MS) })
      }
      scheduleRetry()
    } finally {
      if (mine === generation) {
        keys.forEach((k) => inFlight.delete(k))
        for (const k of keys) {
          if (!stale.delete(k)) continue
          answered.delete(k)
          const sep = k.indexOf(':')
          enqueue(k.slice(0, sep) as Kind, k.slice(sep + 1))
        }
      }
    }
  }

  function land(people: ResolvedPersonWire[]) {
    if (people.length === 0) return
    const byUserId = new Map(snapshot.byUserId)
    const byNodeId = new Map(snapshot.byNodeId)
    for (const p of people) {
      const entry: ResolvedEntry = {
        userId: p.user_id, nodeId: p.ngac_node_id, name: p.display_name?.trim() ?? '', avatarUrl: p.avatar_url ?? '',
      }
      if (entry.userId) {
        byUserId.set(entry.userId, entry)
        answered.add(idKey('user', entry.userId))
      }
      if (entry.nodeId) {
        byNodeId.set(entry.nodeId, entry)
        answered.add(idKey('node', entry.nodeId))
      }
    }
    snapshot = { byUserId, byNodeId }
    notify()
  }

  return {
    request: (userId) => enqueue('user', userId),
    requestNode: (nodeId) => enqueue('node', nodeId),
    refresh(ids) {
      const want = ids?.length ? new Set(ids) : undefined
      const asked: [Kind, string][] = []
      for (const e of snapshot.byUserId.values()) {
        if (!want || want.has(e.userId) || want.has(e.nodeId)) asked.push(['user', e.userId])
      }
      // A person known by node id alone (no user id came back) is refreshed by the node.
      for (const e of snapshot.byNodeId.values()) {
        if (!e.userId && (!want || want.has(e.nodeId))) asked.push(['node', e.nodeId])
      }
      for (const [kind, id] of asked) {
        answered.delete(idKey(kind, id))
        enqueue(kind, id)
      }
      for (const key of inFlight) {
        const id = key.slice(key.indexOf(':') + 1)
        if (!want || want.has(id)) stale.add(key)
      }
    },
    clear() {
      generation++
      if (timer) clearTimeout(timer)
      timer = undefined
      if (retryTimer) clearTimeout(retryTimer)
      retryTimer = undefined
      pending.clear(); inFlight.clear(); answered.clear(); failures.clear(); stale.clear()
      snapshot = EMPTY
      notify()
    },
    subscribe(listener) {
      listeners.add(listener)
      return () => { listeners.delete(listener) }
    },
    getSnapshot: () => snapshot,
  }
}

const stores = new Map<string, PeopleResolver>()

/** The workspace's resolver, made on first use. */
export function resolverFor(workspaceId: string): PeopleResolver {
  let r = stores.get(workspaceId)
  if (!r) {
    r = createPeopleResolver(workspaceId)
    stores.set(workspaceId, r)
  }
  return r
}

/** Sign-out and workspace switch: nobody's names outlive the session that was allowed to see them. */
export function clearResolvers(): void {
  stores.forEach((r) => r.clear())
}

// auth.store cannot import this module (it would pull the API client into the auth module that
// the client itself reads), so sign-out reaches it through the session-end registry.
onSessionEnd(clearResolvers)
