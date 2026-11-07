import { describe, expect, it } from 'vitest'
import type { ChannelUnread } from '../api/messaging'
import type { ConversationActivityEvent } from '../generated/proto/messaging/ws'
import { unreadAfterActivity, type UnreadAnswer } from './unread-activity'

const row = (p: Partial<ChannelUnread> = {}): ChannelUnread => ({
  channel_id: 'ch', unread_count: 0, last_read_message_id: '', last_seq: 4, last_read_seq: 4, ...p,
})
const answer = (p: Partial<ChannelUnread> = {}): UnreadAnswer => ({ channels: [row(p), row({ channel_id: 'other' })] })
const sent = (p: Partial<ConversationActivityEvent> = {}): ConversationActivityEvent => ({
  channelId: 'ch', kind: 'sent', seq: '5', rootMessageId: '', senderId: 'u-a', mentions: [], lastSeq: '5', ...p,
})
const reader = { me: 'u-me', viewing: null }
const ch = (o: { next: UnreadAnswer | undefined }) => o.next!.channels.find((c) => c.channel_id === 'ch')!

describe('unreadAfterActivity', () => {
  it('counts a message from someone else in a conversation not on screen', () => {
    const out = unreadAfterActivity(answer(), sent(), reader)
    expect(ch(out)).toMatchObject({ unread_count: 1, last_seq: 5 })
    expect(out.refetch).toBe(false)
  })

  it('leaves the other conversations as they are', () => {
    const out = unreadAfterActivity(answer(), sent(), reader)
    expect(out.next!.channels[1]).toEqual(row({ channel_id: 'other' }))
  })

  it('counts a mention of the reader, at every level', () => {
    const out = unreadAfterActivity(answer({ notification_level: 'off' }), sent({ mentions: ['u-me'] }), reader)
    expect(ch(out)).toMatchObject({ unread_count: 0, mention_count: 1 })
  })

  it('counts nothing for the reader’s own message or the conversation on screen', () => {
    expect(ch(unreadAfterActivity(answer(), sent({ senderId: 'u-me' }), reader))).toMatchObject({ unread_count: 0, last_seq: 5 })
    expect(ch(unreadAfterActivity(answer(), sent(), { me: 'u-me', viewing: 'ch' }))).toMatchObject({ unread_count: 0, last_seq: 5 })
  })

  it('counts a repeated event once', () => {
    const once = unreadAfterActivity(answer(), sent(), reader).next
    const twice = unreadAfterActivity(once, sent(), reader)
    expect(ch(twice).unread_count).toBe(1)
    expect(twice.refetch).toBe(false)
  })

  it('asks again when a number was skipped', () => {
    const out = unreadAfterActivity(answer(), sent({ seq: '7', lastSeq: '7' }), reader)
    expect(out.refetch).toBe(true)
  })

  it('asks again when the counts held carry no number but the message is not the first', () => {
    expect(unreadAfterActivity(answer({ last_seq: undefined }), sent({ seq: '5' }), reader).refetch).toBe(true)
    expect(unreadAfterActivity(answer({ last_seq: undefined }), sent({ seq: '1' }), reader).refetch).toBe(false)
  })

  it('asks again for a conversation the list does not know', () => {
    expect(unreadAfterActivity(answer(), sent({ channelId: 'new' }), reader).refetch).toBe(true)
  })

  it('takes off a deleted message only when it was unread', () => {
    const unread = answer({ unread_count: 2, last_seq: 6, last_read_seq: 4 })
    expect(ch(unreadAfterActivity(unread, sent({ kind: 'deleted', seq: '6' }), reader)).unread_count).toBe(1)
    expect(ch(unreadAfterActivity(unread, sent({ kind: 'deleted', seq: '3' }), reader)).unread_count).toBe(2)
  })

  it('asks again after a deletion only when a mention is shown', () => {
    expect(unreadAfterActivity(answer({ mention_count: 1 }), sent({ kind: 'deleted', seq: '5' }), reader).refetch).toBe(true)
    expect(unreadAfterActivity(answer(), sent({ kind: 'deleted', seq: '5' }), reader).refetch).toBe(false)
  })

  it('leaves replies to the thread events, unless the conversation hears every reply', () => {
    const reply = sent({ seq: '0', rootMessageId: 'root' })
    const main = unreadAfterActivity(answer(), reply, reader)
    expect(main.next).toEqual(answer())
    expect(main.refetch).toBe(false)
    expect(unreadAfterActivity(answer({ notification_level: 'all' }), reply, reader).refetch).toBe(true)
  })

  it('does nothing before the counts were ever loaded', () => {
    expect(unreadAfterActivity(undefined, sent(), reader)).toEqual({ next: undefined, refetch: false })
  })
})
