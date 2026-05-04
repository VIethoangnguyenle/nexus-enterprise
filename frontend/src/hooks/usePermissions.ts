import { useEffect, useMemo, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { batchCheckAccess } from '../api/access'
import { usePermissionStore, type ObjectPerms } from '../stores/permission.store'

const EMPTY_PERMS: ObjectPerms = { read: false, write: false, delete: false, share: false }

/**
 * Batch permission hook. Given a list of object IDs:
 * 1. Returns cached perms for objects already in cache
 * 2. Fetches uncached objects in a single batch call
 * 3. Coalesces: multiple components calling usePermissions in the same render
 *    cycle share one batch request (via TanStack Query dedup on sorted key).
 *
 * Usage:
 *   const { permsMap, isLoading } = usePermissions(items.map(i => i.ngac_node_id))
 *   const canWrite = permsMap[item.ngac_node_id]?.write ?? false
 */
export function usePermissions(objectIds: string[]) {
  const store = usePermissionStore()

  // Determine which IDs are NOT in cache (misses)
  const uncachedIds = useMemo(() => {
    return objectIds.filter((id) => id && !store.get(id))
  }, [objectIds, store.cache])

  // Stable query key: sort uncached IDs to maximize dedup
  const queryKey = useMemo(
    () => ['batch-access', ...uncachedIds.slice().sort()],
    [uncachedIds],
  )

  const { isLoading } = useQuery({
    queryKey,
    queryFn: async () => {
      if (uncachedIds.length === 0) return {}
      const result = await batchCheckAccess(uncachedIds)
      store.setBatch(result.results)
      return result.results
    },
    enabled: uncachedIds.length > 0,
    staleTime: 30_000,
    gcTime: 60_000,
  })

  // Build a perms map for all requested IDs (cached + freshly fetched)
  const permsMap = useMemo(() => {
    const map: Record<string, ObjectPerms> = {}
    for (const id of objectIds) {
      map[id] = store.get(id) ?? EMPTY_PERMS
    }
    return map
  }, [objectIds, store.cache, isLoading])

  return { permsMap, isLoading: isLoading && uncachedIds.length > 0 }
}

/**
 * Single-object permission hook. Convenience wrapper.
 */
export function useObjectPermissions(objectId: string | undefined): ObjectPerms & { isLoading: boolean } {
  const ids = useMemo(() => (objectId ? [objectId] : []), [objectId])
  const { permsMap, isLoading } = usePermissions(ids)
  const perms = objectId ? (permsMap[objectId] ?? EMPTY_PERMS) : EMPTY_PERMS
  return { ...perms, isLoading }
}
