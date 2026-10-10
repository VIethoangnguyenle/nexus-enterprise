import { apiFetch } from './client'

/** The fields of one's own profile that the server lets a person change. */
export interface ProfileChanges {
  display_name?: string
  title?: string
  location?: string
}

export const profileApi = {
  update: (changes: ProfileChanges) =>
    apiFetch<{ status: string }>('/me/profile', { method: 'PATCH', body: JSON.stringify(changes) }),
}
