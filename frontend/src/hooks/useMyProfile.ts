import { useQuery } from '@tanstack/react-query'
import { authApi } from '../api/auth'
import { keys } from './keys'

/**
 * The signed-in person's own profile, with the department an administrator
 * placed them in for one workspace. Settings reads this instead of searching the
 * workspace directory for "me": the directory is a list of everyone, and a
 * person's own record should not depend on being found in it.
 */
export function useMyProfile(workspaceId: string) {
  return useQuery({
    queryKey: keys.auth.profile(workspaceId),
    queryFn: () => authApi.me(workspaceId),
    enabled: !!workspaceId,
  })
}
