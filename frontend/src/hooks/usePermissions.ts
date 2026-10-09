import { useMemo } from 'react'
import { useQueries } from '@tanstack/react-query'
import { batchCheckAccess, NGAC_OPS, NO_PERMS, type ObjectPerms } from '../api/access'
import { useAuthStore } from '../stores/auth.store'
import { keys } from './keys'

/** Keeps only the operations the policy service knows; anything missing is denied. */
function toPerms(answer: Record<string, boolean> | undefined): ObjectPerms {
  if (!answer) return NO_PERMS
  const perms = { ...NO_PERMS }
  for (const op of NGAC_OPS) perms[op] = answer[op] === true
  return perms
}

interface Waiter {
  resolve: (perms: ObjectPerms) => void
  reject: (err: unknown) => void
}

const waiting = new Map<string, Waiter[]>()
let flushScheduled = false

/** Sends every object asked about since the last tick as one request. */
function flush() {
  flushScheduled = false
  const batch = new Map(waiting)
  waiting.clear()
  batchCheckAccess([...batch.keys()]).then(
    (res) => {
      for (const [id, ws] of batch) for (const w of ws) w.resolve(toPerms(res.results[id]))
    },
    (err) => {
      for (const ws of batch.values()) for (const w of ws) w.reject(err)
    },
  )
}

/**
 * One object's permissions, coalesced with every other object asked about in
 * the same tick. Each object is its own query (so a permission event can
 * refresh exactly that one), but the network sees a single batch call.
 */
function loadPerms(objectId: string): Promise<ObjectPerms> {
  return new Promise((resolve, reject) => {
    const ws = waiting.get(objectId) ?? []
    ws.push({ resolve, reject })
    waiting.set(objectId, ws)
    if (!flushScheduled) {
      flushScheduled = true
      setTimeout(flush, 0)
    }
  })
}

/**
 * Batch permission hook. Given a list of object IDs, returns what the user may
 * do with each. Answers are cached per object in TanStack Query, so a row that
 * scrolls away and back, or a second component asking about the same object,
 * costs nothing. Until an answer arrives (or if the check fails) an object is
 * denied everything.
 *
 * Usage:
 *   const { permsMap, isLoading } = usePermissions(items.map(i => i.ngac_node_id))
 *   const canWrite = permsMap[item.ngac_node_id]?.write ?? false
 */
export function usePermissions(objectIds: string[]) {
  const tenantId = useAuthStore((s) => s.tenantId)
  const ids = useMemo(() => [...new Set(objectIds.filter(Boolean))], [objectIds])

  return useQueries({
    queries: ids.map((id) => ({
      queryKey: keys.permissions.object(tenantId, id),
      queryFn: () => loadPerms(id),
      staleTime: 30_000,
      gcTime: 60_000,
    })),
    combine: (results) => {
      const permsMap: Record<string, ObjectPerms> = {}
      ids.forEach((id, i) => {
        permsMap[id] = results[i]?.data ?? NO_PERMS
      })
      return { permsMap, isLoading: results.some((r) => r.isLoading) }
    },
  })
}

/**
 * Single-object permission hook. Convenience wrapper.
 */
export function useObjectPermissions(objectId: string | undefined): ObjectPerms & { isLoading: boolean } {
  const ids = useMemo(() => (objectId ? [objectId] : []), [objectId])
  const { permsMap, isLoading } = usePermissions(ids)
  const perms = objectId ? (permsMap[objectId] ?? NO_PERMS) : NO_PERMS
  return { ...perms, isLoading }
}
