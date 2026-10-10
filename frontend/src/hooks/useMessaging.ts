import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { messagingApi, type CreateChannelInput, type ChannelMember } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

// --- Channels ---

export const channelsQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: keys.messaging.channels(wsId), queryFn: () => messagingApi.listChannels(wsId), enabled: !!wsId })

export function useChannels(wsId: string) { return useQuery(channelsQueryOptions(wsId)) }

export function useCreateChannel(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: CreateChannelInput) => messagingApi.createChannel(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.messaging.channels(wsId) }),
  })
}

// --- Direct messages ---

export const directMessagesQueryOptions = () =>
  queryOptions({ queryKey: keys.messaging.dms(), queryFn: () => messagingApi.listDMs() })

/** DM channels the user can read. Not workspace-scoped on the backend. */
export function useDirectMessages() { return useQuery(directMessagesQueryOptions()) }

/** Opens (finds or creates) the DM with a person. */
export function useCreateDM() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (target: { userId: string; ngacNodeId: string }) => messagingApi.createDM(target),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.messaging.dms() }),
  })
}

export function useUpdateChannel(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: { name: string }) => messagingApi.updateChannel(channelId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: keys.messaging.channelsAll() })
    },
  })
}

// --- Read Receipts ---

export function useUnreadCounts() {
  return useQuery({
    queryKey: keys.messaging.unreadCounts(),
    queryFn: () => messagingApi.getUnreadCounts(),
    // No polling — WS unreadCount event handles real-time updates
  })
}

export function useMarkRead(channelId: string) {
  return useMutation({
    // A background side effect of reading; the user did not ask for it, so a
    // failure is not theirs to be told about.
    meta: { silentError: true },
    mutationFn: (lastMessageId: string) => messagingApi.markRead(channelId, lastMessageId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() }),
  })
}

// --- Search ---

export function useSearch(channelId: string, query: string) {
  return useQuery({
    queryKey: keys.messaging.search(channelId, query),
    queryFn: () => messagingApi.searchMessages(channelId, query),
    enabled: !!channelId && !!query && query.length >= 2,
  })
}

// --- Members ---

export function useChannelMembers(channelId: string) {
  return useQuery({
    queryKey: keys.messaging.members(channelId),
    queryFn: () => messagingApi.listMembers(channelId),
    enabled: !!channelId,
  })
}

/** Optimistic add member to channel — member appears instantly, rolls back on error. */
export function useAddChannelMember(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ ngacNodeId }: { ngacNodeId: string; username?: string; userId?: string }) =>
      messagingApi.addMember(channelId, ngacNodeId),
    onMutate: async ({ ngacNodeId, username, userId }: { ngacNodeId: string; username?: string; userId?: string }) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.members(channelId) })
      const previous = queryClient.getQueryData(keys.messaging.members(channelId))
      queryClient.setQueryData(
        keys.messaging.members(channelId),
        (old: { members: ChannelMember[] | null } | undefined) => ({
          members: [
            ...(old?.members || []),
            { user_id: userId || '', username: username || '', ngac_node_id: ngacNodeId },
          ],
        }),
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(keys.messaging.members(channelId), context.previous)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: keys.messaging.members(channelId) })
      queryClient.invalidateQueries({ queryKey: keys.messaging.channelsAll() })
    },
  })
}

/** Optimistic remove member from channel — member disappears instantly, rolls back on error. */
export function useRemoveChannelMember(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ nodeId }: { nodeId: string }) =>
      messagingApi.removeMember(channelId, nodeId),
    onMutate: async ({ nodeId }: { nodeId: string }) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.members(channelId) })
      const previous = queryClient.getQueryData(keys.messaging.members(channelId))
      queryClient.setQueryData(
        keys.messaging.members(channelId),
        (old: { members: ChannelMember[] | null } | undefined) => ({
          members: (old?.members || []).filter((m) => m.ngac_node_id !== nodeId),
        }),
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(keys.messaging.members(channelId), context.previous)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: keys.messaging.members(channelId) })
      queryClient.invalidateQueries({ queryKey: keys.messaging.channelsAll() })
    },
  })
}
