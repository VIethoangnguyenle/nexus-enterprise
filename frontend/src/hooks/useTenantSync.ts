import { useEffect, useRef } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { tenantIdFromToken, useAuthStore } from '../stores/auth.store'
import { useActiveWorkspace } from './useActiveWorkspace'
import { rescopeSession } from './useSwitchWorkspace'

/**
 * Keeps the session token and the workspace on screen the same workspace.
 *
 * The workspace comes from the URL (`?ws=`), so a deep link, a reload or a
 * shared link can name one while the token still speaks for another, and
 * tenant-scoped reads would answer for the token's. When they differ the
 * session is re-scoped to the open workspace, once per target, without
 * navigating (the address is already right). While they differ `syncing` is
 * true and the caller must not render or fetch anything tenant-scoped.
 *
 * A refused re-scope drops back to the token's workspace by replacing `ws` in
 * the address (no history entry); the shared failure toast tells the person.
 */
export function useTenantSync() {
  const { workspaceId, workspaces } = useActiveWorkspace()
  const tokenTenant = useAuthStore((s) => tenantIdFromToken(s.accessToken))
  const navigate = useNavigate()
  const attempted = useRef(new Set<string>())

  const { mutate } = useMutation({
    meta: { action: 'chuyển workspace' },
    mutationFn: rescopeSession,
    onError: () => {
      const current = tenantIdFromToken(useAuthStore.getState().accessToken)
      if (!current) return
      void navigate({ replace: true, search: (prev: Record<string, unknown>) => ({ ...prev, ws: current }) } as never)
    },
  })

  // A token without a workspace claim cannot be compared, so it never gates.
  const syncing = workspaces.length > 0 && !!workspaceId && !!tokenTenant && tokenTenant !== workspaceId

  useEffect(() => {
    // Back in step: a later mismatch with the same workspace (history Back) is a new one.
    if (!syncing) return void attempted.current.clear()
    if (attempted.current.has(workspaceId)) return
    attempted.current.add(workspaceId)
    mutate(workspaceId)
  }, [syncing, workspaceId, mutate])

  return { syncing }
}
