import { renderHook, waitFor, act } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { pollApi } from '../api/chat-cards'
import { queryClient } from '../lib/query-client'
import { invalidatePollOnVote, usePoll, useVotePoll, useClosePoll } from './usePolls'
import { chatCardKeys } from './keys/chat-cards'
import { OPT, POLL_ID, pollUnvoted, pollVoted } from '../test/chat-cards-fixtures'

vi.mock('../api/chat-cards', () => ({
  pollApi: { get: vi.fn(), vote: vi.fn(), unvote: vi.fn(), close: vi.fn(), create: vi.fn() },
}))

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
})

describe('usePolls', () => {
  it('keys a poll as [messaging, poll, id]', () => {
    expect(chatCardKeys.poll(POLL_ID)).toEqual(['messaging', 'poll', POLL_ID])
  })

  it('reads the poll as the viewer sees it', async () => {
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted())
    const { result } = renderHook(() => usePoll(POLL_ID), { wrapper })
    await waitFor(() => expect(result.current.data?.results_visible).toBe(false))
  })

  it('replaces the cached poll with the answer to a vote', async () => {
    queryClient.setQueryData(chatCardKeys.poll(POLL_ID), pollUnvoted())
    vi.mocked(pollApi.vote).mockResolvedValue(pollVoted([OPT.a]))
    const { result } = renderHook(() => useVotePoll(POLL_ID), { wrapper })
    await act(() => result.current.mutateAsync(OPT.a))
    expect(pollApi.vote).toHaveBeenCalledWith(POLL_ID, OPT.a)
    expect(queryClient.getQueryData(chatCardKeys.poll(POLL_ID))).toMatchObject({ results_visible: true, my_option_ids: [OPT.a] })
  })

  it('refetches the poll when a vote is refused', async () => {
    queryClient.setQueryData(chatCardKeys.poll(POLL_ID), pollUnvoted())
    vi.mocked(pollApi.vote).mockRejectedValue(new Error('x'))
    vi.mocked(pollApi.get).mockResolvedValue(pollUnvoted({ closed: true }))
    // The card is on screen, so the poll has an observer to refetch.
    const { result } = renderHook(() => ({ poll: usePoll(POLL_ID), vote: useVotePoll(POLL_ID) }), { wrapper })
    await waitFor(() => expect(result.current.poll.data).toBeDefined())
    vi.mocked(pollApi.get).mockClear()
    await act(() => result.current.vote.mutateAsync(OPT.a).catch(() => {}))
    await waitFor(() => expect(pollApi.get).toHaveBeenCalledTimes(1))
  })

  it('keeps the closed poll the server returns', async () => {
    vi.mocked(pollApi.close).mockResolvedValue(pollVoted([], { closed: true }))
    const { result } = renderHook(() => useClosePoll(POLL_ID), { wrapper })
    await act(() => result.current.mutateAsync())
    expect(queryClient.getQueryData(chatCardKeys.poll(POLL_ID))).toMatchObject({ closed: true })
  })
})

describe('invalidatePollOnVote', () => {
  it('invalidates exactly the poll the frame names', async () => {
    const client = new QueryClient()
    const spy = vi.spyOn(client, 'invalidateQueries')
    await invalidatePollOnVote(client, { pollId: POLL_ID })
    expect(spy).toHaveBeenCalledWith({ queryKey: ['messaging', 'poll', POLL_ID] })
  })

  it('ignores a frame that names no poll', async () => {
    const client = new QueryClient()
    const spy = vi.spyOn(client, 'invalidateQueries')
    await invalidatePollOnVote(client, { pollId: '' })
    await invalidatePollOnVote(client, undefined)
    expect(spy).not.toHaveBeenCalled()
  })
})
