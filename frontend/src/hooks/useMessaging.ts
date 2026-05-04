import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { messagingApi, type CreateChannelInput, type SendMessageInput, type Message, type Poll, type ChatTask } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'

// --- Channels ---

export const channelsQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: ['channels', wsId], queryFn: () => messagingApi.listChannels(wsId), enabled: !!wsId })

export function useChannels(wsId: string) { return useQuery(channelsQueryOptions(wsId)) }

export function useCreateChannel(wsId: string) {
  return useMutation({
    mutationFn: (data: CreateChannelInput) => messagingApi.createChannel(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['channels', wsId] }),
  })
}

export function useUpdateChannel(channelId: string) {
  return useMutation({
    mutationFn: (data: { name: string }) => messagingApi.updateChannel(channelId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] })
    },
  })
}

// --- Messages ---

export const messagesQueryOptions = (channelId: string) =>
  queryOptions({ queryKey: ['messages', channelId], queryFn: () => messagingApi.listMessages(channelId), enabled: !!channelId })

export const threadQueryOptions = (messageId: string) =>
  queryOptions({ queryKey: ['thread', messageId], queryFn: () => messagingApi.getThread(messageId), enabled: !!messageId })

export function useMessages(channelId: string) { return useQuery(messagesQueryOptions(channelId)) }
export function useThread(messageId: string) { return useQuery(threadQueryOptions(messageId)) }

/** Sends a message with optimistic UI: message appears immediately, no refetch needed. */
export function useSendMessage(channelId: string) {
  return useMutation({
    mutationFn: (params: string | { content: string; linkedEntity?: { type: string; id: string } }) => {
      if (typeof params === 'string') {
        return messagingApi.sendMessage(channelId, params)
      }
      const input: SendMessageInput = {
        content: params.content,
        content_format: 'html',
        linked_entity_type: params.linkedEntity?.type || '',
        linked_entity_id: params.linkedEntity?.id || '',
      }
      return messagingApi.sendMessage(channelId, input)
    },

    // Optimistic: insert temp message BEFORE server responds
    onMutate: async (params) => {
      await queryClient.cancelQueries({ queryKey: ['messages', channelId] })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(['messages', channelId])
      const user = useAuthStore.getState().user

      const content = typeof params === 'string' ? params : params.content
      const tempMsg: Message & { _optimistic: true } = {
        id: `temp-${Date.now()}`,
        channel_id: channelId,
        sender_id: user?.id || '',
        sender_name: user?.username || '',
        content,
        content_format: 'html',
        created_at: new Date().toISOString(),
        _optimistic: true,
      }

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        ['messages', channelId],
        (old) => old ? { ...old, messages: [tempMsg as Message, ...(old.messages || [])] } : old,
      )

      return { previous }
    },

    // Replace temp message with server response (WS event will dedup via ID match)
    onSuccess: (serverMsg) => {
      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        ['messages', channelId],
        (old) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m: Message & { _optimistic?: boolean }) =>
              m._optimistic ? serverMsg : m,
            ),
          }
        },
      )
    },

    // Rollback on error
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(['messages', channelId], context.previous)
      }
    },
  })
}

// --- Reactions ---

export function useReactions(messageId: string) {
  return useQuery({
    queryKey: ['reactions', messageId],
    queryFn: () => messagingApi.listReactions(messageId),
    enabled: !!messageId,
  })
}

/** Toggles a reaction with optimistic UI — instant feedback, WS event deduplication. */
export function useToggleReaction(channelId: string) {
  return useMutation({
    mutationFn: ({ messageId, emoji, hasReacted }: { messageId: string; emoji: string; hasReacted: boolean }) =>
      hasReacted
        ? messagingApi.removeReaction(messageId, emoji)
        : messagingApi.addReaction(messageId, emoji),
    onMutate: async ({ messageId, emoji, hasReacted }) => {
      await queryClient.cancelQueries({ queryKey: ['messages', channelId] })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(['messages', channelId])
      const userId = useAuthStore.getState().user?.id || ''

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        ['messages', channelId],
        (old) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m) => {
              if (m.id !== messageId) return m
              const reactions = [...(m.reactions || [])]
              const idx = reactions.findIndex((r) => r.emoji === emoji)
              if (hasReacted) {
                // Removing reaction
                if (idx >= 0) {
                  const group = { ...reactions[idx] }
                  group.user_ids = group.user_ids.filter((id) => id !== userId)
                  group.count = group.user_ids.length
                  if (group.count <= 0) reactions.splice(idx, 1)
                  else reactions[idx] = group
                }
              } else {
                // Adding reaction
                if (idx >= 0) {
                  const group = { ...reactions[idx] }
                  if (!group.user_ids.includes(userId)) {
                    group.count += 1
                    group.user_ids = [...group.user_ids, userId]
                  }
                  reactions[idx] = group
                } else {
                  reactions.push({ emoji, count: 1, user_ids: [userId] })
                }
              }
              return { ...m, reactions }
            }),
          }
        },
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['messages', channelId], context.previous)
    },
  })
}

// --- Pins ---

export function usePins(channelId: string) {
  return useQuery({
    queryKey: ['pins', channelId],
    queryFn: () => messagingApi.listPins(channelId),
    enabled: !!channelId,
  })
}

/** Toggles pin status with optimistic UI. */
export function useTogglePin(channelId: string) {
  return useMutation({
    mutationFn: ({ messageId, isPinned }: { messageId: string; isPinned: boolean }) =>
      isPinned
        ? messagingApi.unpinMessage(channelId, messageId)
        : messagingApi.pinMessage(channelId, messageId),
    onMutate: async ({ messageId, isPinned }) => {
      await queryClient.cancelQueries({ queryKey: ['messages', channelId] })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(['messages', channelId])

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        ['messages', channelId],
        (old) => {
          if (!old) return old
          return {
            ...old,
            messages: (old.messages || []).map((m) =>
              m.id === messageId ? { ...m, is_pinned: !isPinned } : m,
            ),
          }
        },
      )
      return { previous }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pins', channelId] })
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['messages', channelId], context.previous)
    },
  })
}

// --- Read Receipts ---

export function useUnreadCounts() {
  return useQuery({
    queryKey: ['unreadCounts'],
    queryFn: () => messagingApi.getUnreadCounts(),
    // No polling — WS unreadCount event handles real-time updates
  })
}

export function useMarkRead(channelId: string) {
  return useMutation({
    mutationFn: (lastMessageId: string) => messagingApi.markRead(channelId, lastMessageId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['unreadCounts'] }),
  })
}

// --- Search ---

export function useSearch(channelId: string, query: string) {
  return useQuery({
    queryKey: ['search', channelId, query],
    queryFn: () => messagingApi.searchMessages(channelId, query),
    enabled: !!channelId && !!query && query.length >= 2,
  })
}

// --- Polls ---

export function usePoll(pollId: string) {
  return useQuery({
    queryKey: ['poll', pollId],
    queryFn: () => messagingApi.getPoll(pollId),
    enabled: !!pollId,
  })
}

export function useCreatePoll(channelId: string) {
  return useMutation({
    mutationFn: (data: { question: string; options: string[]; is_multi?: boolean; is_anonymous?: boolean }) =>
      messagingApi.createPoll(channelId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['messages', channelId] }),
  })
}

/** Votes on a poll option with optimistic count increment. */
export function useVotePoll(pollId: string) {
  return useMutation({
    mutationFn: (optionId: string) => messagingApi.votePoll(pollId, optionId),
    onMutate: async (optionId) => {
      await queryClient.cancelQueries({ queryKey: ['poll', pollId] })
      const previous = queryClient.getQueryData<Poll>(['poll', pollId])

      queryClient.setQueryData<Poll>(
        ['poll', pollId],
        (old) => {
          if (!old) return old
          return {
            ...old,
            total_votes: old.total_votes + 1,
            options: old.options.map((opt) =>
              opt.id === optionId ? { ...opt, vote_count: opt.vote_count + 1 } : opt,
            ),
          }
        },
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['poll', pollId], context.previous)
    },
  })
}

// --- Tasks ---

export function useTasks(channelId: string, status?: string) {
  return useQuery({
    queryKey: ['tasks', channelId, status],
    queryFn: () => messagingApi.listTasks(channelId, status),
    enabled: !!channelId,
  })
}

/** Creates a task with optimistic insert into task list. */
export function useCreateTask(channelId: string) {
  return useMutation({
    mutationFn: (data: { title: string; assignee_id?: string; due_date?: string }) =>
      messagingApi.createTask(channelId, data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: ['tasks', channelId] })
      const previous = queryClient.getQueryData<{ tasks: ChatTask[] }>(['tasks', channelId])
      const user = useAuthStore.getState().user

      const tempTask: ChatTask = {
        id: `temp-${Date.now()}`,
        message_id: '',
        channel_id: channelId,
        title: data.title,
        assignee_id: data.assignee_id || '',
        assignee_name: '',
        status: 'open',
        due_date: data.due_date,
        created_by: user?.id || '',
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }

      queryClient.setQueryData<{ tasks: ChatTask[] }>(
        ['tasks', channelId],
        (old) => old ? { ...old, tasks: [tempTask, ...old.tasks] } : old,
      )
      return { previous }
    },
    onSuccess: (serverTask) => {
      queryClient.setQueryData<{ tasks: ChatTask[] }>(
        ['tasks', channelId],
        (old) => {
          if (!old) return old
          return {
            ...old,
            tasks: old.tasks.map((t) => t.id.startsWith('temp-') ? serverTask : t),
          }
        },
      )
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['tasks', channelId], context.previous)
    },
  })
}

/** Updates a task with optimistic field change. */
export function useUpdateTask(channelId: string) {
  return useMutation({
    mutationFn: ({ taskId, ...data }: { taskId: string; status?: string; assignee_id?: string; title?: string; due_date?: string }) =>
      messagingApi.updateTask(taskId, data),
    onMutate: async ({ taskId, ...data }) => {
      await queryClient.cancelQueries({ queryKey: ['tasks', channelId] })
      const previous = queryClient.getQueryData<{ tasks: ChatTask[] }>(['tasks', channelId])

      queryClient.setQueryData<{ tasks: ChatTask[] }>(
        ['tasks', channelId],
        (old) => {
          if (!old) return old
          return {
            ...old,
            tasks: old.tasks.map((t) =>
              t.id === taskId ? { ...t, ...data, updated_at: new Date().toISOString() } : t,
            ),
          }
        },
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['tasks', channelId], context.previous)
    },
  })
}

// --- Members ---

export function useChannelMembers(channelId: string) {
  return useQuery({
    queryKey: ['channelMembers', channelId],
    queryFn: () => messagingApi.listMembers(channelId),
    enabled: !!channelId,
  })
}

/** Optimistic add member to channel — member appears instantly, rolls back on error. */
export function useAddChannelMember(channelId: string) {
  return useMutation({
    mutationFn: ({ ngacNodeId }: { ngacNodeId: string }) =>
      messagingApi.addMember(channelId, ngacNodeId),
    onMutate: async ({ ngacNodeId, username }: { ngacNodeId: string; username?: string }) => {
      await queryClient.cancelQueries({ queryKey: ['channelMembers', channelId] })
      const previous = queryClient.getQueryData(['channelMembers', channelId])
      queryClient.setQueryData(
        ['channelMembers', channelId],
        (old: { members: { user_id: string; username: string; ngac_node_id: string }[] } | undefined) => ({
          members: [
            ...(old?.members || []),
            { user_id: '', username: username || 'Adding...', ngac_node_id: ngacNodeId },
          ],
        }),
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['channelMembers', channelId], context.previous)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['channelMembers', channelId] })
      queryClient.invalidateQueries({ queryKey: ['channels'] })
    },
  })
}

/** Optimistic remove member from channel — member disappears instantly, rolls back on error. */
export function useRemoveChannelMember(channelId: string) {
  return useMutation({
    mutationFn: ({ nodeId }: { nodeId: string }) =>
      messagingApi.removeMember(channelId, nodeId),
    onMutate: async ({ nodeId }: { nodeId: string }) => {
      await queryClient.cancelQueries({ queryKey: ['channelMembers', channelId] })
      const previous = queryClient.getQueryData(['channelMembers', channelId])
      queryClient.setQueryData(
        ['channelMembers', channelId],
        (old: { members: { user_id: string; username: string; ngac_node_id: string }[] } | undefined) => ({
          members: (old?.members || []).filter((m) => m.ngac_node_id !== nodeId),
        }),
      )
      return { previous }
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(['channelMembers', channelId], context.previous)
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['channelMembers', channelId] })
      queryClient.invalidateQueries({ queryKey: ['channels'] })
    },
  })
}

