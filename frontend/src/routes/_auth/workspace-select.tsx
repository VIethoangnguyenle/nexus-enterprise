import { createFileRoute } from '@tanstack/react-router'
import { WorkspaceSelectScreen } from '../../components/auth/WorkspaceSelectScreen'
import { requireSignedIn } from '../../lib/auth-guards'
import { validateWorkspaceSearch } from '../../lib/workspace'

/**
 * `ws` is the workspace a link named. `verified` and `verify_error` are how
 * Google reports the end of "prove my address": a flag, and a short code the
 * screen turns into a sentence.
 */
export function validateSelectSearch(search: Record<string, unknown>): { ws?: string; verified?: '1'; verify_error?: string } {
  return {
    ...validateWorkspaceSearch(search),
    ...(search.verified === '1' || search.verified === 1 ? { verified: '1' as const } : {}),
    ...(typeof search.verify_error === 'string' && search.verify_error ? { verify_error: search.verify_error } : {}),
  }
}

export const Route = createFileRoute('/_auth/workspace-select')({
  validateSearch: validateSelectSearch,
  beforeLoad: requireSignedIn,
  component: WorkspaceSelectScreen,
})
