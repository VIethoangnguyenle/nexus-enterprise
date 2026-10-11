import { useMutation, useQuery, type QueryClient } from '@tanstack/react-query'
import { pollApi, type CreatePollInput, type PollView } from '../api/chat-cards'
import { queryClient } from '../lib/query-client'
import { statusOf } from '../lib/errors'
import { keys } from './keys'
import { chatCardKeys } from './keys/chat-cards'

/** One poll as the signed-in user sees it. A 403/404 is an answer, not a hiccup. */
export function usePoll(pollId: string) {
  return useQuery({
    queryKey: chatCardKeys.poll(pollId),
    queryFn: () => pollApi.get(pollId),
    enabled: !!pollId,
    retry: (count, err) => ![403, 404].includes(statusOf(err) ?? 0) && count < 1,
  })
}

/** Every write answers with the poll as the caller now sees it, so it replaces the cached copy. */
const keepAnswer = (pollId: string) => (poll: PollView) => queryClient.setQueryData(chatCardKeys.poll(pollId), poll)

export function useVotePoll(pollId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (optionId: string) => pollApi.vote(pollId, optionId),
    onSuccess: keepAnswer(pollId),
    // A refused vote (closed meanwhile) must not leave a stale card behind.
    onError: () => void queryClient.invalidateQueries({ queryKey: chatCardKeys.poll(pollId) }),
  })
}

/** Withdraws the given options one after another; the last answer is the poll as it now stands. */
export function useUnvotePoll(pollId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: async (optionIds: string[]) => {
      let last: PollView | undefined
      for (const id of optionIds) last = await pollApi.unvote(pollId, id)
      return last
    },
    onSuccess: (poll) => {
      if (poll) keepAnswer(pollId)(poll)
    },
    onError: () => void queryClient.invalidateQueries({ queryKey: chatCardKeys.poll(pollId) }),
  })
}

export function useClosePoll(pollId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => pollApi.close(pollId),
    onSuccess: keepAnswer(pollId),
  })
}

/**
 * The poll arrives in the conversation as a message (a reply when `parentMessageId`
 * names a thread root); the list is refetched in case the frame is late.
 */
export function useCreatePoll(channelId: string, parentMessageId?: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (input: CreatePollInput) =>
      pollApi.create(channelId, parentMessageId ? { ...input, parent_message_id: parentMessageId } : input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: keys.messaging.messages(channelId) })
      if (parentMessageId) void queryClient.invalidateQueries({ queryKey: keys.messaging.thread(parentMessageId) })
    },
  })
}

/**
 * A `pollVote` frame names the poll and carries no counts: each viewer refetches
 * it under their own view. Frames without a poll are ignored.
 */
export function invalidatePollOnVote(client: QueryClient, frame: { pollId?: string } | undefined | null) {
  if (!frame?.pollId) return Promise.resolve()
  return client.invalidateQueries({ queryKey: chatCardKeys.poll(frame.pollId) })
}
