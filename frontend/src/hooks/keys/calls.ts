/** Query keys of calls. A call is addressed by its own id, which is unique across workspaces. */
export const callKeys = {
  detail: (callId: string) => ['calls', callId] as const,
  /** The call ringing or running in a conversation, if any (its header's "Tham gia"). */
  open: (channelId: string) => ['calls', 'open', channelId] as const,
}
