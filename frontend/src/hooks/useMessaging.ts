import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { messagingApi, type CreateChannelInput, type SendMessageInput, type Message, type Poll, type ChatTask, type ChannelMember } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { useAuthStore } from '../stores/auth.store'
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

// --- Messages ---

export const messagesQueryOptions = (channelId: string) =>
  queryOptions({ queryKey: keys.messaging.messages(channelId), queryFn: () => messagingApi.listMessages(channelId), enabled: !!channelId })

export const threadQueryOptions = (messageId: string) =>
  queryOptions({ queryKey: keys.messaging.thread(messageId), queryFn: () => messagingApi.getThread(messageId), enabled: !!messageId })

export function useMessages(channelId: string) { return useQuery(messagesQueryOptions(channelId)) }
export function useThread(messageId: string) { return useQuery(threadQueryOptions(messageId)) }

/** Sends a message with optimistic UI: message appears immediately, no refetch needed. */
export function useSendMessage(channelId: string) {
  return useMutation({
    meta: { silentError: true },
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
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))
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
        keys.messaging.messages(channelId),
        (old) => old ? { ...old, messages: [tempMsg as Message, ...(old.messages || [])] } : old,
      )

      return { previous }
    },

    // Replace temp message with server response (WS event will dedup via ID match)
    onSuccess: (serverMsg) => {
      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
        (old) => {
          if (!old) return old
          return {
            ...old,
            // Keep the optimistic row's key so the topic is not remounted (no
            // flash, no second insert animation) when the server copy lands.
            messages: (old.messages || []).some((m) => m.id === serverMsg.id)
              ? (old.messages || []).filter((m: Message & { _optimistic?: boolean }) => !m._optimistic)
              : (old.messages || []).map((m: Message & { _optimistic?: boolean }) =>
                  m._optimistic ? { ...serverMsg, _clientKey: m.id } : m,
                ),
          }
        },
      )
    },

    // Rollback on error
    onError: (_err, _vars, context) => {
      if (context?.previous) {
        queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
      }
    },
  })
}

type MessageList = { messages: Message[]; has_more: boolean }
type Thread = { messages: Message[] }
type Optimistic = Message & { _optimistic?: boolean }

/**
 * Replies inside a topic, with optimistic UI: the reply shows in the thread
 * at once and the topic's reply count moves with it. The WebSocket echo of
 * the same reply is de-duplicated by id in websocket.store.
 */
export function useSendReply(channelId: string, parentId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (content: string) =>
      messagingApi.sendMessage(channelId, { content, content_format: 'html', parent_message_id: parentId }),
    onMutate: async (content) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.thread(parentId) })
      const prevThread = queryClient.getQueryData<Thread>(keys.messaging.thread(parentId))
      const prevList = queryClient.getQueryData<MessageList>(keys.messaging.messages(channelId))
      const user = useAuthStore.getState().user
      const temp: Optimistic = {
        id: `temp-${Date.now()}`,
        channel_id: channelId,
        sender_id: user?.id || '',
        sender_name: user?.username || '',
        content,
        content_format: 'html',
        parent_message_id: parentId,
        created_at: new Date().toISOString(),
        _optimistic: true,
      }
      queryClient.setQueryData<Thread>(keys.messaging.thread(parentId), (old) =>
        old ? { ...old, messages: [...(old.messages || []), temp] } : old,
      )
      queryClient.setQueryData<MessageList>(keys.messaging.messages(channelId), (old) =>
        old
          ? { ...old, messages: old.messages.map((m) => (m.id === parentId ? { ...m, reply_count: (m.reply_count || 0) + 1 } : m)) }
          : old,
      )
      return { prevThread, prevList }
    },
    onSuccess: (serverMsg) => {
      queryClient.setQueryData<Thread>(keys.messaging.thread(parentId), (old) => {
        if (!old) return old
        const already = old.messages.some((m) => m.id === serverMsg.id)
        const rest = old.messages.filter((m: Optimistic) => !m._optimistic)
        return { ...old, messages: already ? rest : [...rest, serverMsg] }
      })
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prevThread) queryClient.setQueryData(keys.messaging.thread(parentId), ctx.prevThread)
      if (ctx?.prevList) queryClient.setQueryData(keys.messaging.messages(channelId), ctx.prevList)
    },
  })
}

// --- Reactions ---

export function useReactions(messageId: string) {
  return useQuery({
    queryKey: keys.messaging.reactions(messageId),
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
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))
      const userId = useAuthStore.getState().user?.id || ''

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
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
      if (context?.previous) queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
    },
  })
}

// --- Pins ---

export function usePins(channelId: string) {
  return useQuery({
    queryKey: keys.messaging.pins(channelId),
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
      await queryClient.cancelQueries({ queryKey: keys.messaging.messages(channelId) })
      const previous = queryClient.getQueryData<{ messages: Message[]; has_more: boolean }>(keys.messaging.messages(channelId))

      queryClient.setQueryData<{ messages: Message[]; has_more: boolean }>(
        keys.messaging.messages(channelId),
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
      queryClient.invalidateQueries({ queryKey: keys.messaging.pins(channelId) })
    },
    onError: (_err, _vars, context) => {
      if (context?.previous) queryClient.setQueryData(keys.messaging.messages(channelId), context.previous)
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

// --- Polls ---

export function usePoll(pollId: string) {
  return useQuery({
    queryKey: keys.messaging.poll(pollId),
    queryFn: () => messagingApi.getPoll(pollId),
    enabled: !!pollId,
  })
}

export function useCreatePoll(channelId: string) {
  return useMutation({
    mutationFn: (data: { question: string; options: string[]; is_multi?: boolean; is_anonymous?: boolean }) =>
      messagingApi.createPoll(channelId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.messaging.messages(channelId) }),
  })
}

/** Votes on a poll option with optimistic count increment. */
export function useVotePoll(pollId: string) {
  return useMutation({
    mutationFn: (optionId: string) => messagingApi.votePoll(pollId, optionId),
    onMutate: async (optionId) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.poll(pollId) })
      const previous = queryClient.getQueryData<Poll>(keys.messaging.poll(pollId))

      queryClient.setQueryData<Poll>(
        keys.messaging.poll(pollId),
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
      if (context?.previous) queryClient.setQueryData(keys.messaging.poll(pollId), context.previous)
    },
  })
}

// --- Tasks ---

export function useTasks(channelId: string, status?: string) {
  return useQuery({
    queryKey: keys.messaging.tasks(channelId, status),
    queryFn: () => messagingApi.listTasks(channelId, status),
    enabled: !!channelId,
  })
}

/** Creates a task with optimistic insert into task list. */
export function useCreateTask(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: { title: string; assignee_id?: string; due_date?: string }) =>
      messagingApi.createTask(channelId, data),
    onMutate: async (data) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.tasksOf(channelId) })
      // The list is cached per status filter, so snapshot and patch every one.
      const previous = queryClient.getQueriesData<{ tasks: ChatTask[] }>({ queryKey: keys.messaging.tasksOf(channelId) })
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

      queryClient.setQueriesData<{ tasks: ChatTask[] }>(
        { queryKey: keys.messaging.tasksOf(channelId) },
        (old) => old ? { ...old, tasks: [tempTask, ...old.tasks] } : old,
      )
      return { previous }
    },
    onSuccess: (serverTask) => {
      queryClient.setQueriesData<{ tasks: ChatTask[] }>(
        { queryKey: keys.messaging.tasksOf(channelId) },
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
      context?.previous.forEach(([key, data]) => queryClient.setQueryData(key, data))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.messaging.tasksOf(channelId) }),
  })
}

/** Updates a task with optimistic field change. */
export function useUpdateTask(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ taskId, ...data }: { taskId: string; status?: string; assignee_id?: string; title?: string; due_date?: string }) =>
      messagingApi.updateTask(taskId, data),
    onMutate: async ({ taskId, ...data }) => {
      await queryClient.cancelQueries({ queryKey: keys.messaging.tasksOf(channelId) })
      // The list is cached per status filter, so snapshot and patch every one.
      const previous = queryClient.getQueriesData<{ tasks: ChatTask[] }>({ queryKey: keys.messaging.tasksOf(channelId) })

      queryClient.setQueriesData<{ tasks: ChatTask[] }>(
        { queryKey: keys.messaging.tasksOf(channelId) },
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
      context?.previous.forEach(([key, data]) => queryClient.setQueryData(key, data))
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: keys.messaging.tasksOf(channelId) }),
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

