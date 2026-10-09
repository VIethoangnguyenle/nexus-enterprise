import { useMemo } from 'react'
import { useSearch } from '@tanstack/react-router'
import { useWorkspaces } from './useWorkspaces'

/**
 * The workspace the user is currently looking at, and the only way to get it.
 *
 * Reads the `?ws=` parameter the sidebar switcher sets, falling back to the
 * first workspace the user can reach. Routes that render workspace data declare
 * the parameter with `validateSearch`, so it arrives here already typed and the
 * router, not `window.location`, is what this subscribes to.
 *
 * Pages used to inline `wsData?.workspaces?.[0]?.id` and ignore the parameter
 * entirely, so switching workspace changed the URL and nothing else. For anyone
 * whose first workspace is not the one they own — a member of someone else's
 * workspace listed ahead of their own — Drive, Documents and Assets were pinned
 * to a workspace they only had read on, and the Upload button answered 403 with
 * no way to get out of it.
 *
 * The switcher still performs a full page load: drive keeps the open folder in
 * a client store that is not keyed by workspace, and a folder from the previous
 * workspace must not survive the switch.
 */
export function useActiveWorkspace() {
  const { data, isLoading, isError } = useWorkspaces()
  const { ws: requested } = useSearch({ strict: false }) as { ws?: string }

  return useMemo(() => {
    const workspaces = data?.workspaces ?? []

    // Only honour a requested workspace the user actually has, so a stale or
    // hand-edited parameter degrades to their own workspace instead of a run
    // of 403s against one they cannot reach.
    const active = workspaces.find((w) => w.id === requested) ?? workspaces[0]

    return {
      workspaceId: active?.id ?? '',
      workspaceName: active?.name ?? '',
      workspaces,
      isLoading,
      isError,
    }
  }, [data, isLoading, isError, requested])
}
