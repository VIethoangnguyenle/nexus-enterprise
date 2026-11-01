import { apiFetch, apiUpload } from './client'

/** The fields of one's own profile that the server lets a person change. */
export interface ProfileChanges {
  display_name?: string
  title?: string
  location?: string
}

export const profileApi = {
  /**
   * The title belongs to a workspace: with `workspaceId` it is that workspace's
   * and the others keep theirs. Without one it is the account's default, shown
   * wherever no title was set. The other fields are the account's either way.
   */
  update: (changes: ProfileChanges, workspaceId?: string) =>
    apiFetch<{ status: string }>(
      workspaceId ? `/me/profile?workspace=${encodeURIComponent(workspaceId)}` : '/me/profile',
      { method: 'PATCH', body: JSON.stringify(changes) },
    ),

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
