import { apiFetch } from './client'

export interface Channel { id: string; name: string; channel_type: string; workspace_id: string; topic?: string; description?: string; member_count?: number }

export interface ReactionGroup { emoji: string; count: number; user_ids: string[] }

export interface Message {
  id: string
  channel_id: string
  sender_id: string
  sender_name?: string
  content: string
  content_format?: string
  mentions?: string[]
  reply_count?: number
  reactions?: ReactionGroup[]
  is_pinned?: boolean
  created_at?: unknown
  linked_entity_type?: string
  linked_entity_id?: string
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
}

export interface CreateChannelInput { name: string; channel_type: string }

export interface SendMessageInput {
  content: string
  content_format?: string
  mentions?: string[]
  parent_message_id?: string
  linked_entity_type?: string
  linked_entity_id?: string
}

export const messagingApi = {
  // Channels
  listChannels: (wsId: string) => apiFetch<{ channels: Channel[] }>(`/workspaces/${wsId}/channels`),
  createChannel: (wsId: string, data: CreateChannelInput) =>
    apiFetch<Channel>(`/workspaces/${wsId}/channels`, { method: 'POST', body: JSON.stringify(data) }),
  getChannel: (channelId: string) => apiFetch<Channel>(`/channels/${channelId}`),
  updateChannel: (channelId: string, data: { name: string }) =>
    apiFetch<Channel>(`/channels/${channelId}`, { method: 'PATCH', body: JSON.stringify(data) }),

  // Messages
  listMessages: (channelId: string, before?: string) => {
    const qs = before ? `?before=${before}` : ''
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
  getThread: (messageId: string) => apiFetch<{ messages: Message[] }>(`/messages/${messageId}/thread`),

  // Members
  listMembers: (channelId: string) =>
    apiFetch<{ members: { user_id: string; username: string; ngac_node_id: string }[] }>(`/channels/${channelId}/members`),
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

  // Search
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
  createTask: (channelId: string, data: { title: string; assignee_id?: string; due_date?: string }) =>
    apiFetch<ChatTask>(`/channels/${channelId}/tasks`, { method: 'POST', body: JSON.stringify(data) }),
  updateTask: (taskId: string, data: { status?: string; assignee_id?: string; title?: string; due_date?: string }) =>
    apiFetch<ChatTask>(`/tasks/${taskId}`, { method: 'PATCH', body: JSON.stringify(data) }),
  listTasks: (channelId: string, status?: string) => {
    const qs = status ? `?status=${status}` : ''
    return apiFetch<{ tasks: ChatTask[] }>(`/channels/${channelId}/tasks${qs}`)
  },
}
