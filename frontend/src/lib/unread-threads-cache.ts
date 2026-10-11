import { keys } from '../hooks/keys'
import { queryClient } from './query-client'

/** WebSocket frames that change which followed threads have unread replies. */
const THREAD_FRAMES: ReadonlySet<string> = new Set(['threadState', 'threadReply'])

/**
 * Marks Trang chủ's "Thread có tin mới" stale when a frame of the given kind
 * (the envelope's `oneofKind`) says a thread's replies or read position moved.
 * Returns whether it did, so the caller's switch stays a one-liner.
 */
export function invalidateUnreadThreadsFor(frameKind: string | undefined): boolean {
  if (!frameKind || !THREAD_FRAMES.has(frameKind)) return false
  void queryClient.invalidateQueries({ queryKey: keys.notificationLevels.unreadThreads() })
  return true
}
