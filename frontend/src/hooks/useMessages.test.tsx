import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { messagingApi } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { useDraftsStore } from '../stores/drafts.store'
import { useSendCardMessage, useSendMessage, useSendReply } from './useMessages'
import { keys } from './keys'

vi.mock('../api/messaging', async (orig) => {
  const real = await orig<typeof import('../api/messaging')>()
  return { ...real, messagingApi: { ...real.messagingApi, sendMessage: vi.fn() } }
})
const send = vi.mocked(messagingApi.sendMessage)
const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const stored = { id: 'm1', channel_id: 'c1', sender_id: 'me', content: '<p>x</p>' }

beforeEach(() => {
  useDraftsStore.getState().reset()
  queryClient.clear()
  vi.clearAllMocks()
  send.mockResolvedValue(stored)
})

describe('sending rich text', () => {
  it('sends html with the people mentioned', async () => {
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p><span data-mention="u2">@Lan</span></p>', mentions: ['u2'] }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send).toHaveBeenCalledWith('c1', expect.objectContaining({ content_format: 'html', mentions: ['u2'] }))
  })

  it('sends a reply as html with its mentions', async () => {
    const { result } = renderHook(() => useSendReply('c1', 'root'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>', mentions: ['u3'] }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send).toHaveBeenCalledWith('c1', { content: '<p>ok</p>', content_format: 'html', parent_message_id: 'root', mentions: ['u3'] })
  })

  it('leaves mentions out when nobody is mentioned', async () => {
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>x</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send.mock.calls[0]![1]).not.toHaveProperty('mentions')
  })
})

const quote = (id = 'q1') => ({ id, sender_id: 'u2', sender_name: 'Lan', text: 'Chào cả nhà' })

describe('sending with a pending quote', () => {
  it('sends the quote of this conversation and clears it once the message is stored', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    useDraftsStore.getState().setQuote('c2', quote('other'))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send).toHaveBeenCalledWith('c1', expect.objectContaining({ quoted_message_id: 'q1' }))
    expect(useDraftsStore.getState().drafts['c1']?.quote).toBeUndefined()
    expect(useDraftsStore.getState().drafts['c2']?.quote?.id).toBe('other')
  })

  it('shows the quote on the optimistic message', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [], has_more: false })
    send.mockReturnValue(new Promise(() => {}))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => {
      const list = queryClient.getQueryData<{ messages: { quoted?: { id: string; text?: string } }[] }>(keys.messaging.messages('c1'))
      expect(list?.messages[0]?.quoted).toMatchObject({ id: 'q1', text: 'Chào cả nhà' })
    })
  })

  it('sends a reply with the thread composer\'s quote only', async () => {
    useDraftsStore.getState().setQuote('thread:root', quote('r1'))
    useDraftsStore.getState().setQuote('c1', quote('main'))
    const { result } = renderHook(() => useSendReply('c1', 'root'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send).toHaveBeenCalledWith('c1', expect.objectContaining({ quoted_message_id: 'r1', parent_message_id: 'root' }))
    expect(useDraftsStore.getState().drafts['c1']?.quote?.id).toBe('main')
  })

  it('sends without a quote that was deleted while pending', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    useDraftsStore.getState().markQuoteDeleted('c1', 'q1')
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send.mock.calls[0]![1]).not.toHaveProperty('quoted_message_id')
  })

  it('a card link never carries the quote', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '', linkedEntity: { type: 'document', id: 'd1' } }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(send.mock.calls[0]![1]).not.toHaveProperty('quoted_message_id')
    expect(useDraftsStore.getState().drafts['c1']?.quote?.id).toBe('q1')
  })

  it('shows the quote as deleted only when the server says the quoted message is unavailable', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    send.mockRejectedValue(new ApiError('bad', 400, { message: 'bad', reason: 'quoted_message_unavailable' }))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(useDraftsStore.getState().drafts['c1']?.quote).toMatchObject({ id: 'q1', deleted: true })
  })

  it('keeps a live quote when the 400 is about something else (an attachment, an empty message)', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    send.mockRejectedValue(new ApiError('bad attachment', 400, { message: 'bad attachment' }))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    const q = useDraftsStore.getState().drafts['c1']?.quote
    expect(q?.id).toBe('q1')
    expect(q?.deleted).toBeUndefined()
  })

  it('does the same for a reply', async () => {
    useDraftsStore.getState().setQuote('thread:root', quote('r1'))
    send.mockRejectedValue(new ApiError('bad', 400, { message: 'bad', reason: 'quoted_message_unavailable' }))
    const { result } = renderHook(() => useSendReply('c1', 'root'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(useDraftsStore.getState().drafts['thread:root']?.quote).toMatchObject({ id: 'r1', deleted: true })
  })

  it('takes the quote off the strip while the send is pending, so a second message does not repeat it', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    send.mockReturnValueOnce(new Promise(() => {}))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>một</p>' }))
    await waitFor(() => expect(send).toHaveBeenCalledTimes(1))
    expect(useDraftsStore.getState().drafts['c1']?.quote).toBeUndefined()
    act(() => result.current.mutate({ content: '<p>hai</p>' }))
    await waitFor(() => expect(send).toHaveBeenCalledTimes(2))
    expect(send.mock.calls[0]![1]).toHaveProperty('quoted_message_id', 'q1')
    expect(send.mock.calls[1]![1]).not.toHaveProperty('quoted_message_id')
  })

  it('does not bring a failed quote back over one chosen meanwhile', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    let fail!: (e: Error) => void
    send.mockReturnValueOnce(new Promise((_, rej) => { fail = rej }))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(send).toHaveBeenCalled())
    act(() => useDraftsStore.getState().setQuote('c1', quote('newer')))
    fail(new Error('offline'))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(useDraftsStore.getState().drafts['c1']?.quote?.id).toBe('newer')
  })

  it('leaves the quote alone when the failure is not about it (network)', async () => {
    useDraftsStore.getState().setQuote('c1', quote())
    send.mockRejectedValue(new Error('offline'))
    const { result } = renderHook(() => useSendMessage('c1'), { wrapper })
    act(() => result.current.mutate({ content: '<p>ok</p>' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(useDraftsStore.getState().drafts['c1']?.quote?.deleted).toBeUndefined()
  })
})

describe('sending a card as a message', () => {
  it('goes to the conversation without a parent', async () => {
    const { result } = renderHook(() => useSendCardMessage('c1'), { wrapper })
    await act(() => result.current.send({ type: 'document', id: 'd1' }))
    expect(send).toHaveBeenCalledWith('c1', expect.objectContaining({ linked_entity_type: 'document', linked_entity_id: 'd1' }))
    expect(send.mock.calls[0]![1]).not.toHaveProperty('parent_message_id')
  })

  it('goes into the thread as a reply when a root is given', async () => {
    const { result } = renderHook(() => useSendCardMessage('c1', 'root'), { wrapper })
    await act(() => result.current.send({ type: 'approval', id: 'a1' }))
    expect(send).toHaveBeenCalledWith('c1', expect.objectContaining({ parent_message_id: 'root', linked_entity_type: 'approval', linked_entity_id: 'a1' }))
  })
})
