import { describe, expect, it } from 'vitest'
import type { Contact } from '../../hooks/useContacts'
import type { Conversation } from '../conversations'
import type { Person } from '../people'
import { recentlyMessaged } from './contacts'

const contact = (id: string, name: string): Contact => ({
  user_id: id, ngac_node_id: `n-${id}`, username: id, display_name: name, email: `${id}@novapay.vn`,
  title: '', department: '', location: '', avatar_url: '', is_online: false,
})

const dm = (partnerId: string, lastActivity: number): Conversation => ({
  id: `dm-${partnerId}`, kind: 'dm', title: partnerId, hueKey: partnerId, unread: 0, memberCount: 2, lastActivity,
  partner: { userId: partnerId } as Person,
})

const DIRECTORY = [contact('lan', 'Nguyễn Thu Lan'), contact('vinh', 'Lê Quang Vinh'), contact('yen', 'Phạm Hải Yến'), contact('me', 'Tôi')]

describe('recentlyMessaged', () => {
  it('lists the people of the directory one has direct messages with, latest message first', () => {
    const directs = [dm('lan', 100), dm('yen', 300), dm('vinh', 200)]
    expect(recentlyMessaged(directs, DIRECTORY, 'me').map((c) => c.display_name)).toEqual(['Phạm Hải Yến', 'Lê Quang Vinh', 'Nguyễn Thu Lan'])
  })

  it('never lists the person using the app', () => {
    expect(recentlyMessaged([dm('me', 500), dm('lan', 100)], DIRECTORY, 'me').map((c) => c.user_id)).toEqual(['lan'])
  })

  it('lists no one outside this workspace\'s directory, even with a direct message', () => {
    // Direct messages are not tied to a workspace; the Danh bạ of this workspace shows only its own people.
    expect(recentlyMessaged([dm('stranger', 900), dm('vinh', 1)], DIRECTORY, 'me').map((c) => c.user_id)).toEqual(['vinh'])
  })

  it('leaves out a direct message nothing is known to have been said in, or whose partner is unknown', () => {
    const unknown: Conversation = { ...dm('x', 50), partner: undefined }
    expect(recentlyMessaged([dm('lan', 0), unknown, dm('yen', 3)], DIRECTORY, 'me').map((c) => c.user_id)).toEqual(['yen'])
  })

  it('lists each person once and stops at the limit', () => {
    const directs = [dm('lan', 9), { ...dm('lan', 8), id: 'dm-lan-2' }, dm('vinh', 7), dm('yen', 6)]
    expect(recentlyMessaged(directs, DIRECTORY, 'me', 2).map((c) => c.user_id)).toEqual(['lan', 'vinh'])
  })
})
