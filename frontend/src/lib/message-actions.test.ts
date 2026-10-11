import { describe, expect, it } from 'vitest'
import type { Message } from '../api/messaging'
import { canQuote, canStar } from './message-actions'

const msg = (over: Partial<Message> & { _optimistic?: boolean } = {}) =>
  ({ id: 'm1', channel_id: 'c1', sender_id: 'u1', content: '<p>x</p>', ...over }) as Message

describe('canStar / canQuote', () => {
  it('allows a stored message of a person', () => {
    expect(canStar(msg())).toBe(true)
    expect(canQuote(msg())).toBe(true)
  })

  it('allows a card, even though polls and tasks are posted as system messages', () => {
    expect(canQuote(msg({ message_type: 'system', linked_entity_type: 'poll', linked_entity_id: 'p' }))).toBe(true)
    expect(canStar(msg({ message_type: 'system', linked_entity_type: 'task', linked_entity_id: 't' }))).toBe(true)
  })

  it('refuses a message still being sent, a deleted one and a system line', () => {
    for (const m of [
      msg({ id: 'temp-123' }),
      msg({ _optimistic: true } as Partial<Message>),
      msg({ deleted: true }),
      msg({ message_type: 'system', linked_entity_type: 'message_deleted' }),
    ]) {
      expect(canStar(m)).toBe(false)
      expect(canQuote(m)).toBe(false)
    }
  })
})
