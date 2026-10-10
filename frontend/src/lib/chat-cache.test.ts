import { beforeEach, describe, expect, it } from 'vitest'
import { keys } from '../hooks/keys'
import type { ReactionEvent } from '../generated/proto/messaging/ws'
import type { Message } from '../api/messaging'
import { applyReaction, convertChatMsgToMessage, previewText, timestampToIso } from './chat-cache'
import { queryClient } from './query-client'

const msg = (over: Partial<Message> = {}): Message =>
  ({ id: 'm1', channel_id: 'c1', sender_id: 'u1', content: 'x', reactions: [], ...over }) as Message

const seed = (messages: Message[]) => queryClient.setQueryData(keys.messaging.messages('c1'), { messages, has_more: false })
const read = () => queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.messages('c1'))!.messages

const event = (action: string, userId: string, emoji = '👍') =>
  ({ channelId: 'c1', messageId: 'm1', emoji, userId, action }) as ReactionEvent

beforeEach(() => queryClient.clear())

describe('applyReaction', () => {
  it('starts a new group on the first add', () => {
    seed([msg()])
    applyReaction(event('add', 'u2'))
    expect(read()[0]!.reactions).toEqual([{ emoji: '👍', count: 1, user_ids: ['u2'] }])
  })

  it('counts a second person once and ignores a repeated add', () => {
    seed([msg({ reactions: [{ emoji: '👍', count: 1, user_ids: ['u2'] }] })])
    applyReaction(event('add', 'u3'))
    applyReaction(event('add', 'u3'))
    expect(read()[0]!.reactions).toEqual([{ emoji: '👍', count: 2, user_ids: ['u2', 'u3'] }])
  })

  it('drops the group when its last reactor leaves, and ignores a remove for an absent emoji', () => {
    seed([msg({ reactions: [{ emoji: '👍', count: 1, user_ids: ['u2'] }] })])
    applyReaction(event('remove', 'u2', '🎉'))
    expect(read()[0]!.reactions).toHaveLength(1)
    applyReaction(event('remove', 'u2'))
    expect(read()[0]!.reactions).toEqual([])
  })

  it('leaves other messages and an unloaded channel alone', () => {
    applyReaction(event('add', 'u2'))
    expect(queryClient.getQueryData(keys.messaging.messages('c1'))).toBeUndefined()
    seed([msg({ id: 'other' })])
    applyReaction(event('add', 'u2'))
    expect(read()[0]!.reactions).toEqual([])
  })
})

describe('chat message helpers', () => {
  it('previews as plain folded text, capped at 120 characters', () => {
    expect(previewText('<p>Xin&nbsp;chào</p>\n<b>bạn</b>')).toBe('Xin chào bạn')
    expect(previewText('a'.repeat(300))).toHaveLength(120)
  })

  it('converts a proto timestamp, falling back to now when absent', () => {
    expect(timestampToIso({ seconds: '1700000000', nanos: 500_000_000 } as never)).toBe('2023-11-14T22:13:20.500Z')
    expect(Math.abs(Date.parse(timestampToIso(undefined)) - Date.now())).toBeLessThan(2000)
  })

  it('maps a proto chat message to the API shape with defaults', () => {
    const out = convertChatMsgToMessage({ id: 'm', channelId: 'c', senderId: 's', senderName: 'An', content: 'hi' } as never)
    expect(out).toMatchObject({ id: 'm', channel_id: 'c', sender_id: 's', content_format: 'markdown', reply_count: 0, is_pinned: false, reactions: [] })
  })
})
