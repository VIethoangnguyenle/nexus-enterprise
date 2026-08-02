import { useMemo } from 'react'
import { useWorkspaces } from './useWorkspaces'

/**
 * The workspace the user is currently looking at.
 *
 * Reads the `?ws=` parameter the sidebar switcher sets, falling back to the
 * first workspace the user can reach.
 *
 * Pages used to inline `wsData?.workspaces?.[0]?.id` and ignore the parameter
 * entirely, so switching workspace changed the URL and nothing else. For anyone
 * whose first workspace is not the one they own — a member of someone else's
 * workspace listed ahead of their own — Drive, Documents and Assets were pinned
 * to a workspace they only had read on, and the Upload button answered 403 with
 * no way to get out of it.
 *
 * The switcher performs a full page load, so reading the parameter at render
 * time is enough; there is no navigation to subscribe to.
 */
export function useActiveWorkspace() {
  const { data, isLoading } = useWorkspaces()

  return useMemo(() => {
    const workspaces = data?.workspaces ?? []
    const requested = new URLSearchParams(window.location.search).get('ws')

    // Only honour a requested workspace the user actually has, so a stale or
    // hand-edited parameter degrades to their own workspace instead of a run
    // of 403s against one they cannot reach.
    const active = workspaces.find((w) => w.id === requested) ?? workspaces[0]

    return {
      workspaceId: active?.id ?? '',
      workspaceName: active?.name ?? '',
      workspaces,
      isLoading,
    }
  }, [data, isLoading])
}
