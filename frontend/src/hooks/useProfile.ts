import { useMutation } from '@tanstack/react-query'
import { profileApi, type ProfileChanges } from '../api/profile'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

/**
 * After any change to one's own profile. Names and avatars everywhere come
 * from the contacts directory, so refreshing it is what makes the change show
 * in the sidebar, chat and tables at once; Settings' own read of the person is
 * refreshed too.
 */
const refreshMyProfile = () =>
  Promise.all([
    queryClient.invalidateQueries({ queryKey: keys.contacts.every() }),
    queryClient.invalidateQueries({ queryKey: keys.auth.profiles() }),
    queryClient.invalidateQueries({ queryKey: keys.auth.me() }),
  ])

/** Saves one's own profile. */
export function useUpdateProfile() {
  return useMutation({
    meta: { action: 'lưu hồ sơ' },
    mutationFn: (changes: ProfileChanges) => profileApi.update(changes),
    onSuccess: refreshMyProfile,
  })
}

/**
 * Uploads one's own picture. Errors are left to the screen, which says what
 * was wrong with the picture (silentError).
 */
export function useUploadAvatar() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (file: File) => profileApi.uploadAvatar(file),
    onSuccess: refreshMyProfile,
  })
}

/** Removes one's own picture. */
export function useRemoveAvatar() {
  return useMutation({
    meta: { action: 'bỏ ảnh đại diện' },
    mutationFn: () => profileApi.removeAvatar(),
    onSuccess: refreshMyProfile,
  })
}
