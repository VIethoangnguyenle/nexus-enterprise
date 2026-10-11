import { beforeEach, describe, expect, it } from 'vitest'
import { keys } from '../hooks/keys'
import type { ReactionEvent } from '../generated/proto/messaging/ws'
import type { Message } from '../api/messaging'
import {
  applyMessageDeleted, applyMessageEdited, applyReaction, convertChatMsgToMessage, previewText, timestampToIso,
} from './chat-cache'
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

describe('convertChatMsgToMessage: a quote', () => {
  it('carries what the quote shows, and nothing when there is none', () => {
    const quoted = { id: 'q', senderId: 'u2', senderName: 'Lan', text: 'Dạ vâng', deleted: false }
    expect(convertChatMsgToMessage({ id: 'm', channelId: 'c', senderId: 's', content: 'hi', quoted } as never).quoted)
      .toEqual({ id: 'q', sender_id: 'u2', sender_name: 'Lan', text: 'Dạ vâng', deleted: false })
    expect(convertChatMsgToMessage({ id: 'm', channelId: 'c', senderId: 's', content: 'hi' } as never).quoted).toBeUndefined()
  })
})

const readThread = (root: string) => queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.thread(root))!.messages
const EDITED_AT = { seconds: '1760000000', nanos: 0 }

describe('applyMessageEdited', () => {
  const frame = (over: Record<string, unknown> = {}) =>
    ({ message: { id: 'm1', channelId: 'c1', senderId: 'u1', content: '<p>mới</p>', contentFormat: 'html', parentMessageId: '', ...over }, editedAt: EDITED_AT }) as never

  it('changes the text in the stream and marks it edited, keeping reactions and thread state', () => {
    seed([msg({ content: 'cũ', reactions: [{ emoji: '👍', count: 1, user_ids: ['u2'] }], reply_count: 3, following: true })])
    applyMessageEdited(frame())
    expect(read()[0]).toMatchObject({
      content: '<p>mới</p>', content_format: 'html', reply_count: 3, following: true,
      reactions: [{ emoji: '👍', count: 1, user_ids: ['u2'] }], edited_at: EDITED_AT,
    })
  })

  it("changes a reply in its thread, not in the channel's stream", () => {
    queryClient.setQueryData(keys.messaging.thread('root'), {
      messages: [msg({ id: 'root' }), msg({ id: 'r1', parent_message_id: 'root', content: 'cũ' })],
    })
    seed([msg({ id: 'root' })])
    applyMessageEdited(frame({ id: 'r1', parentMessageId: 'root' }))
    expect(readThread('root').map((m) => m.content)).toEqual(['x', '<p>mới</p>'])
    expect(read()).toHaveLength(1)
    expect(read()[0]!.edited_at).toBeUndefined()
  })

  it("changes the root inside its own thread's cache too", () => {
    queryClient.setQueryData(keys.messaging.thread('m1'), { messages: [msg({ content: 'cũ' })] })
    applyMessageEdited(frame())
    expect(readThread('m1')[0]!.content).toBe('<p>mới</p>')
  })

  it('refreshes the thread lists, which show the first line of a root', () => {
    queryClient.setQueryData(keys.messaging.threadsList('c1', 'all'), { pages: [], pageParams: [] })
    applyMessageEdited(frame())
    expect(queryClient.getQueryState(keys.messaging.threadsList('c1', 'all'))!.isInvalidated).toBe(true)
  })

  it('does nothing for a message that is not loaded', () => {
    seed([msg({ id: 'other' })])
    applyMessageEdited(frame())
    expect(read()).toEqual([msg({ id: 'other' })])
  })

  it('ignores an edit that arrives for a message already deleted here', () => {
    seed([msg({ deleted: true, content: '' })])
    applyMessageEdited(frame())
    expect(read()[0]).toMatchObject({ deleted: true, content: '' })
  })
})

describe('applyMessageDeleted', () => {
  const del = (over: Record<string, unknown> = {}) =>
    ({ messageId: 'm1', channelId: 'c1', parentMessageId: '', keepPlaceholder: false, replyCount: 0, ...over }) as never

  it('removes a message that has no thread from the stream', () => {
    seed([msg({ id: 'm0' }), msg()])
    applyMessageDeleted(del())
    expect(read().map((m) => m.id)).toEqual(['m0'])
  })

  it('keeps a root with replies as a placeholder with no body, files, reactions or pin', () => {
    seed([msg({ content: 'bí mật', reply_count: 2, is_pinned: true, reactions: [{ emoji: '👍', count: 1, user_ids: ['u2'] }] })])
    applyMessageDeleted(del({ keepPlaceholder: true }))
    expect(read()).toHaveLength(1)
    expect(read()[0]).toMatchObject({ deleted: true, content: '', reactions: [], is_pinned: false, reply_count: 2 })
  })

  it("takes a reply out of its thread and sets the root's count to the server's", () => {
    queryClient.setQueryData(keys.messaging.thread('root'), {
      messages: [msg({ id: 'root', reply_count: 2 }), msg({ id: 'r1', parent_message_id: 'root' }), msg({ id: 'r2', parent_message_id: 'root' })],
    })
    seed([msg({ id: 'root', reply_count: 2, last_reply_at: 'x', reply_participant_ids: ['u1'] })])
    applyMessageDeleted(del({ messageId: 'r1', parentMessageId: 'root', replyCount: 1 }))
    expect(readThread('root').map((m) => m.id)).toEqual(['root', 'r2'])
    expect(readThread('root')[0]!.reply_count).toBe(1)
    expect(read()[0]!.reply_count).toBe(1)
  })

  it('clears the thread summary when the last reply goes', () => {
    seed([msg({ id: 'root', reply_count: 1, last_reply_at: 'x', reply_participant_ids: ['u1'] })])
    applyMessageDeleted(del({ messageId: 'r1', parentMessageId: 'root', replyCount: 0 }))
    expect(read()[0]).toMatchObject({ reply_count: 0, reply_participant_ids: [] })
    expect(read()[0]!.last_reply_at).toBeUndefined()
  })

  it('turns a quote of the deleted message into "deleted" everywhere it is cached', () => {
    const quoting = msg({ id: 'm2', quoted: { id: 'm1', sender_id: 'u1', sender_name: 'An', text: 'bí mật' } })
    seed([quoting, msg()])
    queryClient.setQueryData(keys.messaging.thread('m2'), { messages: [quoting] })
    applyMessageDeleted(del())
    expect(read()[0]!.quoted).toEqual({ id: 'm1', sender_id: 'u1', sender_name: 'An', text: '', deleted: true })
    expect(readThread('m2')[0]!.quoted).toMatchObject({ deleted: true, text: '' })
  })

  it('is harmless twice (the request that deleted it and its own frame)', () => {
    seed([msg({ id: 'm0' }), msg()])
    applyMessageDeleted(del())
    applyMessageDeleted(del())
    expect(read().map((m) => m.id)).toEqual(['m0'])
  })

  it("refreshes the channel's thread lists and unread counts", () => {
    queryClient.setQueryData(keys.messaging.threadsList('c1', 'all'), { pages: [], pageParams: [] })
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [] })
    applyMessageDeleted(del())
    expect(queryClient.getQueryState(keys.messaging.threadsList('c1', 'all'))!.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(keys.messaging.unreadCounts())!.isInvalidated).toBe(true)
  })
})
