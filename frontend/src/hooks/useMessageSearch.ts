import { useEffect, useState } from 'react'
import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { messagingApi, type Message, type MessageSearchFilters } from '../api/messaging'
import { timestampToRfc3339 } from '../lib/stream-model'
import { keys } from './keys'

export const SEARCH_PAGE = 25
export const SEARCH_MIN_CHARS = 2
const DEBOUNCE_MS = 250

/** The value after it has stayed the same for `ms`; `''` until then. With no wait it is the value itself. */
function useSettled(value: string, ms: number): string {
  const [settled, setSettled] = useState('')
  useEffect(() => {
    if (ms <= 0) return
    const id = setTimeout(() => setSettled(value), ms)
    return () => clearTimeout(id)
  }, [value, ms])
  return ms <= 0 ? value : settled
}

interface Options {
  /** False while something else (a chip) hides messages, or the box is not asking. */
  enabled?: boolean
  /** Wait for a pause in typing (the suggestions); the results page asks as soon as it is told. */
  debounce?: boolean
  /** Messages a page holds: a few for the suggestions, a full page for the results. */
  limit?: number
}

/**
 * Messages matching `query` and `filters` in every conversation the person reads (Tìm trong chat). Asks the
 * server only once the words reach two characters, after a pause in typing when `debounce` is on, a page at a
 * time. `waiting` is true from the first keystroke until the answer for the current words is in.
 */
export function useMessageSearch(query: string, filters: MessageSearchFilters = {}, opts: Options = {}) {
  const { enabled = true, debounce = true, limit = SEARCH_PAGE } = opts
  const typed = query.trim()
  const term = useSettled(typed, debounce ? DEBOUNCE_MS : 0)
  const active = enabled && typed.length >= SEARCH_MIN_CHARS
  const q = useInfiniteQuery({
    queryKey: keys.messaging.searchAll(term, filters, limit),
    initialPageParam: undefined as { before: string; beforeId: string } | undefined,
    queryFn: ({ pageParam }) =>
      messagingApi.searchAll({ ...filters, q: term, limit, before: pageParam?.before, beforeId: pageParam?.beforeId }),
    getNextPageParam: (page) => {
      const last = page.messages[page.messages.length - 1]
      if (!page.has_more || !last) return undefined
      const before = timestampToRfc3339(last.created_at)
      return before ? { before, beforeId: last.id } : undefined
    },
    enabled: enabled && term.length >= SEARCH_MIN_CHARS,
    // The results page shows the old list dimmed while a new filter is answered; the suggestions would rather show a skeleton.
    placeholderData: debounce ? undefined : keepPreviousData,
  })
  const settled = active && term === typed
  const messages: Message[] = settled ? (q.data?.pages.flatMap((p) => p.messages) ?? []) : []

  return {
    /** The words are long enough to search. */
    active,
    /** Typing has not paused yet, or the answer for these words is on its way. */
    waiting: active && (term !== typed || q.isPending) && !q.isError,
    /** A new answer is being fetched while the previous one is still shown. */
    refreshing: settled && q.isPlaceholderData,
    isError: settled && q.isError,
    messages,
    hasMore: !!q.hasNextPage,
    loadingMore: q.isFetchingNextPage,
    loadMore: () => q.fetchNextPage(),
    retry: () => void q.refetch(),
  }
}
