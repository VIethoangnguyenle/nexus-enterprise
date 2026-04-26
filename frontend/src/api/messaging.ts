import { apiFetch } from './client'

export interface Channel { id: string; name: string; channel_type: string; workspace_id: string }
export interface Message { id: string; channel_id: string; sender_id: string; sender_name?: string; content: string; reply_count?: number; created_at?: string }

export const messagingApi = {
  listChannels: (wsId: string) => apiFetch<{ channels: Channel[] }>(`/workspaces/${wsId}/channels`),
  createChannel: (wsId: string, data: Partial<Channel>) => apiFetch<Channel>(`/workspaces/${wsId}/channels`, { method: 'POST', body: JSON.stringify(data) }),
  listMessages: (channelId: string, before?: string) => {
    const qs = before ? `?before=${before}` : ''
    return apiFetch<{ messages: Message[]; has_more: boolean }>(`/channels/${channelId}/messages${qs}`)
  },
  sendMessage: (channelId: string, content: string) => apiFetch<Message>(`/channels/${channelId}/messages`, { method: 'POST', body: JSON.stringify({ content }) }),
  getThread: (messageId: string) => apiFetch<{ messages: Message[] }>(`/messages/${messageId}/thread`),
}
