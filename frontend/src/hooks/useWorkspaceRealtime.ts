import { useEffect } from 'react'
import { useWebSocketStore } from '../stores/websocket.store'

/**
 * Follows the open workspace's live changes while the shell is mounted and the
 * workspace is the one in view. Switching workspace releases the old
 * subscription and takes the new one; the server refuses a workspace the
 * session has no right to read.
 */
export function useWorkspaceRealtime(workspaceId: string, signedIn: boolean) {
  const follow = useWebSocketStore((s) => s.followWorkspace)
  useEffect(() => {
    if (!workspaceId || !signedIn) return
    return follow(workspaceId)
  }, [workspaceId, signedIn, follow])
}
