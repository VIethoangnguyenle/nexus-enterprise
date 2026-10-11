import { apiFetch } from './client'
import type { ChatSection } from '../lib/chat-sections'

export interface NewSectionInput {
  name: string
  /** Where to insert (Undo of a delete); omitted: last. */
  position?: number
  /** Conversations the section starts with. */
  channelIds?: string[]
}

const base = (wsId: string) => `/workspaces/${wsId}/chat-sections`

/** Personal navigator sections (docs/specs/chat-sections). Every call is the caller's own. */
export const sectionsApi = {
  list: (wsId: string) => apiFetch<{ sections: ChatSection[] | null }>(base(wsId)),
  create: (wsId: string, input: NewSectionInput) =>
    apiFetch<ChatSection>(base(wsId), {
      method: 'POST',
      body: JSON.stringify({ name: input.name, position: input.position, channel_ids: input.channelIds }),
    }),
  rename: (wsId: string, id: string, name: string) =>
    apiFetch<{ status: string }>(`${base(wsId)}/${id}`, { method: 'PATCH', body: JSON.stringify({ name }) }),
  move: (wsId: string, id: string, move: 'up' | 'down') =>
    apiFetch<{ status: string }>(`${base(wsId)}/${id}`, { method: 'PATCH', body: JSON.stringify({ move }) }),
  /** Answers with the section as it was, so the caller can offer Undo. */
  remove: (wsId: string, id: string) => apiFetch<ChatSection>(`${base(wsId)}/${id}`, { method: 'DELETE' }),
  place: (wsId: string, id: string, channelId: string) =>
    apiFetch<{ status: string }>(`${base(wsId)}/${id}/channels/${channelId}`, { method: 'PUT' }),
  unplace: (wsId: string, channelId: string) =>
    apiFetch<{ status: string }>(`${base(wsId)}/channels/${channelId}`, { method: 'DELETE' }),
}
