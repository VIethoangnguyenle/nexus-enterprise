import { describe, expect, it } from 'vitest'
import type { Message } from '../api/messaging'
import { trimLeftHistory } from './stream-model'

const msgs = (n: number): Message[] =>
  Array.from({ length: n }, (_, i) => ({ id: `m${n - i}`, channel_id: 'c', sender_id: 'u', content: '', seq: n - i }))

describe('trimLeftHistory', () => {
  it('keeps the newest messages of a long history and says more remain', () => {
    const out = trimLeftHistory({ messages: msgs(250), has_more: false }, 100)!
    expect(out.messages).toHaveLength(100)
    expect(out.messages[0]?.id).toBe('m250')
    expect(out.messages[99]?.id).toBe('m151')
    expect(out.has_more).toBe(true)
  })

  it('leaves a short history and an empty cache as they are', () => {
    const short = { messages: msgs(30), has_more: false }
    expect(trimLeftHistory(short, 100)).toBe(short)
    expect(trimLeftHistory(undefined)).toBeUndefined()
  })
})
