import { apiFetch } from './client'

/** A person as the resolver answers: a name and a picture, nothing else. */
export interface ResolvedPersonWire {
  user_id: string
  ngac_node_id: string
  display_name: string
  avatar_url: string
}

export interface ResolveRequest {
  user_ids?: string[]
  node_ids?: string[]
}

export const contactsApi = {
  /**
   * Names and pictures for ids the client met in content, whether or not the directory lists
   * those people (hidden, removed). At most 200 ids per call; unknown ids are left out of the answer.
   */
  resolve: (wsId: string, body: ResolveRequest) =>
    apiFetch<{ people: ResolvedPersonWire[] | null }>(`/workspaces/${wsId}/contacts/resolve`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
}
