import { apiFetch } from './client'

/**
 * Group management (docs/specs/chat-group-management): who may post at the top
 * level, add people and mention everyone, and who runs the group. Only private
 * groups have it; a DM or the workspace-wide channel answers 400.
 */

/** Each rule is either open to every member or kept for the owner and managers. */
export type GroupPolicy = 'everyone' | 'managers'

export interface GroupSettings {
  post_policy: GroupPolicy
  invite_policy: GroupPolicy
  mention_all_policy: GroupPolicy
}

/** GET /channels/:id/management. People are named by user ID (as the roster names them); never rendered. */
export interface GroupManagement extends GroupSettings {
  owner_id: string
  manager_ids: string[]
}

/** The mention that notifies every member of a conversation; the server matches it exactly. */
export const MENTION_ALL = '@Tất cả'

const isPolicy = (v: unknown): v is GroupPolicy => v === 'everyone' || v === 'managers'

/** Validates the server's answer: an unknown rule reads as the open default, a missing list as empty. */
export function parseGroupManagement(raw: unknown): GroupManagement {
  const r = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const ids = Array.isArray(r.manager_ids) ? r.manager_ids.filter((x): x is string => typeof x === 'string' && x !== '') : []
  return {
    post_policy: isPolicy(r.post_policy) ? r.post_policy : 'everyone',
    invite_policy: isPolicy(r.invite_policy) ? r.invite_policy : 'everyone',
    mention_all_policy: isPolicy(r.mention_all_policy) ? r.mention_all_policy : 'everyone',
    owner_id: typeof r.owner_id === 'string' ? r.owner_id : '',
    manager_ids: ids,
  }
}

const enc = encodeURIComponent

export const groupManagementApi = {
  get: async (channelId: string) => parseGroupManagement(await apiFetch<unknown>(`/channels/${enc(channelId)}/management`)),
  /** The server takes all three rules at once. */
  updateSettings: (channelId: string, settings: GroupSettings) =>
    apiFetch<{ status: string }>(`/channels/${enc(channelId)}/management`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),
  appointManager: (channelId: string, userId: string) =>
    apiFetch<{ status: string }>(`/channels/${enc(channelId)}/managers/${enc(userId)}`, { method: 'POST' }),
  removeManager: (channelId: string, userId: string) =>
    apiFetch<{ status: string }>(`/channels/${enc(channelId)}/managers/${enc(userId)}`, { method: 'DELETE' }),
  transferOwnership: (channelId: string, userId: string) =>
    apiFetch<{ status: string }>(`/channels/${enc(channelId)}/transfer`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userId }),
    }),
  disband: (channelId: string) => apiFetch<{ status: string }>(`/channels/${enc(channelId)}`, { method: 'DELETE' }),
}
