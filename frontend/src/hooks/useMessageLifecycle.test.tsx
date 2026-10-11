import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi, type Message } from '../api/messaging'
import { keys } from './keys'
import { queryClient } from '../lib/query-client'
import { useDeleteMessage, useEditMessage, useMarkUnread } from './useMessageLifecycle'

vi.mock('../api/messaging', async (orig) => {
  const real = await orig<typeof import('../api/messaging')>()
  return {
    ...real,
    messagingApi: { ...real.messagingApi, editMessage: vi.fn(), deleteMessage: vi.fn(), markMessageUnread: vi.fn() },
  }
})
const api = vi.mocked(messagingApi)

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const msg = (over: Partial<Message> = {}): Message =>
  ({ id: 'm1', channel_id: 'c1', sender_id: 'me', content: '<p>cũ</p>', content_format: 'html', ...over }) as Message
const list = () => queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.messages('c1'))!.messages

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
  queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ id: 'm0' }), msg()], has_more: false })
})

describe('useEditMessage: mentions', () => {
  const lan = '<span data-mention="u-lan">@Lan</span>'
  const duc = '<span data-mention="u-duc">@Đức</span>'

  it('sends the people mentioned in the edited text, ones added by the edit included', async () => {
    api.editMessage.mockResolvedValue(msg())
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: msg({ content: `<p>${lan}</p>`, mentions: ['u-lan'] }), content: `<p>${lan} ${duc}</p>` }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.editMessage).toHaveBeenCalledWith('m1', expect.objectContaining({ mentions: ['u-lan', 'u-duc'] }))
  })

  it('sends the same list when the edit adds nobody', async () => {
    api.editMessage.mockResolvedValue(msg())
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: msg({ content: `<p>${lan}</p>`, mentions: ['u-lan'] }), content: `<p>${lan} sửa chữ</p>` }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.editMessage).toHaveBeenCalledWith('m1', expect.objectContaining({ mentions: ['u-lan'] }))
  })

  it('sends an empty list when every mention was taken out', async () => {
    api.editMessage.mockResolvedValue(msg())
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: msg({ content: `<p>${lan}</p>`, mentions: ['u-lan'] }), content: '<p>không ai</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.editMessage).toHaveBeenCalledWith('m1', expect.objectContaining({ mentions: [] }))
  })

  it('keeps the mentions of a message written before rich text, whose names are plain text', async () => {
    api.editMessage.mockResolvedValue(msg())
    const legacy = msg({ content: 'chào @Lan', content_format: 'markdown', mentions: ['u-lan'] })
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ id: 'm0' }), legacy], has_more: false })
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: legacy, content: '<p>chào @Lan nhé</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.editMessage).toHaveBeenCalledWith('m1', expect.objectContaining({ mentions: ['u-lan'] }))
  })

  it('adds a mention picked while editing such a message to the ones it had', async () => {
    api.editMessage.mockResolvedValue(msg())
    const legacy = msg({ content: 'chào @Lan', content_format: 'markdown', mentions: ['u-lan'] })
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ id: 'm0' }), legacy], has_more: false })
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: legacy, content: `<p>chào @Lan ${duc}</p>` }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.editMessage).toHaveBeenCalledWith('m1', expect.objectContaining({ mentions: ['u-lan', 'u-duc'] }))
  })

  it('puts the old mentions back when the server refuses', async () => {
    api.editMessage.mockRejectedValue(new Error('403'))
    const before = msg({ content: `<p>${lan}</p>`, mentions: ['u-lan'] })
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ id: 'm0' }), before], has_more: false })
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: before, content: `<p>${lan} ${duc}</p>` }))
    await waitFor(() => expect(list()[1]!.mentions).toEqual(['u-lan', 'u-duc']))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(list()[1]!.mentions).toEqual(['u-lan'])
  })
})

describe('useEditMessage', () => {
  it('shows the new text at once, then keeps what the server stored (filtered, edit time)', async () => {
    api.editMessage.mockResolvedValue(msg({ content: '<p>mới</p>', edited_at: { seconds: 1760000000, nanos: 0 } }))
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: msg(), content: '<p>mới <script>x</script></p>' }))
    await waitFor(() => expect(list()[1]!.content).toContain('mới'))
    expect(api.editMessage).toHaveBeenCalledWith('m1', { content: '<p>mới <script>x</script></p>', content_format: 'html', mentions: [] })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(list()[1]).toMatchObject({ content: '<p>mới</p>', edited_at: { seconds: 1760000000, nanos: 0 } })
  })

  it('puts the old text back when the server refuses', async () => {
    api.editMessage.mockRejectedValue(new Error('403'))
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: msg(), content: '<p>mới</p>' }))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(list()[1]).toMatchObject({ content: '<p>cũ</p>' })
    expect(list()[1]!.edited_at).toBeUndefined()
  })

  it('edits a reply in its thread', async () => {
    const reply = msg({ id: 'r1', parent_message_id: 'root' })
    queryClient.setQueryData(keys.messaging.thread('root'), { messages: [msg({ id: 'root' }), reply] })
    api.editMessage.mockResolvedValue({ ...reply, content: '<p>mới</p>' })
    const { result } = renderHook(() => useEditMessage('c1'), { wrapper })
    act(() => result.current.mutate({ message: reply, content: '<p>mới</p>' }))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    const thread = queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.thread('root'))!.messages
    expect(thread[1]!.content).toBe('<p>mới</p>')
    expect(list().find((m) => m.id === 'r1')).toBeUndefined()
  })
})

describe('useDeleteMessage', () => {
  it('takes a message with no thread out of the stream once the server has deleted it', async () => {
    api.deleteMessage.mockResolvedValue({ status: 'ok' })
    const { result } = renderHook(() => useDeleteMessage('c1'), { wrapper })
    act(() => result.current.mutate(msg()))
    expect(list()).toHaveLength(2)
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.deleteMessage).toHaveBeenCalledWith('m1')
    expect(list().map((m) => m.id)).toEqual(['m0'])
  })

  it('leaves a root with replies as a placeholder', async () => {
    api.deleteMessage.mockResolvedValue({ status: 'ok' })
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ reply_count: 3 })], has_more: false })
    const { result } = renderHook(() => useDeleteMessage('c1'), { wrapper })
    act(() => result.current.mutate(msg({ reply_count: 3 })))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(list()[0]).toMatchObject({ deleted: true, content: '', reply_count: 3 })
  })

  it('counts one reply fewer on the root when a reply is deleted', async () => {
    api.deleteMessage.mockResolvedValue({ status: 'ok' })
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ id: 'root', reply_count: 3 })], has_more: false })
    const reply = msg({ id: 'r1', parent_message_id: 'root' })
    const { result } = renderHook(() => useDeleteMessage('c1'), { wrapper })
    act(() => result.current.mutate(reply))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(list()[0]!.reply_count).toBe(2)
  })

  // The server's own frame often arrives before the HTTP answer and has already
  // set the true count: the answer must not take one more away.
  it('does not count the reply twice when the live frame got there first', async () => {
    api.deleteMessage.mockResolvedValue({ status: 'ok' })
    queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [msg({ id: 'root', reply_count: 2 })], has_more: false })
    queryClient.setQueryData(keys.messaging.thread('root'), { messages: [msg({ id: 'root', reply_count: 2 }), msg({ id: 'r2', parent_message_id: 'root' })] })
    const reply = msg({ id: 'r1', parent_message_id: 'root' }) // already gone from the thread
    const { result } = renderHook(() => useDeleteMessage('c1'), { wrapper })
    act(() => result.current.mutate(reply))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(list()[0]!.reply_count).toBe(2)
  })

  it('changes nothing when the server refuses', async () => {
    api.deleteMessage.mockRejectedValue(new Error('403'))
    const { result } = renderHook(() => useDeleteMessage('c1'), { wrapper })
    act(() => result.current.mutate(msg()))
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(list()).toHaveLength(2)
  })
})

describe('useMarkUnread', () => {
  it('asks the server, then refreshes the unread counts and the root summaries', async () => {
    api.markMessageUnread.mockResolvedValue({ status: 'ok' })
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [] })
    const { result } = renderHook(() => useMarkUnread('c1'), { wrapper })
    act(() => result.current.mutate(msg({ sender_id: 'lan' })))
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(api.markMessageUnread).toHaveBeenCalledWith('m1')
    expect(queryClient.getQueryState(keys.messaging.unreadCounts())!.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(keys.messaging.messages('c1'))!.isInvalidated).toBe(true)
  })
})
