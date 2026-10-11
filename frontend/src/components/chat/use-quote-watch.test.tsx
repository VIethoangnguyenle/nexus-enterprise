import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider, useQuery } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Message } from '../../api/messaging'
import { keys } from '../../hooks/keys'
import { queryClient } from '../../lib/query-client'
import { quoteFromMessage, useDraftsStore } from '../../stores/drafts.store'
import { isQuoteGone, useQuoteWatch } from './use-quote-watch'

const at = (min: number) => new Date(2026, 9, 9, 10, min).toISOString()
const msg = (id: string, min: number, over: Partial<Message> = {}): Message =>
  ({ id, channel_id: 'c1', sender_id: 'u1', content: '<p>x</p>', created_at: at(min), ...over }) as Message
const quote = quoteFromMessage(msg('q1', 5))

describe('isQuoteGone', () => {
  it('is false while the quoted message is in the list', () => {
    expect(isQuoteGone(quote, [msg('q1', 5), msg('m2', 6)], false)).toBe(false)
  })
  it('is true when the list marks it deleted', () => {
    expect(isQuoteGone(quote, [msg('q1', 5, { deleted: true })], false)).toBe(true)
  })
  it('is true when it left a conversation list that reaches back past it', () => {
    expect(isQuoteGone(quote, [msg('m0', 1), msg('m2', 6)], false)).toBe(true)
  })
  it('says nothing when the conversation list starts after it (not loaded that far)', () => {
    expect(isQuoteGone(quote, [msg('m2', 6), msg('m3', 7)], false)).toBe(false)
  })
  it('says nothing when the list is empty or unloaded', () => {
    expect(isQuoteGone(quote, undefined, false)).toBe(false)
    expect(isQuoteGone(quote, [], true)).toBe(false)
  })
  it('is true when a reply is missing from its thread, whatever the dates', () => {
    expect(isQuoteGone(quote, [msg('root', 1), msg('r2', 9)], true)).toBe(true)
  })
})

const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>

describe('useQuoteWatch', () => {
  beforeEach(() => {
    useDraftsStore.getState().reset()
    queryClient.clear()
  })

  it('marks the pending quote deleted when its message disappears from the loaded list, and not before', async () => {
    useDraftsStore.getState().setQuote('c1', quote)
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg('m0', 1), msg('q1', 5)], has_more: false })
    renderHook(() => useQuoteWatch('c1', 'c1', useDraftsStore.getState().drafts['c1']?.quote), { wrapper })
    expect(useDraftsStore.getState().drafts['c1']?.quote?.deleted).toBeUndefined()
    act(() => {
      queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg('m0', 1), msg('m2', 6)], has_more: false })
    })
    await waitFor(() => expect(useDraftsStore.getState().drafts['c1']?.quote?.deleted).toBe(true))
  })

  it('watches the thread list for a thread composer', () => {
    const reply = quoteFromMessage(msg('r1', 5, { parent_message_id: 'root' }))
    useDraftsStore.getState().setQuote('thread:root', reply)
    queryClient.setQueryData(keys.messaging.thread('root'), { messages: [msg('root', 1), msg('r1', 5, { deleted: true })] })
    renderHook(() => useQuoteWatch('thread:root', 'c1', useDraftsStore.getState().drafts['thread:root']?.quote), { wrapper })
    expect(useDraftsStore.getState().drafts['thread:root']?.quote?.deleted).toBe(true)
  })

  it('never fetches', () => {
    useDraftsStore.getState().setQuote('c1', quote)
    renderHook(() => useQuoteWatch('c1', 'c1', quote), { wrapper })
    expect(queryClient.isFetching()).toBe(0)
    expect(useDraftsStore.getState().drafts['c1']?.quote?.deleted).toBeUndefined()
  })

  it('leaves the list its screen loads alone: a refresh still runs the screen\'s own fetch', async () => {
    useDraftsStore.getState().setQuote('c1', quote)
    const load = vi.fn(async () => ({ messages: [msg('q1', 5)], has_more: false }))
    const { result } = renderHook(
      () => {
        const list = useQuery({ queryKey: keys.messaging.messages('c1'), queryFn: load })
        useQuoteWatch('c1', 'c1', quote)
        return list
      },
      { wrapper },
    )
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    load.mockClear()

    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: keys.messaging.messages('c1') })
    })

    expect(load).toHaveBeenCalledTimes(1)
    expect(result.current.isError).toBe(false)
    expect(result.current.data?.messages).toHaveLength(1)
  })
})
