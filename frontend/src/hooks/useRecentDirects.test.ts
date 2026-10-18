import { describe, expect, it } from 'vitest'
import type { Contact } from './useContacts'
import type { Conversation } from '../lib/conversations'
import { FREQUENT_CONTACTS, frequentContacts } from './useRecentDirects'

const person = (n: number): Contact => ({
  user_id: `u${n}`, ngac_node_id: `n${n}`, username: `p${n}`, display_name: `Người ${n}`, email: '', title: '', department: '',
  department_id: '', roles: [], location: '', avatar_url: '', is_online: false,
})
const dm = (n: number, at: number): Conversation => ({
  id: `dm${n}`, kind: 'dm', title: `Người ${n}`, hueKey: `u${n}`, unread: 0, memberCount: 2, lastActivity: at,
  partner: { userId: `u${n}`, nodeId: `n${n}`, name: `Người ${n}`, username: `p${n}`, avatarUrl: '' },
} as Conversation)

describe('frequentContacts (Thường liên lạc)', () => {
  it('is the people of recent direct messages, latest first, at most eight', () => {
    const directory = Array.from({ length: 12 }, (_, i) => person(i + 1))
    const directs = directory.map((_, i) => dm(i + 1, 1000 + i))
    const got = frequentContacts(directs, directory, 'me')
    expect(FREQUENT_CONTACTS).toBe(8)
    expect(got.map((c) => c.user_id)).toEqual(['u12', 'u11', 'u10', 'u9', 'u8', 'u7', 'u6', 'u5'])
  })

  it('leaves out a partner the directory does not show (hidden by the scope, or of another workspace) and the viewer', () => {
    const directory = [person(1), person(3)]
    const directs = [dm(1, 10), dm(2, 30), dm(3, 20), { ...dm(9, 40), partner: { ...dm(9, 40).partner!, userId: 'me' } }]
    expect(frequentContacts(directs, directory, 'me').map((c) => c.user_id)).toEqual(['u3', 'u1'])
  })

  it('is empty when nobody was messaged', () => {
    expect(frequentContacts([], [person(1)], 'me')).toEqual([])
  })
})
