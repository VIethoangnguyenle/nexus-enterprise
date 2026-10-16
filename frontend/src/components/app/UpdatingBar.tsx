import { useEffect, useState, useSyncExternalStore } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { keys } from '../../hooks/keys'
import { useConnectionStatus } from '../../hooks/useConnectionLost'
import { getRestoredCopy } from '../../lib/query-persist'

/** A copy that is still on screen after this long is no longer "being updated": the bar stops waiting for it. */
export const UPDATING_GIVE_UP_MS = 15_000

/**
 * "Đang cập nhật…" (DESIGN.md, Đang cập nhật): a quiet strip shown while the
 * app displays the copy it kept on the device and the fresh state has not
 * arrived. It ends when the chat state is fetched after the restore, when that
 * fetch fails (the connection bar or the blocks themselves speak then), or when
 * the connection bar is up. Without a restored copy it never shows.
 */
export function UpdatingBar() {
  const queryClient = useQueryClient()
  const status = useConnectionStatus()
  const [gaveUp, setGaveUp] = useState(false)

  const waiting = useSyncExternalStore(
    (notify) => queryClient.getQueryCache().subscribe(notify),
    () => {
      const copy = getRestoredCopy()
      if (!copy) return false
      const state = queryClient.getQueryState(keys.messaging.chatState(copy.workspaceId))
      return !!state && state.status !== 'error' && state.dataUpdatedAt <= copy.at
    },
  )

  useEffect(() => {
    if (!waiting) return
    const t = setTimeout(() => setGaveUp(true), UPDATING_GIVE_UP_MS)
    return () => clearTimeout(t)
  }, [waiting])

  const show = waiting && !gaveUp && status !== 'lost'

  // The live region is always present and only its content changes, so screen readers announce it.
  return (
    <div role="status" aria-live="polite">
      {show && (
        <div className="flex items-center gap-2 h-7 px-4 shrink-0 text-xs font-medium bg-sunk text-ink-muted">
          <RefreshCw size={14} strokeWidth={1.75} className="shrink-0 animate-spin motion-reduce:animate-none [animation-duration:1.2s]" aria-hidden />
          <span>Đang cập nhật…</span>
        </div>
      )}
    </div>
  )
}
