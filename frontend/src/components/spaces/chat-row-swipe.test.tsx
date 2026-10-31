import { afterEach, describe, expect, it, vi } from 'vitest'
import { messagingApi } from '../../api/messaging'
import { useToastStore } from '../primitives/Toast'
import { setConversationRead } from './chat-row-swipe'

afterEach(() => {
  vi.restoreAllMocks()
  useToastStore.getState().clear()
})

const toasts = () => useToastStore.getState().toasts

describe('setConversationRead', () => {
  it('marks read up to the newest message and offers Hoàn tác, which marks it unread again', async () => {
    const read = vi.spyOn(messagingApi, 'markRead').mockResolvedValue({ status: 'ok' } as never)
    const unread = vi.spyOn(messagingApi, 'markMessageUnread').mockResolvedValue({ status: 'ok' })
    await setConversationRead({ channelId: 'c1', name: 'general', messageId: 'm9', read: true })
    expect(read).toHaveBeenCalledWith('c1', 'm9')
    const t = toasts().find((x) => x.action?.label === 'Hoàn tác')!
    expect(t.message).toBe('Đã đánh dấu đã đọc “general”')
    t.action!.onClick()
    await vi.waitFor(() => expect(unread).toHaveBeenCalledWith('m9'))
    // The undo is itself quiet: no second Hoàn tác to chain.
    expect(toasts().filter((x) => x.action?.label === 'Hoàn tác')).toHaveLength(1)
  })

  it('a failure says so and offers Thử lại, never a Hoàn tác for what did not happen', async () => {
    vi.spyOn(messagingApi, 'markMessageUnread').mockRejectedValue(new Error('offline'))
    await setConversationRead({ channelId: 'c1', name: 'general', messageId: 'm9', read: false })
    expect(toasts()).toHaveLength(1)
    expect(toasts()[0]!.tone).toBe('error')
    expect(toasts()[0]!.action?.label).toBe('Thử lại')
  })
})
