import { onTenantMoved } from '../api/client'
import { resetTenantScopedState } from '../hooks/useSwitchWorkspace'
import { toast } from '../components/primitives/Toast'
import { leftOnPurpose, noteSelfLeave } from './self-leave'

export { noteSelfLeave }

/** Where the app goes when its session has been moved. */
export type MovedDestination =
  | { to: '/channels'; search: { ws: string } }
  | { to: '/workspace-select'; search: Record<string, never> }

/**
 * Makes the app follow a session that the server moved to another workspace.
 *
 * A refresh moves it (and says `moved`) when the person no longer belongs to the
 * workspace their token named: they were removed, or left from another device. It hands
 * back a token for the workspace they still have, or for none. The token is already
 * stored by then, so the auth store speaks for the new workspace; what is left is
 * everything the app holds of the old one (queries, the copy on the device), the
 * address, and telling the person why the screen changed, without naming an id.
 *
 * `navigate` goes to the destination: a workspace they still have, or the workspace
 * picker when none is left. It is passed in because the router is built at the app's
 * root and cannot be imported from here. Returns the unregister.
 */
export function installTenantMoved(navigate: (to: MovedDestination) => void): () => void {
  return onTenantMoved(({ from, to }) => {
    if (leftOnPurpose(from)) return
    resetTenantScopedState()
    toast.info(
      to
        ? 'Bạn không còn là thành viên của workspace đó. Đã chuyển sang workspace khác của bạn.'
        : 'Bạn không còn thuộc workspace nào.',
      { duration: 8000 },
    )
    navigate(to ? { to: '/channels', search: { ws: to } } : { to: '/workspace-select', search: {} })
  })
}
