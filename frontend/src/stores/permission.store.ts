import { create } from 'zustand'
import { useAuthStore } from './auth.store'

/** Default cache TTL in milliseconds. */
const CACHE_TTL_MS = 60_000

/**
 * Resolved permissions for one object.
 *
 * These are the operations the backend actually evaluates. There is no separate
 * `delete`: deletion — trashing, restoring and permanent removal alike — is
 * enforced as a write on the item's object attribute, so asking the policy
 * service about a `delete` operation returns DENY for everyone and hides the
 * action from users who can genuinely perform it.
 */
export interface ObjectPerms {
  read: boolean
  write: boolean
  share: boolean
}

const EMPTY_PERMS: ObjectPerms = { read: false, write: false, share: false }

interface CacheEntry {
  perms: ObjectPerms
  expiresAt: number
}

/**
 * Cache key.
 *
 * Scoping by tenant makes cross-tenant reuse structurally impossible rather
 * than dependent on a clear() firing at the right moment. The same object ID
 * seen under two tenants occupies two entries, so a missed clear degrades into
 * a stale-within-tenant read instead of one tenant reading another's answer.
 */
function cacheKey(objectId: string): string {
  const tenantId = useAuthStore.getState().tenantId ?? 'no-tenant'
  return `${tenantId}:${objectId}`
}

interface PermissionState {
  /** Cache keyed by `{tenantId}:{objectId}`. */
  cache: Map<string, CacheEntry>

  /** Get cached permissions (returns undefined if miss or expired). */
  get: (objectId: string) => ObjectPerms | undefined

  /** Bulk set permissions from a batch API response. */
  setBatch: (results: Record<string, Record<string, boolean>>) => void

  /** Invalidate a single object's cache entry. */
  invalidate: (objectId: string) => void

  /** Clear entire cache (used on tenant switch or reconnect). */
  clear: () => void
}

export const usePermissionStore = create<PermissionState>()((set, get) => ({
  cache: new Map(),

  get: (objectId) => {
    const key = cacheKey(objectId)
    const entry = get().cache.get(key)
    if (!entry) return undefined
    if (Date.now() > entry.expiresAt) {
      // Expired — remove and return miss
      get().cache.delete(key)
      return undefined
    }
    return entry.perms
  },

  setBatch: (results) => {
    const now = Date.now()
    const cache = new Map(get().cache)
    for (const [objectId, perms] of Object.entries(results)) {
      cache.set(cacheKey(objectId), {
        perms: {
          read: perms.read ?? false,
          write: perms.write ?? false,
          share: perms.share ?? false,
        },
        expiresAt: now + CACHE_TTL_MS,
      })
    }
    set({ cache })
  },

  invalidate: (objectId) => {
    const cache = new Map(get().cache)
    cache.delete(cacheKey(objectId))
    set({ cache })
  },

  clear: () => {
    set({ cache: new Map() })
  },
}))

/** Convenience: get perms or fallback to all-false. */
export function getPermsOrDefault(objectId: string): ObjectPerms {
  return usePermissionStore.getState().get(objectId) ?? EMPTY_PERMS
}
