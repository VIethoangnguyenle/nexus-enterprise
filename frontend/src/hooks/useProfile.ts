import { useMutation } from '@tanstack/react-query'
import { profileApi, type ProfileChanges } from '../api/profile'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

/**
 * Saves one's own profile. Names and avatars everywhere come from the contacts
 * directory, so refreshing it is what makes the new name show in the sidebar,
 * chat and tables at once; Settings' own read of the person is refreshed too.
 */
export function useUpdateProfile() {
  return useMutation({
    meta: { action: 'lưu hồ sơ' },
    mutationFn: (changes: ProfileChanges) => profileApi.update(changes),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.contacts.every() }),
        // Settings reads the person's own record; a save changes it.
        queryClient.invalidateQueries({ queryKey: keys.auth.profiles() }),
        queryClient.invalidateQueries({ queryKey: keys.auth.me() }),
      ]),
  })
}
