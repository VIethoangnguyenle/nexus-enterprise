import { useEffect } from 'react'
import { skipToken, useQuery } from '@tanstack/react-query'
import type { Message } from '../../api/messaging'
import { keys } from '../../hooks/keys'
import { toMillis } from '../../lib/format'
import { useDraftsStore, type PendingQuote } from '../../stores/drafts.store'

/**
 * Whether the message a composer is quoting has been deleted, judged from the
 * messages already loaded. Gone means it is marked deleted, or it is missing from
 * a list that should hold it: a thread holds every reply, and the conversation's
 * list holds everything newer than its oldest loaded message. A list that has not
 * loaded, or does not reach back that far, says nothing.
 */
export function isQuoteGone(quote: PendingQuote, messages: Message[] | undefined, inThread: boolean): boolean {
  if (!messages || messages.length === 0) return false
  const found = messages.find((m) => m.id === quote.id)
  if (found) return !!found.deleted
  if (inThread) return true
  const since = toMillis(quote.created_at)
  if (!since) return false
  const oldest = Math.min(...messages.map((m) => toMillis(m.created_at)).filter((t) => t > 0))
  return Number.isFinite(oldest) && oldest <= since
}

/**
 * Keeps an eye on the pending quote of one composer: when its message is
 * deleted (here or by someone else, through the realtime frame that patches the
 * same caches) the quote is marked deleted, and the next message goes without it.
 */
export function useQuoteWatch(scope: string, channelId: string, quote: PendingQuote | undefined) {
  const inThread = scope.startsWith('thread:')
  const key = inThread ? keys.messaging.thread(scope.slice('thread:'.length)) : keys.messaging.messages(channelId)
  // Read-only: the lists are loaded by their screens; this never fetches.
  const { data } = useQuery<{ messages: Message[] }>({ queryKey: key, queryFn: skipToken })
  const gone = !!quote && !quote.deleted && isQuoteGone(quote, data?.messages, inThread)
  const id = quote?.id
  useEffect(() => {
    if (gone && id) useDraftsStore.getState().markQuoteDeleted(scope, id)
  }, [gone, id, scope])
}
