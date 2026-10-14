import { apiFetch, apiUpload } from './client'

/** The fields of one's own profile that the server lets a person change. */
export interface ProfileChanges {
  display_name?: string
  title?: string
  location?: string
}

export const profileApi = {
  update: (changes: ProfileChanges) =>
    apiFetch<{ status: string }>('/me/profile', { method: 'PATCH', body: JSON.stringify(changes) }),

  /**
   * Makes `file` the person's picture. The server re-encodes it into three
   * sizes and answers the address the profile now points at.
   */
  uploadAvatar: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return apiUpload<{ avatar_url: string }>('/me/avatar', form, 'PUT')
  },

  /** Removes the person's picture; their initials show again. */
  removeAvatar: () => apiFetch<{ avatar_url: string }>('/me/avatar', { method: 'DELETE' }),
}
