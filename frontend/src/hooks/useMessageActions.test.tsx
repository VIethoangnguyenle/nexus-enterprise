import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { messagingApi, type Message } from '../api/messaging'
import { keys } from './keys'
import { queryClient } from '../lib/query-client'
import { toast } from '../components/primitives'
import { useMessageActions } from './useMessageActions'

vi.mock('../api/messaging', async (orig) => {
  const real = await orig<typeof import('../api/messaging')>()
  return {
    ...real,
    messagingApi: { ...real.messagingApi, editMessage: vi.fn(), deleteMessage: vi.fn(), markMessageUnread: vi.fn() },
  }
})
let manager = false
vi.mock('./useCanManageChannel', () => ({ useCanManageChannel: () => manager }))
vi.mock('../components/primitives', async (orig) => {
  const real = await orig<typeof import('../components/primitives')>()
  return { ...real, toast: Object.assign(vi.fn(), { error: vi.fn() }) }
})
const api = vi.mocked(messagingApi)

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
)
const mine = (over: Partial<Message> = {}): Message =>
  ({ id: 'm1', channel_id: 'c1', sender_id: 'me', content: '<p>cũ</p>', content_format: 'html', ...over }) as Message
const list = () => queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.messages('c1'))!.messages
const setup = () => renderHook(() => useMessageActions('c1', 'me'), { wrapper })

beforeEach(() => {
  queryClient.clear()
  vi.clearAllMocks()
  manager = false
  queryClient.setQueryData(keys.messaging.messages('c1'), { messages: [mine({ id: 'm0' }), mine()], has_more: false })
})

describe('useMessageActions: abilities', () => {
  it('reads the rules with who is signed in and whether they manage the channel', () => {
    const { result } = setup()
    expect(result.current.abilities(mine({ sender_id: 'lan' }))).toEqual({ edit: false, remove: false, markUnread: true })
    manager = true
    const again = setup()
    expect(again.result.current.abilities(mine({ sender_id: 'lan' })).remove).toBe(true)
  })
})

describe('useMessageActions: editing', () => {
  it('opens a message for editing, and refuses one that is not editable', () => {
    const { result } = setup()
    act(() => result.current.startEdit(mine({ sender_id: 'lan' })))
    expect(result.current.editing).toBeNull()
    act(() => result.current.startEdit(mine()))
    expect(result.current.editing?.message.id).toBe('m1')
    act(() => result.current.cancelEdit())
    expect(result.current.editing).toBeNull()
  })

  it('saves the new text and closes the editor', async () => {
    api.editMessage.mockResolvedValue(mine({ content: '<p>mới</p>' }))
    const { result } = setup()
    act(() => result.current.startEdit(mine()))
    act(() => result.current.saveEdit('<p>mới</p>'))
    expect(result.current.editing).toBeNull()
    await waitFor(() => expect(api.editMessage).toHaveBeenCalledWith('m1', { content: '<p>mới</p>', content_format: 'html', mentions: [] }))
  })

  it('closes without a request when nothing changed', () => {
    const { result } = setup()
    act(() => result.current.startEdit(mine()))
    act(() => result.current.saveEdit('<p>cũ</p>'))
    expect(result.current.editing).toBeNull()
    expect(api.editMessage).not.toHaveBeenCalled()
  })

  it('asks to delete instead when the text was emptied', () => {
    const { result } = setup()
    act(() => result.current.startEdit(mine()))
    act(() => result.current.saveEdit('', true))
    expect(api.editMessage).not.toHaveBeenCalled()
    expect(result.current.deleting?.id).toBe('m1')
  })

  it('says why and reopens the editor with the typed text when the server refuses', async () => {
    api.editMessage.mockRejectedValue(Object.assign(new Error('x'), { status: 403 }))
    const { result } = setup()
    act(() => result.current.startEdit(mine()))
    act(() => result.current.saveEdit('<p>mới</p>'))
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    expect(result.current.editing).toMatchObject({ message: { id: 'm1' }, draft: '<p>mới</p>' })
    expect(list()[1]!.content).toBe('<p>cũ</p>')
  })
})

describe('useMessageActions: deleting', () => {
  it('asks first, deletes on confirm and closes the question', async () => {
    api.deleteMessage.mockResolvedValue({ status: 'ok' })
    const { result } = setup()
    act(() => result.current.askDelete(mine()))
    expect(result.current.deleting?.id).toBe('m1')
    expect(api.deleteMessage).not.toHaveBeenCalled()
    act(() => result.current.confirmDelete())
    await waitFor(() => expect(result.current.deleting).toBeNull())
    expect(api.deleteMessage).toHaveBeenCalledWith('m1')
    expect(list().map((m) => m.id)).toEqual(['m0'])
  })

  it('keeps everything when cancelled', () => {
    const { result } = setup()
    act(() => result.current.askDelete(mine()))
    act(() => result.current.cancelDelete())
    expect(result.current.deleting).toBeNull()
    expect(api.deleteMessage).not.toHaveBeenCalled()
  })

  it('does not even ask for a message the reader may not delete', () => {
    const { result } = setup()
    act(() => result.current.askDelete(mine({ sender_id: 'lan' })))
    expect(result.current.deleting).toBeNull()
  })

  it('says why, and keeps the question open, when the server refuses', async () => {
    api.deleteMessage.mockRejectedValue(Object.assign(new Error('x'), { status: 403 }))
    const { result } = setup()
    act(() => result.current.askDelete(mine()))
    act(() => result.current.confirmDelete())
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    expect(list()).toHaveLength(2)
  })

  it('closes the editor too when the message being edited is deleted', async () => {
    api.deleteMessage.mockResolvedValue({ status: 'ok' })
    const { result } = setup()
    act(() => result.current.startEdit(mine()))
    act(() => result.current.saveEdit('', true))
    act(() => result.current.confirmDelete())
    await waitFor(() => expect(result.current.deleting).toBeNull())
    expect(result.current.editing).toBeNull()
  })
})

describe('useMessageActions: mark unread', () => {
  it('marks it and says so', async () => {
    api.markMessageUnread.mockResolvedValue({ status: 'ok' })
    const { result } = setup()
    act(() => result.current.markUnread(mine({ sender_id: 'lan' })))
    await waitFor(() => expect(toast).toHaveBeenCalledWith('Đã đánh dấu chưa đọc'))
  })

  it('says why when it fails', async () => {
    api.markMessageUnread.mockRejectedValue(new Error('offline'))
    const { result } = setup()
    act(() => result.current.markUnread(mine({ sender_id: 'lan' })))
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
  })
})

describe('useMessageActions: marking unread', () => {
  it('tells the caller which message was marked unread once the server has accepted it', async () => {
    api.markMessageUnread.mockResolvedValue({ status: 'ok' })
    const onMarkedUnread = vi.fn()
    const { result } = renderHook(() => useMessageActions('c1', 'me', { onMarkedUnread }), { wrapper })
    act(() => result.current.markUnread(mine({ id: 'm9', sender_id: 'lan' })))
    await waitFor(() => expect(onMarkedUnread).toHaveBeenCalledWith(expect.objectContaining({ id: 'm9' })))
  })

  it('says nothing when the server refuses', async () => {
    api.markMessageUnread.mockRejectedValue(new Error('403'))
    const onMarkedUnread = vi.fn()
    const { result } = renderHook(() => useMessageActions('c1', 'me', { onMarkedUnread }), { wrapper })
    act(() => result.current.markUnread(mine({ id: 'm9', sender_id: 'lan' })))
    await waitFor(() => expect(api.markMessageUnread).toHaveBeenCalled())
    await waitFor(() => expect(toast.error).toHaveBeenCalled())
    expect(onMarkedUnread).not.toHaveBeenCalled()
  })
})
