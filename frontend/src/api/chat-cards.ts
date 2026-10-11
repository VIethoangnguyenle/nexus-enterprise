import { apiFetch } from './client'
import type { Poll } from './messaging'

/**
 * A poll as the signed-in user sees it. Until they have voted (or the poll is
 * closed) the server leaves out counts, totals and voters: `results_visible`
 * says whether they are there, and the client never computes them itself.
 */
export interface PollView extends Poll {
  results_visible?: boolean
  /** Options the viewer has chosen. */
  my_option_ids?: string[]
  /** Closed early, or past `ends_at`. */
  closed?: boolean
  closed_at?: string
}

export interface CreatePollInput {
  question: string
  options: string[]
  is_multi: boolean
  is_anonymous: boolean
  /** RFC 3339, in the future; omitted for a poll that closes only by hand. */
  ends_at?: string
  /** A thread root of the same conversation: the poll is posted as a reply to it. */
  parent_message_id?: string
}

/** Every call answers with the poll as the caller now sees it. */
export const pollApi = {
  create: (channelId: string, input: CreatePollInput) =>
    apiFetch<PollView>(`/channels/${channelId}/polls`, { method: 'POST', body: JSON.stringify(input) }),
  get: (pollId: string) => apiFetch<PollView>(`/polls/${pollId}`),
  vote: (pollId: string, optionId: string) =>
    apiFetch<PollView>(`/polls/${pollId}/vote`, { method: 'POST', body: JSON.stringify({ option_id: optionId }) }),
  unvote: (pollId: string, optionId: string) =>
    apiFetch<PollView>(`/polls/${pollId}/vote`, { method: 'DELETE', body: JSON.stringify({ option_id: optionId }) }),
  close: (pollId: string) => apiFetch<PollView>(`/polls/${pollId}/close`, { method: 'POST' }),
}
