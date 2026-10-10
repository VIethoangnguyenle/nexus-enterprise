import { useCallback } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { authApi } from '../api/auth'
import { resetArrivals } from '../lib/notification-arrivals'
import { queryClient } from '../lib/query-client'
import { tenantIdFromToken, useAuthStore } from '../stores/auth.store'
import { useDriveStore } from '../stores/drive.store'
import { useActiveWorkspace } from './useActiveWorkspace'
import { useMyWorkspaces } from './useAuth'

/** The module roots a person can stay on across a switch; anything deeper belongs to the old workspace. */
const MODULE_ROOTS = ['/channels', '/drive', '/documents', '/approval', '/assets', '/contacts', '/admin', '/settings'] as const
type ModuleRoot = (typeof MODULE_ROOTS)[number]

/**
 * The module the person is in, as its root route. A channel, a document or an
 * open request belongs to the workspace being left, so the new workspace opens
 * on the module's own front page rather than on a record that is not there.
 */
export function moduleRootOf(pathname: string): ModuleRoot {
  return MODULE_ROOTS.find((root) => pathname === root || pathname.startsWith(`${root}/`)) ?? '/channels'
}

/**
 * Everything the client remembers about the workspace being left: the whole
 * query cache (one tenant's rows must never be served as another's) and the
 * Tài liệu client state, which is not keyed by workspace. The open folder and
 * view live in the URL and go with the navigation.
 */
export function resetTenantScopedState() {
  queryClient.clear()
  // Pending toasts and washes belong to the workspace being left.
  resetArrivals()
  useDriveStore.setState({ selectedItemId: null, expandedFolders: new Set<string>() })
}

/**
 * Opens another workspace from inside the app. Services that keep
 * per-workspace data read the workspace from the token, so the session is
 * re-scoped first (the same call the workspace picker makes); only when that
 * succeeds is the token replaced and the old workspace's data dropped. A
 * refusal leaves the person exactly where they were.
 */
export function useSwitchWorkspace() {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  return useMutation({
    meta: { action: 'chuyển workspace' },
    mutationFn: async (workspaceId: string) => {
      const { accessToken, setAccessToken } = useAuthStore.getState()
      let token: string | null = null
      if (tenantIdFromToken(accessToken) !== workspaceId) {
        token = (await authApi.switchTenant(workspaceId)).access_token
        // Requests still in flight were made for the old workspace.
        await queryClient.cancelQueries()
      }
      if (token) setAccessToken(token)
      resetTenantScopedState()
      await navigate({ to: moduleRootOf(pathname), search: { ws: workspaceId } })
      return workspaceId
    },
  })
}

/**
 * What a workspace switcher needs: only the workspaces the person can enter
 * (their own memberships, the list the picker uses), which one is open, and a
 * `choose` that switches. Choosing the open one does nothing but report done.
 */
export function useWorkspaceSwitcher() {
  const { workspaceId } = useActiveWorkspace()
  const mine = useMyWorkspaces()
  const switchTo = useSwitchWorkspace()
  const choose = useCallback(
    (id: string, onDone?: () => void) => {
      if (id === workspaceId) return onDone?.()
      switchTo.mutate(id, { onSuccess: () => onDone?.() })
    },
    [workspaceId, switchTo],
  )
  return {
    currentId: workspaceId,
    workspaces: mine.data ?? [],
    isLoading: mine.isPending,
    isError: mine.isError,
    refetch: mine.refetch,
    choose,
    switchingId: switchTo.isPending ? switchTo.variables : undefined,
  }
}
