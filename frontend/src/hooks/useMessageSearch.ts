import { useEffect, useState } from 'react'
import { useInfiniteQuery } from '@tanstack/react-query'
import { messagingApi, type Message } from '../api/messaging'
import { timestampToRfc3339 } from '../lib/stream-model'
import { keys } from './keys'

export const SEARCH_PAGE = 25
export const SEARCH_MIN_CHARS = 2
const DEBOUNCE_MS = 250

/** The value after it has stayed the same for `ms`; `''` until then. */
function useSettled(value: string, ms: number): string {
  const [settled, setSettled] = useState('')
  useEffect(() => {
    const id = setTimeout(() => setSettled(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return settled
}

/**
 * Messages matching `query` in every conversation the person reads (Tìm trong chat). Asks the
 * server only once typing pauses and the words reach two characters, a page of 25 at a time.
 * `enabled` is false while a filter hides messages. `waiting` is true from the first keystroke until the answer for the current words is in.
 */
export function useMessageSearch(query: string, hasFile: boolean, enabled = true) {
  const typed = query.trim()
  const term = useSettled(typed, DEBOUNCE_MS)
  const active = enabled && typed.length >= SEARCH_MIN_CHARS
  const q = useInfiniteQuery({
    queryKey: keys.messaging.searchAll(term, hasFile),
    initialPageParam: undefined as { before: string; beforeId: string } | undefined,
    queryFn: ({ pageParam }) =>
      messagingApi.searchAll({ q: term, hasFile, limit: SEARCH_PAGE, before: pageParam?.before, beforeId: pageParam?.beforeId }),
    getNextPageParam: (page) => {
      const last = page.messages[page.messages.length - 1]
      if (!page.has_more || !last) return undefined
      const before = timestampToRfc3339(last.created_at)
      return before ? { before, beforeId: last.id } : undefined
    },
    enabled: enabled && term.length >= SEARCH_MIN_CHARS,
  })
  const messages: Message[] = active && term === typed ? (q.data?.pages.flatMap((p) => p.messages) ?? []) : []

  return {
    /** The words are long enough to search. */
    active,
    /** Typing has not paused yet, or the answer for these words is on its way. */
    waiting: active && (term !== typed || q.isPending) && !q.isError,
    isError: active && term === typed && q.isError,
    messages,
    hasMore: !!q.hasNextPage,
    loadingMore: q.isFetchingNextPage,
    loadMore: () => q.fetchNextPage(),
    retry: () => void q.refetch(),
  }
}
