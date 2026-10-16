import { useEffect } from 'react'
import { prefetchChatParts } from '../../components/chat/lazy-parts'

/** Run `task` when the browser is idle, or shortly after where it has no idle callback (Safari). */
function whenIdle(task: () => void): () => void {
  if (typeof window.requestIdleCallback === 'function') {
    const id = window.requestIdleCallback(task, { timeout: 4000 })
    return () => window.cancelIdleCallback(id)
  }
  const id = window.setTimeout(task, 1500)
  return () => window.clearTimeout(id)
}

/**
 * Once Trang chủ has painted, fetch the code of the next likely screen, a
 * conversation: its view, composer, emoji picker and message editor. Opening a
 * conversation then finds them already in the browser cache. Failures are ignored:
 * the screen loads its own code when it is opened.
 */
export function useConversationPrefetch(): void {
  useEffect(
    () =>
      whenIdle(() => {
        void import('../../components/spaces/SpaceView').catch(() => undefined)
        prefetchChatParts()
      }),
    [],
  )
}
