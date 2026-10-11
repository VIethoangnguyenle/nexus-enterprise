import { describe, expect, it } from 'vitest'
import type { Message } from '../api/messaging'
import {
  cardKindOf, deleteConsequence, isDeletedPlaceholder, lastEditableOf, messageAbilities, messageText, quoteSummary, toEditorHtml,
} from './message-model'

const msg = (over: Partial<Message> = {}): Message =>
  ({ id: 'm1', channel_id: 'c1', sender_id: 'me', content: '<p>xin chào</p>', content_format: 'html', ...over }) as Message

describe('messageAbilities', () => {
  it('lets the sender edit and delete, and nobody mark their own message unread', () => {
    expect(messageAbilities(msg(), { me: 'me', canManage: false })).toEqual({ edit: true, remove: true, markUnread: false })
  })

  it("gives another person's message mark-unread, and delete only to a manager", () => {
    const other = msg({ sender_id: 'lan' })
    expect(messageAbilities(other, { me: 'me', canManage: false })).toEqual({ edit: false, remove: false, markUnread: true })
    expect(messageAbilities(other, { me: 'me', canManage: true })).toEqual({ edit: false, remove: true, markUnread: true })
  })

  it('never edits a card, but a card can still be deleted', () => {
    const poll = msg({ linked_entity_type: 'poll', linked_entity_id: 'p1' })
    expect(messageAbilities(poll, { me: 'me', canManage: false })).toEqual({ edit: false, remove: true, markUnread: false })
  })

  it('offers nothing on a deleted message, a system line or a message still being sent', () => {
    const none = { edit: false, remove: false, markUnread: false }
    expect(messageAbilities(msg({ deleted: true }), { me: 'me', canManage: true })).toEqual(none)
    expect(messageAbilities(msg({ message_type: 'system', sender_id: 'lan' }), { me: 'me', canManage: true })).toEqual(none)
    expect(messageAbilities(msg({ id: 'temp-1' }), { me: 'me', canManage: true })).toEqual(none)
    expect(messageAbilities(msg({ _optimistic: true } as Partial<Message>), { me: 'me', canManage: true })).toEqual(none)
  })

})

describe('lastEditableOf', () => {
  it('is the newest text message of mine, skipping cards, deleted ones and other people', () => {
    const list = [
      msg({ id: 'a' }),
      msg({ id: 'b' }),
      msg({ id: 'c', sender_id: 'lan' }),
      msg({ id: 'd', linked_entity_type: 'poll' }),
      msg({ id: 'e', deleted: true }),
    ]
    expect(lastEditableOf(list, 'me')?.id).toBe('b')
  })

  it('is undefined when there is none', () => {
    expect(lastEditableOf([msg({ sender_id: 'lan' })], 'me')).toBeUndefined()
    expect(lastEditableOf([], 'me')).toBeUndefined()
  })
})

describe('isDeletedPlaceholder', () => {
  it('is a deleted message that still stands in the stream for its thread', () => {
    expect(isDeletedPlaceholder(msg({ deleted: true, reply_count: 2 }))).toBe(true)
    expect(isDeletedPlaceholder(msg({ deleted: true }))).toBe(true)
    expect(isDeletedPlaceholder(msg())).toBe(false)
  })
})

describe('deleteConsequence', () => {
  it('says what deleting your own message does', () => {
    expect(deleteConsequence({ mine: true, senderName: 'Bạn', hasThread: false })).toEqual([
      'Tin sẽ bị xoá với mọi người trong cuộc trò chuyện và không khôi phục được.',
    ])
  })

  it("names the sender and the group log for someone else's message", () => {
    expect(deleteConsequence({ mine: false, senderName: 'Nguyễn Thu Lan', hasThread: false })).toEqual([
      'Tin của Nguyễn Thu Lan sẽ bị xoá với mọi người trong cuộc trò chuyện và không khôi phục được.',
      'Nhật ký nhóm ghi lại bạn đã xoá tin này.',
    ])
  })

  it('adds that the thread stays when there is one', () => {
    expect(deleteConsequence({ mine: true, senderName: 'Bạn', hasThread: true }).slice(-1)[0]).toBe('Thread của tin vẫn giữ lại.')
  })
})

describe('quoteSummary', () => {
  it('shows the name and the opening of the quoted text', () => {
    expect(quoteSummary({ id: 'q', sender_id: 'lan', sender_name: 'Lan', text: 'Dạ vâng' }, (id, n) => n ?? id))
      .toEqual({ name: 'Lan', text: 'Dạ vâng', deleted: false })
  })

  it('says the message was deleted instead of showing text', () => {
    expect(quoteSummary({ id: 'q', sender_id: 'lan', sender_name: 'Lan', text: 'bí mật', deleted: true }, (id, n) => n ?? id))
      .toEqual({ name: 'Lan', text: 'Tin nhắn đã bị xoá', deleted: true })
  })
})

describe('toEditorHtml', () => {
  it('hands rich text to the editor as it is', () => {
    expect(toEditorHtml(msg({ content: '<p>a <strong>b</strong></p>' }))).toBe('<p>a <strong>b</strong></p>')
  })

  it('turns a plain or markdown body into escaped paragraphs, one per line', () => {
    expect(toEditorHtml(msg({ content: 'a < b\n& c', content_format: 'plain' }))).toBe('<p>a &lt; b</p><p>&amp; c</p>')
    expect(toEditorHtml(msg({ content: 'xin chào', content_format: 'markdown' }))).toBe('<p>xin chào</p>')
  })
})

describe('messageText', () => {
  it('keeps line breaks and drops markup', () => {
    expect(messageText('<p>một</p><p>hai<br>ba</p>')).toBe('một\nhai\nba')
  })

  it('reads a plain body as it is', () => {
    expect(messageText('a < b')).toBe('a < b')
  })
})

describe('cardKindOf', () => {
  it('names the card a message links to, and nothing for an unknown kind or a link without an id', () => {
    expect(cardKindOf({ linked_entity_type: 'poll', linked_entity_id: 'p1' })).toBe('poll')
    expect(cardKindOf({ linked_entity_type: 'approval', linked_entity_id: 'a1' })).toBe('approval')
    expect(cardKindOf({ linked_entity_type: 'drive_file', linked_entity_id: 'f1' })).toBeUndefined()
    expect(cardKindOf({ linked_entity_type: 'poll', linked_entity_id: '' })).toBeUndefined()
  })
})
