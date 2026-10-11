import { redirect } from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'
import { workspacesQueryOptions } from '../hooks/useWorkspaces'

/**
 * `beforeLoad` guard for the areas a personal workspace does not offer
 * (Phê duyệt, Tài sản). It waits for the workspace list, so a reload, a
 * bookmark, a notification link or a workspace switch (which empties the cache)
 * is sent to Tin nhắn before the area renders or asks for any data.
 *
 * The list is read the same way the app reads it (`?ws=`, else the first
 * workspace). If it cannot be fetched the area is let through: the services
 * refuse the calls anyway, and the screen shows its own error.
 */
export async function redirectIfPersonal({ context, search }: { context: unknown; search: { ws?: string } }): Promise<void> {
  // The root route does not type its context; main.tsx provides the query client.
  const queryClient = (context as { queryClient?: QueryClient } | undefined)?.queryClient
  if (!queryClient) return

  let workspaces
  try {
    ;({ workspaces } = await queryClient.ensureQueryData(workspacesQueryOptions()))
  } catch {
    return
  }
  const active = workspaces.find((w) => w.id === search.ws) ?? workspaces[0]
  if (active?.type === 'personal') {
    throw redirect({ to: '/channels', search: search.ws ? { ws: search.ws } : {}, replace: true })
  }
}
