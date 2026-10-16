import { describe, expect, it } from 'vitest'
import type { Channel } from '../api/messaging'
import { buildConversations } from './conversations'
import { EMPTY_DIRECTORY } from './people'

const PIC = '/api/channels/avatars/0123456789abcdef0123456789abcdef'
const ch = (over: Partial<Channel>): Channel => ({ id: 'c1', name: 'Nhóm', channel_type: 'workspace', workspace_id: 'w', ...over })
const rows = (spaces: Channel[], dms: Channel[] = []) =>
  buildConversations({ spaces, dms, unread: {}, lastMessages: {}, me: { id: 'u', username: 'hoa' }, dir: EMPTY_DIRECTORY })

describe('conversations and a group picture', () => {
  it('carry the picture of a group, and nothing when it has none', () => {
    const [a, b] = rows([ch({ id: 'a', avatar_url: PIC }), ch({ id: 'b', avatar_url: '' })])
    expect(a!.avatarUrl).toBe(PIC)
    expect(b!.avatarUrl).toBeUndefined()
  })

  it('never carry one for a direct message', () => {
    const [dm] = rows([], [ch({ id: 'd', name: 'hoa, yen', channel_type: 'dm', avatar_url: PIC })])
    expect(dm!.avatarUrl).toBeUndefined()
  })
})
