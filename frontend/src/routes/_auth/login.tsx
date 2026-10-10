import { createFileRoute } from '@tanstack/react-router'
import { LoginScreen } from '../../components/auth/LoginScreen'
import { redirectIfSignedIn } from '../../lib/auth-guards'
import { validateWorkspaceSearch } from '../../lib/workspace'

/**
 * `error` is the code the auth service appends when Google sign-in fails; `ws`
 * is the workspace a shared link pointed at.
 */
export function validateLoginSearch(search: Record<string, unknown>): { error?: string; ws?: string } {
  return {
    ...validateWorkspaceSearch(search),
    ...(typeof search.error === 'string' && search.error ? { error: search.error } : {}),
  }
}

export const Route = createFileRoute('/_auth/login')({
  validateSearch: validateLoginSearch,
  beforeLoad: redirectIfSignedIn,
  component: LoginScreen,
})
