import { beforeEach, describe, expect, it } from 'vitest'
import { keys } from '../hooks/keys'
import type { ThreadReplyEvent, ThreadStateEvent } from '../generated/proto/messaging/ws'
import type { Message } from '../api/messaging'
import { applyThreadReplySummary, applyThreadState, applyThreadStateAnswer, resetThreadStateClock } from './thread-cache'
import { queryClient } from './query-client'

const msg = (over: Partial<Message> = {}): Message =>
  ({ id: 'root', channel_id: 'c1', sender_id: 'u1', content: 'x', reply_count: 1, ...over }) as Message

const seedChannel = (messages: Message[]) =>
  queryClient.setQueryData(keys.messaging.messages('c1'), { messages, has_more: false })
const channel = () => queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.messages('c1'))!.messages
const seedThread = (messages: Message[]) => queryClient.setQueryData(keys.messaging.thread('root'), { messages })
const thread = () => queryClient.getQueryData<{ messages: Message[] }>(keys.messaging.thread('root'))!.messages

const reply = (over: Partial<ThreadReplyEvent> = {}): ThreadReplyEvent =>
  ({
    parentMessageId: 'root',
    replyCount: 4,
    lastReplyAt: { seconds: '1760000000', nanos: 0 },
    replyParticipantIds: ['u3', 'u2'],
    message: { id: 'r4', channelId: 'c1', parentMessageId: 'root' },
    ...over,
  }) as ThreadReplyEvent

const state = (over: Partial<ThreadStateEvent> = {}): ThreadStateEvent =>
  ({ rootMessageId: 'root', channelId: 'c1', following: true, unreadCount: 3, ...over }) as ThreadStateEvent

beforeEach(() => {
  queryClient.clear()
  resetThreadStateClock()
})

describe('late frames', () => {
  it('a summary with a lower count than the one shown is ignored', () => {
    seedChannel([msg({ reply_count: 6 })])
    applyThreadReplySummary(reply({ replyCount: 4 }))
    expect(channel()[0]!.reply_count).toBe(6)
  })

  it('a state read before the one already applied is ignored', () => {
    seedChannel([msg()])
    applyThreadState(state({ unreadCount: 0, asOf: { seconds: '200', nanos: 0 } }))
    applyThreadState(state({ unreadCount: 5, asOf: { seconds: '100', nanos: 0 } }))
    expect(channel()[0]!.thread_unread_count).toBe(0)
  })

  it('a reply announced without its summary leaves the count alone', () => {
    seedChannel([msg({ reply_count: 2 })])
    applyThreadReplySummary(reply({ replyCount: 0, replyParticipantIds: [] }))
    expect(channel()[0]!.reply_count).toBe(2)
  })
})

describe('applyThreadReplySummary', () => {
  it('takes the server summary for the root message, not a local guess', () => {
    seedChannel([msg(), msg({ id: 'other' })])
    applyThreadReplySummary(reply())
    const [root, other] = channel()
    expect(root!.reply_count).toBe(4)
    expect(root!.reply_participant_ids).toEqual(['u3', 'u2'])
    expect(root!.last_reply_at).toEqual({ seconds: '1760000000', nanos: 0 })
    expect(other!.reply_count).toBe(1)
  })

  it('updates the root inside an open thread as well', () => {
    seedThread([msg()])
    applyThreadReplySummary(reply())
    expect(thread()[0]!.reply_count).toBe(4)
  })

  it('marks the channel thread lists stale and leaves an unloaded channel alone', () => {
    queryClient.setQueryData(keys.messaging.threadsList('c1', 'all'), { messages: [] })
    applyThreadReplySummary(reply())
    expect(queryClient.getQueryState(keys.messaging.threadsList('c1', 'all'))!.isInvalidated).toBe(true)
    expect(queryClient.getQueryData(keys.messaging.messages('c1'))).toBeUndefined()
  })
})

describe('applyThreadState', () => {
  it("sets the caller's follow state and unread count on the root", () => {
    seedChannel([msg({ following: false, thread_unread_count: 0 })])
    seedThread([msg()])
    applyThreadState(state())
    expect(channel()[0]).toMatchObject({ following: true, thread_unread_count: 3 })
    expect(thread()[0]).toMatchObject({ following: true, thread_unread_count: 3 })
  })

  it('reading on another device brings the count back to zero', () => {
    seedChannel([msg({ following: true, thread_unread_count: 5 })])
    applyThreadState(state({ unreadCount: 0 }))
    expect(channel()[0]!.thread_unread_count).toBe(0)
  })

  it('marks the unread counts and the thread lists stale', () => {
    queryClient.setQueryData(keys.messaging.unreadCounts(), { channels: [] })
    queryClient.setQueryData(keys.messaging.threadsList('c1', 'unread'), { messages: [] })
    applyThreadState(state())
    expect(queryClient.getQueryState(keys.messaging.unreadCounts())!.isInvalidated).toBe(true)
    expect(queryClient.getQueryState(keys.messaging.threadsList('c1', 'unread'))!.isInvalidated).toBe(true)
  })
})

describe('applyThreadStateAnswer', () => {
  it('applies the caller\'s state from a follow answer to the root in the stream and in the open thread', () => {
    seedChannel([msg({ following: false })])
    seedThread([msg({ following: false })])
    applyThreadStateAnswer({ root_message_id: 'root', channel_id: 'c1', following: true, unread_count: 0, as_of: { seconds: '300', nanos: 0 } })
    expect(channel()[0]!.following).toBe(true)
    expect(thread()[0]!.following).toBe(true)
  })

  it('does not undo a newer state that arrived over the socket first', () => {
    seedChannel([msg()])
    applyThreadState(state({ following: false, unreadCount: 0, asOf: { seconds: '400', nanos: 0 } }))
    applyThreadStateAnswer({ root_message_id: 'root', channel_id: 'c1', following: true, unread_count: 2, as_of: { seconds: '300', nanos: 0 } })
    expect(channel()[0]!.following).toBe(false)
  })
})
