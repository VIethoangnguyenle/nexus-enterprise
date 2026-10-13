import { useMemo } from 'react'
import { useSearch } from '@tanstack/react-router'
import { tenantIdFromToken, useAuthStore } from '../stores/auth.store'
import { useWorkspaces } from './useWorkspaces'

/**
 * The workspace the user is currently looking at, and the only way to get it.
 *
 * Reads the `?ws=` parameter the sidebar switcher sets, falling back to the
 * workspace the session token speaks for, then the first one the user can reach.
 * The token's workspace comes first because tenant-scoped reads answer for it:
 * showing another one would pair that workspace's name with this one's data. Routes that render workspace data declare
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
 * The switcher (`useWorkspaceSwitcher`) re-scopes the session to the chosen
 * workspace, drops the cache and the Tài liệu client state, and only then
 * changes this parameter, so nothing from the previous workspace survives.
 */
export function useActiveWorkspace() {
  const { data, isLoading, isError } = useWorkspaces()
  const { ws: requested } = useSearch({ strict: false }) as { ws?: string }
  const tokenTenant = useAuthStore((s) => tenantIdFromToken(s.accessToken))

  return useMemo(() => {
    const workspaces = data?.workspaces ?? []

    // Only honour a requested workspace the user actually has, so a stale or
    // hand-edited parameter degrades to the session's workspace instead of a
    // run of 403s against one they cannot reach.
    const active =
      workspaces.find((w) => w.id === requested) ??
      workspaces.find((w) => w.id === tokenTenant) ??
      workspaces[0]

    return {
      workspaceId: active?.id ?? '',
      workspaceName: active?.name ?? '',
      workspaces,
      isLoading,
      isError,
    }
  }, [data, isLoading, isError, requested, tokenTenant])
}
