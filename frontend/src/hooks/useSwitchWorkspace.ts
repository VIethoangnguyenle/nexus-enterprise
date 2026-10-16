import { useCallback } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { authApi } from '../api/auth'
import { queryClient } from '../lib/query-client'
import { wipeDeviceCache } from '../lib/query-persist'
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
  // The copy on the device is the old workspace's too.
  void wipeDeviceCache()
  useDriveStore.setState({ selectedItemId: null, expandedFolders: new Set<string>() })
}

/**
 * Makes the session speak for a workspace: the token is re-scoped with the same
 * call the workspace picker makes, and only when that succeeds is it replaced
 * and the previous workspace's data dropped. A refusal throws and leaves the
 * token and the cache as they were. Shared by the switcher and by the layout's
 * tenant sync, so both re-scope identically.
 *
 * When the token already speaks for the workspace nothing changed, so nothing
 * is dropped: clearing the cache then would only throw away the lists the app
 * has just fetched (the workspace list first) and fetch them again.
 */
export async function rescopeSession(workspaceId: string) {
  const { accessToken, setAccessToken } = useAuthStore.getState()
  if (tenantIdFromToken(accessToken) === workspaceId) return
  const token = (await authApi.switchTenant(workspaceId)).access_token
  // Requests still in flight were made for the old workspace.
  await queryClient.cancelQueries()
  setAccessToken(token)
  resetTenantScopedState()
}

/**
 * Opens another workspace from inside the app. Services that keep
 * per-workspace data read the workspace from the token, so the session is
 * re-scoped first (`rescopeSession`). A refusal leaves the person exactly where
 * they were. The new workspace opens on the module the person is in, or, with
 * `landOnChat`, on Trang chủ of Chat.
 */
export function useSwitchWorkspace({ landOnChat = false }: { landOnChat?: boolean } = {}) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  return useMutation({
    meta: { action: 'chuyển workspace' },
    mutationFn: async (workspaceId: string) => {
      await rescopeSession(workspaceId)
      await navigate({ to: landOnChat ? '/channels' : moduleRootOf(pathname), search: { ws: workspaceId } })
      return workspaceId
    },
  })
}

/**
 * What a workspace switcher needs: only the workspaces the person can enter
 * (their own memberships, the list the picker uses), which one is open, and a
 * `choose` that switches. Choosing the open one does nothing but report done.
 */
export function useWorkspaceSwitcher(opts: { landOnChat?: boolean } = {}) {
  const { workspaceId } = useActiveWorkspace()
  const mine = useMyWorkspaces()
  const switchTo = useSwitchWorkspace(opts)
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
