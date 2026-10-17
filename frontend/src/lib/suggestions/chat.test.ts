import { describe, expect, it } from 'vitest'
import type { Conversation } from '../conversations'
import { recentConversations } from './chat'

const conv = (id: string, lastActivity: number, over: Partial<Conversation> = {}): Conversation => ({
  id, kind: 'space', title: `Nhóm ${id}`, hueKey: id, unread: 0, memberCount: 3, lastActivity, ...over,
})

describe('recentConversations', () => {
  it('lists the conversations something was said in lately, newest first', () => {
    const list = [conv('a', 100), conv('b', 300), conv('c', 200)]
    expect(recentConversations(list).map((c) => c.id)).toEqual(['b', 'c', 'a'])
  })

  it('stops at the limit', () => {
    const list = Array.from({ length: 9 }, (_, i) => conv(`n${i}`, i + 1))
    expect(recentConversations(list)).toHaveLength(5)
    expect(recentConversations(list, 2).map((c) => c.id)).toEqual(['n8', 'n7'])
  })

  it('leaves out a conversation nothing is known to have been said in', () => {
    expect(recentConversations([conv('quiet', 0), conv('busy', 5)]).map((c) => c.id)).toEqual(['busy'])
  })

  it('suggests only conversations in the list it is given: one the caller can no longer read is never suggested', () => {
    // The list is what the server returned for the caller; a conversation dropped from it is gone from the suggestions.
    const readable = [conv('still-member', 10)]
    expect(recentConversations(readable).map((c) => c.id)).toEqual(['still-member'])
    expect(recentConversations([]).map((c) => c.id)).toEqual([])
  })
})
