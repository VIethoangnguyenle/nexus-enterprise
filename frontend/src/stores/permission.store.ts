import { create } from 'zustand'

/** Default cache TTL in milliseconds. */
const CACHE_TTL_MS = 60_000

export interface ObjectPerms {
  read: boolean
  write: boolean
  delete: boolean
  share: boolean
}

const EMPTY_PERMS: ObjectPerms = { read: false, write: false, delete: false, share: false }

interface CacheEntry {
  perms: ObjectPerms
  expiresAt: number
}

interface PermissionState {
  /** Cache keyed by objectId (tenant-scoped via separate clear on switch). */
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
    const entry = get().cache.get(objectId)
    if (!entry) return undefined
    if (Date.now() > entry.expiresAt) {
      // Expired — remove and return miss
      get().cache.delete(objectId)
      return undefined
    }
    return entry.perms
  },

  setBatch: (results) => {
    const now = Date.now()
    const cache = new Map(get().cache)
    for (const [objectId, perms] of Object.entries(results)) {
      cache.set(objectId, {
        perms: {
          read: perms.read ?? false,
          write: perms.write ?? false,
          delete: perms.delete ?? false,
          share: perms.share ?? false,
        },
        expiresAt: now + CACHE_TTL_MS,
      })
    }
    set({ cache })
  },

  invalidate: (objectId) => {
    const cache = new Map(get().cache)
    cache.delete(objectId)
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
