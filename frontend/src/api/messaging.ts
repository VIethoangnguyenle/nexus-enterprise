import { apiFetch, apiUpload } from './client'

/**
 * A conversation. `channel_type` is `dm` for a direct message and
 * `workspace` (or `private`) for a space; the backend maps `group` to
 * `workspace` on create. DMs are not returned by the workspace channel list;
 * they come from `GET /dms`.
 */
export interface Channel {
  id: string
  name: string
  channel_type: string
  workspace_id: string
  topic?: string
  description?: string
  member_count?: number
  /** The content object attribute messages are authorized on; used to ask what the caller may do here. */
  ngac_oa_id?: string
  created_at?: unknown
  /** The signed-in user starred this conversation; omitted when not. */
  starred?: boolean
  /** The group's picture (`/api/channels/avatars/<token>`); empty or omitted when it has none. Never set on a DM. */
  avatar_url?: string
}

/** A channel member as the messaging service returns it. Ids are for API calls only. */
export interface ChannelMember { user_id: string; username: string; ngac_node_id: string }

/**
 * A conversation's people, with whether the signed-in user may take others out.
 * The server decides (the group's creator or a manager, never in a DM or the
 * workspace-wide channel); a response without the flag means no.
 */
export interface ChannelMembersResponse { members: ChannelMember[] | null; can_remove_members?: boolean }

export interface ReactionGroup { emoji: string; count: number; user_ids: string[] }

export interface MessageAttachment {
  drive_file_id: string
  file_name: string
  mime_type: string
  size_bytes: number
  width?: number
  height?: number
  /** Client-only: local object URL for optimistic previews, revoked on settle */
  localUrl?: string
}

export interface Message {
  id: string
  channel_id: string
  sender_id: string
  sender_name?: string
  content: string
  content_format?: string
  mentions?: string[]
  /** A main message's number in its conversation (1, 2, 3…); absent on a reply. */
  seq?: number
  /** A reply's number in its thread; absent on a main message. */
  reply_seq?: number
  reply_count?: number
  /** Thread summary of a root message: unset until the first reply. Same shape as created_at. */
  last_reply_at?: unknown
  /** users.id of the three most recent distinct repliers, newest first. */
  reply_participant_ids?: string[]
  /** The signed-in user follows this message's thread. */
  following?: boolean
  /** Replies in this thread the signed-in user has not read; 0 when not following. */
  thread_unread_count?: number
  /** Set on thread replies; empty on top-level messages (topics). */
  parent_message_id?: string
  /** Client-only: the optimistic id this message replaced, kept as a stable React key. */
  _clientKey?: string
  reactions?: ReactionGroup[]
  is_pinned?: boolean
  created_at?: unknown
  linked_entity_type?: string
  linked_entity_id?: string
  /** "user" or "system" (a manager's deletion leaves a system line). */
  message_type?: string
  /** Same shape as created_at; set once the sender has changed the text. */
  edited_at?: unknown
  /** Deleted: no body, kept only as the placeholder of a thread. */
  deleted?: boolean
  /** What the message quotes: the sender's name and the opening of the text. */
  quoted?: QuotedMessage
  attachments?: MessageAttachment[]
  /** The signed-in user starred this message; omitted when not. */
  starred?: boolean
}

/** What a quote shows of the message it quotes (no body when that message was deleted). */
export interface QuotedMessage {
  id: string
  sender_id: string
  sender_name?: string
  text?: string
  deleted?: boolean
}

export interface PinnedMessage {
  message: Message
  pinned_by: string
  pinned_at: string
}

export interface PollOption {
  id: string
  text: string
  position: number
  vote_count: number
  voter_ids?: string[]
}

export interface Poll {
  id: string
  message_id: string
  channel_id: string
  question: string
  options: PollOption[]
  is_multi: boolean
  is_anonymous: boolean
  created_by: string
  total_votes: number
  ends_at?: string
  created_at: string
}

export interface ChatTask {
  id: string
  message_id: string
  channel_id: string
  title: string
  assignee_id: string
  assignee_name: string
  status: string
  due_date?: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface ChannelUnread {
  channel_id: string
  unread_count: number
  last_read_message_id: string
  /** Unread replies in the threads of this channel the signed-in user follows; omitted when 0. */
  thread_unread_count?: number
  /** The caller's notification level for this conversation; absent means the default, `main`. */
  notification_level?: 'all' | 'main' | 'off'
  /** Unread main messages that mention the caller, at every level. */
  mention_count?: number
  /** The signed-in user starred this conversation; omitted when not. */
  starred?: boolean
  /** When the caller starred it (a protobuf timestamp, or an RFC 3339 string on a fresh optimistic star); omitted when not starred. */
  starred_at?: unknown
  /** The conversation's last main message number when counted; omitted when 0. */
  last_seq?: number
  /** The caller has read the main messages numbered up to this; omitted when 0. */
  last_read_seq?: number
}

/** The caller's own state of one thread, as follow / unfollow / read answer. */
export interface ThreadState {
  root_message_id: string
  channel_id: string
  following: boolean
  unread_count?: number
  as_of?: unknown
}

/** The opening of one message, as a conversation list previews it. */
export interface ActivityMessage {
  id: string
  sender_id: string
  sender_name: string
  /** Plain text, at most 140 characters; empty for a file or card sent without text. */
  preview: string
  kind: 'text' | 'attachment' | 'poll' | 'task' | 'card' | 'call'
  /** File name of the first attachment; only when `kind` is `attachment`. */
  attachment_name?: string
  mentions?: string[]
  /** A protobuf timestamp (RFC 3339 on the wire). */
  created_at: string
  /** Set only on a thread reply. */
  parent_message_id?: string
}

/** What one conversation last said: its newest main-stream message, and its newest thread reply when that is newer. */
export interface ConversationActivity {
  channel_id: string
  latest: ActivityMessage
  latest_reply?: ActivityMessage
}

export type ThreadFilter = 'all' | 'following' | 'unread'

export interface ThreadPage { messages: Message[]; has_more: boolean }

export interface CreateChannelInput { name: string; channel_type: string }

export interface SendMessageInput {
  content: string
  content_format?: string
  mentions?: string[]
  parent_message_id?: string
  linked_entity_type?: string
  linked_entity_id?: string
  /** A live message of the same channel this one quotes. */
  quoted_message_id?: string
  attachments?: { drive_file_id: string; width?: number; height?: number }[]
}

export interface EditMessageInput { content: string; content_format: string; mentions?: string[] }

/** What Chat's results can be narrowed by; the names the server reads them under are in `messageSearchParams`. */
export interface MessageSearchFilters {
  /** Sender's user id. */
  from?: string
  /** Conversation id. */
  channelId?: string
  /** First day, `YYYY-MM-DD` in the reader's zone. */
  after?: string
  /** Last day, inclusive, `YYYY-MM-DD` in the reader's zone. */
  beforeDate?: string
  hasFile?: boolean
  hasLink?: boolean
  mentionsMe?: boolean
}

export interface MessageSearchQuery extends MessageSearchFilters {
  q: string
  /** Paging cursor: send time (RFC 3339) and id of the last message of the previous page. */
  before?: string
  beforeId?: string
  limit?: number
}

/**
 * The one place that maps a search to `GET /api/messages/search`. A filter that is off sends nothing. The day
 * filters mean days in the reader's zone, so they travel with that zone's IANA name as `tz`. The server
 * (docs/specs/chat-search) combines every filter given with AND, reads the days in `tz` (Asia/Ho_Chi_Minh when
 * absent), answers 400 for a malformed id, day or zone, and an empty list for a well-formed id nobody has.
 */
export function messageSearchParams(q: MessageSearchQuery): URLSearchParams {
  const qs = new URLSearchParams({ q: q.q })
  if (q.from) qs.set('from', q.from)
  if (q.channelId) qs.set('channel_id', q.channelId)
  if (q.after) qs.set('after', q.after)
  if (q.beforeDate) qs.set('before_date', q.beforeDate)
  if (q.after || q.beforeDate) qs.set('tz', Intl.DateTimeFormat().resolvedOptions().timeZone)
  if (q.hasFile) qs.set('has_file', 'true')
  if (q.hasLink) qs.set('has_link', 'true')
  if (q.mentionsMe) qs.set('mentions_me', 'true')
  if (q.before) qs.set('before', q.before)
  if (q.beforeId) qs.set('before_id', q.beforeId)
  if (q.limit) qs.set('limit', String(q.limit))
  return qs
}

export const messagingApi = {
  // Channels
  listChannels: (wsId: string) => apiFetch<{ channels: Channel[] }>(`/workspaces/${wsId}/channels`),
  createChannel: (wsId: string, data: CreateChannelInput) =>
    apiFetch<Channel>(`/workspaces/${wsId}/channels`, { method: 'POST', body: JSON.stringify(data) }),
  getChannel: (channelId: string) => apiFetch<Channel>(`/channels/${channelId}`),
  updateChannel: (channelId: string, data: { name: string }) =>
    apiFetch<Channel>(`/channels/${channelId}`, { method: 'PATCH', body: JSON.stringify(data) }),

  /** Makes `file` the group's picture; the server re-encodes it and answers the address the channel now points at. */
  uploadChannelAvatar: (channelId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return apiUpload<{ avatar_url: string }>(`/channels/${channelId}/avatar`, form, 'PUT')
  },
  /** Removes the group's picture; its initials show again. */
  removeChannelAvatar: (channelId: string) =>
    apiFetch<{ avatar_url: string }>(`/channels/${channelId}/avatar`, { method: 'DELETE' }),

  // Messages
  /** The newest page of a conversation, or the page of main messages numbered below `beforeSeq`. */
  listMessages: (channelId: string, beforeSeq?: number) => {
    const qs = beforeSeq ? `?before_seq=${beforeSeq}` : ''
    return apiFetch<{ messages: Message[]; has_more: boolean }>(`/channels/${channelId}/messages${qs}`)
  },
  sendMessage: (channelId: string, data: string | SendMessageInput) => {
    const body = typeof data === 'string'
      ? { content: data }
      : data
    return apiFetch<Message>(`/channels/${channelId}/messages`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  /** Changes the text of the caller's own message; answers with the message as stored (filtered). */
  editMessage: (messageId: string, data: EditMessageInput) =>
    apiFetch<Message>(`/messages/${messageId}`, { method: 'PATCH', body: JSON.stringify(data) }),
  /** Soft delete: the sender, or whoever manages the channel. */
  deleteMessage: (messageId: string) =>
    apiFetch<{ status: string }>(`/messages/${messageId}`, { method: 'DELETE' }),
  /** Moves the caller's read position to just before this message. */
  markMessageUnread: (messageId: string) =>
    apiFetch<{ status: string }>(`/messages/${messageId}/unread`, { method: 'POST' }),
  /** A thread's root and a page of its replies, oldest first: those numbered above `afterReplySeq`. */
  getThread: (messageId: string, afterReplySeq?: number) =>
    apiFetch<{ messages: Message[]; has_more?: boolean }>(
      `/messages/${messageId}/thread${afterReplySeq ? `?after_reply_seq=${afterReplySeq}` : ''}`,
    ),
  /** A whole thread: its root and every reply, read a page at a time. */
  getWholeThread: async (messageId: string) => {
    const first = await messagingApi.getThread(messageId)
    const messages = [...(first.messages ?? [])]
    let more = !!first.has_more
    while (more) {
      const after = messages[messages.length - 1]?.reply_seq
      if (!after) break
      const page = await messagingApi.getThread(messageId, after)
      messages.push(...(page.messages ?? []).slice(1))
      more = !!page.has_more
    }
    return { messages }
  },

  // Thread follow, read position and the channel's thread list
  followThread: (messageId: string) =>
    apiFetch<ThreadState>(`/messages/${messageId}/follow`, { method: 'POST' }),
  unfollowThread: (messageId: string) =>
    apiFetch<ThreadState>(`/messages/${messageId}/follow`, { method: 'DELETE' }),
  markThreadRead: (messageId: string) =>
    apiFetch<ThreadState>(`/messages/${messageId}/thread/read`, { method: 'POST' }),
  /** Root messages that have replies, most recently active first. `before` is the last reply time (RFC 3339) of the previous page's last item, `beforeId` its root id. */
  listThreads: (channelId: string, q: { filter: ThreadFilter; before?: string; beforeId?: string; limit?: number }) => {
    const qs = new URLSearchParams({ filter: q.filter })
    if (q.before) qs.set('before', q.before)
    if (q.beforeId) qs.set('before_id', q.beforeId)
    if (q.limit) qs.set('limit', String(q.limit))
    // The server leaves empty fields out: an empty page is just `{}` or `{ "has_more": false }`.
    return apiFetch<Partial<ThreadPage>>(`/channels/${channelId}/threads?${qs}`).then(
      (page): ThreadPage => ({ messages: page.messages ?? [], has_more: !!page.has_more }),
    )
  },

  // Direct messages (each belongs to one workspace: the session's)
  listDMs: () => apiFetch<{ channels: Channel[] | null }>(`/dms`),
  /** Finds the existing DM with this person or creates it. */
  createDM: (target: { userId: string; ngacNodeId: string }) =>
    apiFetch<Channel>(`/dms`, {
      method: 'POST',
      body: JSON.stringify({ target_user_id: target.userId, target_ngac_node_id: target.ngacNodeId }),
    }),

  // Members
  listMembers: (channelId: string) =>
    apiFetch<ChannelMembersResponse>(`/channels/${channelId}/members`),
  addMember: (channelId: string, ngacNodeId: string) =>
    apiFetch<{ status: string }>(`/channels/${channelId}/members`, {
      method: 'POST',
      body: JSON.stringify({ ngac_node_id: ngacNodeId }),
    }),
  removeMember: (channelId: string, nodeId: string) =>
    apiFetch<{ status: string }>(`/channels/${channelId}/members/${nodeId}`, { method: 'DELETE' }),

  // Reactions
  addReaction: (messageId: string, emoji: string) =>
    apiFetch(`/messages/${messageId}/reactions`, { method: 'POST', body: JSON.stringify({ emoji }) }),
  removeReaction: (messageId: string, emoji: string) =>
    apiFetch(`/messages/${messageId}/reactions/${encodeURIComponent(emoji)}`, { method: 'DELETE' }),
  listReactions: (messageId: string) =>
    apiFetch<{ reactions: ReactionGroup[] }>(`/messages/${messageId}/reactions`),

  // Pins
  pinMessage: (channelId: string, messageId: string) =>
    apiFetch(`/channels/${channelId}/pins`, { method: 'POST', body: JSON.stringify({ message_id: messageId }) }),
  unpinMessage: (channelId: string, messageId: string) =>
    apiFetch(`/channels/${channelId}/pins/${messageId}`, { method: 'DELETE' }),
  listPins: (channelId: string) =>
    apiFetch<{ pins: PinnedMessage[] }>(`/channels/${channelId}/pins`),

  // Read Receipts
  markRead: (channelId: string, lastMessageId: string) =>
    apiFetch(`/channels/${channelId}/read`, { method: 'POST', body: JSON.stringify({ last_message_id: lastMessageId }) }),
  getUnreadCounts: () =>
    apiFetch<{ channels: ChannelUnread[] }>(`/channels/unread`),
  /** The newest message and newest thread reply of every conversation the signed-in user reads; conversations with no message are absent. */
  conversationActivity: () =>
    apiFetch<{ channels: ConversationActivity[] | null }>(`/channels/activity`).then(
      (r): ConversationActivity[] => r.channels ?? [],
    ),
  /** Threads the signed-in user follows across every conversation they read, most recently active first. Paged like `listThreads`. */
  followedThreads: (q: { before?: string; beforeId?: string; limit?: number } = {}) => {
    const qs = new URLSearchParams()
    if (q.before) qs.set('before', q.before)
    if (q.beforeId) qs.set('before_id', q.beforeId)
    if (q.limit) qs.set('limit', String(q.limit))
    const suffix = qs.size ? `?${qs}` : ''
    return apiFetch<Partial<ThreadPage>>(`/threads/followed${suffix}`).then(
      (page): ThreadPage => ({ messages: page.messages ?? [], has_more: !!page.has_more }),
    )
  },

  // Search
  /**
   * Messages, main and thread replies, whose text matches `q`, across every conversation the
   * signed-in user reads; newest first, paged like `followedThreads`. A `q` shorter than two
   * characters answers an empty page. A reply names its thread in `parent_message_id`.
   */
  searchAll: (q: MessageSearchQuery) =>
    apiFetch<Partial<ThreadPage>>(`/messages/search?${messageSearchParams(q)}`).then(
      (page): ThreadPage => ({ messages: page.messages ?? [], has_more: !!page.has_more }),
    ),
  searchMessages: (channelId: string, query: string, limit = 20) =>
    apiFetch<{ messages: Message[] }>(`/channels/${channelId}/search?q=${encodeURIComponent(query)}&limit=${limit}`),

  // Polls
  createPoll: (channelId: string, data: { question: string; options: string[]; is_multi?: boolean; is_anonymous?: boolean }) =>
    apiFetch<Poll>(`/channels/${channelId}/polls`, { method: 'POST', body: JSON.stringify(data) }),
  votePoll: (pollId: string, optionId: string) =>
    apiFetch(`/polls/${pollId}/vote`, { method: 'POST', body: JSON.stringify({ option_id: optionId }) }),
  removeVote: (pollId: string, optionId: string) =>
    apiFetch(`/polls/${pollId}/vote`, { method: 'DELETE', body: JSON.stringify({ option_id: optionId }) }),
  getPoll: (pollId: string) =>
    apiFetch<Poll>(`/polls/${pollId}`),

  // Tasks
  createTask: (channelId: string, data: { title: string; assignee_id?: string; due_date?: string; parent_message_id?: string }) =>
    apiFetch<ChatTask>(`/channels/${channelId}/tasks`, { method: 'POST', body: JSON.stringify(data) }),
  updateTask: (taskId: string, data: { status?: string; assignee_id?: string; title?: string; due_date?: string }) =>
    apiFetch<ChatTask>(`/tasks/${taskId}`, { method: 'PATCH', body: JSON.stringify(data) }),
  listTasks: (channelId: string, status?: string) => {
    const qs = status ? `?status=${status}` : ''
    // The server answers a conversation with no task as `{"tasks": null}`.
    return apiFetch<{ tasks: ChatTask[] | null }>(`/channels/${channelId}/tasks${qs}`).then((r) => ({ ...r, tasks: r.tasks ?? [] }))
  },
}
