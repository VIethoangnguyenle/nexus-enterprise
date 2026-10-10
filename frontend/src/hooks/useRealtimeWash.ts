import { useAuthStore } from '../stores/auth.store'
import { useWebSocketStore } from '../stores/websocket.store'
import { useArrivals } from './useArrivals'

interface Options<T> {
  /** The entity's id, as realtime events name it. */
  idOf: (item: T) => string
  /** Identity of an item *version*: a new value means the item changed. */
  versionOf: (item: T) => string
  /** False until the list has loaded; what is there then is history, not news. */
  ready: boolean
  /** Changing this forgets what has been seen. */
  scope?: string
}

/**
 * Washes rows that changed because of somebody else, in that person's hue
 * (DESIGN.md §7). The realtime event says who acted and on which ids; the
 * refetch that follows it brings the new version, and `useArrivals` tells the
 * two apart from history. A change with no known author, or one of my own,
 * gets no wash.
 */
export function useRealtimeWash<T>(items: T[], { idOf, versionOf, ready, scope }: Options<T>) {
  const me = useAuthStore((s) => s.user?.id)
  const changes = useWebSocketStore((s) => s.recentChanges)
  return useArrivals(items, {
    keyOf: versionOf,
    authorOf: (item) => changes[idOf(item)]?.actorUserId,
    me,
    ready,
    scope,
  })
}
