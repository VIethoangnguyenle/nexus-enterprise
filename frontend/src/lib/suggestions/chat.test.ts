import { describe, expect, it } from 'vitest'
import type { Conversation } from '../conversations'
import type { Person } from '../people'
import { conversationsMatching, peopleMatching, recentConversations } from './chat'

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

const person = (userId: string, name: string, username = userId): Person => ({
  userId, nodeId: `n-${userId}`, username, name, role: '', avatarUrl: '',
})

describe('peopleMatching', () => {
  const people = [
    person('me', 'Lê Thị Hoa', 'hoa'),
    person('a', 'Trần Lan Anh', 'lananh'),
    person('b', 'Nguyễn Thu Lan', 'lan'),
    person('c', 'Phạm Hải Yến', 'yen'),
    person('d', 'Lê Quang Vinh', 'vinh'),
    person('e', 'Lâm Văn Lợi', 'loi'),
  ]

  it('matches names without case or accents, every word required', () => {
    expect(peopleMatching(people, 'lan', 'me').map((p) => p.userId)).toEqual(['a', 'b'])
    expect(peopleMatching(people, 'thu LAN', 'me').map((p) => p.userId)).toEqual(['b'])
  })

  it('matches the username too', () => {
    expect(peopleMatching(people, 'lananh', 'me').map((p) => p.userId)).toEqual(['a'])
  })

  it('never lists the person who is asking', () => {
    expect(peopleMatching(people, 'hoa', 'me')).toEqual([])
  })

  it('stops at three', () => {
    expect(peopleMatching(people, 'l', 'me')).toHaveLength(3)
  })
})

describe('conversationsMatching', () => {
  const list = [
    conv('s1', 1, { title: 'Đối soát giao dịch' }),
    conv('s2', 1, { title: 'Kế toán tổng hợp' }),
    conv('d1', 1, { kind: 'dm', title: 'Trần Lan Anh', partner: person('a', 'Trần Lan Anh') }),
    conv('d2', 1, { kind: 'dm', title: 'Lan Chi', partner: person('z', 'Lan Chi') }),
  ]

  it('matches groups and direct messages by name', () => {
    expect(conversationsMatching(list, 'doi soat').map((c) => c.id)).toEqual(['s1'])
    expect(conversationsMatching(list, 'lan').map((c) => c.id)).toEqual(['d1', 'd2'])
  })

  it('leaves out a direct message whose person is already listed above', () => {
    expect(conversationsMatching(list, 'lan', new Set(['a'])).map((c) => c.id)).toEqual(['d2'])
  })

  it('has nothing for words that match no name', () => {
    expect(conversationsMatching(list, 'không có')).toEqual([])
  })
})
