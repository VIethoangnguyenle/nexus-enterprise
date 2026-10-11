import { useCallback, useRef, useState } from 'react'
import { useInfiniteQuery, useMutation } from '@tanstack/react-query'
import { messagingApi, type Message, type ThreadFilter, type ThreadState } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { patchRoot, applyThreadStateAnswer } from '../lib/thread-cache'
import { timestampToRfc3339 } from '../lib/stream-model'
import { keys } from './keys'

type MessageList = { messages: Message[]; has_more: boolean }

const THREADS_PAGE = 20

/** Follow or unfollow a thread. The toggle moves at once; the server's answer settles it. */
export function useFollowThread(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ rootId, follow }: { rootId: string; follow: boolean }): Promise<ThreadState> =>
      follow ? messagingApi.followThread(rootId) : messagingApi.unfollowThread(rootId),
    onMutate: ({ rootId, follow }) => {
      const before = rootState(channelId, rootId)
      patchRoot(channelId, rootId, (m) => ({ following: follow, thread_unread_count: follow ? m.thread_unread_count : 0 }))
      return { before }
    },
    onSuccess: (st) => applyThreadStateAnswer(st),
    onError: (_err, { rootId }, ctx) => {
      if (ctx?.before) patchRoot(channelId, rootId, () => ctx.before!)
    },
  })
}

/** Moves the caller's read position in a thread to now. Failures are not the reader's to hear about. */
export function useMarkThreadRead(channelId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (rootId: string) => messagingApi.markThreadRead(rootId),
    onMutate: (rootId) => {
      patchRoot(channelId, rootId, (m) => ((m.thread_unread_count ?? 0) > 0 ? { thread_unread_count: 0 } : null))
    },
    onSuccess: (st) => applyThreadStateAnswer(st),
  })
}

function rootState(channelId: string, rootId: string): Pick<Message, 'following' | 'thread_unread_count'> | undefined {
  const root = queryClient
    .getQueryData<MessageList>(keys.messaging.messages(channelId))
    ?.messages.find((m) => m.id === rootId)
  return root ? { following: root.following, thread_unread_count: root.thread_unread_count } : undefined
}

/** The channel's threads for one filter, newest activity first, a page at a time. */
export function useThreadsList(channelId: string, filter: ThreadFilter) {
  return useInfiniteQuery({
    queryKey: keys.messaging.threadsList(channelId, filter),
    initialPageParam: undefined as { before: string; beforeId: string } | undefined,
    queryFn: ({ pageParam }) =>
      messagingApi.listThreads(channelId, {
        filter, limit: THREADS_PAGE, before: pageParam?.before, beforeId: pageParam?.beforeId,
      }),
    getNextPageParam: (page) => {
      const last = page.messages[page.messages.length - 1]
      if (!page.has_more || !last) return undefined
      const before = timestampToRfc3339(last.last_reply_at)
      return before ? { before, beforeId: last.id } : undefined
    },
    enabled: !!channelId,
  })
}

/**
 * Older pages of a channel's stream (`before=` the oldest message held),
 * appended to the cached list. The cache is newest first, so the oldest
 * persisted message is the last one.
 */
export function useLoadOlderMessages(channelId: string) {
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)
  const busy = useRef(false)

  const loadOlder = useCallback(async () => {
    if (busy.current) return
    const cached = queryClient.getQueryData<MessageList>(keys.messaging.messages(channelId))
    if (!cached?.has_more) return
    const oldest = [...cached.messages].reverse().find((m) => !m.id.startsWith('temp-'))
    const before = oldest ? timestampToRfc3339(oldest.created_at) : ''
    if (!before) return
    busy.current = true
    setLoading(true)
    setFailed(false)
    try {
      const page = await messagingApi.listMessages(channelId, before)
      queryClient.setQueryData<MessageList>(keys.messaging.messages(channelId), (old) => {
        if (!old) return old
        const have = new Set(old.messages.map((m) => m.id))
        return { ...old, messages: [...old.messages, ...(page.messages ?? []).filter((m) => !have.has(m.id))], has_more: !!page.has_more }
      })
    } catch {
      setFailed(true)
    } finally {
      busy.current = false
      setLoading(false)
    }
  }, [channelId])

  return { loadOlder, loading, failed }
}
