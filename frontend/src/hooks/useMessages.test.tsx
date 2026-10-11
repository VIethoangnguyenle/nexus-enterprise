import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../api/messaging'
import { queryClient } from '../lib/query-client'
import { useSendMessage, useSendReply } from './useMessages'

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
